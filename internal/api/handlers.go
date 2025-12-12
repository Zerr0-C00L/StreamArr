package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
			enriched[i].HasEPG = h.epgManager.HasEPG(ch.ID)
			enriched[i].CurrentProgram = h.epgManager.GetCurrentProgram(ch.ID)
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
	Version   = "dev"
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
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/Zerr0-C00L/StreamArr/commits/%s", branch))
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
	updateScript := "./update.sh"
	if _, err := os.Stat(updateScript); os.IsNotExist(err) {
		respondError(w, http.StatusNotImplemented, "Update script not found. Please update manually.")
		return
	}

	// Get update branch from settings
	branch := "main"
	if h.settings != nil {
		settings := h.settings.Get()
		if settings.UpdateBranch != "" {
			branch = settings.UpdateBranch
		}
	}

	log.Println("[Update] Starting update process...")
	
	// Run update script completely detached using nohup + setsid
	// This ensures the script survives even when the server process dies
	cmd := exec.Command("/bin/bash", "-c", 
		fmt.Sprintf("nohup setsid /bin/bash %s %s > /opt/StreamArr/logs/update.log 2>&1 &", updateScript, branch))
	cmd.Dir = "."
	
	// Start the command but don't wait for it
	if err := cmd.Start(); err != nil {
		log.Printf("[Update] Failed to start update: %v", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start update: %v", err))
		return
	}

	// Detach from the process immediately
	go cmd.Wait()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Update started. The server will restart shortly. Check logs/update.log for progress.",
	})
}
