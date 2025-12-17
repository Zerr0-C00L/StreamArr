package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// StremioManifest represents the Stremio addon manifest
type StremioManifest struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Resources   []string          `json:"resources"`
	Types       []string          `json:"types"`
	Catalogs    []StremioCatalog  `json:"catalogs,omitempty"`
	IDPrefixes  []string          `json:"idPrefixes"`
	Background  string            `json:"background,omitempty"`
	Logo        string            `json:"logo,omitempty"`
}

// StremioCatalog represents a catalog in Stremio
type StremioCatalog struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Extra []StremioCatalogExtra  `json:"extra,omitempty"`
}

// StremioCatalogExtra represents extra catalog options
type StremioCatalogExtra struct {
	Name     string   `json:"name"`
	IsRequired bool   `json:"isRequired,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// StremioStream represents a stream response for Stremio
type StremioStream struct {
	Name          string                    `json:"name,omitempty"`
	Description   string                    `json:"description,omitempty"`
	URL           string                    `json:"url"`
	InfoHash      string                    `json:"infoHash,omitempty"`
	FileIdx       int                       `json:"fileIdx,omitempty"`
	BehaviorHints StremioStreamBehaviorHints `json:"behaviorHints,omitempty"`
}

// StremioStreamBehaviorHints provides hints to Stremio about stream behavior
type StremioStreamBehaviorHints struct {
	NotWebReady bool   `json:"notWebReady,omitempty"`
	BingeGroup  string `json:"bingeGroup,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

// getStremioProxyPosterURL builds a proxy URL for poster images
func getStremioProxyPosterURL(r *http.Request, posterPath string) string {
	if posterPath == "" {
		return ""
	}
	
	// Build base URL from request
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	if host == "" {
		host = "localhost:8080"
	}
	
	// Add TMDB poster size prefix if not already present
	// posterPath comes as /filename.jpg, we need to make it /w500/filename.jpg
	if !strings.Contains(posterPath, "/w") && !strings.Contains(posterPath, "/h") && !strings.Contains(posterPath, "/original") {
		posterPath = "/w500" + posterPath
	}
	
	// URL-encode the poster path to preserve slashes and other characters
	encodedPath := strings.ReplaceAll(posterPath, "/", "%2F")
	
	// Return proxy URL
	proxyURL := fmt.Sprintf("%s://%s/stremio/poster/%s", proto, host, encodedPath)
	log.Printf("[Stremio] Generated proxy URL for %s -> %s", posterPath, proxyURL)
	return proxyURL
}

// StremioManifestHandler serves the Stremio addon manifest
func (h *Handler) StremioManifestHandler(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil {
		log.Printf("[Stremio] StremioManifestHandler: settingsManager is nil")
		respondError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}

	settings := h.settingsManager.Get()
	log.Printf("[Stremio] StremioManifestHandler: enabled=%v, token=%s, query_token=%s", settings.StremioAddon.Enabled, settings.StremioAddon.SharedToken, r.URL.Query().Get("token"))
	
	// Allow if either: addon is explicitly enabled OR a token has been generated
	if !settings.StremioAddon.Enabled && settings.StremioAddon.SharedToken == "" {
		respondError(w, http.StatusNotFound, "Stremio addon is not configured")
		return
	}

	// Validate token if provided
	token := r.URL.Query().Get("token")
	if settings.StremioAddon.SharedToken != "" && token != settings.StremioAddon.SharedToken {
		log.Printf("[Stremio] Token validation failed: expected %s, got %s", settings.StremioAddon.SharedToken, token)
		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	manifest := StremioManifest{
		ID:          "com.streamarr.addon",
		Version:     "1.0.0",
		Name:        settings.StremioAddon.AddonName,
		Description: "Stream movies and series from your StreamArr Pro library",
		Resources:   []string{"stream"},
		Types:       []string{"movie", "series"},
		IDPrefixes:  []string{"tt"},
	}

	// Add catalogs based on configuration
	if len(settings.StremioAddon.Catalogs) > 0 {
		catalogs := []StremioCatalog{}
		
		placement := settings.StremioAddon.CatalogPlacement
		extra := []StremioCatalogExtra{}
		
		// Add skip parameter for pagination
		if placement == "home" || placement == "both" {
			extra = append(extra, StremioCatalogExtra{
				Name: "skip",
			})
		}

		// Add enabled catalogs
		for _, cat := range settings.StremioAddon.Catalogs {
			if cat.Enabled {
				catalog := StremioCatalog{
					Type:  cat.Type,
					ID:    cat.ID,
					Name:  cat.Name,
					Extra: extra,
				}
				catalogs = append(catalogs, catalog)
			}
		}

		if len(catalogs) > 0 {
			manifest.Catalogs = catalogs
			// Add catalog resource
			manifest.Resources = append([]string{"catalog"}, manifest.Resources...)
		}
	}

	// Set proper headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	
	json.NewEncoder(w).Encode(manifest)
}

// StremioCatalogHandler serves catalog listings for Stremio
func (h *Handler) StremioCatalogHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	
	catalogID := vars["id"]      // catalog ID
	
	log.Printf("[Stremio] StremioCatalogHandler: catalogID=%s, token=%s", catalogID, r.URL.Query().Get("token"))
	
	if h.settingsManager == nil {
		log.Printf("[Stremio] StremioCatalogHandler: settingsManager is nil")
		respondError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}

	settings := h.settingsManager.Get()
	// Allow if either: addon is explicitly enabled OR a token has been generated
	if !settings.StremioAddon.Enabled && settings.StremioAddon.SharedToken == "" {
		log.Printf("[Stremio] StremioCatalogHandler: addon not configured (enabled=%v, token=%s)", settings.StremioAddon.Enabled, settings.StremioAddon.SharedToken)
		respondError(w, http.StatusNotFound, "catalogs not configured")
		return
	}

	// Check if this catalog is enabled
	catalogEnabled := false
	for _, cat := range settings.StremioAddon.Catalogs {
		if cat.ID == catalogID && cat.Enabled {
			catalogEnabled = true
			break
		}
	}

	log.Printf("[Stremio] StremioCatalogHandler: catalogID=%s, catalogEnabled=%v", catalogID, catalogEnabled)
	if !catalogEnabled {
		log.Printf("[Stremio] StremioCatalogHandler: catalog %s not enabled", catalogID)
		respondError(w, http.StatusNotFound, "catalog not enabled")
		return
	}

	// Validate token
	token := r.URL.Query().Get("token")
	if settings.StremioAddon.SharedToken != "" && token != settings.StremioAddon.SharedToken {
		log.Printf("[Stremio] StremioCatalogHandler: token validation failed")

		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	// Parse pagination
	skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
	limit := 100 // Stremio default

	var metas []map[string]interface{}

	// Helper function to build movie meta
	buildMovieMeta := func(movie interface{}) map[string]interface{} {
		var m struct {
			ID          int64
			Title       string
			PosterPath  string
			ReleaseDate *time.Time
			Overview    string
			Genres      []string
			Runtime     int
			Metadata    map[string]interface{}
		}
		
		// Type assertion based on actual movie type - will be models.Movie from database
		if mv, err := json.Marshal(movie); err == nil {
			json.Unmarshal(mv, &m)
		}

		// Extract poster path from metadata if not at top level
		if m.PosterPath == "" && m.Metadata != nil {
			if poster, ok := m.Metadata["poster_path"].(string); ok {
				m.PosterPath = poster
			}
		}

		// Extract overview from metadata if not at top level
		if m.Overview == "" && m.Metadata != nil {
			if overview, ok := m.Metadata["overview"].(string); ok {
				m.Overview = overview
			}
		}

		// Extract genres from metadata if not at top level
		if len(m.Genres) == 0 && m.Metadata != nil {
			if genres, ok := m.Metadata["genres"].([]interface{}); ok {
				m.Genres = make([]string, len(genres))
				for i, g := range genres {
					if gs, ok := g.(string); ok {
						m.Genres[i] = gs
					}
				}
			}
		}

		// Extract runtime from metadata if not at top level
		if m.Runtime == 0 && m.Metadata != nil {
			if runtime, ok := m.Metadata["runtime"].(float64); ok {
				m.Runtime = int(runtime)
			}
		}

		imdbID := ""
		if m.Metadata != nil {
			if imdb, ok := m.Metadata["imdb_id"].(string); ok {
				imdbID = imdb
			}
		}
		if imdbID == "" {
			return nil
		}

		// Return metadata with Cinemeta poster URL - minimal for Stremio to fetch full details
		meta := map[string]interface{}{
			"id":     imdbID,
			"type":   "movie",
			"name":   m.Title,
			"poster": fmt.Sprintf("https://images.metahub.space/poster/medium/%s/img", imdbID),
		}

		// Include year if available
		if m.ReleaseDate != nil {
			meta["releaseInfo"] = m.ReleaseDate.Format("2006")
		}
		
		// Include description
		if m.Overview != "" {
			meta["description"] = m.Overview
		}
		
		// Include genres
		if len(m.Genres) > 0 {
			meta["genres"] = m.Genres
		}
		
		// Include runtime
		if m.Runtime > 0 {
			meta["runtime"] = fmt.Sprintf("%d min", m.Runtime)
		}

		// Try to extract extended metadata from JSONB if available
		if m.Metadata != nil {
			// IMDB Rating
			if rating, ok := m.Metadata["vote_average"].(float64); ok && rating > 0 {
				meta["imdbRating"] = fmt.Sprintf("%.1f", rating)
			}
			
			// Cast
			if cast, ok := m.Metadata["cast"].([]interface{}); ok && len(cast) > 0 {
				castNames := []string{}
				for i, c := range cast {
					if i >= 5 {
						break
					}
					if castMap, ok := c.(map[string]interface{}); ok {
						if name, ok := castMap["name"].(string); ok && name != "" {
							castNames = append(castNames, name)
						}
					}
				}
				if len(castNames) > 0 {
					meta["cast"] = castNames
				}
			}
			
			// Director
			if crew, ok := m.Metadata["crew"].([]interface{}); ok && len(crew) > 0 {
				directors := []string{}
				for _, c := range crew {
					if crewMap, ok := c.(map[string]interface{}); ok {
						if job, ok := crewMap["job"].(string); ok && job == "Director" {
							if name, ok := crewMap["name"].(string); ok && name != "" {
								directors = append(directors, name)
								if len(directors) >= 3 {
									break
								}
							}
						}
					}
				}
				if len(directors) > 0 {
					meta["director"] = directors
				}
			}
		}

		return meta
	}

	// Helper function to build series meta
	buildSeriesMeta := func(series interface{}) map[string]interface{} {
		var s struct {
			ID           int64
			Title        string
			PosterPath   string
			FirstAirDate *time.Time
			Overview     string
			Genres       []string
			Metadata     map[string]interface{}
		}
		
		// Type assertion - will be models.Series from database
		if sv, err := json.Marshal(series); err == nil {
			json.Unmarshal(sv, &s)
		}

		// Extract poster path from metadata if not at top level
		if s.PosterPath == "" && s.Metadata != nil {
			if poster, ok := s.Metadata["poster_path"].(string); ok {
				s.PosterPath = poster
			}
		}

		// Extract overview from metadata if not at top level
		if s.Overview == "" && s.Metadata != nil {
			if overview, ok := s.Metadata["overview"].(string); ok {
				s.Overview = overview
			}
		}

		// Extract genres from metadata if not at top level
		if len(s.Genres) == 0 && s.Metadata != nil {
			if genres, ok := s.Metadata["genres"].([]interface{}); ok {
				s.Genres = make([]string, len(genres))
				for i, g := range genres {
					if gs, ok := g.(string); ok {
						s.Genres[i] = gs
					}
				}
			}
		}

		imdbID := ""
		if s.Metadata != nil {
			if imdb, ok := s.Metadata["imdb_id"].(string); ok {
				imdbID = imdb
			}
		}
		if imdbID == "" {
			return nil
		}

		// Return metadata with Cinemeta poster URL - minimal for Stremio to fetch full details
		meta := map[string]interface{}{
			"id":     imdbID,
			"type":   "series",
			"name":   s.Title,
			"poster": fmt.Sprintf("https://images.metahub.space/poster/medium/%s/img", imdbID),
		}

		// Include year if available
		if s.FirstAirDate != nil {
			meta["releaseInfo"] = s.FirstAirDate.Format("2006")
		}
		
		// Include description
		if s.Overview != "" {
			meta["description"] = s.Overview
		}
		
		// Include genres
		if len(s.Genres) > 0 {
			meta["genres"] = s.Genres
		}

		// Try to extract extended metadata from JSONB if available
		if s.Metadata != nil {
			// IMDB Rating
			if rating, ok := s.Metadata["vote_average"].(float64); ok && rating > 0 {
				meta["imdbRating"] = fmt.Sprintf("%.1f", rating)
			}
			
			// Cast
			if cast, ok := s.Metadata["cast"].([]interface{}); ok && len(cast) > 0 {
				castNames := []string{}
				for i, c := range cast {
					if i >= 5 {
						break
					}
					if castMap, ok := c.(map[string]interface{}); ok {
						if name, ok := castMap["name"].(string); ok && name != "" {
							castNames = append(castNames, name)
						}
					}
				}
				if len(castNames) > 0 {
					meta["cast"] = castNames
				}
			}
			
			// Creator (for series)
			if creators, ok := s.Metadata["created_by"].([]interface{}); ok && len(creators) > 0 {
				creatorNames := []string{}
				for i, c := range creators {
					if i >= 3 {
						break
					}
					if creatorMap, ok := c.(map[string]interface{}); ok {
						if name, ok := creatorMap["name"].(string); ok && name != "" {
							creatorNames = append(creatorNames, name)
						}
					}
				}
				if len(creatorNames) > 0 {
					meta["director"] = creatorNames
				}
			}
		}

		return meta
	}

	// Handle different catalog types
	switch catalogID {
	case "streamarr_recent", "streamarr_recent_movies":
		// Recently added movies
		movies, err := h.movieStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch movies: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, movie := range movies {
			if meta := buildMovieMeta(movie); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_recent_series":
		// Recently added series
		series, err := h.seriesStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch series: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, s := range series {
			if meta := buildSeriesMeta(s); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_trending", "streamarr_trending_movies":
		// Trending movies (use TMDB trending)
		// For now, return library movies - can be enhanced with TMDB API
		movies, err := h.movieStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch movies: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, movie := range movies {
			if meta := buildMovieMeta(movie); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_trending_series":
		// Trending series
		series, err := h.seriesStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch series: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, s := range series {
			if meta := buildSeriesMeta(s); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_popular", "streamarr_popular_movies":
		// Popular movies
		movies, err := h.movieStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch movies: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, movie := range movies {
			if meta := buildMovieMeta(movie); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_popular_series":
		// Popular series
		series, err := h.seriesStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch series: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, s := range series {
			if meta := buildSeriesMeta(s); meta != nil {
				metas = append(metas, meta)
			}
		}

	case "streamarr_coming_soon":
		// Coming soon (future releases)
		movies, err := h.movieStore.List(ctx, skip, limit, nil)
		if err != nil {
			log.Printf("Failed to fetch movies: %v", err)
			respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
			return
		}
		for _, movie := range movies {
			if meta := buildMovieMeta(movie); meta != nil {
				metas = append(metas, meta)
			}
		}

	default:
		// Unknown catalog
		respondJSON(w, http.StatusOK, map[string]interface{}{"metas": []interface{}{}})
		return
	}

	log.Printf("[Stremio] StremioCatalogHandler: returning %d metas for catalog %s", len(metas), catalogID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"metas": metas,
	})
}

// StremioStreamHandler serves streams for Stremio
func (h *Handler) StremioStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	
	contentType := vars["type"] // movie or series
	id := vars["id"]            // IMDB ID (tt123456 or tt123456:1:2 for series)
	
	if h.settingsManager == nil {
		respondError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}

	settings := h.settingsManager.Get()
	// Allow if either: addon is explicitly enabled OR a token has been generated
	if !settings.StremioAddon.Enabled && settings.StremioAddon.SharedToken == "" {
		respondError(w, http.StatusNotFound, "Stremio addon is not configured")
		return
	}

	// Validate token
	token := r.URL.Query().Get("token")
	if settings.StremioAddon.SharedToken != "" && token != settings.StremioAddon.SharedToken {
		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	// Parse ID
	parts := strings.Split(id, ":")
	imdbID := parts[0]

	if h.streamProvider == nil {
		log.Printf("[Stremio] Stream provider not configured, returning empty streams")
		respondJSON(w, http.StatusOK, map[string]interface{}{"streams": []interface{}{}})
		return
	}

	var providerStreams []interface{}
	var err error

	if contentType == "movie" {
		log.Printf("[Stremio] Fetching streams for movie %s", imdbID)
		streams, streamErr := h.streamProvider.GetMovieStreams(imdbID)
		err = streamErr
		if err != nil {
			log.Printf("[Stremio] GetMovieStreams error: %v", err)
		} else {
			log.Printf("[Stremio] GetMovieStreams returned %d streams", len(streams))
		}
		if err == nil && len(streams) > 0 {
			// Apply release filters
			streams = h.applyReleaseFilters(streams)
			sortStreams(streams)
			
			for _, ps := range streams {
				stream := StremioStream{
					Name:        fmt.Sprintf("StreamArr - %s", ps.Quality),
					Description: fmt.Sprintf("%s • %.2f GB", ps.Source, float64(ps.Size)/(1024*1024*1024)),
					URL:         ps.URL,
					BehaviorHints: StremioStreamBehaviorHints{
						NotWebReady: false,
						Filename:    ps.Title,
					},
				}
				
				if ps.Cached {
					stream.Name += " ⚡"
				}
				
				providerStreams = append(providerStreams, stream)
			}
		}
	} else if contentType == "series" && len(parts) == 3 {
		season, _ := strconv.Atoi(parts[1])
		episode, _ := strconv.Atoi(parts[2])
		
		log.Printf("[Stremio] Fetching streams for series %s S%02dE%02d", imdbID, season, episode)
		streams, streamErr := h.streamProvider.GetSeriesStreams(imdbID, season, episode)
		err = streamErr
		if err != nil {
			log.Printf("[Stremio] GetSeriesStreams error: %v", err)
		} else {
			log.Printf("[Stremio] GetSeriesStreams returned %d streams", len(streams))
		}
		if err == nil && len(streams) > 0 {
			// Apply release filters
			streams = h.applyReleaseFilters(streams)
			sortStreams(streams)
			
			for _, ps := range streams {
				stream := StremioStream{
					Name:        fmt.Sprintf("StreamArr - %s", ps.Quality),
					Description: fmt.Sprintf("%s • %.2f GB", ps.Source, float64(ps.Size)/(1024*1024*1024)),
					URL:         ps.URL,
					BehaviorHints: StremioStreamBehaviorHints{
						NotWebReady: false,
						Filename:    ps.Title,
						BingeGroup:  fmt.Sprintf("streamarr-%s", imdbID),
					},
				}
				
				if ps.Cached {
					stream.Name += " ⚡"
				}
				
				providerStreams = append(providerStreams, stream)
			}
		}
	}

	if err != nil {
		log.Printf("[Stremio] Failed to get streams: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"streams": providerStreams,
	})
}

// GenerateStremioToken generates a new random token for Stremio addon access
func (h *Handler) GenerateStremioToken(w http.ResponseWriter, r *http.Request) {
	// Generate a secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	
	token := hex.EncodeToString(tokenBytes)
	
	// Update settings with new token
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		settings.StremioAddon.SharedToken = token
		if err := h.settingsManager.Update(settings); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save token")
			return
		}
	}
	
	respondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// GetStremioManifestURL returns the full manifest URL for copying
func (h *Handler) GetStremioManifestURL(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil {
		log.Printf("[Stremio] GetStremioManifestURL: settingsManager is nil")
		respondError(w, http.StatusServiceUnavailable, "settings not configured")
		return
	}

	settings := h.settingsManager.Get()
	log.Printf("[Stremio] GetStremioManifestURL: enabled=%v, token=%s, addon=%+v", settings.StremioAddon.Enabled, settings.StremioAddon.SharedToken, settings.StremioAddon)
	
	// Allow if either: addon is explicitly enabled OR a token has been generated
	if !settings.StremioAddon.Enabled && settings.StremioAddon.SharedToken == "" {
		respondError(w, http.StatusBadRequest, "Stremio addon is not configured - generate a token first")
		return
	}

	// Determine base URL: use PublicServerURL if set, otherwise auto-detect from request
	baseURL := settings.StremioAddon.PublicServerURL
	if baseURL == "" {
		// Auto-detect from request host
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := r.Host
		baseURL = fmt.Sprintf("%s://%s", scheme, host)
		log.Printf("[Stremio] Auto-detected base URL: %s", baseURL)
	} else {
		log.Printf("[Stremio] Using configured base URL: %s", baseURL)
		// Ensure proper scheme if not provided
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "http://" + baseURL
		}
	}

	// Build manifest URL
	manifestURL := fmt.Sprintf("%s/stremio/manifest.json", strings.TrimRight(baseURL, "/"))
	if settings.StremioAddon.SharedToken != "" {
		manifestURL += "?token=" + settings.StremioAddon.SharedToken
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"manifest_url": manifestURL,
		"install_url":  manifestURL,
		"server_url":   baseURL,
	})
}

// StremioPostersProxyHandler proxies poster images through the server
// This helps with CORS and caching in Stremio
func (h *Handler) StremioPostersProxyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	posterPath := vars["path"]
	
	// URL-decode the poster path (slashes were encoded as %2F)
	posterPath = strings.ReplaceAll(posterPath, "%2F", "/")
	
	log.Printf("[Stremio] StremioPostersProxyHandler: proxying poster %s", posterPath)
	
	// Construct TMDB poster URL
	tmdbURL := fmt.Sprintf("https://image.tmdb.org/t/p/%s", posterPath)
	
	// Fetch the poster from TMDB
	resp, err := http.Get(tmdbURL)
	if err != nil {
		log.Printf("[Stremio] Failed to fetch poster from TMDB: %v", err)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Failed to fetch poster"))
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Stremio] TMDB returned status %d for poster %s", resp.StatusCode, posterPath)
		w.WriteHeader(resp.StatusCode)
		return
	}
	
	// Copy response headers
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	
	// Copy status and body
	w.WriteHeader(http.StatusOK)
	_, err = io.CopyN(w, resp.Body, 50*1024*1024) // 50MB max
	if err != nil && err != io.EOF {
		log.Printf("[Stremio] Failed to write poster response: %v", err)
	}
}
