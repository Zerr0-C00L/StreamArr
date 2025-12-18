package api

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/epg"
	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
	"github.com/gorilla/mux"
)

type Handler struct {
	movieStore      *database.MovieStore
	seriesStore     *database.SeriesStore
	episodeStore    *database.EpisodeStore
	streamStore     *database.StreamStore
	settingsStore   *database.SettingsStore
	userStore       *database.UserStore
	collectionStore *database.CollectionStore
	tmdbClient      *services.TMDBClient
	rdClient        *services.RealDebridClient
	channelManager  *livetv.ChannelManager
	settingsManager *settings.Manager
	epgManager      *epg.Manager
	streamProvider  *providers.MultiProvider
}

func NewHandler(
	movieStore *database.MovieStore,
	seriesStore *database.SeriesStore,
	episodeStore *database.EpisodeStore,
	streamStore *database.StreamStore,
	settingsStore *database.SettingsStore,
	userStore *database.UserStore,
	collectionStore *database.CollectionStore,
	tmdbClient *services.TMDBClient,
	rdClient *services.RealDebridClient,
) *Handler {
	return &Handler{
		movieStore:      movieStore,
		seriesStore:     seriesStore,
		episodeStore:    episodeStore,
		streamStore:     streamStore,
		settingsStore:   settingsStore,
		collectionStore: collectionStore,
		tmdbClient:      tmdbClient,
		rdClient:        rdClient,
	}
}

func NewHandlerWithComponents(
	movieStore *database.MovieStore,
	seriesStore *database.SeriesStore,
	episodeStore *database.EpisodeStore,
	streamStore *database.StreamStore,
	settingsStore *database.SettingsStore,
	userStore *database.UserStore,
	collectionStore *database.CollectionStore,
	tmdbClient *services.TMDBClient,
	rdClient *services.RealDebridClient,
	channelManager *livetv.ChannelManager,
	settingsManager *settings.Manager,
	epgManager *epg.Manager,
	streamProvider *providers.MultiProvider,
) *Handler {
	return &Handler{
		movieStore:      movieStore,
		seriesStore:     seriesStore,
		episodeStore:    episodeStore,
		streamStore:     streamStore,
		settingsStore:   settingsStore,
		userStore:       userStore,
		collectionStore: collectionStore,
		tmdbClient:      tmdbClient,
		rdClient:        rdClient,
		channelManager:  channelManager,
		settingsManager: settingsManager,
		epgManager:      epgManager,
		streamProvider:  streamProvider,
	}
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// applyReleaseFilters filters out streams that match the release filter patterns
func (h *Handler) applyReleaseFilters(streams []providers.TorrentioStream) []providers.TorrentioStream {
	if h.settingsManager == nil {
		return streams
	}

	settings := h.settingsManager.Get()
	if !settings.EnableReleaseFilters {
		return streams
	}

	// Build exclude patterns
	patterns := make([]string, 0)
	if settings.ExcludedQualities != "" {
		patterns = append(patterns, settings.ExcludedQualities)
	}
	if settings.ExcludedReleaseGroups != "" {
		patterns = append(patterns, settings.ExcludedReleaseGroups)
	}
	if settings.ExcludedLanguageTags != "" {
		patterns = append(patterns, settings.ExcludedLanguageTags)
	}

	if len(patterns) == 0 {
		return streams
	}

	// Build regex pattern that matches terms separated by dots, spaces, dashes, etc.
	combinedPattern := `(?i)(?:^|[\s.\-_\[\]()])(` + strings.Join(patterns, "|") + `)(?:$|[\s.\-_\[\]()])`
	excludePattern, err := regexp.Compile(combinedPattern)
	if err != nil {
		log.Printf("Invalid release filter pattern: %v", err)
		return streams
	}

	// Filter streams
	filtered := make([]providers.TorrentioStream, 0)
	for _, s := range streams {
		checkStr := s.Name + " " + s.Title
		if excludePattern.MatchString(checkStr) {
			log.Printf("Filtered out stream (release filter): %s", s.Title)
			continue
		}
		filtered = append(filtered, s)
	}

	log.Printf("Release filter: %d streams -> %d streams (filtered %d)", len(streams), len(filtered), len(streams)-len(filtered))
	return filtered
}

// sortStreams sorts streams by cached status first, then by size (larger first)
func sortStreams(streams []providers.TorrentioStream) {
	sort.Slice(streams, func(i, j int) bool {
		// First: sort by cached status (cached first)
		if streams[i].Cached != streams[j].Cached {
			return streams[i].Cached
		}

		// Second: sort by size (larger files = better quality)
		return streams[i].Size > streams[j].Size
	})
}

// qualityToInt converts quality string to integer for sorting (Unknown = 0)
func qualityToInt(quality string) int {
	q := strings.ToUpper(quality)
	if strings.Contains(q, "4K") || strings.Contains(q, "2160") {
		return 2160
	}
	if strings.Contains(q, "1080") {
		return 1080
	}
	if strings.Contains(q, "720") {
		return 720
	}
	if strings.Contains(q, "480") {
		return 480
	}
	if strings.Contains(q, "360") {
		return 360
	}
	// Unknown quality goes last
	return 0
}

// ListMovies handles GET /api/movies
func (h *Handler) ListMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	var monitored *bool
	if m := r.URL.Query().Get("monitored"); m != "" {
		val := m == "true"
		monitored = &val
	}

	movies, err := h.movieStore.List(ctx, offset, limit, monitored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, movies)
}

// GetMovie handles GET /api/movies/{id}
func (h *Handler) GetMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	movie, err := h.movieStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "movie not found")
		return
	}

	respondJSON(w, http.StatusOK, movie)
}

// AddMovie handles POST /api/movies
func (h *Handler) AddMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		TMDBID         int    `json:"tmdb_id"`
		Monitored      bool   `json:"monitored"`
		QualityProfile string `json:"quality_profile"`
		AddCollection  *bool  `json:"add_collection,omitempty"` // Override for auto-add collections
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if movie already exists in library
	existingMovie, err := h.movieStore.GetByTMDBID(ctx, req.TMDBID)
	if err == nil && existingMovie != nil {
		// Movie already exists, return it with 200 OK
		respondJSON(w, http.StatusOK, existingMovie)
		return
	}

	// Fetch movie details from TMDB with collection info
	movie, collection, err := h.tmdbClient.GetMovieWithCollection(ctx, req.TMDBID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to fetch movie from TMDB: %v", err))
		return
	}

	movie.Monitored = req.Monitored
	movie.QualityProfile = req.QualityProfile

	// Handle collection if present
	var collectionID *int64
	if collection != nil && h.collectionStore != nil {
		// Check if collection already exists in our database
		existingCollection, _ := h.collectionStore.GetByTMDBID(ctx, collection.TMDBID)
		if existingCollection != nil {
			collectionID = &existingCollection.ID
		} else {
			// Fetch full collection details from TMDB
			fullCollection, _, err := h.tmdbClient.GetCollection(ctx, collection.TMDBID)
			if err == nil {
				// Create collection in database
				if err := h.collectionStore.Create(ctx, fullCollection); err == nil {
					collectionID = &fullCollection.ID
				}
			}
		}
		movie.CollectionID = collectionID
	}

	// Add to database
	if err := h.movieStore.Add(ctx, movie); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add movie")
		return
	}

	// Handle auto-add collection setting
	shouldAddCollection := false
	if req.AddCollection != nil {
		shouldAddCollection = *req.AddCollection
	} else if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		shouldAddCollection = settings.AutoAddCollections
	}

	// If auto-add collection is enabled and movie belongs to a collection
	if shouldAddCollection && collection != nil && h.collectionStore != nil {
		if isIPTVVODMovie(movie) {
			log.Printf("[Collection Sync] Skipping auto-add for IPTV VOD movie %s", movie.Title)
		} else {
			go h.addCollectionMovies(ctx, collection.TMDBID, req.Monitored, req.QualityProfile)
		}
	}

	// Load collection data for response
	if collectionID != nil && h.collectionStore != nil {
		movie.Collection, _ = h.collectionStore.GetByID(ctx, *collectionID)
	}

	respondJSON(w, http.StatusCreated, movie)
}

// addCollectionMovies adds all movies from a collection in the background
func (h *Handler) addCollectionMovies(ctx context.Context, collectionTMDBID int, monitored bool, qualityProfile string) {
	// Get collection details with all movie IDs
	collection, movieIDs, err := h.tmdbClient.GetCollection(ctx, collectionTMDBID)
	if err != nil {
		fmt.Printf("[Collection Sync] Failed to get collection %d: %v\n", collectionTMDBID, err)
		return
	}

	fmt.Printf("[Collection Sync] Adding missing movies from '%s' (%d movies total)\n", collection.Name, len(movieIDs))
	added := 0

	for _, tmdbID := range movieIDs {
		// Check if movie already exists
		existing, _ := h.movieStore.GetByTMDBID(ctx, tmdbID)
		if existing != nil {
			continue
		}

		// Fetch and add movie
		movie, coll, err := h.tmdbClient.GetMovieWithCollection(ctx, tmdbID)
		if err != nil {
			continue
		}

		movie.Monitored = monitored
		movie.QualityProfile = qualityProfile

		// Link to collection
		if coll != nil && h.collectionStore != nil {
			existingColl, _ := h.collectionStore.GetByTMDBID(ctx, coll.TMDBID)
			if existingColl != nil {
				movie.CollectionID = &existingColl.ID
			}
		}

		if err := h.movieStore.Add(ctx, movie); err != nil {
			fmt.Printf("[Collection Sync] Failed to add movie '%s': %v\n", movie.Title, err)
		} else {
			added++
			fmt.Printf("[Collection Sync] Added '%s' from '%s'\n", movie.Title, collection.Name)
		}

		// Small delay to avoid hitting API rate limits
		time.Sleep(250 * time.Millisecond)
	}

	fmt.Printf("[Collection Sync] Finished '%s': %d new movies added\n", collection.Name, added)
}

// scanAndLinkCollections scans all movies without a collection and checks if they belong to one
func (h *Handler) scanAndLinkCollections(ctx context.Context) error {
	// Get only movies that haven't been checked yet
	movies, err := h.movieStore.ListUncheckedForCollection(ctx)
	if err != nil {
		fmt.Printf("[Collection Sync] Failed to list unchecked movies: %v\n", err)
		return err
	}

	totalMovies := len(movies)
	if totalMovies == 0 {
		fmt.Println("[Collection Sync] All movies have been checked for collections")
		services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, 0, 0, "All movies already checked")
		return nil
	}

	fmt.Printf("[Collection Sync] Starting scan of %d unchecked movies...\n", totalMovies)
	services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, 0, totalMovies, "Scanning movies for collections...")

	linked := 0
	noCollection := 0
	errors := 0

	for i, movie := range movies {
		// Update progress
		services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, i+1, totalMovies,
			fmt.Sprintf("Checking: %s", movie.Title))

		// Fetch movie with collection info from TMDB
		_, collection, err := h.tmdbClient.GetMovieWithCollection(ctx, movie.TMDBID)
		if err != nil {
			errors++
			// Still mark as checked so we don't retry failed ones endlessly
			h.movieStore.MarkCollectionChecked(ctx, movie.ID)
			continue
		}

		// If movie belongs to a collection, create/update collection and link
		if collection != nil {
			// Get full collection details
			fullCollection, _, err := h.tmdbClient.GetCollection(ctx, collection.TMDBID)
			if err != nil {
				fmt.Printf("[Collection Sync] Failed to get collection %d for movie %s: %v\n", collection.TMDBID, movie.Title, err)
				errors++
				h.movieStore.MarkCollectionChecked(ctx, movie.ID)
				continue
			}

			// Create or update collection in database
			if err := h.collectionStore.Create(ctx, fullCollection); err != nil {
				fmt.Printf("[Collection Sync] Failed to create collection %s: %v\n", fullCollection.Name, err)
				errors++
				h.movieStore.MarkCollectionChecked(ctx, movie.ID)
				continue
			}

			// Link movie to collection - fullCollection.ID is populated by Create's RETURNING clause
			if err := h.collectionStore.UpdateMovieCollection(ctx, movie.ID, fullCollection.ID); err != nil {
				fmt.Printf("[Collection Sync] Failed to link movie %s (ID:%d) to collection %s (ID:%d): %v\n",
					movie.Title, movie.ID, fullCollection.Name, fullCollection.ID, err)
				errors++
				h.movieStore.MarkCollectionChecked(ctx, movie.ID)
				continue
			}

			linked++
			fmt.Printf("[Collection Sync] Linked '%s' to '%s'\n", movie.Title, fullCollection.Name)

			// Update progress message with linked info
			services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, i+1, totalMovies,
				fmt.Sprintf("Linked: %s → %s", movie.Title, fullCollection.Name))
		} else {
			noCollection++
		}

		// Mark as checked regardless of whether it has a collection
		h.movieStore.MarkCollectionChecked(ctx, movie.ID)

		// Progress log every 100 movies
		if (i+1)%100 == 0 {
			fmt.Printf("[Collection Sync] Progress: %d/%d movies processed, %d linked, %d no collection, %d errors\n",
				i+1, totalMovies, linked, noCollection, errors)
		}

		// Rate limit
		time.Sleep(200 * time.Millisecond)
	}

	services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, totalMovies, totalMovies,
		fmt.Sprintf("Complete: %d linked, %d no collection, %d errors", linked, noCollection, errors))

	fmt.Printf("[Collection Sync] Complete: %d movies linked, %d have no collection, %d errors\n",
		linked, noCollection, errors)
	return nil
}

// scanEpisodesForAllSeries fetches episode metadata from TMDB for all series in the library
func (h *Handler) scanEpisodesForAllSeries(ctx context.Context) error {
	fmt.Println("[Episode Scan] Starting episode scan for all series...")
	services.GlobalScheduler.UpdateProgress(services.ServiceEpisodeScan, 0, 0, "Loading series list...")

	// Get all series from the library
	allSeries, err := h.seriesStore.List(ctx, 0, 10000, nil)
	if err != nil {
		fmt.Printf("[Episode Scan] Failed to list series: %v\n", err)
		return err
	}

	totalSeries := len(allSeries)
	if totalSeries == 0 {
		fmt.Println("[Episode Scan] No series in library")
		services.GlobalScheduler.UpdateProgress(services.ServiceEpisodeScan, 0, 0, "No series in library")
		return nil
	}

	fmt.Printf("[Episode Scan] Found %d series to scan\n", totalSeries)
	services.GlobalScheduler.UpdateProgress(services.ServiceEpisodeScan, 0, totalSeries,
		fmt.Sprintf("Scanning %d series...", totalSeries))

	totalEpisodes := 0
	seriesProcessed := 0
	errors := 0

	for i, series := range allSeries {
		// Update progress
		services.GlobalScheduler.UpdateProgress(services.ServiceEpisodeScan, i+1, totalSeries,
			fmt.Sprintf("Scanning: %s", series.Title))

		// Get series details from TMDB to get number of seasons
		tmdbSeries, err := h.tmdbClient.GetSeries(ctx, series.TMDBID)
		if err != nil {
			fmt.Printf("[Episode Scan] Failed to get details for '%s' (TMDB:%d): %v\n", series.Title, series.TMDBID, err)
			errors++
			continue
		}

		numSeasons := tmdbSeries.Seasons
		if numSeasons == 0 {
			fmt.Printf("[Episode Scan] '%s' has 0 seasons, skipping\n", series.Title)
			continue
		}

		// Get all episodes for this series
		episodes, err := h.tmdbClient.GetEpisodes(ctx, series.ID, series.TMDBID, numSeasons)
		if err != nil {
			fmt.Printf("[Episode Scan] Failed to get episodes for '%s': %v\n", series.Title, err)
			errors++
			continue
		}

		// Set the series ID for all episodes
		for _, ep := range episodes {
			ep.SeriesID = series.ID
			ep.Monitored = series.Monitored
		}

		// Add episodes to database (batch insert)
		if len(episodes) > 0 {
			if err := h.episodeStore.AddBatch(ctx, episodes); err != nil {
				// Check if it's a duplicate error, if so, skip silently
				if err.Error() != "" && !containsDuplicateError(err.Error()) {
					fmt.Printf("[Episode Scan] Failed to add episodes for '%s': %v\n", series.Title, err)
					errors++
				}
			} else {
				totalEpisodes += len(episodes)
				fmt.Printf("[Episode Scan] Added %d episodes for '%s'\n", len(episodes), series.Title)
			}
		}

		seriesProcessed++

		// Rate limit TMDB requests (40 per 10s limit)
		time.Sleep(300 * time.Millisecond)
	}

	services.GlobalScheduler.UpdateProgress(services.ServiceEpisodeScan, totalSeries, totalSeries,
		fmt.Sprintf("Complete: %d episodes from %d series", totalEpisodes, seriesProcessed))

	fmt.Printf("[Episode Scan] Complete: %d episodes added from %d series (%d errors)\n",
		totalEpisodes, seriesProcessed, errors)
	return nil
}

// scanStreamAvailability checks stream availability for movies and updates the 'available' column
func (h *Handler) scanStreamAvailability(ctx context.Context) error {
	fmt.Println("[Stream Scan] Starting stream availability scan...")
	services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, 0, 0, "Loading movies to scan...")

	// Get movies that haven't been checked recently (or never checked)
	// We'll check movies that were added in the last 30 days OR haven't been checked in 7 days
	query := `
		SELECT id, tmdb_id, imdb_id, title 
		FROM library_movies 
		WHERE monitored = true 
		AND (last_checked IS NULL OR last_checked < NOW() - INTERVAL '7 days')
		ORDER BY added_at DESC
		LIMIT 100
	`

	rows, err := h.movieStore.GetDB().QueryContext(ctx, query)
	if err != nil {
		fmt.Printf("[Stream Scan] Failed to query movies: %v\n", err)
		return err
	}
	defer rows.Close()

	type movieToScan struct {
		ID     int64
		TMDBID int
		IMDBID string
		Title  string
	}

	var movies []movieToScan
	for rows.Next() {
		var m movieToScan
		var imdbID sql.NullString
		if err := rows.Scan(&m.ID, &m.TMDBID, &imdbID, &m.Title); err != nil {
			continue
		}
		m.IMDBID = imdbID.String
		if m.IMDBID != "" {
			movies = append(movies, m)
		}
	}

	total := len(movies)
	if total == 0 {
		fmt.Println("[Stream Scan] No movies to scan")
		services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, 0, 0, "No movies to scan")
		return nil
	}

	fmt.Printf("[Stream Scan] Found %d movies to check\n", total)
	services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, 0, total,
		fmt.Sprintf("Checking %d movies...", total))

	// Get settings for providers
	appSettings := h.settingsManager.Get()
	stremioAddons := appSettings.StremioAddons
	if len(stremioAddons) == 0 {
		fmt.Println("[Stream Scan] No Stremio addons configured")
		services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, 0, 0, "No addons configured")
		return fmt.Errorf("no Stremio addons configured")
	}

	// Create a simple stream checker
	available := 0
	unavailable := 0

	for i, movie := range movies {
		services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, i+1, total,
			fmt.Sprintf("Checking: %s", movie.Title))

		hasStreams := false

		// Check each enabled addon for streams
		for _, addon := range stremioAddons {
			if !addon.Enabled {
				continue
			}

			// Quick check using generic addon endpoint
			url := fmt.Sprintf("%s/stream/movie/%s.json", addon.URL, movie.IMDBID)
			resp, err := http.Get(url)
			if err == nil {
				defer resp.Body.Close()
				var result struct {
					Streams []interface{} `json:"streams"`
				}
				if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.Streams) > 0 {
					hasStreams = true
					break
				}
			}

			if hasStreams {
				break // Found streams, no need to check other addons
			}
		}

		// Update the database
		updateQuery := `UPDATE library_movies SET available = $1, last_checked = NOW() WHERE id = $2`
		h.movieStore.GetDB().ExecContext(ctx, updateQuery, hasStreams, movie.ID)

		if hasStreams {
			available++
		} else {
			unavailable++
			fmt.Printf("[Stream Scan] No streams for: %s (%s)\n", movie.Title, movie.IMDBID)
		}

		// Rate limit - don't hammer the providers
		time.Sleep(500 * time.Millisecond)
	}

	services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, total, total,
		fmt.Sprintf("Movies: %d available, %d unavailable", available, unavailable))

	fmt.Printf("[Stream Scan] Movies complete: %d available, %d unavailable out of %d\n",
		available, unavailable, total)

	// ========== Phase 2: Scan Episodes ==========
	fmt.Println("[Stream Scan] Starting episode availability scan...")

	// Get episodes that need checking, joined with series to get IMDB ID
	episodeQuery := `
		SELECT e.id, e.series_id, e.season_number, e.episode_number, e.title, s.imdb_id, s.title as series_title
		FROM library_episodes e
		JOIN library_series s ON e.series_id = s.id
		WHERE e.monitored = true 
		AND s.imdb_id IS NOT NULL AND s.imdb_id != ''
		AND (e.last_checked IS NULL OR e.last_checked < NOW() - INTERVAL '7 days')
		AND e.air_date IS NOT NULL AND e.air_date < NOW()
		ORDER BY e.air_date DESC
		LIMIT 200
	`

	episodeRows, err := h.movieStore.GetDB().QueryContext(ctx, episodeQuery)
	if err != nil {
		fmt.Printf("[Stream Scan] Failed to query episodes: %v\n", err)
		return err
	}
	defer episodeRows.Close()

	type episodeToScan struct {
		ID            int64
		SeriesID      int64
		SeasonNumber  int
		EpisodeNumber int
		Title         string
		SeriesIMDBID  string
		SeriesTitle   string
	}

	var episodes []episodeToScan
	for episodeRows.Next() {
		var ep episodeToScan
		var imdbID sql.NullString
		var epTitle sql.NullString
		if err := episodeRows.Scan(&ep.ID, &ep.SeriesID, &ep.SeasonNumber, &ep.EpisodeNumber, &epTitle, &imdbID, &ep.SeriesTitle); err != nil {
			continue
		}
		ep.SeriesIMDBID = imdbID.String
		ep.Title = epTitle.String
		if ep.SeriesIMDBID != "" {
			episodes = append(episodes, ep)
		}
	}

	episodeTotal := len(episodes)
	if episodeTotal == 0 {
		fmt.Println("[Stream Scan] No episodes to scan")
		return nil
	}

	fmt.Printf("[Stream Scan] Found %d episodes to check\n", episodeTotal)
	services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, 0, episodeTotal,
		fmt.Sprintf("Checking %d episodes...", episodeTotal))

	epAvailable := 0
	epUnavailable := 0

	for i, ep := range episodes {
		displayTitle := fmt.Sprintf("%s S%02dE%02d", ep.SeriesTitle, ep.SeasonNumber, ep.EpisodeNumber)
		services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, i+1, episodeTotal,
			fmt.Sprintf("Checking: %s", displayTitle))

		hasStreams := false

		// Build the episode stream ID format: tt1234567:1:5 (imdb:season:episode)
		episodeStreamID := fmt.Sprintf("%s:%d:%d", ep.SeriesIMDBID, ep.SeasonNumber, ep.EpisodeNumber)

		// Check each enabled addon for streams
		for _, addon := range stremioAddons {
			if !addon.Enabled {
				continue
			}

			url := fmt.Sprintf("%s/stream/series/%s.json", addon.URL, episodeStreamID)
			resp, err := http.Get(url)
			if err == nil {
				var result struct {
					Streams []interface{} `json:"streams"`
				}
				if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.Streams) > 0 {
					hasStreams = true
				}
				resp.Body.Close()
				break
			}
		}

		// Update the episode
		updateEpisodeQuery := `UPDATE library_episodes SET available = $1, last_checked = NOW() WHERE id = $2 AND series_id = $3`
		h.movieStore.GetDB().ExecContext(ctx, updateEpisodeQuery, hasStreams, ep.ID, ep.SeriesID)

		if hasStreams {
			epAvailable++
		} else {
			epUnavailable++
			fmt.Printf("[Stream Scan] No streams for: %s (%s)\n", displayTitle, episodeStreamID)
		}

		// Rate limit
		time.Sleep(300 * time.Millisecond)
	}

	services.GlobalScheduler.UpdateProgress(services.ServiceStreamSearch, episodeTotal, episodeTotal,
		fmt.Sprintf("Episodes: %d available, %d unavailable", epAvailable, epUnavailable))

	fmt.Printf("[Stream Scan] Episodes complete: %d available, %d unavailable out of %d\n",
		epAvailable, epUnavailable, episodeTotal)

	return nil
}

// containsDuplicateError checks if an error message indicates a duplicate key violation
func containsDuplicateError(errMsg string) bool {
	return strings.Contains(errMsg, "duplicate key") || strings.Contains(errMsg, "UNIQUE constraint")
}

// UpdateMovie handles PUT /api/movies/{id}
func (h *Handler) UpdateMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	movie, err := h.movieStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "movie not found")
		return
	}

	var updates struct {
		Monitored      *bool   `json:"monitored"`
		QualityProfile *string `json:"quality_profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if updates.Monitored != nil {
		movie.Monitored = *updates.Monitored
	}
	if updates.QualityProfile != nil {
		movie.QualityProfile = *updates.QualityProfile
	}

	if err := h.movieStore.Update(ctx, movie); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update movie")
		return
	}

	respondJSON(w, http.StatusOK, movie)
}

// DeleteMovie handles DELETE /api/movies/{id}
func (h *Handler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	if err := h.movieStore.Delete(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete movie")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "movie deleted"})
}

// SearchMovies handles GET /api/search/movies
func (h *Handler) SearchMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "search query required")
		return
	}

	movies, err := h.tmdbClient.SearchMovies(ctx, query, 1)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to search movies")
		return
	}

	respondJSON(w, http.StatusOK, movies)
}

// GetMovieStreams handles GET /api/movies/{id}/streams
func (h *Handler) GetMovieStreams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	// Get movie from database
	movie, err := h.movieStore.Get(ctx, id)
	if err != nil {
		log.Printf("Movie not found: %d, error: %v", id, err)
		respondError(w, http.StatusNotFound, "movie not found")
		return
	}

	// Get IMDB ID from metadata
	var imdbID string
	if movie.Metadata != nil {
		if imdb, ok := movie.Metadata["imdb_id"].(string); ok {
			imdbID = imdb
		}
	}

	// If this movie came from IPTV VOD import, expose only VOD playlist links
	if isIPTVVODMovie(movie) {
		apiStreams := buildIPTVVODStreams(movie)
		log.Printf("Returning %d IPTV VOD streams for movie %d (%s)", len(apiStreams), id, movie.Title)
		respondJSON(w, http.StatusOK, apiStreams)
		return
	}

	if imdbID == "" {
		log.Printf("Movie %d (%s) has no IMDB ID in metadata", id, movie.Title)
		respondError(w, http.StatusBadRequest, "movie has no IMDB ID")
		return
	}

	// Fetch live streams from providers
	if h.streamProvider == nil {
		log.Printf("Stream provider not configured")
		respondError(w, http.StatusServiceUnavailable, "stream provider not configured")
		return
	}

	log.Printf("Fetching streams for movie %d (%s) with IMDB ID: %s", id, movie.Title, imdbID)
	providerStreams, err := h.streamProvider.GetMovieStreams(imdbID)
	if err != nil {
		log.Printf("Failed to get streams for movie %d (%s): %v", id, imdbID, err)
		respondJSON(w, http.StatusOK, []interface{}{}) // Return empty array instead of error
		return
	}

	// Apply release filters from settings
	providerStreams = h.applyReleaseFilters(providerStreams)

	// Sort streams: by quality (4K > 1080 > 720 > 480 > Unknown), then by cached status
	sortStreams(providerStreams)

	log.Printf("Found %d streams for movie %d (%s) after filtering", len(providerStreams), id, movie.Title)

	apiStreams := make([]map[string]interface{}, 0, len(providerStreams))

	// Convert provider streams to API response format
	for _, ps := range providerStreams {
		// Extract codec from stream name if available (e.g., "x265", "x264", "HEVC")
		codec := ""
		name := strings.ToUpper(ps.Name)
		if strings.Contains(name, "X265") || strings.Contains(name, "HEVC") {
			codec = "HEVC"
		} else if strings.Contains(name, "X264") || strings.Contains(name, "H264") {
			codec = "H.264"
		}

		stream := map[string]interface{}{
			"source":   ps.Source,
			"quality":  ps.Quality,
			"codec":    codec,
			"url":      ps.URL,
			"cached":   ps.Cached,
			"seeds":    ps.Seeders,
			"size_gb":  float64(ps.Size) / (1024 * 1024 * 1024),
			"title":    ps.Title,
			"name":     ps.Name,
			"filename": ps.Title, // Use title as filename for display
		}
		apiStreams = append(apiStreams, stream)
	}

	respondJSON(w, http.StatusOK, apiStreams)
}

// GetEpisodeStreams handles GET /api/stream/series/{imdb_id}:{season}:{episode}
func (h *Handler) GetEpisodeStreams(w http.ResponseWriter, r *http.Request) {
	// Parse the stream ID from URL path (format: imdbId:season:episode)
	vars := mux.Vars(r)
	streamID := vars["stream_id"]

	// Parse stream ID format: tt1234567:1:1
	parts := strings.Split(streamID, ":")
	if len(parts) != 3 {
		respondError(w, http.StatusBadRequest, "invalid stream ID format, expected imdb:season:episode")
		return
	}

	imdbID := parts[0]
	season, err := strconv.Atoi(parts[1])
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid season number")
		return
	}

	episode, err := strconv.Atoi(parts[2])
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid episode number")
		return
	}

	// Check if this is IPTV VOD series and return IPTV streams
	ctx := r.Context()
	series, err := h.seriesStore.GetByIMDBID(ctx, imdbID)
	if err == nil && series != nil && isIPTVVODSeries(series) {
		apiStreams := buildIPTVVODSeriesStreams(series, season, episode)
		log.Printf("Returning %d IPTV VOD streams for series %s S%02dE%02d", len(apiStreams), imdbID, season, episode)
		respondJSON(w, http.StatusOK, apiStreams)
		return
	}

	// Fetch live streams from providers
	if h.streamProvider == nil {
		log.Printf("Stream provider not configured")
		respondError(w, http.StatusServiceUnavailable, "stream provider not configured")
		return
	}

	log.Printf("Fetching streams for series %s S%02dE%02d", imdbID, season, episode)
	providerStreams, err := h.streamProvider.GetSeriesStreams(imdbID, season, episode)
	if err != nil {
		log.Printf("Failed to get streams for series %s S%02dE%02d: %v", imdbID, season, episode, err)
		respondJSON(w, http.StatusOK, []interface{}{}) // Return empty array instead of error
		return
	}

	// Apply release filters from settings
	providerStreams = h.applyReleaseFilters(providerStreams)

	// Sort streams: by quality (4K > 1080 > 720 > 480 > Unknown), then by cached status
	sortStreams(providerStreams)

	log.Printf("Found %d streams for series %s S%02dE%02d after filtering", len(providerStreams), imdbID, season, episode)

	// Convert provider streams to API response format
	apiStreams := make([]map[string]interface{}, 0, len(providerStreams))
	for _, ps := range providerStreams {
		// Extract codec from stream name if available
		codec := ""
		name := strings.ToUpper(ps.Name)
		if strings.Contains(name, "X265") || strings.Contains(name, "HEVC") {
			codec = "HEVC"
		} else if strings.Contains(name, "X264") || strings.Contains(name, "H264") {
			codec = "H.264"
		}

		stream := map[string]interface{}{
			"source":   ps.Source,
			"quality":  ps.Quality,
			"codec":    codec,
			"url":      ps.URL,
			"cached":   ps.Cached,
			"seeds":    ps.Seeders,
			"size_gb":  float64(ps.Size) / (1024 * 1024 * 1024),
			"title":    ps.Title,
			"name":     ps.Name,
			"filename": ps.Title, // Use title as filename for display
		}
		apiStreams = append(apiStreams, stream)
	}

	respondJSON(w, http.StatusOK, apiStreams)
}

// PlayMovie handles GET /api/movies/{id}/play
func (h *Handler) PlayMovie(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	movie, err := h.movieStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "movie not found")
		return
	}

	// Build exclude patterns from settings
	var excludePatterns []string
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		if settings.EnableReleaseFilters {
			if settings.ExcludedQualities != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedQualities)
			}
			if settings.ExcludedReleaseGroups != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedReleaseGroups)
			}
			if settings.ExcludedLanguageTags != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedLanguageTags)
			}
		}
	}

	// Find best available stream with filters
	stream, err := h.streamStore.FindBestStreamWithFilters(ctx, "movie", id, movie.QualityProfile, excludePatterns)
	if err != nil {
		respondError(w, http.StatusNotFound, "no streams available")
		return
	}

	// Get Real-Debrid streaming URL
	streamURL, err := h.rdClient.GetStreamURL(ctx, stream.InfoHash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stream URL")
		return
	}

	log.Printf("Playing movie %s - selected stream: %s (Quality: %s)", movie.Title, stream.Title, stream.Quality)

	respondJSON(w, http.StatusOK, map[string]string{
		"stream_url": streamURL,
		"quality":    stream.Quality,
		"codec":      stream.Codec,
		"source":     stream.Source,
	})
}

// ListSeries handles GET /api/series
func (h *Handler) ListSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	var monitored *bool
	if m := r.URL.Query().Get("monitored"); m != "" {
		val := m == "true"
		monitored = &val
	}

	series, err := h.seriesStore.List(ctx, offset, limit, monitored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, series)
}

// GetSeries handles GET /api/series/{id}
func (h *Handler) GetSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	series, err := h.seriesStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "series not found")
		return
	}

	respondJSON(w, http.StatusOK, series)
}

// AddSeries handles POST /api/series
func (h *Handler) AddSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		TMDBID         int64  `json:"tmdb_id"`
		Monitored      bool   `json:"monitored"`
		QualityProfile string `json:"quality_profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if series already exists in library
	existingSeries, err := h.seriesStore.GetByTMDBID(ctx, int(req.TMDBID))
	if err == nil && existingSeries != nil {
		// Series already exists, return it with 200 OK
		respondJSON(w, http.StatusOK, existingSeries)
		return
	}

	// Fetch series details from TMDB
	tmdbSeries, err := h.tmdbClient.GetSeries(ctx, int(req.TMDBID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to fetch series from TMDB: %v", err))
		return
	}

	// Create series model
	series := tmdbSeries
	series.Monitored = req.Monitored
	series.QualityProfile = req.QualityProfile

	// Add to library
	if err := h.seriesStore.Add(ctx, series); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add series: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, series)
}

// UpdateSeries handles PUT /api/series/{id}
func (h *Handler) UpdateSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	var updates struct {
		Monitored      *bool   `json:"monitored,omitempty"`
		QualityProfile *string `json:"quality_profile,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing series
	series, err := h.seriesStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "series not found")
		return
	}

	// Apply updates
	if updates.Monitored != nil {
		series.Monitored = *updates.Monitored
	}
	if updates.QualityProfile != nil {
		series.QualityProfile = *updates.QualityProfile
	}

	// Save changes
	if err := h.seriesStore.Update(ctx, series); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update series")
		return
	}

	respondJSON(w, http.StatusOK, series)
}

// DeleteSeries handles DELETE /api/series/{id}
func (h *Handler) DeleteSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	if err := h.seriesStore.Delete(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete series")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetSeriesEpisodes handles GET /api/series/{id}/episodes
func (h *Handler) GetSeriesEpisodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get series ID from path parameter
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	// Check for optional season filter
	seasonParam := r.URL.Query().Get("season")

	episodes, err := h.episodeStore.ListBySeries(ctx, id)
	if err != nil {
		log.Printf("Error fetching episodes for series %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, "failed to get episodes")
		return
	}

	// If no episodes in database, fetch from TMDB
	if len(episodes) == 0 {
		// Get series details to get TMDB ID and number of seasons
		series, err := h.seriesStore.Get(ctx, id)
		if err != nil {
			log.Printf("Error fetching series %d: %v", id, err)
			respondJSON(w, http.StatusOK, []*models.Episode{})
			return
		}

		// Fetch IMDB ID if not present in metadata
		if series.Metadata["imdb_id"] == nil || series.Metadata["imdb_id"] == "" {
			externalIDs, err := h.tmdbClient.GetSeriesExternalIDs(ctx, series.TMDBID)
			if err == nil && externalIDs.IMDBID != "" {
				series.Metadata["imdb_id"] = externalIDs.IMDBID
				log.Printf("Fetched IMDB ID %s for series %d (%s)", externalIDs.IMDBID, id, series.Title)

				// Update the series in database to store IMDB ID
				if err := h.seriesStore.Update(ctx, series); err != nil {
					log.Printf("Error updating series %d with IMDB ID: %v", id, err)
				}
			}
		}

		// Fetch series details from TMDB to get season count
		tmdbSeries, err := h.tmdbClient.GetSeries(ctx, series.TMDBID)
		if err != nil {
			log.Printf("Error fetching TMDB series %d: %v", series.TMDBID, err)
			respondJSON(w, http.StatusOK, []*models.Episode{})
			return
		}

		if tmdbSeries.Seasons > 0 {
			log.Printf("Fetching %d seasons for series %d (%s) from TMDB", tmdbSeries.Seasons, id, series.Title)
			tmdbEpisodes, err := h.tmdbClient.GetEpisodes(ctx, id, series.TMDBID, tmdbSeries.Seasons)
			if err != nil {
				log.Printf("Error fetching episodes from TMDB for series %d: %v", id, err)
				respondJSON(w, http.StatusOK, []*models.Episode{})
				return
			}

			// Store episodes in database using batch insert for performance
			if len(tmdbEpisodes) > 0 {
				if err := h.episodeStore.AddBatch(ctx, tmdbEpisodes); err != nil {
					log.Printf("Error storing episodes for series %d: %v", id, err)
					// Still return the episodes even if storage failed
				} else {
					log.Printf("Stored %d episodes for series %d in database", len(tmdbEpisodes), id)
				}
			}

			episodes = tmdbEpisodes
		}
	}

	// Ensure we return an empty array instead of null
	if episodes == nil {
		episodes = []*models.Episode{}
	}

	// Filter by season if provided
	if seasonParam != "" {
		season, err := strconv.Atoi(seasonParam)
		if err == nil {
			filtered := make([]*models.Episode, 0)
			for _, ep := range episodes {
				if ep.SeasonNumber == season {
					filtered = append(filtered, ep)
				}
			}
			episodes = filtered
		}
	}

	log.Printf("Returning %d episodes for series %d", len(episodes), id)

	// Always try to get series IMDB ID and add it to each episode's metadata
	if series, err := h.seriesStore.Get(ctx, id); err == nil {
		var seriesImdbID string
		if imdbID, ok := series.Metadata["imdb_id"].(string); ok && imdbID != "" {
			seriesImdbID = imdbID
		} else {
			// If IMDB ID not in metadata, try to fetch it from TMDB
			if externalIDs, err := h.tmdbClient.GetSeriesExternalIDs(ctx, series.TMDBID); err == nil && externalIDs.IMDBID != "" {
				seriesImdbID = externalIDs.IMDBID
				series.Metadata["imdb_id"] = externalIDs.IMDBID
				log.Printf("Fetched IMDB ID %s for series %d (%s)", externalIDs.IMDBID, id, series.Title)

				// Update the series in database to store IMDB ID
				if err := h.seriesStore.Update(ctx, series); err != nil {
					log.Printf("Error updating series %d with IMDB ID: %v", id, err)
				}
			}
		}

		// Add series IMDB ID to each episode's metadata
		if seriesImdbID != "" {
			for _, ep := range episodes {
				if ep.Metadata == nil {
					ep.Metadata = make(map[string]interface{})
				}
				ep.Metadata["series_imdb_id"] = seriesImdbID
			}
		}
	}

	respondJSON(w, http.StatusOK, episodes)
}

// PlayEpisode handles GET /api/episodes/{id}/play
func (h *Handler) PlayEpisode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid episode ID")
		return
	}

	episode, err := h.episodeStore.Get(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "episode not found")
		return
	}

	// Get series for quality profile
	series, err := h.seriesStore.Get(ctx, episode.SeriesID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get series")
		return
	}

	// Build exclude patterns from settings
	var excludePatterns []string
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		if settings.EnableReleaseFilters {
			if settings.ExcludedQualities != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedQualities)
			}
			if settings.ExcludedReleaseGroups != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedReleaseGroups)
			}
			if settings.ExcludedLanguageTags != "" {
				excludePatterns = append(excludePatterns, settings.ExcludedLanguageTags)
			}
		}
	}

	// Always find best stream with current filters applied
	stream, err := h.streamStore.FindBestStreamWithFilters(ctx, "series", episode.ID, series.QualityProfile, excludePatterns)
	if err != nil {
		respondError(w, http.StatusNotFound, "no streams available")
		return
	}

	streamURL, err := h.rdClient.GetStreamURL(ctx, stream.InfoHash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stream URL")
		return
	}

	// Cache the stream URL
	episode.Available = true
	episode.StreamURL = &streamURL
	h.episodeStore.UpdateAvailability(ctx, episode.ID, true, &streamURL)

	log.Printf("Playing episode S%02dE%02d - selected stream: %s (Quality: %s)", episode.SeasonNumber, episode.EpisodeNumber, stream.Title, stream.Quality)

	respondJSON(w, http.StatusOK, map[string]string{
		"stream_url": streamURL,
		"quality":    stream.Quality,
		"codec":      stream.Codec,
	})
}

// RootHandler handles GET /
func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":    "StreamArr API",
		"version": "1.1.0",
		"status":  "running",
		"endpoints": map[string]string{
			"health":   "/api/v1/health",
			"settings": "/api/v1/settings",
			"movies":   "/api/v1/movies",
			"series":   "/api/v1/series",
			"search":   "/api/v1/search/movies?q=query",
		},
	})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SearchSeries handles GET /api/search/series
func (h *Handler) SearchSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	results, err := h.tmdbClient.SearchSeries(ctx, query, page)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	respondJSON(w, http.StatusOK, results)
}

// ListChannels handles GET /api/channels
func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "channel manager not initialized")
		return
	}

	// Get query parameters
	category := r.URL.Query().Get("category")
	query := r.URL.Query().Get("search")

	var channels []*livetv.Channel

	if query != "" {
		channels = h.channelManager.SearchChannels(query)
	} else if category != "" {
		channels = h.channelManager.GetChannelsByCategory(category)
	} else {
		channels = h.channelManager.GetAllChannels()
	}

	// Apply source and category filters from settings
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()

		// Filter by enabled sources (if any are specified)
		if len(settings.LiveTVEnabledSources) > 0 {
			enabledSourcesMap := make(map[string]bool)
			for _, s := range settings.LiveTVEnabledSources {
				enabledSourcesMap[s] = true
			}

			filtered := make([]*livetv.Channel, 0, len(channels))
			for _, ch := range channels {
				if enabledSourcesMap[ch.Source] {
					filtered = append(filtered, ch)
				}
			}
			channels = filtered
		}

		// Filter by enabled categories (if any are specified)
		if len(settings.LiveTVEnabledCategories) > 0 {
			enabledCategoriesMap := make(map[string]bool)
			for _, c := range settings.LiveTVEnabledCategories {
				enabledCategoriesMap[c] = true
			}

			filtered := make([]*livetv.Channel, 0, len(channels))
			for _, ch := range channels {
				if enabledCategoriesMap[ch.Category] {
					filtered = append(filtered, ch)
				}
			}
			channels = filtered
		}
	}

	// Create enriched response with EPG data
	type EnrichedChannel struct {
		*livetv.Channel
		CurrentProgram *livetv.EPGProgram `json:"current_program,omitempty"`
		HasEPG         bool               `json:"has_epg"`
	}

	enriched := make([]EnrichedChannel, len(channels))
	for i, ch := range channels {
		enriched[i] = EnrichedChannel{
			Channel: ch,
			HasEPG:  false,
		}
		if h.epgManager != nil {
			// Try fallback matching for better EPG coverage
			program := h.epgManager.GetCurrentProgramWithFallback(ch.ID, ch.Name)
			if program != nil {
				enriched[i].HasEPG = true
				enriched[i].CurrentProgram = program
			} else {
				// Fallback to direct ID match
				enriched[i].HasEPG = h.epgManager.HasEPG(ch.ID)
				enriched[i].CurrentProgram = h.epgManager.GetCurrentProgram(ch.ID)
			}
		}
	}

	respondJSON(w, http.StatusOK, enriched)
}

// GetChannelStats handles GET /api/channels/stats - returns categories and sources
func (h *Handler) GetChannelStats(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "channel manager not initialized")
		return
	}

	channels := h.channelManager.GetAllChannels()

	// Collect unique categories and sources
	categoryMap := make(map[string]int)
	sourceMap := make(map[string]int)

	for _, ch := range channels {
		if ch.Category != "" {
			categoryMap[ch.Category]++
		}
		if ch.Source != "" {
			sourceMap[ch.Source]++
		}
	}

	// Convert to arrays with counts
	categories := make([]map[string]interface{}, 0, len(categoryMap))
	for name, count := range categoryMap {
		categories = append(categories, map[string]interface{}{
			"name":  name,
			"count": count,
		})
	}

	sources := make([]map[string]interface{}, 0, len(sourceMap))
	for name, count := range sourceMap {
		sources = append(sources, map[string]interface{}{
			"name":  name,
			"count": count,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_channels": len(channels),
		"categories":     categories,
		"sources":        sources,
	})
}

// CheckM3USourceStatus handles POST /api/channels/check-source - checks if an M3U URL is accessible
func (h *Handler) CheckM3USourceStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required")
		return
	}

	// Check if the URL is accessible
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	httpReq, err := http.NewRequest("HEAD", req.URL, nil)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"url":     req.URL,
			"status":  "error",
			"online":  false,
			"message": "Invalid URL",
		})
		return
	}

	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(httpReq)
	if err != nil {
		// Try GET if HEAD fails
		httpReq.Method = "GET"
		resp, err = client.Do(httpReq)
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"url":     req.URL,
				"status":  "offline",
				"online":  false,
				"message": fmt.Sprintf("Connection failed: %v", err),
			})
			return
		}
	}
	defer resp.Body.Close()

	online := resp.StatusCode >= 200 && resp.StatusCode < 400
	status := "online"
	if !online {
		status = "offline"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"url":         req.URL,
		"status":      status,
		"online":      online,
		"status_code": resp.StatusCode,
		"message":     http.StatusText(resp.StatusCode),
	})
}

// GetChannel handles GET /api/channels/{id}
func (h *Handler) GetChannel(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "channel manager not initialized")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing channel ID")
		return
	}

	channel, err := h.channelManager.GetChannel(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "channel not found")
		return
	}

	respondJSON(w, http.StatusOK, channel)
}

// GetChannelStream handles GET /api/channels/{id}/stream
func (h *Handler) GetChannelStream(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "channel manager not initialized")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing channel ID")
		return
	}

	channel, err := h.channelManager.GetChannel(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "channel not found")
		return
	}

	// Return stream URL
	respondJSON(w, http.StatusOK, map[string]string{
		"stream_url": channel.StreamURL,
	})
}

// GetChannelEPG handles GET /api/channels/{id}/epg
func (h *Handler) GetChannelEPG(w http.ResponseWriter, r *http.Request) {
	if h.epgManager == nil {
		respondError(w, http.StatusServiceUnavailable, "EPG manager not initialized")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing channel ID")
		return
	}

	// Get channel name for fallback matching
	var channelName string
	if h.channelManager != nil {
		if ch, err := h.channelManager.GetChannel(id); err == nil && ch != nil {
			channelName = ch.Name
		}
	}

	// Parse date parameter (default to today)
	dateStr := r.URL.Query().Get("date")
	date := time.Now()
	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = parsedDate
		}
	}

	// Try fallback matching for better EPG coverage
	programs := h.epgManager.GetEPGWithFallback(id, channelName, date)
	respondJSON(w, http.StatusOK, programs)
}

// GetSettings handles GET /api/settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager != nil {
		// Use new settings manager
		settings := h.settingsManager.Get()
		respondJSON(w, http.StatusOK, settings)
		return
	}

	// Fallback to database store
	ctx := r.Context()
	settings, err := h.settingsStore.GetAll(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

// UpdateSettings handles PUT /api/settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager != nil {
		// Use new settings manager
		var newSettings settings.Settings
		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			respondError(w, http.StatusBadRequest, "invalid settings data")
			return
		}

		log.Printf("[Settings] UpdateSettings: StremioAddon.Enabled=%v", newSettings.StremioAddon.Enabled)

		if err := h.settingsManager.Update(&newSettings); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}

		// Apply M3U source changes to channel manager immediately
		if h.channelManager != nil && len(newSettings.M3USources) >= 0 {
			// Update Live TV enabled flag and IPTV import mode
			h.channelManager.SetIncludeLiveTV(newSettings.IncludeLiveTV)
			h.channelManager.SetIPTVImportMode(newSettings.IPTVImportMode)
			m3uSources := make([]livetv.M3USource, len(newSettings.M3USources))
			var customEPGURLs []string
			for i, s := range newSettings.M3USources {
				m3uSources[i] = livetv.M3USource{
					Name:    s.Name,
					URL:     s.URL,
					Enabled: s.Enabled,
					EPGURL:  s.EPGURL,
				}
				// Collect EPG URLs from enabled M3U sources
				if s.Enabled && s.EPGURL != "" {
					customEPGURLs = append(customEPGURLs, s.EPGURL)
				}
			}
			h.channelManager.SetM3USources(m3uSources)
			log.Printf("Live TV: Updated M3U sources (%d configured)", len(m3uSources))

			// Add custom EPG URLs to EPG manager
			if h.epgManager != nil && len(customEPGURLs) > 0 {
				h.epgManager.AddCustomEPGURLs(customEPGURLs)
			}
		}

		// Apply Xtream source changes to channel manager immediately
		if h.channelManager != nil && len(newSettings.XtreamSources) >= 0 {
			xtreamSources := make([]livetv.XtreamSource, len(newSettings.XtreamSources))
			for i, s := range newSettings.XtreamSources {
				xtreamSources[i] = livetv.XtreamSource{
					Name:      s.Name,
					ServerURL: s.ServerURL,
					Username:  s.Username,
					Password:  s.Password,
					Enabled:   s.Enabled,
				}
			}
			h.channelManager.SetXtreamSources(xtreamSources)
			log.Printf("Live TV: Updated Xtream sources (%d configured)", len(xtreamSources))
		}

		// Reload channels to apply M3U/Xtream changes
		if h.channelManager != nil {
			if err := h.channelManager.LoadChannels(); err != nil {
				log.Printf("Warning: Failed to reload channels after source update: %v", err)
			}
		}

		// Apply stream validation setting
		if h.channelManager != nil {
			h.channelManager.SetStreamValidation(newSettings.LiveTVValidateStreams)
			log.Printf("Live TV: Stream validation enabled=%v", newSettings.LiveTVValidateStreams)
		}

		// Apply IPTV-org settings
		if h.channelManager != nil {
			h.channelManager.SetIPTVOrgConfig(livetv.IPTVOrgConfig{
				Enabled:    newSettings.IPTVOrgEnabled,
				Countries:  newSettings.IPTVOrgCountries,
				Languages:  newSettings.IPTVOrgLanguages,
				Categories: newSettings.IPTVOrgCategories,
			})
			if newSettings.IPTVOrgEnabled {
				log.Printf("Live TV: IPTV-org enabled (countries: %v, languages: %v, categories: %v)",
					newSettings.IPTVOrgCountries, newSettings.IPTVOrgLanguages, newSettings.IPTVOrgCategories)
			} else {
				log.Println("Live TV: IPTV-org disabled")
			}

			// Reload channels to apply IPTV-org changes
			if err := h.channelManager.LoadChannels(); err != nil {
				log.Printf("Warning: Failed to reload channels after IPTV-org update: %v", err)
			} else {
				log.Printf("Live TV: Reloaded %d channels after settings update", len(h.channelManager.GetAllChannels()))
			}
		}

		// Trigger IPTV VOD import/cleanup in background if mode includes VOD or sources changed
		go func(ns settings.Settings) {
			if h.tmdbClient == nil || h.movieStore == nil || h.seriesStore == nil || h.settingsManager == nil {
				return
			}
			mode := strings.ToLower(ns.IPTVImportMode)
			includesVOD := mode == "vod_only" || mode == "both"
			if includesVOD {
				ctx := context.Background()
				summary, err := services.ImportIPTVVOD(ctx, &ns, h.tmdbClient, h.movieStore, h.seriesStore)
				if err != nil {
					log.Printf("[Settings] IPTV VOD import error: %v", err)
				} else if summary != nil {
					log.Printf("[Settings] IPTV VOD import: sources=%d items=%d movies=%d series=%d skipped=%d errors=%d",
						summary.SourcesChecked, summary.ItemsFound, summary.MoviesImported, summary.SeriesImported, summary.Skipped, summary.Errors)
				}
				_ = services.CleanupIPTVVOD(ctx, &ns, h.movieStore, h.seriesStore)
			}
		}(newSettings)

		respondJSON(w, http.StatusOK, newSettings)
		return
	}

	// Fallback to database store
	ctx := r.Context()

	var settings models.SettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.settingsStore.SetAll(ctx, &settings); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

// GetCalendar handles GET /api/calendar
func (h *Handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse date range from query params
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		respondError(w, http.StatusBadRequest, "start and end date parameters are required")
		return
	}

	// Get upcoming movies
	movies, err := h.movieStore.GetUpcoming(ctx, startStr, endStr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get upcoming movies")
		return
	}

	// Get upcoming episodes
	episodes, err := h.episodeStore.GetUpcoming(ctx, startStr, endStr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get upcoming episodes")
		return
	}

	// Combine into calendar entries
	var entries []models.CalendarEntry

	for _, movie := range movies {
		year := 0
		if movie.ReleaseDate != nil {
			year = movie.ReleaseDate.Year()
		}
		voteAverage := 0.0
		if v, ok := movie.Metadata["vote_average"].(float64); ok {
			voteAverage = v
		}
		entries = append(entries, models.CalendarEntry{
			ID:          movie.ID,
			Type:        "movie",
			Title:       movie.Title,
			Date:        movie.ReleaseDate,
			PosterPath:  movie.PosterPath,
			Overview:    movie.Overview,
			VoteAverage: voteAverage,
			Year:        year,
		})
	}

	for _, episode := range episodes {
		series, _ := h.seriesStore.Get(ctx, episode.SeriesID)
		seriesTitle := ""
		voteAverage := 0.0
		if series != nil {
			seriesTitle = series.Title
			if v, ok := series.Metadata["vote_average"].(float64); ok {
				voteAverage = v
			}
		}

		entries = append(entries, models.CalendarEntry{
			ID:            episode.ID,
			Type:          "episode",
			Title:         episode.Title,
			Date:          episode.AirDate,
			PosterPath:    episode.StillPath,
			Overview:      episode.Overview,
			VoteAverage:   voteAverage,
			SeriesID:      &episode.SeriesID,
			SeriesTitle:   seriesTitle,
			SeasonNumber:  &episode.SeasonNumber,
			EpisodeNumber: &episode.EpisodeNumber,
		})
	}

	respondJSON(w, http.StatusOK, entries)
}

// GetMDBListUserLists fetches the user's MDBList lists
func (h *Handler) GetMDBListUserLists(w http.ResponseWriter, r *http.Request) {
	// Get API key from query parameter first, fall back to settings
	apiKey := r.URL.Query().Get("apiKey")

	if apiKey == "" {
		ctx := r.Context()
		setting, err := h.settingsStore.Get(ctx, "mdblist_api_key")
		if err == nil && setting != nil {
			apiKey = setting.Value
		}
	}

	if apiKey == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "MDBList API key not configured",
		})
		return
	}

	// Create MDBList client and fetch user lists
	mdbClient := services.NewMDBListClient(apiKey, "./cache/mdblist")
	result, err := mdbClient.GetUserLists()
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Failed to fetch user lists: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetTrending handles GET /api/discover/trending
func (h *Handler) GetTrending(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mediaType := r.URL.Query().Get("media_type")
	if mediaType == "" {
		mediaType = "all"
	}

	timeWindow := r.URL.Query().Get("time_window")
	if timeWindow == "" {
		timeWindow = "day"
	}

	items, err := h.tmdbClient.GetTrending(ctx, mediaType, timeWindow)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get trending: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, items)
}

// GetPopular handles GET /api/discover/popular
func (h *Handler) GetPopular(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mediaType := r.URL.Query().Get("media_type")
	if mediaType == "" {
		mediaType = "movie"
	}

	items, err := h.tmdbClient.GetPopular(ctx, mediaType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get popular: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, items)
}

// GetNowPlaying handles GET /api/discover/now-playing
func (h *Handler) GetNowPlaying(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mediaType := r.URL.Query().Get("media_type")
	if mediaType == "" {
		mediaType = "movie"
	}

	items, err := h.tmdbClient.GetNowPlaying(ctx, mediaType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get now playing: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, items)
}

// ListCollections handles GET /api/collections
func (h *Handler) ListCollections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	collections, total, err := h.collectionStore.GetCollectionsWithProgress(ctx, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list collections")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"collections": collections,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

// GetCollection handles GET /api/collections/{id}
func (h *Handler) GetCollection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	collection, err := h.collectionStore.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	// Also get movies in this collection
	movies, _ := h.collectionStore.GetMoviesInCollection(ctx, id)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"collection": collection,
		"movies":     movies,
	})
}

// SyncCollection handles POST /api/collections/{id}/sync - adds all missing movies from a collection
func (h *Handler) SyncCollection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	collection, err := h.collectionStore.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	var req struct {
		Monitored      bool   `json:"monitored"`
		QualityProfile string `json:"quality_profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Monitored = true
		req.QualityProfile = "default"
	}

	// Start background sync
	go h.addCollectionMovies(ctx, collection.TMDBID, req.Monitored, req.QualityProfile)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Collection sync started",
		"status":  "syncing",
	})
}

// GetCollectionMovies handles GET /api/collections/{id}/movies
func (h *Handler) GetCollectionMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	movies, err := h.collectionStore.GetMoviesInCollection(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get collection movies")
		return
	}

	respondJSON(w, http.StatusOK, movies)
}

// SearchCollections handles GET /api/search/collections - search TMDB for collections
func (h *Handler) SearchCollections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "search query required")
		return
	}

	collections, err := h.tmdbClient.SearchCollections(ctx, query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to search collections")
		return
	}

	respondJSON(w, http.StatusOK, collections)
}

// GetServices handles GET /api/services - returns status of all background services
func (h *Handler) GetServices(w http.ResponseWriter, r *http.Request) {
	statuses := services.GlobalScheduler.GetAllStatus()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"services": statuses,
	})
}

// TriggerService handles POST /api/services/{name}/trigger - manually triggers a service
func (h *Handler) TriggerService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["name"]
	if serviceName == "" {
		respondError(w, http.StatusBadRequest, "service name required")
		return
	}

	status := services.GlobalScheduler.GetStatus(serviceName)
	if status == nil {
		respondError(w, http.StatusNotFound, "service not found")
		return
	}

	if status.Running {
		respondError(w, http.StatusConflict, "service is already running")
		return
	}

	// Trigger the service based on name
	go h.runService(serviceName)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Service triggered",
		"service": serviceName,
		"status":  "running",
	})
}

// runService executes a specific service
func (h *Handler) runService(serviceName string) {
	ctx := context.Background()
	services.GlobalScheduler.MarkRunning(serviceName)

	var err error
	var interval time.Duration

	switch serviceName {
	case services.ServiceEPGUpdate:
		interval = 6 * time.Hour
		if h.channelManager != nil && h.epgManager != nil {
			channels := h.channelManager.GetAllChannels()
			channelList := make([]livetv.Channel, len(channels))
			for i, ch := range channels {
				channelList[i] = *ch
			}
			err = h.epgManager.UpdateEPG(channelList)
		}

	case services.ServiceChannelRefresh:
		interval = 1 * time.Hour
		if h.channelManager != nil {
			err = h.channelManager.LoadChannels()
		}

	case services.ServiceCacheCleanup:
		interval = 1 * time.Hour
		// Cache cleanup would be handled by cache manager
		// For now just mark as complete

	case services.ServicePlaylist:
		interval = 12 * time.Hour
		// Playlist generation requires the playlist generator
		// For now just mark as complete

	case services.ServiceMDBListSync:
		interval = 6 * time.Hour
		// MDBList sync requires the sync service
		// For now just mark as complete

	case services.ServiceCollectionSync:
		interval = 24 * time.Hour
		// Phase 1: Scan existing movies for collections and link them
		if h.collectionStore != nil && h.movieStore != nil {
			err = h.scanAndLinkCollections(ctx)
		}
		// Phase 2: Sync incomplete collections (add missing movies)
		if h.collectionStore != nil && h.settingsManager != nil {
			settings := h.settingsManager.Get()
			if settings.AutoAddCollections {
				fmt.Println("[Collection Sync] Phase 2: Adding missing movies from incomplete collections...")
				services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, 0, 0, "Phase 2: Checking incomplete collections...")

				collections, _, _ := h.collectionStore.GetCollectionsWithProgress(ctx, 1000, 0)
				fmt.Printf("[Collection Sync] Found %d collections to check\n", len(collections))

				// Find incomplete collections
				var incompleteColls []*models.Collection
				for _, coll := range collections {
					if coll.MoviesInLibrary < coll.TotalMovies {
						incompleteColls = append(incompleteColls, coll)
					}
				}

				totalIncomplete := len(incompleteColls)
				if totalIncomplete == 0 {
					fmt.Println("[Collection Sync] Phase 2: All collections are complete!")
					services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, 0, 0, "All collections complete!")
				} else {
					fmt.Printf("[Collection Sync] Found %d incomplete collections\n", totalIncomplete)

					for i, coll := range incompleteColls {
						services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, i+1, totalIncomplete,
							fmt.Sprintf("Adding movies to: %s (%d/%d)", coll.Name, coll.MoviesInLibrary, coll.TotalMovies))

						// Skip auto-adding collections that are seeded only by IPTV VOD imports
						if h.collectionStore != nil {
							if movies, err := h.collectionStore.GetMoviesInCollection(ctx, coll.ID); err == nil {
								if !collectionHasNonIPTVVOD(movies) {
									fmt.Printf("[Collection Sync] Skipping collection '%s' (only IPTV VOD items)\n", coll.Name)
									continue
								}
							}
						}

						fmt.Printf("[Collection Sync] '%s' is incomplete (%d/%d), adding missing movies...\n",
							coll.Name, coll.MoviesInLibrary, coll.TotalMovies)
						h.addCollectionMovies(ctx, coll.TMDBID, true, "default")
						time.Sleep(500 * time.Millisecond) // Rate limit
					}

					services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, totalIncomplete, totalIncomplete,
						fmt.Sprintf("Phase 2 complete: %d collections updated", totalIncomplete))
					fmt.Printf("[Collection Sync] Phase 2 complete: processed %d incomplete collections\n", totalIncomplete)
				}
			} else {
				fmt.Println("[Collection Sync] Phase 2 skipped: AutoAddCollections is disabled")
				services.GlobalScheduler.UpdateProgress(services.ServiceCollectionSync, 0, 0, "Phase 2 skipped (AutoAddCollections disabled)")
			}
		} else {
			fmt.Println("[Collection Sync] Phase 2 skipped: collectionStore or settingsManager is nil")
		}

	case services.ServiceEpisodeScan:
		interval = 24 * time.Hour
		if h.seriesStore != nil && h.episodeStore != nil && h.tmdbClient != nil {
			err = h.scanEpisodesForAllSeries(ctx)
		} else {
			fmt.Println("[Episode Scan] Skipped: required stores not initialized")
		}

	case services.ServiceStreamSearch:
		interval = 6 * time.Hour
		err = h.scanStreamAvailability(ctx)

	case services.ServiceIPTVVODSync:
		interval = 12 * time.Hour
		// Only run when VOD mode is enabled and TMDB key is present
		if h.settingsManager != nil && h.tmdbClient != nil && h.movieStore != nil && h.seriesStore != nil {
			current := h.settingsManager.Get()
			mode := strings.ToLower(current.IPTVImportMode)
			includesVOD := mode == "vod_only" || mode == "both"
			if includesVOD && current.TMDBAPIKey != "" {
				_, err = services.ImportIPTVVOD(ctx, current, h.tmdbClient, h.movieStore, h.seriesStore)
				_ = services.CleanupIPTVVOD(ctx, current, h.movieStore, h.seriesStore)
			} else {
				err = nil
			}
		}

	default:
		interval = 1 * time.Hour
	}

	services.GlobalScheduler.MarkComplete(serviceName, err, interval)
}

// UpdateServiceEnabled handles PUT /api/services/{name} - enables/disables a service
func (h *Handler) UpdateServiceEnabled(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["name"]
	if serviceName == "" {
		respondError(w, http.StatusBadRequest, "service name required")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	services.GlobalScheduler.SetEnabled(serviceName, req.Enabled)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"service": serviceName,
		"enabled": req.Enabled,
	})
}

// GetDatabaseStats handles GET /api/database/stats
func (h *Handler) GetDatabaseStats(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var movieCount, seriesCount, episodeCount, streamCount, collectionCount int64

	// Count movies using Count method
	if h.movieStore != nil {
		if count, err := h.movieStore.Count(ctx); err == nil {
			movieCount = count
		}
	}

	// Count series using Count method
	if h.seriesStore != nil {
		if count, err := h.seriesStore.Count(ctx, nil); err == nil {
			seriesCount = int64(count)
		}
	}

	// Count episodes using CountAll method
	if h.episodeStore != nil {
		if count, err := h.episodeStore.CountAll(ctx); err == nil {
			episodeCount = int64(count)
		}
	}

	// Count streams using CountAll method
	if h.streamStore != nil {
		if count, err := h.streamStore.CountAll(ctx); err == nil {
			streamCount = int64(count)
		}
	}

	// Count collections
	if h.collectionStore != nil {
		collections, err := h.collectionStore.ListAll(ctx)
		if err == nil {
			collectionCount = int64(len(collections))
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"movies":      movieCount,
		"series":      seriesCount,
		"episodes":    episodeCount,
		"streams":     streamCount,
		"collections": collectionCount,
	})
}

// GetStats handles GET /api/stats - dashboard stats
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var totalMovies, monitoredMovies, availableMovies int64
	var totalSeries, monitoredSeries int64
	var totalEpisodes int
	var totalChannels, activeChannels int
	var totalCollections int

	// Movie counts
	if h.movieStore != nil {
		if count, err := h.movieStore.Count(ctx); err == nil {
			totalMovies = count
		}
		if count, err := h.movieStore.CountMonitored(ctx); err == nil {
			monitoredMovies = count
		}
		if count, err := h.movieStore.CountAvailable(ctx); err == nil {
			availableMovies = count
		}
	}

	// Series counts
	if h.seriesStore != nil {
		if count, err := h.seriesStore.Count(ctx, nil); err == nil {
			totalSeries = int64(count)
		}
		monitored := true
		if count, err := h.seriesStore.Count(ctx, &monitored); err == nil {
			monitoredSeries = int64(count)
		}
		if count, err := h.seriesStore.CountEpisodes(ctx); err == nil {
			totalEpisodes = count
		}
	}

	// Channel counts
	if h.channelManager != nil {
		channels := h.channelManager.GetAllChannels()
		totalChannels = len(channels)
		for _, ch := range channels {
			if ch.Active {
				activeChannels++
			}
		}
	}

	// Collection counts
	if h.collectionStore != nil {
		if count, err := h.collectionStore.Count(ctx); err == nil {
			totalCollections = count
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_movies":      totalMovies,
		"monitored_movies":  monitoredMovies,
		"available_movies":  availableMovies,
		"total_series":      totalSeries,
		"monitored_series":  monitoredSeries,
		"total_episodes":    totalEpisodes,
		"total_channels":    totalChannels,
		"active_channels":   activeChannels,
		"total_collections": totalCollections,
	})
}

// ExecuteDatabaseAction handles POST /api/database/{action}
func (h *Handler) ExecuteDatabaseAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	action := vars["action"]
	ctx := context.Background()

	var message string
	var err error

	switch action {
	case "clear-movies":
		if h.movieStore != nil {
			err = h.movieStore.DeleteAll(ctx)
			message = "All movies have been cleared from the library"
		}

	case "clear-series":
		if h.seriesStore != nil && h.episodeStore != nil {
			// First clear episodes
			err = h.episodeStore.DeleteAll(ctx)
			if err == nil {
				err = h.seriesStore.DeleteAll(ctx)
			}
			message = "All series and episodes have been cleared from the library"
		}

	case "clear-streams":
		if h.streamStore != nil {
			err = h.streamStore.DeleteAll(ctx)
			message = "All cached streams have been cleared"
		}

	case "clear-stale-streams":
		if h.streamStore != nil {
			err = h.streamStore.DeleteStale(ctx, 7) // 7 days
			message = "Stale streams (older than 7 days) have been cleared"
		}

	case "clear-collections":
		if h.collectionStore != nil {
			err = h.collectionStore.DeleteAll(ctx)
			message = "All collections have been cleared"
		}

	case "resync-collections":
		if h.collectionStore != nil {
			err = h.collectionStore.DeleteAll(ctx)
			if err == nil {
				// Reset collection_checked flag on all movies
				if h.movieStore != nil {
					err = h.movieStore.ResetCollectionChecked(ctx)
				}
				// Trigger collection sync service
				go h.runService(services.ServiceCollectionSync)
				message = "Collections cleared and re-sync triggered"
			}
		}

	case "reset-movie-status":
		if h.movieStore != nil {
			err = h.movieStore.ResetStatus(ctx)
			message = "Movie status and collection_checked flags have been reset"
		}

	case "reset-series-status":
		if h.seriesStore != nil {
			err = h.seriesStore.ResetStatus(ctx)
			message = "Series status has been reset"
		}

	case "reload-livetv":
		if h.channelManager != nil {
			err = h.channelManager.LoadChannels()
			if err == nil && h.epgManager != nil {
				h.epgManager.Update()
			}
			message = "Live TV channels and EPG have been reloaded"
		}

	case "clear-epg":
		if h.epgManager != nil {
			h.epgManager.Clear()
			message = "EPG cache has been cleared"
		}

	case "clear-all-vod":
		// Clear everything VOD-related
		if h.streamStore != nil {
			h.streamStore.DeleteAll(ctx)
		}
		if h.episodeStore != nil {
			h.episodeStore.DeleteAll(ctx)
		}
		if h.seriesStore != nil {
			h.seriesStore.DeleteAll(ctx)
		}
		if h.movieStore != nil {
			h.movieStore.DeleteAll(ctx)
		}
		if h.collectionStore != nil {
			h.collectionStore.DeleteAll(ctx)
		}
		message = "All VOD content (movies, series, episodes, streams, collections) has been cleared"

	case "factory-reset":
		// Clear everything
		if h.streamStore != nil {
			h.streamStore.DeleteAll(ctx)
		}
		if h.episodeStore != nil {
			h.episodeStore.DeleteAll(ctx)
		}
		if h.seriesStore != nil {
			h.seriesStore.DeleteAll(ctx)
		}
		if h.movieStore != nil {
			h.movieStore.DeleteAll(ctx)
		}
		if h.collectionStore != nil {
			h.collectionStore.DeleteAll(ctx)
		}
		if h.epgManager != nil {
			h.epgManager.Clear()
		}
		if h.channelManager != nil {
			h.channelManager.LoadChannels()
		}
		message = "Database has been reset to factory defaults"

	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Unknown action: %s", action))
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": message,
	})
}

// Version info - set at build time via ldflags
var (
	Version   = "1.1.0"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// GetVersion returns the current version info
func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"current_version": Version,
		"current_commit":  Commit,
		"build_date":      BuildDate,
	})
}

// CheckForUpdates checks GitHub for the latest version
func (h *Handler) CheckForUpdates(w http.ResponseWriter, r *http.Request) {
	// Get the configured branch from settings
	branch := "main"
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		if settings.UpdateBranch != "" {
			branch = settings.UpdateBranch
		}
	}

	// Fetch latest commit from GitHub API
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/Zerr0-C00L/StreamArr_Pro/commits/%s", branch))
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"current_version": Version,
			"current_commit":  Commit,
			"build_date":      BuildDate,
			"error":           "Failed to check for updates",
		})
		return
	}
	defer resp.Body.Close()

	var commitData struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&commitData); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"current_version": Version,
			"current_commit":  Commit,
			"build_date":      BuildDate,
			"error":           "Failed to parse update info",
		})
		return
	}

	// Check if update is available by comparing commits
	latestCommitShort := commitData.SHA
	if len(latestCommitShort) > 7 {
		latestCommitShort = latestCommitShort[:7]
	}

	updateAvailable := false
	if Commit != "unknown" && len(Commit) >= 7 {
		// Compare short commit hashes
		currentShort := Commit
		if len(currentShort) > 7 {
			currentShort = currentShort[:7]
		}
		updateAvailable = currentShort != latestCommitShort
	} else {
		// If commit is unknown, always show update available
		// so user can update to get proper version tracking
		updateAvailable = true
	}

	// Get first line of commit message as changelog
	changelog := commitData.Commit.Message
	if idx := strings.Index(changelog, "\n"); idx > 0 {
		changelog = changelog[:idx]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"current_version":  Version,
		"current_commit":   Commit,
		"build_date":       BuildDate,
		"latest_version":   "latest",
		"latest_commit":    commitData.SHA,
		"latest_date":      commitData.Commit.Author.Date,
		"update_available": updateAvailable,
		"changelog":        changelog,
		"update_branch":    branch,
	})
}

// InstallUpdate triggers the update process
func (h *Handler) InstallUpdate(w http.ResponseWriter, r *http.Request) {
	// Check if update script exists
	updateScript := "./scripts/update.sh"
	if _, err := os.Stat(updateScript); os.IsNotExist(err) {
		// Try old location for backward compatibility
		updateScript = "./update.sh"
		if _, err := os.Stat(updateScript); os.IsNotExist(err) {
			respondError(w, http.StatusNotImplemented, "Update script not found. Please update manually with: git pull && docker compose down && docker compose up -d --build")
			return
		}
	}

	// Get update branch from settings
	branch := "main"
	if h.settingsManager != nil {
		s := h.settingsManager.Get()
		if s.UpdateBranch != "" {
			branch = s.UpdateBranch
		}
	}

	log.Println("[Update] Starting update process...")

	// Check if running in Docker
	isDocker := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		isDocker = true
		log.Println("[Update] Detected Docker environment")
	}

	if isDocker {
		// In Docker, check prerequisites
		hostDir := "/app/host"
		if _, err := os.Stat(hostDir); os.IsNotExist(err) {
			respondError(w, http.StatusInternalServerError, "Host directory not mounted at /app/host. Add '- .:/app/host' to docker-compose volumes.")
			return
		}

		dockerSocket := "/var/run/docker.sock"
		if _, err := os.Stat(dockerSocket); os.IsNotExist(err) {
			respondError(w, http.StatusInternalServerError, "Docker socket not mounted. Add '- /var/run/docker.sock:/var/run/docker.sock' to docker-compose volumes.")
			return
		}

		// Check for git in host directory
		gitDir := hostDir + "/.git"
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			respondError(w, http.StatusInternalServerError, "Git repository not found in host directory. Ensure the project was cloned with git.")
			return
		}

		// Check if update.sh is in scripts/ folder (new location) or root (old location)
		scriptPath := hostDir + "/scripts/update.sh"
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			scriptPath = hostDir + "/update.sh"
			if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
				respondError(w, http.StatusNotImplemented, "Update script not found in host directory. Please update manually: cd /root/StreamArrPro && git pull && docker compose down && docker compose up -d --build")
				return
			}
		}

		log.Printf("[Update] Using script: %s", scriptPath)
		log.Printf("[Update] Branch: %s", branch)
		log.Printf("[Update] Host directory: %s", hostDir)

		// Ensure logs directory exists in host
		os.MkdirAll(hostDir+"/logs", 0755)

		// Make sure script is executable
		exec.Command("chmod", "+x", scriptPath).Run()

		// Run update script in background (detached) so it survives container stop
		// We use nohup and & to fully detach
		log.Println("[Update] Executing update script in background...")
		cmd := exec.Command("/bin/sh", "-c",
			fmt.Sprintf("cd %s && nohup /bin/sh %s %s >> %s/logs/update.log 2>&1 &", hostDir, scriptPath, branch, hostDir))

		// Set environment
		cmd.Env = append(os.Environ(),
			"HOME=/root",
			"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		)

		err := cmd.Start()
		if err != nil {
			log.Printf("[Update] Failed to start update: %v", err)
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start update: %v", err))
			return
		}

		// Don't wait - let it run in background
		go func() {
			cmd.Wait()
		}()

		log.Println("[Update] Script started in background")
	} else {
		// Non-Docker environment
		cmd := exec.Command("/bin/bash", "-c",
			fmt.Sprintf("nohup setsid /bin/bash %s %s > logs/update.log 2>&1 &", updateScript, branch))
		cmd.Dir = "."

		if err := cmd.Start(); err != nil {
			log.Printf("[Update] Failed to start update: %v", err)
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start update: %v", err))
			return
		}

		// Detach from the process immediately
		go cmd.Wait()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Update started. The server will restart shortly. Check logs/update.log for progress.",
	})
}

// ImportAdultVOD handles POST /api/v1/adult-vod/import
func (h *Handler) ImportAdultVOD(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil || h.movieStore == nil {
		respondError(w, http.StatusInternalServerError, "Importer not available")
		return
	}
	ctx := r.Context()
	imported, skipped, errs, err := services.ImportAdultMoviesFromGitHub(ctx, h.movieStore)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"imported": imported,
		"skipped":  skipped,
		"errors":   errs,
		"message":  fmt.Sprintf("Imported %d adult movies from GitHub", imported),
	})
}

// GetAdultVODStats handles GET /api/v1/adult-vod/stats
func (h *Handler) GetAdultVODStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Count adult movies in database (by metadata stream_type = adult)
	query := `
		SELECT COUNT(*) 
		FROM library_movies 
		WHERE metadata->>'stream_type' = 'adult'
	`

	var count int
	err := h.movieStore.GetDB().QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		log.Printf("[Adult VOD Stats] Error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get adult VOD stats")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_adult_movies": count,
		"source":             "github_public_files",
	})
}

// shouldIncludeCategory determines if a category should be included based on import mode
func shouldIncludeCategory(category, url, importMode string) bool {
	if importMode == "both" {
		return true
	}
	
	lowerURL := strings.ToLower(url)
	lowerCategory := strings.ToLower(category)
	
	// Check if it's VOD content
	isVOD := strings.Contains(lowerURL, "/movie/") || 
		strings.Contains(lowerURL, "/series/") || 
		strings.Contains(lowerURL, "/serije/") ||
		strings.HasSuffix(lowerURL, ".mp4") || 
		strings.HasSuffix(lowerURL, ".mkv") || 
		strings.HasSuffix(lowerURL, ".avi") ||
		strings.Contains(lowerCategory, "vod") || 
		strings.Contains(lowerCategory, "movie") || 
		strings.Contains(lowerCategory, "series") || 
		strings.Contains(lowerCategory, "film")
	
	if importMode == "vod_only" {
		return isVOD
	} else if importMode == "live_only" {
		return !isVOD
	}
	
	return true
}

// PreviewM3UCategories handles POST /api/v1/iptv-vod/preview-categories
func (h *Handler) PreviewM3UCategories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		ImportMode string `json:"import_mode"` // "vod_only", "live_only", or "both"
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required")
		return
	}
	
	// Default to "both" if not specified
	if req.ImportMode == "" {
		req.ImportMode = "both"
	}
	
	// Fetch M3U file
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("GET", req.URL, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to create request: %v", err))
		return
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0")
	
	resp, err := client.Do(httpReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to fetch M3U: %v", err))
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Server returned HTTP %d. Please verify the M3U URL is correct and accessible.", resp.StatusCode))
		return
	}
	
	// Parse categories with content type tracking
	categories := make(map[string]int) // category -> count
	scanner := bufio.NewScanner(resp.Body)
	var currentGroup string
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#EXTINF:") {
			currentGroup = ""
			// Extract group-title
			if idx := strings.Index(line, "group-title=\""); idx != -1 {
				start := idx + 13
				if end := strings.Index(line[start:], "\""); end != -1 {
					currentGroup = line[start : start+end]
				}
			}
		} else if currentGroup != "" && !strings.HasPrefix(line, "#") && line != "" {
			// URL line - check if it's VOD or Live TV
			if shouldIncludeCategory(currentGroup, line, req.ImportMode) {
				categories[currentGroup]++
			}
			currentGroup = "" // Reset after processing URL
		}
	}
	
	if err := scanner.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Error parsing M3U: %v", err))
		return
	}
	
	// Convert to sorted list
	type Category struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	
	result := make([]Category, 0, len(categories))
	for name, count := range categories {
		result = append(result, Category{Name: name, Count: count})
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": result,
	})
}

// PreviewXtreamCategories handles POST /api/v1/iptv-vod/preview-xtream-categories
func (h *Handler) PreviewXtreamCategories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerURL  string `json:"server_url"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		ImportMode string `json:"import_mode"` // "vod_only", "live_only", or "both"
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	if req.ServerURL == "" || req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Server URL, username, and password are required")
		return
	}
	
	// Default to "both" if not specified
	if req.ImportMode == "" {
		req.ImportMode = "both"
	}
	
	// Fetch categories based on import mode
	server := strings.TrimSuffix(req.ServerURL, "/")
	
	allCategories := make(map[string]int) // category name -> count
	
	// Fetch VOD categories if needed
	if req.ImportMode == "vod_only" || req.ImportMode == "both" {
		// Fetch VOD movie categories
		url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_categories", server, req.Username, req.Password)
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var categories []struct {
				CategoryID   string `json:"category_id"`
				CategoryName string `json:"category_name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&categories); err == nil {
				// Get VOD stream counts
				vodURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_streams", server, req.Username, req.Password)
				vodResp, err := client.Get(vodURL)
				if err == nil {
					defer vodResp.Body.Close()
					if vodResp.StatusCode == http.StatusOK {
						var streams []struct {
							CategoryID string `json:"category_id"`
						}
						if err := json.NewDecoder(vodResp.Body).Decode(&streams); err == nil {
							counts := make(map[string]int)
							for _, s := range streams {
								counts[s.CategoryID]++
							}
							for _, cat := range categories {
								if count, ok := counts[cat.CategoryID]; ok && count > 0 {
									allCategories[cat.CategoryName] = count
								}
							}
						}
					}
				}
			}
		}
		
		// Fetch series categories
		seriesURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series_categories", server, req.Username, req.Password)
		seriesResp, err := client.Get(seriesURL)
		
		if err == nil && seriesResp.StatusCode == http.StatusOK {
			defer seriesResp.Body.Close()
			var categories []struct {
				CategoryID   string `json:"category_id"`
				CategoryName string `json:"category_name"`
			}
			if err := json.NewDecoder(seriesResp.Body).Decode(&categories); err == nil {
				// Get series counts
				seriesStreamsURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series", server, req.Username, req.Password)
				seriesStreamsResp, err := client.Get(seriesStreamsURL)
				if err == nil {
					defer seriesStreamsResp.Body.Close()
					if seriesStreamsResp.StatusCode == http.StatusOK {
						var streams []struct {
							CategoryID string `json:"category_id"`
						}
						if err := json.NewDecoder(seriesStreamsResp.Body).Decode(&streams); err == nil {
							counts := make(map[string]int)
							for _, s := range streams {
								counts[s.CategoryID]++
							}
							for _, cat := range categories {
								if count, ok := counts[cat.CategoryID]; ok && count > 0 {
									allCategories[cat.CategoryName] = count
								}
							}
						}
					}
				}
			}
		}
	}
	
	// Fetch Live TV categories if needed
	if req.ImportMode == "live_only" || req.ImportMode == "both" {
		url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_categories", server, req.Username, req.Password)
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(url)
		
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var categories []struct {
				CategoryID   string `json:"category_id"`
				CategoryName string `json:"category_name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&categories); err == nil {
				// Get live stream counts
				liveURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_live_streams", server, req.Username, req.Password)
				liveResp, err := client.Get(liveURL)
				if err == nil {
					defer liveResp.Body.Close()
					if liveResp.StatusCode == http.StatusOK {
						var streams []struct {
							CategoryID string `json:"category_id"`
						}
						if err := json.NewDecoder(liveResp.Body).Decode(&streams); err == nil {
							counts := make(map[string]int)
							for _, s := range streams {
								counts[s.CategoryID]++
							}
							for _, cat := range categories {
								if count, ok := counts[cat.CategoryID]; ok && count > 0 {
									allCategories[cat.CategoryName] = count
								}
							}
						}
					}
				}
			}
		}
	}
	
	// If we got categories, return them
	if len(allCategories) > 0 {
		type Category struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		
		result := make([]Category, 0, len(allCategories))
		for name, count := range allCategories {
			result = append(result, Category{Name: name, Count: count})
		}
		
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
		
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"categories": result,
		})
		return
	}
	
	// If API failed, fall back to M3U parsing
	client := &http.Client{Timeout: 30 * time.Second}
	m3uURL := fmt.Sprintf("%s/get.php?username=%s&password=%s&type=m3u_plus&output=ts", server, req.Username, req.Password)
	httpReq, err := http.NewRequest("GET", m3uURL, nil)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to create request: %v", err))
		return
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0")
	
	resp, err := client.Do(httpReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to fetch M3U: %v", err))
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("Server returned HTTP %d. Please verify your credentials and server URL.", resp.StatusCode))
		return
	}
	// Parse M3U categories
	categories := make(map[string]int)
	scanner := bufio.NewScanner(resp.Body)
	var currentGroup string
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#EXTINF:") {
			currentGroup = ""
			if idx := strings.Index(line, "group-title=\""); idx != -1 {
				start := idx + 13
				if end := strings.Index(line[start:], "\""); end != -1 {
					currentGroup = line[start : start+end]
				}
			}
		} else if currentGroup != "" && !strings.HasPrefix(line, "#") && line != "" {
			// URL line - check if it's VOD or Live TV
			if shouldIncludeCategory(currentGroup, line, req.ImportMode) {
				categories[currentGroup]++
			}
			currentGroup = ""
		}
	}
	
	if err := scanner.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Error parsing M3U: %v", err))
		return
	}
	
	// Convert to sorted list
	type Category struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	
	result := make([]Category, 0, len(categories))
	for name, count := range categories {
		result = append(result, Category{Name: name, Count: count})
	}
	
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": result,
	})
}

// ImportIPTVVOD handles POST /api/v1/iptv-vod/import
func (h *Handler) ImportIPTVVOD(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil || h.tmdbClient == nil || h.movieStore == nil || h.seriesStore == nil {
		respondError(w, http.StatusInternalServerError, "Importer not available")
		return
	}

	// Optional: ensure TMDB key exists
	if h.settingsManager.GetTMDBAPIKey() == "" {
		respondError(w, http.StatusBadRequest, "TMDB API key is required (Settings → API)")
		return
	}

	cfg := h.settingsManager.Get()
	ctx := r.Context()

	summary, err := services.ImportIPTVVOD(ctx, cfg, h.tmdbClient, h.movieStore, h.seriesStore)
	if err != nil {
		log.Printf("[IPTV VOD Import] error: %v", err)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":         err == nil,
		"error":           errorString(err),
		"sources_checked": summary.SourcesChecked,
		"items_found":     summary.ItemsFound,
		"movies_imported": summary.MoviesImported,
		"series_imported": summary.SeriesImported,
		"skipped":         summary.Skipped,
		"errors":          summary.Errors,
	})
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// buildIPTVVODStreams converts metadata.iptv_vod_sources into API stream entries
func buildIPTVVODStreams(movie *models.Movie) []map[string]interface{} {
	streams := []map[string]interface{}{}
	if movie == nil || movie.Metadata == nil {
		return streams
	}

	if sources, ok := movie.Metadata["iptv_vod_sources"].([]interface{}); ok {
		for _, s := range sources {
			m, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			url, _ := m["url"].(string)
			name = strings.TrimSpace(name)
			url = strings.TrimSpace(url)
			if name == "" {
				name = "IPTV VOD"
			}
			if url == "" {
				continue
			}

			streams = append(streams, map[string]interface{}{
				"source":   name,
				"quality":  "VOD",
				"codec":    "",
				"url":      url,
				"cached":   false,
				"seeds":    0,
				"size_gb":  0.0,
				"title":    fmt.Sprintf("%s (%s)", movie.Title, name),
				"name":     name,
				"filename": name,
			})
		}
	}

	return streams
}

func isIPTVVODMovie(movie *models.Movie) bool {
	if movie == nil || movie.Metadata == nil {
		return false
	}
	if src, ok := movie.Metadata["source"].(string); ok {
		return strings.EqualFold(src, "iptv_vod")
	}
	return false
}

func collectionHasNonIPTVVOD(movies []*models.Movie) bool {
	for _, m := range movies {
		if !isIPTVVODMovie(m) {
			return true
		}
	}
	return false
}

func isIPTVVODSeries(series *models.Series) bool {
	if series == nil || series.Metadata == nil {
		return false
	}
	if src, ok := series.Metadata["source"].(string); ok {
		return strings.EqualFold(src, "iptv_vod")
	}
	return false
}

func buildIPTVVODSeriesStreams(series *models.Series, season, episode int) []map[string]interface{} {
	streams := []map[string]interface{}{}
	if series == nil || series.Metadata == nil {
		return streams
	}

	if sources, ok := series.Metadata["iptv_vod_sources"].([]interface{}); ok {
		for _, s := range sources {
			m, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			url, _ := m["url"].(string)
			name = strings.TrimSpace(name)
			url = strings.TrimSpace(url)
			if name == "" {
				name = "IPTV VOD"
			}
			if url == "" {
				continue
			}

			streams = append(streams, map[string]interface{}{
				"source":   name,
				"quality":  "VOD",
				"codec":    "",
				"url":      url,
				"cached":   false,
				"seeds":    0,
				"size_gb":  0.0,
				"title":    fmt.Sprintf("%s S%02dE%02d (%s)", series.Title, season, episode, name),
				"name":     name,
				"filename": name,
			})
		}
	}

	return streams
}
