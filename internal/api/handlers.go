package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/epg"
	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
	"github.com/gorilla/mux"
)

type Handler struct {
	movieStore       *database.MovieStore
	seriesStore      *database.SeriesStore
	episodeStore     *database.EpisodeStore
	streamStore      *database.StreamStore
	settingsStore    *database.SettingsStore
	userStore        *database.UserStore
	collectionStore  *database.CollectionStore
	tmdbClient       *services.TMDBClient
	rdClient         *services.RealDebridClient
	torrentio        *services.TorrentioClient
	channelManager   *livetv.ChannelManager
	settingsManager  *settings.Manager
	epgManager       *epg.Manager
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
	torrentio *services.TorrentioClient,
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
		torrentio:       torrentio,
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
	torrentio *services.TorrentioClient,
	channelManager *livetv.ChannelManager,
	settingsManager *settings.Manager,
	epgManager *epg.Manager,
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
		torrentio:       torrentio,
		channelManager:  channelManager,
		settingsManager: settingsManager,
		epgManager:      epgManager,
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
		go h.addCollectionMovies(ctx, collection.TMDBID, req.Monitored, req.QualityProfile)
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
	// Get all movies
	movies, err := h.movieStore.List(ctx, 0, 10000, nil)
	if err != nil {
		fmt.Printf("[Collection Sync] Failed to list movies: %v\n", err)
		return err
	}

	fmt.Printf("[Collection Sync] Starting scan of %d movies...\n", len(movies))
	linked := 0
	skipped := 0
	errors := 0

	for i, movie := range movies {
		// Skip if already linked to a collection
		if movie.CollectionID != nil {
			skipped++
			continue
		}

		// Fetch movie with collection info from TMDB
		_, collection, err := h.tmdbClient.GetMovieWithCollection(ctx, movie.TMDBID)
		if err != nil {
			errors++
			continue
		}

		// If movie belongs to a collection, create/update collection and link
		if collection != nil {
			// Get full collection details
			fullCollection, _, err := h.tmdbClient.GetCollection(ctx, collection.TMDBID)
			if err != nil {
				fmt.Printf("[Collection Sync] Failed to get collection %d for movie %s: %v\n", collection.TMDBID, movie.Title, err)
				errors++
				continue
			}

			// Create or update collection in database
			if err := h.collectionStore.Create(ctx, fullCollection); err != nil {
				fmt.Printf("[Collection Sync] Failed to create collection %s: %v\n", fullCollection.Name, err)
				errors++
				continue
			}

			// Link movie to collection - fullCollection.ID is populated by Create's RETURNING clause
			if err := h.collectionStore.UpdateMovieCollection(ctx, movie.ID, fullCollection.ID); err != nil {
				fmt.Printf("[Collection Sync] Failed to link movie %s (ID:%d) to collection %s (ID:%d): %v\n", 
					movie.Title, movie.ID, fullCollection.Name, fullCollection.ID, err)
				errors++
				continue
			}

			linked++
			fmt.Printf("[Collection Sync] Linked '%s' to '%s'\n", movie.Title, fullCollection.Name)
		}

		// Progress log every 100 movies
		if (i+1)%100 == 0 {
			fmt.Printf("[Collection Sync] Progress: %d/%d movies processed, %d linked, %d skipped, %d errors\n", 
				i+1, len(movies), linked, skipped, errors)
		}

		// Rate limit
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("[Collection Sync] Complete: %d movies linked to collections, %d skipped (already linked), %d errors\n", 
		linked, skipped, errors)
	return nil
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

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid movie ID")
		return
	}

	streams, err := h.streamStore.ListByContent(ctx, "movie", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get streams")
		return
	}

	respondJSON(w, http.StatusOK, streams)
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

	// Find best available stream
	stream, err := h.streamStore.FindBestStream(ctx, "movie", id, movie.QualityProfile)
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

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid series ID")
		return
	}

	episodes, err := h.episodeStore.ListBySeries(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get episodes")
		return
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

	// If we already have a cached stream URL, return it
	if episode.StreamURL != nil && *episode.StreamURL != "" {
		respondJSON(w, http.StatusOK, map[string]string{
			"stream_url": *episode.StreamURL,
		})
		return
	}

	// Otherwise find and generate new stream
	series, err := h.seriesStore.Get(ctx, episode.SeriesID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get series")
		return
	}

	stream, err := h.streamStore.FindBestStream(ctx, "series", episode.ID, series.QualityProfile)
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
		"version": "1.0.0",
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

	respondJSON(w, http.StatusOK, channels)
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

	// Parse date parameter (default to today)
	dateStr := r.URL.Query().Get("date")
	date := time.Now()
	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = parsedDate
		}
	}

	programs := h.epgManager.GetEPG(id, date)
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

		if err := h.settingsManager.Update(&newSettings); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}

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
		// First, scan existing movies for collections and link them
		if h.collectionStore != nil && h.movieStore != nil {
			err = h.scanAndLinkCollections(ctx)
		}
		// Then sync incomplete collections (add missing movies)
		if h.collectionStore != nil && h.settingsManager != nil {
			settings := h.settingsManager.Get()
			if settings.AutoAddCollections {
				fmt.Println("[Collection Sync] Phase 2: Adding missing movies from incomplete collections...")
				collections, _, _ := h.collectionStore.GetCollectionsWithProgress(ctx, 100, 0)
				fmt.Printf("[Collection Sync] Found %d collections to check\n", len(collections))
				incomplete := 0
				for _, coll := range collections {
					if coll.MoviesInLibrary < coll.TotalMovies {
						incomplete++
						fmt.Printf("[Collection Sync] '%s' is incomplete (%d/%d), adding missing movies...\n", 
							coll.Name, coll.MoviesInLibrary, coll.TotalMovies)
						h.addCollectionMovies(ctx, coll.TMDBID, true, "default")
						time.Sleep(500 * time.Millisecond) // Rate limit
					}
				}
				fmt.Printf("[Collection Sync] Phase 2 complete: processed %d incomplete collections\n", incomplete)
			} else {
				fmt.Println("[Collection Sync] Phase 2 skipped: AutoAddCollections is disabled")
			}
		} else {
			fmt.Println("[Collection Sync] Phase 2 skipped: collectionStore or settingsManager is nil")
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
