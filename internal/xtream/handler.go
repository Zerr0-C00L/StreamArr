package xtream

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/config"
	"github.com/Zerr0-C00L/StreamArr/internal/epg"
	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/gorilla/mux"
)

// EpisodeLookup stores the mapping from TMDB episode ID to playback info
type EpisodeLookup struct {
	SeriesID   string `json:"series_id"`
	Season     int    `json:"season"`
	Episode    int    `json:"episode"`
	IMDBID     string `json:"imdb_id"`
}

// episodeCacheFile is the path to the episode cache JSON file
const episodeCacheFile = "cache/episode_lookup.json"

// XtreamSettings interface for the settings we need
type XtreamSettings interface {
	HideUnavailable() bool
}

// settingsAdapter wraps a generic settings getter
type settingsAdapter struct {
	getter func() bool
}

func (s *settingsAdapter) HideUnavailable() bool {
	return s.getter()
}

type XtreamHandler struct {
	cfg             *config.Config
	db              *sql.DB
	tmdb            *services.TMDBClient
	rdClient        *services.RealDebridClient
	multiProvider   *providers.MultiProvider
	channelManager  *livetv.ChannelManager
	epgManager      *epg.Manager
	hideUnavailable func() bool
	baseURL         string
	episodeCache    map[string]EpisodeLookup
	episodeMu       sync.RWMutex
}

func NewXtreamHandler(cfg *config.Config, db *sql.DB, tmdb *services.TMDBClient, rdClient *services.RealDebridClient, channelManager *livetv.ChannelManager, epgManager *epg.Manager) *XtreamHandler {
	multiProvider := providers.NewMultiProvider(
		cfg.RealDebridAPIKey,
		cfg.StreamProviders,
		cfg.TorrentioProviders,
		cfg.CometIndexers,
		tmdb,
	)
	
	h := &XtreamHandler{
		cfg:            cfg,
		db:             db,
		tmdb:           tmdb,
		rdClient:       rdClient,
		multiProvider:  multiProvider,
		channelManager: channelManager,
		epgManager:     epgManager,
		baseURL:        fmt.Sprintf("http://%s:%d", cfg.Host, cfg.ServerPort),
		episodeCache:   make(map[string]EpisodeLookup),
	}
	
	// Load episode cache from file
	h.loadEpisodeCache()
	
	return h
}

// SetSettingsManager sets the settings manager for dynamic settings access
func (h *XtreamHandler) SetHideUnavailable(getter func() bool) {
	h.hideUnavailable = getter
}

// loadEpisodeCache loads the episode cache from disk
func (h *XtreamHandler) loadEpisodeCache() {
	data, err := os.ReadFile(episodeCacheFile)
	if err != nil {
		log.Printf("No episode cache file found, starting fresh")
		return
	}
	
	h.episodeMu.Lock()
	defer h.episodeMu.Unlock()
	
	if err := json.Unmarshal(data, &h.episodeCache); err != nil {
		log.Printf("Error loading episode cache: %v", err)
		return
	}
	
	log.Printf("Loaded %d episodes from cache", len(h.episodeCache))
}

// saveEpisodeCache saves the episode cache to disk
func (h *XtreamHandler) saveEpisodeCache() {
	h.episodeMu.RLock()
	data, err := json.Marshal(h.episodeCache)
	h.episodeMu.RUnlock()
	
	if err != nil {
		log.Printf("Error marshaling episode cache: %v", err)
		return
	}
	
	// Ensure cache directory exists
	if err := os.MkdirAll("cache", 0755); err != nil {
		log.Printf("Error creating cache directory: %v", err)
		return
	}
	
	if err := os.WriteFile(episodeCacheFile, data, 0644); err != nil {
		log.Printf("Error saving episode cache: %v", err)
		return
	}
	
	log.Printf("Saved %d episodes to cache", len(h.episodeCache))
}

// getReleaseFilters loads release filter settings from the database
func (h *XtreamHandler) getReleaseFilters() *providers.ReleaseFilters {
	filters := &providers.ReleaseFilters{}
	
	// Query settings from database
	rows, err := h.db.Query(`SELECT key, value FROM settings WHERE key IN ('enable_release_filters', 'excluded_release_groups', 'excluded_languages', 'excluded_qualities')`)
	if err != nil {
		log.Printf("Error loading release filters: %v", err)
		return filters
	}
	defer rows.Close()
	
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "enable_release_filters":
			filters.Enabled = value == "true"
		case "excluded_release_groups":
			filters.ExcludedGroups = value
		case "excluded_languages":
			filters.ExcludedLanguages = value
		case "excluded_qualities":
			filters.ExcludedQualities = value
		}
	}
	
	return filters
}

// getStreamSortOptions loads stream sorting settings from the database
func (h *XtreamHandler) getStreamSortOptions() *providers.StreamSortOptions {
	opts := &providers.StreamSortOptions{
		SortOrder:  "quality,size,seeders", // default
		SortPrefer: "best",                  // default
	}
	
	rows, err := h.db.Query(`SELECT key, value FROM settings WHERE key IN ('stream_sort_order', 'stream_sort_prefer')`)
	if err != nil {
		log.Printf("Error loading sort options: %v", err)
		return opts
	}
	defer rows.Close()
	
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "stream_sort_order":
			if value != "" {
				opts.SortOrder = value
			}
		case "stream_sort_prefer":
			if value != "" {
				opts.SortPrefer = value
			}
		}
	}
	
	return opts
}

// getYouTubeTrailer fetches the YouTube trailer ID for a movie or series from TMDB
func (h *XtreamHandler) getYouTubeTrailer(tmdbID int64, mediaType string) string {
	ctx := context.Background()
	
	// Fetch videos from TMDB API (use api_key query param for v3 API keys)
	apiURL := fmt.Sprintf("https://api.themoviedb.org/3/%s/%d/videos?language=en-US&api_key=%s", mediaType, tmdbID, h.cfg.TMDBAPIKey)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return ""
	}
	
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	
	var result struct {
		Results []struct {
			Key      string `json:"key"`
			Site     string `json:"site"`
			Type     string `json:"type"`
			Official bool   `json:"official"`
		} `json:"results"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	
	// Look for official trailer first
	for _, video := range result.Results {
		if video.Site == "YouTube" && video.Type == "Trailer" && video.Official {
			return video.Key
		}
	}
	
	// Fall back to any trailer
	for _, video := range result.Results {
		if video.Site == "YouTube" && video.Type == "Trailer" {
			return video.Key
		}
	}
	
	// Fall back to any YouTube video (teaser, clip, etc.)
	for _, video := range result.Results {
		if video.Site == "YouTube" {
			return video.Key
		}
	}
	
	return ""
}

func (h *XtreamHandler) RegisterRoutes(r *mux.Router) {
	// Xtream Codes API endpoints
	r.HandleFunc("/player_api.php", h.handlePlayerAPI).Methods("GET")
	r.HandleFunc("/panel_api.php", h.handlePanelAPI).Methods("GET")
	r.HandleFunc("/xmltv.php", h.handleXMLTV).Methods("GET")
	r.HandleFunc("/play.php", h.handlePlay).Methods("GET")
	r.HandleFunc("/get.php", h.handleGetPlaylist).Methods("GET")
	
	// Xtream playback routes - /movie/user/pass/{id}.{ext} format
	// Support both GET and HEAD (some media players check with HEAD first)
	// Quality suffix route (e.g., /movie/user/pass/550_1080p.mp4)
	r.HandleFunc("/movie/{username}/{password}/{id}_{quality}.{ext}", h.handleMoviePlayWithQuality).Methods("GET", "HEAD")
	r.HandleFunc("/movie/{username}/{password}/{id}.{ext}", h.handleMoviePlay).Methods("GET", "HEAD")
	r.HandleFunc("/series/{username}/{password}/{id}.{ext}", h.handleSeriesPlay).Methods("GET", "HEAD")
	r.HandleFunc("/live/{username}/{password}/{id}.{ext}", h.handleLivePlay).Methods("GET", "HEAD")
	
	// Direct VOD format (some apps use this without /movie/ prefix)
	r.HandleFunc("/{username}/{password}/{id}.{ext}", h.handleDirectPlay).Methods("GET", "HEAD")
}

func (h *XtreamHandler) handlePlayerAPI(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	
	log.Printf("Xtream API: action=%s", action)
	
	w.Header().Set("Content-Type", "application/json")
	
	switch action {
	case "get_vod_categories":
		h.getVODCategories(w, r)
	case "get_vod_streams":
		h.getVODStreams(w, r)
	case "get_vod_info":
		h.getVODInfo(w, r)
	case "get_series_categories":
		h.getSeriesCategories(w, r)
	case "get_series":
		h.getSeries(w, r)
	case "get_series_info":
		h.getSeriesInfo(w, r)
	case "get_live_categories":
		h.getLiveCategories(w, r)
	case "get_live_streams":
		h.getLiveStreams(w, r)
	default:
		h.getServerInfo(w, r)
	}
}

func (h *XtreamHandler) getServerInfo(w http.ResponseWriter, r *http.Request) {
	// Get the actual host from the request (PHP returns just host, not full URL)
	host := r.Host
	if host == "" {
		host = fmt.Sprintf("%s:%d", h.cfg.Host, h.cfg.ServerPort)
	}
	
	// Extract just the hostname without port for URL field (like PHP does)
	hostOnly := host
	if idx := strings.Index(host, ":"); idx > 0 {
		hostOnly = host[:idx]
	}
	
	// Get port separately
	port := fmt.Sprintf("%d", h.cfg.ServerPort)
	if idx := strings.Index(host, ":"); idx > 0 {
		port = host[idx+1:]
	}
	
	// Get username from request for response
	username := r.URL.Query().Get("username")
	if username == "" {
		username = "user"
	}

	// Current timestamp for update tracking
	now := time.Now().Unix()

	info := map[string]interface{}{
		"user_info": map[string]interface{}{
			"username":             username,
			"password":             r.URL.Query().Get("password"),
			"message":              "",
			"auth":                 1,
			"status":               "Active",
			"exp_date":             "4095101905",
			"is_trial":             "0",
			"active_cons":          "0",
			"created_at":           "1684851647",
			"max_connections":      "1000",
			"allowed_output_formats": []string{"m3u8", ""},
		},
		"server_info": map[string]interface{}{
			"url":                    hostOnly,
			"port":                   port,
			"https_port":             "",
			"server_protocol":        "http",
			"rtmp_port":              "",
			"timezone":               "America/New_York",
			"timestamp_now":          now,
			"time_now":               time.Now().Format("2006-01-02 15:04:05"),
			"process":                true,
		},
	}

	json.NewEncoder(w).Encode(info)
}

// handlePanelAPI handles the panel_api.php endpoint used by some IPTV apps for content updates
func (h *XtreamHandler) handlePanelAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Get counts from database
	var movieCount, seriesCount int
	h.db.QueryRow("SELECT COUNT(*) FROM library_movies").Scan(&movieCount)
	h.db.QueryRow("SELECT COUNT(*) FROM library_series").Scan(&seriesCount)
	
	// Get live channel count
	liveCount := 0
	if h.channelManager != nil {
		liveCount = h.channelManager.GetChannelCount()
	}
	
	// Current timestamp for updates
	now := time.Now().Unix()
	
	// Panel API response with update timestamps
	// This tells apps when content was last updated so they can sync
	response := map[string]interface{}{
		"user_info": map[string]interface{}{
			"username":        "zeroq",
			"password":        "streamarr",
			"auth":            1,
			"status":          "Active",
			"exp_date":        "4095101905",
			"is_trial":        "0",
			"active_cons":     "0",
			"created_at":      "1684851647",
			"max_connections": "1000",
		},
		"server_info": map[string]interface{}{
			"timezone":      "America/New_York",
			"timestamp_now": now,
			"time_now":      time.Now().Format("2006-01-02 15:04:05"),
		},
		"available_channels": map[string]interface{}{
			"live_streams":   liveCount,
			"vod_streams":    movieCount,
			"series_streams": seriesCount,
			"radio_streams":  0,
		},
		"categories": map[string]interface{}{
			"live":   h.getLiveCategoryCount(),
			"movie":  h.getVODCategoryCount(),
			"series": h.getSeriesCategoryCount(),
		},
		// Update timestamps - these tell apps when to refresh content
		"update_info": map[string]interface{}{
			"live_last_modified":   now,
			"vod_last_modified":    now,
			"series_last_modified": now,
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// Helper functions for category counts
func (h *XtreamHandler) getLiveCategoryCount() int {
	if h.channelManager == nil {
		return 0
	}
	return len(h.channelManager.GetCategories())
}

func (h *XtreamHandler) getVODCategoryCount() int {
	return 21 // Number of movie categories we have
}

func (h *XtreamHandler) getSeriesCategoryCount() int {
	return 17 // Number of series categories (genres + networks)
}

func (h *XtreamHandler) getVODCategories(w http.ResponseWriter, r *http.Request) {
	// Movie genres - using TMDB genre IDs for proper mapping
	categories := []map[string]interface{}{
		{"category_id": "999992", "category_name": "Now Playing", "parent_id": 0},
		{"category_id": "999991", "category_name": "Popular", "parent_id": 0},
		{"category_id": "28", "category_name": "Action", "parent_id": 0},
		{"category_id": "12", "category_name": "Adventure", "parent_id": 0},
		{"category_id": "16", "category_name": "Animation", "parent_id": 0},
		{"category_id": "35", "category_name": "Comedy", "parent_id": 0},
		{"category_id": "80", "category_name": "Crime", "parent_id": 0},
		{"category_id": "99", "category_name": "Documentary", "parent_id": 0},
		{"category_id": "18", "category_name": "Drama", "parent_id": 0},
		{"category_id": "10751", "category_name": "Family", "parent_id": 0},
		{"category_id": "14", "category_name": "Fantasy", "parent_id": 0},
		{"category_id": "36", "category_name": "History", "parent_id": 0},
		{"category_id": "27", "category_name": "Horror", "parent_id": 0},
		{"category_id": "10402", "category_name": "Music", "parent_id": 0},
		{"category_id": "9648", "category_name": "Mystery", "parent_id": 0},
		{"category_id": "10749", "category_name": "Romance", "parent_id": 0},
		{"category_id": "878", "category_name": "Science Fiction", "parent_id": 0},
		{"category_id": "10770", "category_name": "TV Movie", "parent_id": 0},
		{"category_id": "53", "category_name": "Thriller", "parent_id": 0},
		{"category_id": "10752", "category_name": "War", "parent_id": 0},
		{"category_id": "37", "category_name": "Western", "parent_id": 0},
	}
	
	json.NewEncoder(w).Encode(categories)
}

func (h *XtreamHandler) getVODStreams(w http.ResponseWriter, r *http.Request) {
	// Get category_id filter from query params
	categoryFilter := r.URL.Query().Get("category_id")
	
	// Check if we should hide unavailable content
	hideUnavailable := false
	if h.hideUnavailable != nil {
		hideUnavailable = h.hideUnavailable()
	}
	
	// Query movies - using TMDB ID as stream_id for proper playback routing
	// If hideUnavailable is true, show movies where available = true OR not yet checked (last_checked IS NULL)
	// This ensures newly added movies still show in IPTV until they've been scanned
	var query string
	if hideUnavailable {
		query = `
			SELECT id, tmdb_id, title, year, metadata, COALESCE(EXTRACT(EPOCH FROM added_at), 0) as added_ts
			FROM library_movies
			WHERE monitored = true AND (available = true OR last_checked IS NULL)
			ORDER BY id DESC
		`
	} else {
		query = `
			SELECT id, tmdb_id, title, year, metadata, COALESCE(EXTRACT(EPOCH FROM added_at), 0) as added_ts
			FROM library_movies
			WHERE monitored = true
			ORDER BY id DESC
		`
	}
	
	rows, err := h.db.Query(query)
	if err != nil {
		log.Printf("Error querying movies: %v", err)
		json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}
	defer rows.Close()
	
	streams := make([]map[string]interface{}, 0)
	num := 1
	
	for rows.Next() {
		var id, tmdbID int64
		var title string
		var year sql.NullInt64
		var metadataJSON []byte
		var addedTs float64
		
		if err := rows.Scan(&id, &tmdbID, &title, &year, &metadataJSON, &addedTs); err != nil {
			continue
		}
		
		var metadata map[string]interface{}
		json.Unmarshal(metadataJSON, &metadata)
		
		// Build full poster URL
		posterPath := ""
		if pp, ok := metadata["poster_path"].(string); ok && pp != "" {
			posterPath = "https://image.tmdb.org/t/p/w500" + pp
		}
		
		backdropPath := ""
		if bp, ok := metadata["backdrop_path"].(string); ok && bp != "" {
			backdropPath = "https://image.tmdb.org/t/p/original" + bp
		}
		
		// Get numeric rating
		rating := float64(0)
		if r, ok := metadata["vote_average"].(float64); ok {
			rating = r
		}
		
		plot := ""
		if p, ok := metadata["overview"].(string); ok {
			plot = p
		}
		
		// TMDB movie genre name to ID mapping
		genreNameToID := map[string]string{
			"Action": "28", "Adventure": "12", "Animation": "16", "Comedy": "35",
			"Crime": "80", "Documentary": "99", "Drama": "18", "Family": "10751",
			"Fantasy": "14", "History": "36", "Horror": "27", "Music": "10402",
			"Mystery": "9648", "Romance": "10749", "Science Fiction": "878",
			"TV Movie": "10770", "Thriller": "53", "War": "10752", "Western": "37",
		}
		
		// Get genre category IDs from metadata
		categoryID := "999992" // Default to "Now Playing"
		categoryIDs := []string{"999992"}
		
		if genres, ok := metadata["genres"].([]interface{}); ok && len(genres) > 0 {
			categoryIDs = make([]string, 0, len(genres)+1)
			categoryIDs = append(categoryIDs, "999992") // Always include default
			
			for i, g := range genres {
				// Try object format first: {"id": 27, "name": "Horror"}
				if gm, ok := g.(map[string]interface{}); ok {
					if genreID, ok := gm["id"].(float64); ok {
						genreIDStr := fmt.Sprintf("%.0f", genreID)
						categoryIDs = append(categoryIDs, genreIDStr)
						if i == 0 {
							categoryID = genreIDStr
						}
					}
				} else if genreName, ok := g.(string); ok {
					// String format: "Horror"
					if genreID, found := genreNameToID[genreName]; found {
						categoryIDs = append(categoryIDs, genreID)
						if i == 0 {
							categoryID = genreID
						}
					}
				}
			}
		}
		
		// Convert timestamp to int
		addedInt := int64(addedTs)
		if addedInt == 0 {
			addedInt = time.Now().Unix()
		}
		
		// IMPORTANT: Use TMDB ID as stream_id for playback
		stream := map[string]interface{}{
			"num":                 num,
			"stream_id":           tmdbID,
			"name":                title,
			"title":               title,
			"year":                fmt.Sprintf("%d", year.Int64),
			"stream_type":         "movie",
			"stream_icon":         posterPath,
			"cover":               posterPath,
			"backdrop_path":       backdropPath,
			"rating":              rating,
			"rating_5based":       rating / 2,
			"category_id":         categoryID,
			"category_ids":        categoryIDs,
			"container_extension": "mp4",
			"custom_sid":          "",
			"direct_source":       "",
			"plot":                plot,
			"added":               addedInt,
			"last_modified":       addedInt,
			"group":               "StreamArr",
		}
		
		// Filter by category if specified
		if categoryFilter != "" {
			found := false
			for _, cid := range categoryIDs {
				if cid == categoryFilter {
					found = true
					break
				}
			}
			if !found {
				continue // Skip this movie
			}
		}
		
		streams = append(streams, stream)
		num++
	}
	
	json.NewEncoder(w).Encode(streams)
}

func (h *XtreamHandler) getVODInfo(w http.ResponseWriter, r *http.Request) {
	vodID := r.URL.Query().Get("vod_id")
	if vodID == "" {
		http.Error(w, "Missing vod_id", http.StatusBadRequest)
		return
	}
	
	tmdbID, err := strconv.ParseInt(vodID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid vod_id", http.StatusBadRequest)
		return
	}
	
	// Query movie by TMDB ID first, then fallback to database ID
	var id int64
	var title string
	var year sql.NullInt64
	var metadataJSON []byte
	var imdbID sql.NullString
	
	query := `SELECT id, title, year, metadata, imdb_id FROM library_movies WHERE tmdb_id = $1`
	err = h.db.QueryRow(query, tmdbID).Scan(&id, &title, &year, &metadataJSON, &imdbID)
	if err != nil {
		// Try by database ID as fallback
		query = `SELECT id, tmdb_id, title, year, metadata, imdb_id FROM library_movies WHERE id = $1`
		err = h.db.QueryRow(query, tmdbID).Scan(&id, &tmdbID, &title, &year, &metadataJSON, &imdbID)
		if err != nil {
			http.Error(w, "Movie not found", http.StatusNotFound)
			return
		}
	}
	
	var metadata map[string]interface{}
	json.Unmarshal(metadataJSON, &metadata)
	
	// Get IMDB ID from metadata if column is empty
	imdbIDStr := imdbID.String
	if imdbIDStr == "" {
		if id, ok := metadata["imdb_id"].(string); ok && id != "" {
			imdbIDStr = id
		}
	}
	
	// Build full image URLs
	posterPath := ""
	if pp, ok := metadata["poster_path"].(string); ok && pp != "" {
		posterPath = "https://image.tmdb.org/t/p/original" + pp
	}
	backdropPath := ""
	if bp, ok := metadata["backdrop_path"].(string); ok && bp != "" {
		backdropPath = "https://image.tmdb.org/t/p/original" + bp
	}
	
	rating := float64(0)
	if r, ok := metadata["vote_average"].(float64); ok {
		rating = r
	}
	
	runtime := 0
	if rt, ok := metadata["runtime"].(float64); ok {
		runtime = int(rt)
	}
	hours := runtime / 60
	minutes := runtime % 60
	duration := fmt.Sprintf("%02d:%02d:00", hours, minutes)
	
	// Get genres as string
	genresString := ""
	if genres, ok := metadata["genres"].([]interface{}); ok {
		genreNames := make([]string, 0)
		for _, g := range genres {
			if gm, ok := g.(map[string]interface{}); ok {
				if name, ok := gm["name"].(string); ok {
					genreNames = append(genreNames, name)
				}
			}
		}
		genresString = strings.Join(genreNames, ", ")
	}
	
	// Get cast
	castString := ""
	if cast, ok := metadata["cast"].([]interface{}); ok {
		castNames := make([]string, 0, 4)
		for i, c := range cast {
			if i >= 4 {
				break
			}
			if cm, ok := c.(map[string]interface{}); ok {
				if name, ok := cm["name"].(string); ok {
					castNames = append(castNames, name)
				}
			}
		}
		castString = strings.Join(castNames, ", ")
	}
	
	// Get director
	director := ""
	if crew, ok := metadata["crew"].([]interface{}); ok {
		for _, c := range crew {
			if cm, ok := c.(map[string]interface{}); ok {
				if job, ok := cm["job"].(string); ok && job == "Director" {
					if name, ok := cm["name"].(string); ok {
						director = name
						break
					}
				}
			}
		}
	}
	
	// Fetch YouTube trailer from TMDB
	youtubeTrailer := h.getYouTubeTrailer(tmdbID, "movie")
	
	info := map[string]interface{}{
		"info": map[string]interface{}{
			"movie_image":     posterPath,
			"tmdb_id":         tmdbID,
			"imdb_id":         imdbIDStr,
			"youtube_trailer": youtubeTrailer,
			"genre":           genresString,
			"director":        director,
			"plot":            metadata["overview"],
			"rating":          rating,
			"releasedate":     metadata["release_date"],
			"duration_secs":   runtime * 60,
			"duration":        duration,
			"cast":            castString,
			"video":           []interface{}{},
			"audio":           []interface{}{},
			"bitrate":         0,
			"backdrop_path":   []string{backdropPath},
			"cover":           posterPath,
		},
		"movie_data": map[string]interface{}{
			"stream_id":           tmdbID,
			"name":                title,
			"added":               time.Now().Unix(),
			"category_id":         "999992",
			"container_extension": "mkv",
			"custom_sid":          imdbIDStr,
			"direct_source":       "",
		},
	}
	
	json.NewEncoder(w).Encode(info)
}

func (h *XtreamHandler) getSeriesCategories(w http.ResponseWriter, r *http.Request) {
	// TV series genres - using TMDB genre IDs
	// Include TV networks like PHP version does (category_id format: 99999+networkId)
	categories := []map[string]interface{}{
		{"category_id": "88883", "category_name": "On The Air", "parent_id": 0},
		{"category_id": "88882", "category_name": "Top Rated", "parent_id": 0},
		{"category_id": "88881", "category_name": "Popular", "parent_id": 0},
		// TV Networks (using 99999+networkId format like PHP)
		{"category_id": "999992552", "category_name": "Apple TV+", "parent_id": 0},
		{"category_id": "9999964", "category_name": "Discovery", "parent_id": 0},
		{"category_id": "999992739", "category_name": "Disney+", "parent_id": 0},
		{"category_id": "9999949", "category_name": "HBO", "parent_id": 0},
		{"category_id": "9999965", "category_name": "History", "parent_id": 0},
		{"category_id": "99999453", "category_name": "Hulu", "parent_id": 0},
		{"category_id": "99999244", "category_name": "Investigation", "parent_id": 0},
		{"category_id": "9999934", "category_name": "Lifetime", "parent_id": 0},
		{"category_id": "99999213", "category_name": "Netflix", "parent_id": 0},
		{"category_id": "99999132", "category_name": "Oxygen", "parent_id": 0},
		// Genres
		{"category_id": "10759", "category_name": "Action & Adventure", "parent_id": 0},
		{"category_id": "16", "category_name": "Animation", "parent_id": 0},
		{"category_id": "35", "category_name": "Comedy", "parent_id": 0},
		{"category_id": "80", "category_name": "Crime", "parent_id": 0},
		{"category_id": "99", "category_name": "Documentary", "parent_id": 0},
		{"category_id": "18", "category_name": "Drama", "parent_id": 0},
		{"category_id": "10751", "category_name": "Family", "parent_id": 0},
		{"category_id": "10762", "category_name": "Kids", "parent_id": 0},
		{"category_id": "9648", "category_name": "Mystery", "parent_id": 0},
		{"category_id": "10763", "category_name": "News", "parent_id": 0},
		{"category_id": "10764", "category_name": "Reality", "parent_id": 0},
		{"category_id": "10765", "category_name": "Sci-Fi & Fantasy", "parent_id": 0},
		{"category_id": "10766", "category_name": "Soap", "parent_id": 0},
		{"category_id": "10767", "category_name": "Talk", "parent_id": 0},
		{"category_id": "10768", "category_name": "War & Politics", "parent_id": 0},
		{"category_id": "37", "category_name": "Western", "parent_id": 0},
	}
	
	json.NewEncoder(w).Encode(categories)
}

func (h *XtreamHandler) getSeries(w http.ResponseWriter, r *http.Request) {
	// Get category_id filter from query params
	categoryFilter := r.URL.Query().Get("category_id")
	
	// Query all series - using TMDB ID as series_id for proper playback routing
	query := `
		SELECT id, tmdb_id, title, year, metadata, COALESCE(EXTRACT(EPOCH FROM added_at), 0) as added_ts
		FROM library_series
		WHERE monitored = true
		ORDER BY id DESC
	`
	
	rows, err := h.db.Query(query)
	if err != nil {
		log.Printf("Error querying series: %v", err)
		json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}
	defer rows.Close()
	
	series := make([]map[string]interface{}, 0)
	num := 1
	
	for rows.Next() {
		var id, tmdbID int64
		var title string
		var year sql.NullInt64
		var metadataJSON []byte
		var addedTs float64
		
		if err := rows.Scan(&id, &tmdbID, &title, &year, &metadataJSON, &addedTs); err != nil {
			continue
		}
		
		var metadata map[string]interface{}
		json.Unmarshal(metadataJSON, &metadata)
		
		// Build full poster URL
		posterPath := ""
		if pp, ok := metadata["poster_path"].(string); ok && pp != "" {
			posterPath = "https://image.tmdb.org/t/p/w500" + pp
		}
		
		backdropPath := ""
		if bp, ok := metadata["backdrop_path"].(string); ok && bp != "" {
			backdropPath = "https://image.tmdb.org/t/p/original" + bp
		}
		
		rating := float64(0)
		if r, ok := metadata["vote_average"].(float64); ok {
			rating = r
		}
		
		plot := ""
		if p, ok := metadata["overview"].(string); ok {
			plot = p
		}
		
		// TMDB TV genre name to ID mapping
		tvGenreNameToID := map[string]string{
			"Action & Adventure": "10759", "Animation": "16", "Comedy": "35",
			"Crime": "80", "Documentary": "99", "Drama": "18", "Family": "10751",
			"Kids": "10762", "Mystery": "9648", "News": "10763", "Reality": "10764",
			"Sci-Fi & Fantasy": "10765", "Soap": "10766", "Talk": "10767",
			"War & Politics": "10768", "Western": "37",
		}
		
		// Get genre category IDs from metadata
		categoryID := "88881" // Default to "Popular"
		categoryIDs := []string{"88881"}
		genreStr := ""
		
		if genres, ok := metadata["genres"].([]interface{}); ok && len(genres) > 0 {
			categoryIDs = make([]string, 0, len(genres)+1)
			categoryIDs = append(categoryIDs, "88881") // Always include default
			genreNames := make([]string, 0, len(genres))
			
			for i, g := range genres {
				// Try object format first: {"id": 27, "name": "Horror"}
				if gm, ok := g.(map[string]interface{}); ok {
					if genreID, ok := gm["id"].(float64); ok {
						genreIDStr := fmt.Sprintf("%.0f", genreID)
						categoryIDs = append(categoryIDs, genreIDStr)
						if i == 0 {
							categoryID = genreIDStr
						}
					}
					if name, ok := gm["name"].(string); ok {
						genreNames = append(genreNames, name)
					}
				} else if genreName, ok := g.(string); ok {
					// String format: "Drama"
					genreNames = append(genreNames, genreName)
					if genreID, found := tvGenreNameToID[genreName]; found {
						categoryIDs = append(categoryIDs, genreID)
						if i == 0 {
							categoryID = genreID
						}
					}
				}
			}
			genreStr = strings.Join(genreNames, ", ")
		}
		
		// IMPORTANT: Use TMDB ID as series_id for playback
		// Include added and last_modified for app caching
		addedInt := int64(addedTs)
		if addedInt == 0 {
			addedInt = time.Now().Unix() - 86400 // Default to 24h ago if not set
		}
		
		s := map[string]interface{}{
			"num":           num,
			"series_id":     tmdbID,
			"name":          title,
			"title":         title,
			"year":          fmt.Sprintf("%d", year.Int64),
			"cover":         posterPath,
			"backdrop_path": []string{backdropPath},
			"rating":        rating,
			"rating_5based": rating / 2,
			"plot":          plot,
			"cast":          "",
			"director":      "",
			"genre":         genreStr,
			"releaseDate":   year.Int64,
			"category_id":   categoryID,
			"category_ids":  categoryIDs,
			"added":         addedInt,
			"last_modified": addedInt,
		}
		
		// Filter by category if specified
		if categoryFilter != "" {
			found := false
			for _, cid := range categoryIDs {
				if cid == categoryFilter {
					found = true
					break
				}
			}
			if !found {
				continue // Skip this series
			}
		}
		
		series = append(series, s)
		num++
	}
	
	json.NewEncoder(w).Encode(series)
}

func (h *XtreamHandler) getSeriesInfo(w http.ResponseWriter, r *http.Request) {
	seriesID := r.URL.Query().Get("series_id")
	if seriesID == "" {
		http.Error(w, "Missing series_id", http.StatusBadRequest)
		return
	}
	
	tmdbID, err := strconv.ParseInt(seriesID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid series_id", http.StatusBadRequest)
		return
	}
	
	// Query series by TMDB ID first, then fallback to database ID
	var id int64
	var title string
	var year sql.NullInt64
	var metadataJSON []byte
	var imdbID sql.NullString
	
	query := `SELECT id, title, year, metadata, imdb_id FROM library_series WHERE tmdb_id = $1`
	err = h.db.QueryRow(query, tmdbID).Scan(&id, &title, &year, &metadataJSON, &imdbID)
	if err != nil {
		// Try by database ID as fallback
		query = `SELECT id, tmdb_id, title, year, metadata, imdb_id FROM library_series WHERE id = $1`
		err = h.db.QueryRow(query, tmdbID).Scan(&id, &tmdbID, &title, &year, &metadataJSON, &imdbID)
		if err != nil {
			http.Error(w, "Series not found", http.StatusNotFound)
			return
		}
	}
	
	var metadata map[string]interface{}
	json.Unmarshal(metadataJSON, &metadata)
	
	// Build full image URLs
	posterPath := ""
	if pp, ok := metadata["poster_path"].(string); ok && pp != "" {
		posterPath = "https://image.tmdb.org/t/p/original" + pp
	}
	backdropPath := ""
	if bp, ok := metadata["backdrop_path"].(string); ok && bp != "" {
		backdropPath = "https://image.tmdb.org/t/p/original" + bp
	}
	
	rating := float64(0)
	if r, ok := metadata["vote_average"].(float64); ok {
		rating = r
	}
	
	// Get cast
	castString := ""
	if cast, ok := metadata["cast"].([]interface{}); ok {
		castNames := make([]string, 0, 4)
		for i, c := range cast {
			if i >= 4 {
				break
			}
			if cm, ok := c.(map[string]interface{}); ok {
				if name, ok := cm["name"].(string); ok {
					castNames = append(castNames, name)
				}
			}
		}
		castString = strings.Join(castNames, ", ")
	}
	
	// Get genres
	genresString := ""
	if genres, ok := metadata["genres"].([]interface{}); ok {
		genreNames := make([]string, 0)
		for _, g := range genres {
			if gm, ok := g.(map[string]interface{}); ok {
				if name, ok := gm["name"].(string); ok {
					genreNames = append(genreNames, name)
				}
			}
		}
		genresString = strings.Join(genreNames, ", ")
	}
	
	// Get IMDB ID string
	imdbIDStr := imdbID.String
	
	// Get number of seasons from metadata, or fetch from TMDB
	numberOfSeasons := 0
	if ns, ok := metadata["number_of_seasons"].(float64); ok {
		numberOfSeasons = int(ns)
	}
	
	ctx := r.Context()
	
	// If number_of_seasons is not in metadata, fetch from TMDB API
	if numberOfSeasons == 0 {
		tmdbSeries, err := h.tmdb.GetSeries(ctx, int(tmdbID))
		if err != nil {
			log.Printf("Error fetching series details from TMDB: %v", err)
		} else {
			numberOfSeasons = tmdbSeries.Seasons
			// Update images if not set
			if posterPath == "" && tmdbSeries.PosterPath != "" {
				posterPath = "https://image.tmdb.org/t/p/original" + tmdbSeries.PosterPath
			}
			if backdropPath == "" && tmdbSeries.BackdropPath != "" {
				backdropPath = "https://image.tmdb.org/t/p/original" + tmdbSeries.BackdropPath
			}
			if rating == 0 {
				if va, ok := tmdbSeries.Metadata["vote_average"].(float64); ok {
					rating = va
				}
			}
			if genresString == "" && len(tmdbSeries.Genres) > 0 {
				genresString = strings.Join(tmdbSeries.Genres, ", ")
			}
			log.Printf("Fetched series %d from TMDB: %d seasons", tmdbID, numberOfSeasons)
		}
	}
	
	// Build a map of episode availability from the database if hideUnavailable is enabled
	episodeAvailability := make(map[string]bool) // key: "season:episode" -> available
	hideUnavailableEpisodes := h.hideUnavailable != nil && h.hideUnavailable()
	if hideUnavailableEpisodes {
		availQuery := `
			SELECT e.season_number, e.episode_number, e.available 
			FROM library_episodes e
			JOIN library_series s ON e.series_id = s.id
			WHERE s.tmdb_id = $1 AND e.last_checked IS NOT NULL
		`
		availRows, err := h.db.Query(availQuery, tmdbID)
		if err == nil {
			defer availRows.Close()
			for availRows.Next() {
				var seasonNum, episodeNum int
				var available bool
				if availRows.Scan(&seasonNum, &episodeNum, &available) == nil {
					key := fmt.Sprintf("%d:%d", seasonNum, episodeNum)
					episodeAvailability[key] = available
				}
			}
		}
	}
	
	seasons := make([]map[string]interface{}, 0)
	episodes := make(map[string][]map[string]interface{})
	
	// Fetch seasons and episodes from TMDB API dynamically
	for seasonNum := 1; seasonNum <= numberOfSeasons; seasonNum++ {
		season, err := h.tmdb.GetSeason(ctx, int(tmdbID), seasonNum)
		if err != nil {
			log.Printf("Error fetching season %d for series %d: %v", seasonNum, tmdbID, err)
			continue
		}
		
		// Build season info
		seasonKey := fmt.Sprintf("%d", seasonNum)
		episodeCount := 0
		
		// Process episodes for this season
		for _, ep := range season.Episodes {
			// Skip future episodes
			if ep.AirDate != "" {
				airTime, err := time.Parse("2006-01-02", ep.AirDate)
				if err == nil && airTime.After(time.Now()) {
					continue
				}
			} else {
				continue // Skip episodes without air date
			}
			
			// Skip unavailable episodes if hideUnavailable is enabled
			if hideUnavailableEpisodes {
				key := fmt.Sprintf("%d:%d", seasonNum, ep.EpisodeNumber)
				if available, checked := episodeAvailability[key]; checked && !available {
					continue // Episode was checked and has no streams
				}
			}
			
			episodeCount++
			
			// Encode episode data for custom_sid: imdb_id:tmdb_id/season/X/episode/Y
			customSid := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%d/season/%d/episode/%d",
				imdbIDStr, tmdbID, seasonNum, ep.EpisodeNumber)))
			
			// Cache episode lookup info (like PHP's episode_lookup.json)
			episodeIDStr := fmt.Sprintf("%d", ep.ID)
			h.episodeMu.Lock()
			h.episodeCache[episodeIDStr] = EpisodeLookup{
				SeriesID: fmt.Sprintf("%d", tmdbID),
				Season:   seasonNum,
				Episode:  ep.EpisodeNumber,
				IMDBID:   imdbIDStr,
			}
			h.episodeMu.Unlock()
			
			// Build episode image URL
			episodeImage := backdropPath
			if ep.StillPath != "" {
				episodeImage = "https://image.tmdb.org/t/p/original" + ep.StillPath
			}
			
			runtime := ep.Runtime
			if runtime == 0 {
				runtime = 45
			}
			
			// Use TMDB episode ID as the id (like PHP version)
			episode := map[string]interface{}{
				"id":                  episodeIDStr,
				"episode_num":         ep.EpisodeNumber,
				"title":               fmt.Sprintf("%s - S%02dE%02d - %s", title, seasonNum, ep.EpisodeNumber, ep.Name),
				"container_extension": "mkv",
				"custom_sid":          customSid,
				"added":               "",
				"season":              seasonNum,
				"direct_source":       "",
				"info": map[string]interface{}{
					"tmdb_id":       fmt.Sprintf("%d", tmdbID),
					"name":          ep.Name,
					"air_date":      ep.AirDate,
					"rating":        ep.VoteAverage,
					"cover_big":     episodeImage,
					"plot":          ep.Overview,
					"movie_image":   episodeImage,
					"duration_secs": runtime * 60,
					"duration":      fmt.Sprintf("%02d:%02d:00", runtime/60, runtime%60),
				},
			}
			
			episodes[seasonKey] = append(episodes[seasonKey], episode)
		}
		
		// Add season to seasons array
		seasons = append(seasons, map[string]interface{}{
			"air_date":      season.AirDate,
			"episode_count": episodeCount,
			"id":            fmt.Sprintf("%d", season.ID),
			"name":          fmt.Sprintf("Season %d", seasonNum),
			"overview":      season.Overview,
			"season_number": seasonNum,
			"backdrop_path": backdropPath,
			"cover":         posterPath,
			"cover_big":     posterPath,
		})
	}
	
	// Fetch YouTube trailer for series
	youtubeTrailer := h.getYouTubeTrailer(tmdbID, "tv")
	
	result := map[string]interface{}{
		"seasons": seasons,
		"info": map[string]interface{}{
			"name":             title,
			"cover":            posterPath,
			"plot":             metadata["overview"],
			"cast":             castString,
			"director":         "",
			"genre":            genresString,
			"releaseDate":      metadata["first_air_date"],
			"last_modified":    time.Now().Unix(),
			"rating":           rating,
			"rating_5based":    rating / 2,
			"backdrop_path":    []string{backdropPath},
			"youtube_trailer":  youtubeTrailer,
			"episode_run_time": 45,
			"category_id":      "88881",
		},
		"episodes": episodes,
	}
	
	// Save episode cache to disk (like PHP's episode_lookup.json)
	go h.saveEpisodeCache()
	
	json.NewEncoder(w).Encode(result)
}

func (h *XtreamHandler) getLiveCategories(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}
	
	// Get unique categories from actual channel data
	channelCategories := h.channelManager.GetCategories()
	
	// Build categories list dynamically from channels
	categories := make([]map[string]interface{}, 0)
	categoryMap := make(map[string]bool)
	
	for _, ch := range h.channelManager.GetAllChannels() {
		catName := ch.Category
		if catName == "" {
			catName = "General"
		}
		if categoryMap[catName] {
			continue
		}
		categoryMap[catName] = true
	}
	
	// Sort and assign category IDs
	sortedCats := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		sortedCats = append(sortedCats, cat)
	}
	sort.Strings(sortedCats)
	
	for i, cat := range sortedCats {
		categories = append(categories, map[string]interface{}{
			"category_id":   fmt.Sprintf("%d", i+1),
			"category_name": cat,
			"parent_id":     0,
		})
	}
	
	// If no categories found, use defaults
	if len(categories) == 0 {
		categories = []map[string]interface{}{
			{"category_id": "1", "category_name": "Live TV", "parent_id": 0},
		}
	}
	
	_ = channelCategories // Use the variable
	json.NewEncoder(w).Encode(categories)
}

func (h *XtreamHandler) getLiveStreams(w http.ResponseWriter, r *http.Request) {
	// Get category_id filter from query params
	categoryFilter := r.URL.Query().Get("category_id")
	
	if h.channelManager == nil {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
		return
	}

	channels := h.channelManager.GetAllChannels()
	
	// Build dynamic category mapping (same logic as getLiveCategories)
	categoryMap := make(map[string]string)
	uniqueCats := make(map[string]bool)
	for _, ch := range channels {
		catName := ch.Category
		if catName == "" {
			catName = "General"
		}
		uniqueCats[catName] = true
	}
	
	sortedCats := make([]string, 0, len(uniqueCats))
	for cat := range uniqueCats {
		sortedCats = append(sortedCats, cat)
	}
	sort.Strings(sortedCats)
	
	for i, cat := range sortedCats {
		categoryMap[cat] = fmt.Sprintf("%d", i+1)
	}
	
	streams := make([]map[string]interface{}, 0, len(channels))
	
	for i, ch := range channels {
		catName := ch.Category
		if catName == "" {
			catName = "General"
		}
		catID := categoryMap[catName]
		if catID == "" {
			catID = "1"
		}
		
		// Filter by category if specified
		if categoryFilter != "" && catID != categoryFilter {
			continue
		}
		
		streamID := i + 1
		stream := map[string]interface{}{
			"num":                 streamID,
			"stream_id":           streamID,
			"name":                ch.Name,
			"stream_type":         "live",
			"stream_icon":         ch.Logo,
			"epg_channel_id":      ch.ID,
			"category_id":         catID,
			"category_ids":        []string{catID},
			"direct_source":       "",
			"custom_sid":          "",
			"tv_archive":          0,
			"tv_archive_duration": 0,
			"is_adult":            "0",
			"added":               time.Now().Unix(),
		}
		streams = append(streams, stream)
	}
	
	json.NewEncoder(w).Encode(streams)
}

func (h *XtreamHandler) handleXMLTV(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	
	// Get all channels
	channels := h.channelManager.GetAllChannels()
	channelList := make([]livetv.Channel, len(channels))
	for i, ch := range channels {
		channelList[i] = *ch
	}
	
	// Generate XMLTV EPG data
	xmltv, err := h.epgManager.GenerateXMLTV(channelList)
	if err != nil {
		log.Printf("Error generating XMLTV: %v", err)
		// Return empty structure on error
		xmltv = `<?xml version="1.0" encoding="UTF-8"?>
<tv generator-info-name="StreamArr">
</tv>`
	}
	
	w.Write([]byte(xmltv))
}

func (h *XtreamHandler) handlePlay(w http.ResponseWriter, r *http.Request) {
	vodID := r.URL.Query().Get("stream_id")
	if vodID == "" {
		vodID = r.URL.Query().Get("id")
	}
	seriesID := r.URL.Query().Get("series_id")
	season := r.URL.Query().Get("season")
	episode := r.URL.Query().Get("episode")
	
	if vodID != "" {
		h.playMovie(w, r, vodID)
	} else if seriesID != "" && season != "" && episode != "" {
		h.playEpisode(w, r, seriesID, season, episode)
	} else {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
	}
}

// handleMoviePlay handles /movie/{username}/{password}/{id}.{ext}
func (h *XtreamHandler) handleMoviePlay(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["id"]
	
	log.Printf("Movie play request: id=%s", movieID)
	h.playMovie(w, r, movieID)
}

// handleMoviePlayWithQuality handles /movie/{username}/{password}/{id}_{quality}.{ext}
// Supports quality suffixes: 4k, 2160p, 1080p, 720p, 480p
func (h *XtreamHandler) handleMoviePlayWithQuality(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["id"]
	quality := strings.ToLower(vars["quality"])
	
	log.Printf("Movie play request with quality: id=%s, quality=%s", movieID, quality)
	
	// Map quality suffix to max resolution (integer height)
	qualityMap := map[string]int{
		"4k":    2160,
		"2160p": 2160,
		"1080p": 1080,
		"720p":  720,
		"480p":  480,
	}
	
	if maxRes, ok := qualityMap[quality]; ok {
		// Store original max resolution
		originalMaxRes := h.cfg.MaxResolution
		// Temporarily set max resolution for this request
		h.cfg.MaxResolution = maxRes
		h.playMovie(w, r, movieID)
		// Restore original max resolution
		h.cfg.MaxResolution = originalMaxRes
	} else {
		h.playMovie(w, r, movieID)
	}
}

// handleSeriesPlay handles /series/{username}/{password}/{id}.{ext}
func (h *XtreamHandler) handleSeriesPlay(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	episodeID := vars["id"]
	
	log.Printf("Series play request: id=%s", episodeID)
	
	// First, check the episode cache (like PHP's episode_lookup.json)
	h.episodeMu.RLock()
	lookup, found := h.episodeCache[episodeID]
	h.episodeMu.RUnlock()
	
	if found && lookup.IMDBID != "" {
		log.Printf("Found episode in cache: IMDB %s S%02dE%02d", lookup.IMDBID, lookup.Season, lookup.Episode)
		h.playEpisodeByIMDB(w, r, lookup.IMDBID, lookup.Season, lookup.Episode)
		return
	}
	
	// If found in cache but IMDB ID is empty, try to fetch it from TMDB
	if found && lookup.SeriesID != "" {
		log.Printf("Episode found in cache but missing IMDB ID, fetching from TMDB for series %s", lookup.SeriesID)
		seriesID, err := strconv.Atoi(lookup.SeriesID)
		if err == nil {
			externalIDs, err := h.tmdb.GetSeriesExternalIDs(r.Context(), seriesID)
			if err == nil && externalIDs.IMDBID != "" {
				log.Printf("Got IMDB ID from TMDB: %s", externalIDs.IMDBID)
				// Update the cache with the IMDB ID
				h.episodeMu.Lock()
				lookup.IMDBID = externalIDs.IMDBID
				h.episodeCache[episodeID] = lookup
				h.episodeMu.Unlock()
				go h.saveEpisodeCache()
				
				h.playEpisodeByIMDB(w, r, externalIDs.IMDBID, lookup.Season, lookup.Episode)
				return
			} else if err != nil {
				log.Printf("Error fetching IMDB ID from TMDB: %v", err)
			}
		}
	}
	
	// Try to decode as base64 custom_sid (fallback)
	decoded, err := base64.StdEncoding.DecodeString(episodeID)
	if err != nil {
		// Try URL decoding first (some apps URL-encode the base64)
		unescaped, err2 := url.QueryUnescape(episodeID)
		if err2 == nil {
			decoded, err = base64.StdEncoding.DecodeString(unescaped)
		}
	}
	
	if err == nil && len(decoded) > 0 {
		// Format: imdb_id:tmdb_id/season/X/episode/Y
		decodedStr := string(decoded)
		log.Printf("Decoded episode data: %s", decodedStr)
		
		parts := strings.Split(decodedStr, "/")
		if len(parts) >= 5 {
			// Extract imdb_id from first part (format: imdb_id:tmdb_id)
			firstPart := parts[0]
			colonIdx := strings.Index(firstPart, ":")
			if colonIdx > 0 {
				imdbID := firstPart[:colonIdx]
				seasonNum := 0
				episodeNum := 0
				
				for i, p := range parts {
					if p == "season" && i+1 < len(parts) {
						seasonNum, _ = strconv.Atoi(parts[i+1])
					}
					if p == "episode" && i+1 < len(parts) {
						episodeNum, _ = strconv.Atoi(parts[i+1])
					}
				}
				
				if imdbID != "" && seasonNum > 0 && episodeNum > 0 {
					log.Printf("Playing episode from custom_sid: IMDB %s S%02dE%02d", imdbID, seasonNum, episodeNum)
					h.playEpisodeByIMDB(w, r, imdbID, seasonNum, episodeNum)
					return
				}
			}
		}
	}
	
	log.Printf("Episode not found in cache and not valid base64: %s", episodeID)
	http.Error(w, "Episode not found - please refresh series info", http.StatusNotFound)
}

// playEpisodeByIMDB streams an episode given IMDB ID and season/episode numbers
func (h *XtreamHandler) playEpisodeByIMDB(w http.ResponseWriter, r *http.Request, imdbID string, seasonNum, episodeNum int) {
	log.Printf("Playing episode: IMDB ID %s, S%02dE%02d", imdbID, seasonNum, episodeNum)
	
	// Get release filters and sort options from settings
	filters := h.getReleaseFilters()
	sortOpts := h.getStreamSortOptions()
	
	// Get stream from providers
	stream, err := h.multiProvider.GetBestStream(imdbID, &seasonNum, &episodeNum, h.cfg.MaxResolution, filters, sortOpts)
	if err != nil {
		log.Printf("Error getting stream: %v", err)
		http.Error(w, "Stream not available", http.StatusNotFound)
		return
	}
	
	// Redirect to stream URL
	if stream.URL != "" {
		log.Printf("Redirecting to stream: %s", stream.URL)
		http.Redirect(w, r, stream.URL, http.StatusFound)
	} else {
		http.Error(w, "Stream URL not available", http.StatusNotFound)
	}
}

// handleLivePlay handles /live/{username}/{password}/{id}.{ext}
func (h *XtreamHandler) handleLivePlay(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	streamID := vars["id"]
	
	log.Printf("Live play request: id=%s", streamID)
	
	id, err := strconv.Atoi(streamID)
	if err != nil {
		http.Error(w, "Invalid stream ID", http.StatusBadRequest)
		return
	}
	
	if h.channelManager == nil {
		http.Error(w, "Live TV not available", http.StatusNotFound)
		return
	}
	
	channels := h.channelManager.GetAllChannels()
	if id < 1 || id > len(channels) {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}
	
	channel := channels[id-1]
	if channel.StreamURL != "" {
		http.Redirect(w, r, channel.StreamURL, http.StatusFound)
	} else {
		http.Error(w, "Stream URL not available", http.StatusNotFound)
	}
}

// handleDirectPlay handles /{username}/{password}/{id}.{ext}
func (h *XtreamHandler) handleDirectPlay(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	movieID := vars["id"]
	
	log.Printf("Direct play request: id=%s", movieID)
	h.playMovie(w, r, movieID)
}

func (h *XtreamHandler) playMovie(w http.ResponseWriter, r *http.Request, vodID string) {
	tmdbID, _ := strconv.ParseInt(vodID, 10, 64)
	
	// Get IMDB ID from database by TMDB ID first - try both imdb_id column and metadata
	var imdbID sql.NullString
	var metadataJSON []byte
	
	query := `SELECT imdb_id, metadata FROM library_movies WHERE tmdb_id = $1`
	err := h.db.QueryRow(query, tmdbID).Scan(&imdbID, &metadataJSON)
	if err != nil {
		// Try by database ID as fallback
		query = `SELECT imdb_id, metadata FROM library_movies WHERE id = $1`
		err = h.db.QueryRow(query, tmdbID).Scan(&imdbID, &metadataJSON)
		if err != nil {
			http.Error(w, "Movie not found", http.StatusNotFound)
			return
		}
	}
	
	// If imdb_id column is empty, try to get from metadata JSON
	if !imdbID.Valid || imdbID.String == "" {
		var metadata map[string]interface{}
		if json.Unmarshal(metadataJSON, &metadata) == nil {
			if id, ok := metadata["imdb_id"].(string); ok && id != "" {
				imdbID.String = id
				imdbID.Valid = true
			}
		}
	}
	
	if !imdbID.Valid || imdbID.String == "" {
		http.Error(w, "IMDB ID not found", http.StatusNotFound)
		return
	}
	
	log.Printf("Playing movie TMDB ID %d, IMDB ID %s", tmdbID, imdbID.String)
	
	// Get release filters and sort options from settings
	filters := h.getReleaseFilters()
	sortOpts := h.getStreamSortOptions()
	
	// Get stream from providers
	stream, err := h.multiProvider.GetBestStream(imdbID.String, nil, nil, h.cfg.MaxResolution, filters, sortOpts)
	if err != nil {
		log.Printf("Error getting stream: %v", err)
		http.Error(w, "Stream not available", http.StatusNotFound)
		return
	}
	
	// Redirect to stream URL
	if stream.URL != "" {
		log.Printf("Redirecting to stream: %s", stream.URL)
		http.Redirect(w, r, stream.URL, http.StatusFound)
	} else {
		http.Error(w, "Stream URL not available", http.StatusNotFound)
	}
}

func (h *XtreamHandler) playEpisode(w http.ResponseWriter, r *http.Request, seriesID, seasonStr, episodeStr string) {
	tmdbID, _ := strconv.ParseInt(seriesID, 10, 64)
	seasonNum, _ := strconv.Atoi(seasonStr)
	episodeNum, _ := strconv.Atoi(episodeStr)
	
	// Get IMDB ID from database by TMDB ID first - try both imdb_id column and metadata
	var imdbID sql.NullString
	var metadataJSON []byte
	
	query := `SELECT imdb_id, metadata FROM library_series WHERE tmdb_id = $1`
	err := h.db.QueryRow(query, tmdbID).Scan(&imdbID, &metadataJSON)
	if err != nil {
		// Try by database ID as fallback
		query = `SELECT imdb_id, metadata FROM library_series WHERE id = $1`
		err = h.db.QueryRow(query, tmdbID).Scan(&imdbID, &metadataJSON)
		if err != nil {
			http.Error(w, "Series not found", http.StatusNotFound)
			return
		}
	}
	
	// If imdb_id column is empty, try to get from metadata JSON
	if !imdbID.Valid || imdbID.String == "" {
		var metadata map[string]interface{}
		if json.Unmarshal(metadataJSON, &metadata) == nil {
			// Try external_ids first, then imdb_id directly
			if extIds, ok := metadata["external_ids"].(map[string]interface{}); ok {
				if id, ok := extIds["imdb_id"].(string); ok && id != "" {
					imdbID.String = id
					imdbID.Valid = true
				}
			}
			if !imdbID.Valid {
				if id, ok := metadata["imdb_id"].(string); ok && id != "" {
					imdbID.String = id
					imdbID.Valid = true
				}
			}
		}
	}
	
	if !imdbID.Valid || imdbID.String == "" {
		http.Error(w, "IMDB ID not found", http.StatusNotFound)
		return
	}
	
	log.Printf("Playing series TMDB ID %d, IMDB ID %s, S%02dE%02d", tmdbID, imdbID.String, seasonNum, episodeNum)
	
	// Get release filters and sort options from settings
	filters := h.getReleaseFilters()
	sortOpts := h.getStreamSortOptions()
	
	// Get stream from providers
	stream, err := h.multiProvider.GetBestStream(imdbID.String, &seasonNum, &episodeNum, h.cfg.MaxResolution, filters, sortOpts)
	if err != nil {
		log.Printf("Error getting stream: %v", err)
		http.Error(w, "Stream not available", http.StatusNotFound)
		return
	}
	
	// Redirect to stream URL
	if stream.URL != "" {
		log.Printf("Redirecting to stream: %s", stream.URL)
		http.Redirect(w, r, stream.URL, http.StatusFound)
	} else {
		http.Error(w, "Stream URL not available", http.StatusNotFound)
	}
}

func (h *XtreamHandler) handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	outputType := r.URL.Query().Get("type")
	if outputType == "" {
		outputType = "m3u_plus"
	}

	// Get the actual host from the request
	host := r.Host
	if host == "" {
		host = fmt.Sprintf("%s:%d", h.cfg.Host, h.cfg.ServerPort)
	}
	serverURL := fmt.Sprintf("http://%s", host)
	
	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")
	if username == "" {
		username = "user"
	}
	if password == "" {
		password = "pass"
	}

	w.Header().Set("Content-Type", "audio/x-mpegurl")
	w.Header().Set("Content-Disposition", "attachment; filename=\"playlist.m3u\"")

	fmt.Fprintf(w, "#EXTM3U\n")

	// Add VOD streams (movies) - using TMDB ID
	movieRows, err := h.db.Query(`
		SELECT tmdb_id, title, year, metadata
		FROM library_movies
		WHERE monitored = true
		ORDER BY title
	`)
	if err == nil {
		defer movieRows.Close()
		for movieRows.Next() {
			var tmdbID int64
			var title string
			var year sql.NullInt64
			var metadataJSON []byte
			if err := movieRows.Scan(&tmdbID, &title, &year, &metadataJSON); err != nil {
				continue
			}

			var metadata map[string]interface{}
			json.Unmarshal(metadataJSON, &metadata)

			logo := ""
			if poster, ok := metadata["poster_path"].(string); ok && poster != "" {
				logo = fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", poster)
			}

			yearStr := ""
			if year.Valid {
				yearStr = fmt.Sprintf(" (%d)", year.Int64)
			}

			fmt.Fprintf(w, "#EXTINF:-1 tvg-id=\"movie_%d\" tvg-name=\"%s%s\" tvg-logo=\"%s\" group-title=\"Movies\",%s%s\n",
				tmdbID, title, yearStr, logo, title, yearStr)
			fmt.Fprintf(w, "%s/movie/%s/%s/%d.mp4\n", serverURL, username, password, tmdbID)
		}
	}

	// Add Live TV
	if h.channelManager != nil {
		channels := h.channelManager.GetAllChannels()
		for i, ch := range channels {
			fmt.Fprintf(w, "#EXTINF:-1 tvg-id=\"%s\" tvg-name=\"%s\" tvg-logo=\"%s\" group-title=\"Live TV\",%s\n",
				ch.ID, ch.Name, ch.Logo, ch.Name)
			fmt.Fprintf(w, "%s/live/%s/%s/%d.m3u8\n", serverURL, username, password, i+1)
		}
	}
}
