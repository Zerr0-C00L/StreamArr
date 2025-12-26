package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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
	"github.com/Zerr0-C00L/StreamArr/internal/services/streams"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
	"github.com/gorilla/mux"
)

// Version information - injected at build time via ldflags
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type Handler struct {
	movieStore      *database.MovieStore
	seriesStore     *database.SeriesStore
	episodeStore    *database.EpisodeStore
	streamStore     *database.StreamStore
	settingsStore   *database.SettingsStore
	userStore       *database.UserStore
	collectionStore *database.CollectionStore
	blacklistStore  *database.BlacklistStore
	tmdbClient      *services.TMDBClient
	rdClient        *services.RealDebridClient
	channelManager  *livetv.ChannelManager
	settingsManager *settings.Manager
	epgManager      *epg.Manager
	streamProvider  *providers.MultiProvider
	mdbSyncService  *services.MDBListSyncService
	// Phase 1: Smart Stream Caching
	streamCacheStore *database.StreamCacheStore
	streamService    interface{} // Will be *streams.StreamService if initialized
	cacheScanner     *CacheScanner
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
	blacklistStore *database.BlacklistStore,
	tmdbClient *services.TMDBClient,
	rdClient *services.RealDebridClient,
	channelManager *livetv.ChannelManager,
	settingsManager *settings.Manager,
	epgManager *epg.Manager,
	streamProvider *providers.MultiProvider,
	mdbSyncService *services.MDBListSyncService,
	streamCacheStore *database.StreamCacheStore,
	streamService interface{},
	cacheScanner *CacheScanner,
) *Handler {
	return &Handler{
		movieStore:       movieStore,
		seriesStore:      seriesStore,
		episodeStore:     episodeStore,
		streamStore:      streamStore,
		settingsStore:    settingsStore,
		userStore:        userStore,
		collectionStore:  collectionStore,
		blacklistStore:   blacklistStore,
		tmdbClient:       tmdbClient,
		rdClient:         rdClient,
		channelManager:   channelManager,
		settingsManager:  settingsManager,
		epgManager:       epgManager,
		streamProvider:   streamProvider,
		mdbSyncService:   mdbSyncService,
		streamCacheStore: streamCacheStore,
		streamService:    streamService,
		cacheScanner:     cacheScanner,
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

	// Sorting parameters
	sortBy := r.URL.Query().Get("sort") // title, date_added, release_date, rating, runtime, monitored, genre
	sortOrder := r.URL.Query().Get("order") // asc, desc
	if sortOrder == "" {
		sortOrder = "asc"
	}

	movies, err := h.movieStore.List(ctx, offset, limit, monitored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Apply Bollywood filter for library listing
	if h.settingsManager != nil {
		st := h.settingsManager.Get()
		if st.BlockBollywood {
			filtered := make([]*models.Movie, 0, len(movies))
			for _, m := range movies {
				if !services.IsIndianMovie(m) {
					filtered = append(filtered, m)
				}
			}
			movies = filtered
		}
	}

	// Apply sorting
	if sortBy != "" {
		sortMovies(movies, sortBy, sortOrder)
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

	// Bollywood blocking (Indian-origin content)
	if h.settingsManager != nil {
		st := h.settingsManager.Get()
		if st.BlockBollywood && services.IsIndianMovie(movie) {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   "content blocked by settings (bollywood)",
			})
			return
		}
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
			// Use background context since request context will be canceled after response
			go h.addCollectionMovies(context.Background(), collection.TMDBID, req.Monitored, req.QualityProfile)
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

// sortStreams sorts a slice of streams based on user's sorting preferences
func sortStreams(streams []providers.TorrentioStream, sortOrder, sortPrefer string) {
	if sortOrder == "" {
		sortOrder = "quality,size,seeders"
	}
	if sortPrefer == "" {
		sortPrefer = "best"
	}

	sortFields := strings.Split(sortOrder, ",")

	sort.Slice(streams, func(i, j int) bool {
		for _, field := range sortFields {
			field = strings.TrimSpace(field)
			cmp := compareStreamsByField(streams[i], streams[j], field, sortPrefer)
			if cmp != 0 {
				return cmp > 0 // We want better streams first
			}
			// cmp == 0, continue to next field
		}
		return false
	})
}

// compareStreamsByField compares two streams by a specific field
// Returns: 1 if a > b (a is better), -1 if a < b (b is better), 0 if equal
func compareStreamsByField(a, b providers.TorrentioStream, field string, prefer string) int {
	switch field {
	case "quality":
		aQuality := parseQualityValue(a.Quality)
		bQuality := parseQualityValue(b.Quality)
		if prefer == "smallest" || prefer == "lowest" {
			// For smallest preference, lower quality is better
			if aQuality < bQuality {
				return 1
			} else if aQuality > bQuality {
				return -1
			}
		} else {
			// Default: higher quality is better
			if aQuality > bQuality {
				return 1
			} else if aQuality < bQuality {
				return -1
			}
		}
	case "size":
		if prefer == "smallest" || prefer == "lowest" {
			// Smaller size is better
			if a.Size < b.Size && a.Size > 0 {
				return 1
			} else if a.Size > b.Size && b.Size > 0 {
				return -1
			}
		} else {
			// Default: larger size is better (usually better quality)
			if a.Size > b.Size {
				return 1
			} else if a.Size < b.Size {
				return -1
			}
		}
	case "seeders":
		if prefer == "smallest" || prefer == "lowest" {
			// Fewer seeders (unusual preference)
			if a.Seeders < b.Seeders {
				return 1
			} else if a.Seeders > b.Seeders {
				return -1
			}
		} else {
			// Default: more seeders is better
			if a.Seeders > b.Seeders {
				return 1
			} else if a.Seeders < b.Seeders {
				return -1
			}
		}
	}
	return 0
}

// parseQualityValue converts a quality string to an integer value for comparison
func parseQualityValue(quality string) int {
	q := strings.ToUpper(quality)

	// Check for resolution-based quality indicators
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

	// Default to 0 for unknown quality
	return 0
}

// GetMediaVideos handles GET /api/movies/{id}/videos and /api/series/{id}/videos
func (h *Handler) GetMediaVideos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	// Determine if this is a movie or series based on the URL path
	path := r.URL.Path
	var mediaType string
	var tmdbID int
	
	if strings.Contains(path, "/movies/") {
		mediaType = "movie"
		movie, err := h.movieStore.Get(ctx, id)
		if err != nil {
			respondError(w, http.StatusNotFound, "movie not found")
			return
		}
		tmdbID = movie.TMDBID
	} else {
		mediaType = "tv"
		series, err := h.seriesStore.Get(ctx, id)
		if err != nil {
			respondError(w, http.StatusNotFound, "series not found")
			return
		}
		tmdbID = series.TMDBID
	}

	// Fetch videos from TMDB
	videos, err := h.tmdbClient.GetVideos(ctx, mediaType, tmdbID)
	if err != nil {
		log.Printf("Failed to fetch videos for %s %d: %v", mediaType, tmdbID, err)
		respondError(w, http.StatusInternalServerError, "failed to fetch videos")
		return
	}

	// Sort videos: official trailers first, then teasers, then others
	// Also prioritize YouTube videos
	sortedVideos := make([]services.Video, len(videos))
	copy(sortedVideos, videos)
	
	sort.SliceStable(sortedVideos, func(i, j int) bool {
		vi, vj := sortedVideos[i], sortedVideos[j]
		
		// YouTube videos first
		if vi.Site == "YouTube" && vj.Site != "YouTube" {
			return true
		}
		if vi.Site != "YouTube" && vj.Site == "YouTube" {
			return false
		}
		
		// Official videos first
		if vi.Official && !vj.Official {
			return true
		}
		if !vi.Official && vj.Official {
			return false
		}
		
		// Trailers before teasers, teasers before others
		typeOrder := map[string]int{"Trailer": 0, "Teaser": 1, "Clip": 2, "Featurette": 3, "Behind the Scenes": 4}
		orderI, okI := typeOrder[vi.Type]
		orderJ, okJ := typeOrder[vj.Type]
		if !okI {
			orderI = 99
		}
		if !okJ {
			orderJ = 99
		}
		return orderI < orderJ
	})

	respondJSON(w, http.StatusOK, sortedVideos)
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

	// Phase 1: Prepare cached stream (if exists) - will be included with live results
	var cachedStreamObj map[string]interface{}
	if h.streamCacheStore != nil {
		cached, err := h.streamCacheStore.GetCachedStream(ctx, int(id))
		if err == nil && cached != nil && cached.IsAvailable {
			log.Printf("[CACHE-HIT] ⚡ Cached stream available for movie %d (quality: %d, checked: %v ago)",
				id, cached.QualityScore, time.Since(cached.LastChecked).Round(time.Minute))

			// Prepare cached stream in API format (will be returned with live streams)
			cachedStreamObj = map[string]interface{}{
				"title":         fmt.Sprintf("⚡ [CACHED] %s %s %s", cached.Resolution, cached.HDRType, cached.AudioFormat),
				"url":           cached.StreamURL,
				"quality":       cached.Resolution,
				"size_gb":       cached.FileSizeGB,
				"source":        "Phase1-Cache",
				"cached":        true,
				"quality_score": cached.QualityScore,
				"last_checked":  cached.LastChecked,
				"indexer":       cached.Indexer,
			}
		}
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

	// Log full metadata for debugging
	log.Printf("[DEBUG] Movie %d (%s) metadata: %+v", id, movie.Title, movie.Metadata)

	// If this movie came from Balkan VOD import, expose only Balkan VOD streams
	if isBalkanVODMovie(movie) {
		apiStreams := buildBalkanVODStreams(movie)
		log.Printf("Returning %d Balkan VOD streams for movie %d (%s)", len(apiStreams), id, movie.Title)
		respondJSON(w, http.StatusOK, apiStreams)
		return
	}

	// If this movie came from IPTV VOD import, expose only VOD playlist links
	if isIPTVVODMovie(movie) {
		apiStreams := buildIPTVVODStreams(movie)
		log.Printf("Returning %d IPTV VOD streams for movie %d (%s)", len(apiStreams), id, movie.Title)
		respondJSON(w, http.StatusOK, apiStreams)
		return
	}

	if imdbID == "" {
		log.Printf("[ERROR] Movie %d (%s) has no IMDB ID in metadata - cannot fetch streams", id, movie.Title)
		respondError(w, http.StatusBadRequest, "movie has no IMDB ID")
		return
	}

	// Validate IMDb ID format (should be tt followed by digits)
	if !strings.HasPrefix(imdbID, "tt") {
		log.Printf("[WARNING] Movie %d (%s) has invalid IMDB ID format: %s", id, movie.Title, imdbID)
	}

	// Fetch live streams from providers
	if h.streamProvider == nil {
		log.Printf("Stream provider not configured")
		respondError(w, http.StatusServiceUnavailable, "stream provider not configured")
		return
	}

	log.Printf("[STREAM-FETCH] Fetching streams for movie %d (%s) with IMDB ID: %s", id, movie.Title, imdbID)

	// Extract release year for filtering
	releaseYear := 0
	if movie.ReleaseDate != nil && !movie.ReleaseDate.IsZero() {
		releaseYear = movie.ReleaseDate.Year()
	}
	log.Printf("[STREAM-FETCH] Movie %s (%s) release year: %d", movie.Title, imdbID, releaseYear)

	// Use the new year-aware method
	providerStreams, err := h.streamProvider.GetMovieStreamsWithYear(imdbID, releaseYear)
	if err != nil {
		log.Printf("[ERROR] Failed to get streams for movie %d (%s, %s): %v", id, movie.Title, imdbID, err)
		respondJSON(w, http.StatusOK, []interface{}{}) // Return empty array instead of error
		return
	}

	log.Printf("[STREAM-FETCH] Retrieved %d streams before filtering for %s (%s)", len(providerStreams), movie.Title, imdbID)

	// Validate streams match the requested movie by checking title
	validatedStreams := validateMovieStreams(providerStreams, movie.Title, releaseYear)
	if len(validatedStreams) < len(providerStreams) {
		log.Printf("[VALIDATION] Removed %d streams with mismatched titles (kept %d)",
			len(providerStreams)-len(validatedStreams), len(validatedStreams))
	}
	providerStreams = validatedStreams

	// Streams are already filtered by year from GetMovieStreamsWithYear

	// Check Real-Debrid instant availability for TorrentsDB streams FIRST (needed for caching)
	// This sets the Cached field based on actual Real-Debrid cache status
	// Note: Torrentio streams already have Cached status from the [RD+] indicator
	if h.rdClient != nil && len(providerStreams) > 0 {
		// Collect all unique info hashes from TorrentsDB
		hashes := make([]string, 0)
		hashMap := make(map[string]int) // hash -> stream index
		for i, s := range providerStreams {
			if s.InfoHash != "" && s.Source == "TorrentsDB" {
				hashes = append(hashes, s.InfoHash)
				hashMap[s.InfoHash] = i
			}
		}

		// Check availability in Real-Debrid for TorrentsDB streams only
		if len(hashes) > 0 {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			availability, err := h.rdClient.CheckInstantAvailability(ctx, hashes)
			cancel()

			if err == nil {
				// Update cached status based on Real-Debrid availability
				for hash, isAvailable := range availability {
					if idx, exists := hashMap[hash]; exists {
						providerStreams[idx].Cached = isAvailable
					}
				}
				log.Printf("[RD-CHECK] Updated cached status for %d TorrentsDB streams", len(availability))
			} else {
				log.Printf("[RD-CHECK] Error checking Real-Debrid availability: %v", err)
			}
		}
	}

	// Phase 1: Cache the best available stream BEFORE user filters (for instant playback next time)
	// This ensures we always cache the best quality, regardless of user's filter preferences
	if h.streamCacheStore != nil && h.streamService != nil && len(providerStreams) > 0 {
		log.Printf("[CACHE-PHASE1] Checking %d streams for caching (movie ID: %d)", len(providerStreams), id)
		// Find the best cached stream
		var bestCached *providers.TorrentioStream
		for i := range providerStreams {
			if providerStreams[i].Cached {
				bestCached = &providerStreams[i]
				log.Printf("[CACHE-PHASE1] Found best cached stream: %s (source: %s, hash: %s)",
					bestCached.Name, bestCached.Source, bestCached.InfoHash)
				break
			}
		}

		if bestCached != nil {
			// Extract hash from URL if InfoHash is empty (Torrentio case)
			hash := bestCached.InfoHash
			// For Torrentio streams, Title contains the actual filename (set in stremio_generic.go)
			// Name contains the formatted display text like "[RD+] Torrentio\n1080p"
			torrentName := bestCached.Title
			if hash == "" && bestCached.URL != "" {
				// Torrentio URLs contain the hash in the path
				// Example: /resolve/realdebrid/APIKEY/HASH/null/1/filename.mp4
				parts := strings.Split(bestCached.URL, "/")
				for _, part := range parts {
					if len(part) == 40 { // Torrent hashes are 40 chars
						hash = part
						break
					}
				}
			}

			log.Printf("[CACHE-PHASE1] Processing stream for caching: hash=%s, torrentName=%s",
				hash, torrentName)
			// Convert to Phase 1 format
			phase1Stream := models.TorrentStream{
				Hash:        hash,
				Title:       bestCached.Name, // Use Name for display title
				TorrentName: torrentName,     // Use Title (which contains actual filename) for scoring
				Resolution:  bestCached.Quality,
				SizeGB:      float64(bestCached.Size) / (1024 * 1024 * 1024),
				Seeders:     bestCached.Seeders,
				Indexer:     bestCached.Source,
			}

			// Score it using Phase 1 quality algorithm
			if svc, ok := h.streamService.(*streams.StreamService); ok {
				log.Printf("[CACHE-PHASE1] Calling ParseStreamFromTorrentName with name='%s', hash='%s', indexer='%s'",
					phase1Stream.TorrentName, phase1Stream.Hash, phase1Stream.Indexer)
				scoredStream := svc.ParseStreamFromTorrentName(
					phase1Stream.TorrentName,
					phase1Stream.Hash,
					phase1Stream.Indexer,
					phase1Stream.Seeders,
				)

				// Calculate the actual quality score
				quality := streams.StreamQuality{
					Resolution:  scoredStream.Resolution,
					HDRType:     scoredStream.HDRType,
					AudioFormat: scoredStream.AudioFormat,
					Source:      scoredStream.Source,
					Codec:       scoredStream.Codec,
					SizeGB:      scoredStream.SizeGB,
					Seeders:     scoredStream.Seeders,
				}
				scoreBreakdown := streams.CalculateScore(quality)
				scoredStream.QualityScore = scoreBreakdown.TotalScore

				log.Printf("[CACHE-PHASE1] Scoring result: quality=%d, res=%s, hdr=%s, audio=%s, source=%s, codec=%s",
					scoredStream.QualityScore, scoredStream.Resolution, scoredStream.HDRType,
					scoredStream.AudioFormat, scoredStream.Source, scoredStream.Codec)
				phase1Stream.QualityScore = scoredStream.QualityScore
				phase1Stream.Resolution = scoredStream.Resolution
				phase1Stream.HDRType = scoredStream.HDRType
				phase1Stream.AudioFormat = scoredStream.AudioFormat
				phase1Stream.Source = scoredStream.Source
				phase1Stream.Codec = scoredStream.Codec

				// Cache it with the stream URL (async to not block response)
				go func() {
					cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := h.streamCacheStore.CacheStream(cacheCtx, int(id), phase1Stream, bestCached.URL); err != nil {
						log.Printf("[CACHE-WRITE] ❌ Failed to cache stream for movie %d: %v", id, err)
					} else {
						log.Printf("[CACHE-WRITE] ✅ Cached stream for movie %d (quality: %d, res: %s, hdr: %s)",
							id, phase1Stream.QualityScore, phase1Stream.Resolution, phase1Stream.HDRType)
					}
				}()
			}
		}
	}

	// Apply quality filters from user settings
	if h.settingsManager != nil && h.streamService != nil {
		settings := h.settingsManager.Get()
		excludedQualities := settings.ExcludedQualities
		
		// Apply quality type exclusions (REMUX, HDR, DV, etc.)
		if excludedQualities != "" {
			log.Printf("[FILTER] Applying quality exclusions: %s", excludedQualities)
			
			filteredStreams := make([]providers.TorrentioStream, 0, len(providerStreams))
			for _, ps := range providerStreams {
				// Parse stream to get quality details
				if svc, ok := h.streamService.(*streams.StreamService); ok {
					parsed := svc.ParseStreamFromTorrentName(ps.Title, ps.InfoHash, ps.Source, 0)
					
					// Check if should be excluded based on quality type
					if svc.ShouldExcludeByQualityType(ps.Title, parsed.Resolution, parsed.HDRType, excludedQualities) {
						log.Printf("[FILTER] 🚫 Excluded stream: %s", ps.Title)
						continue
					}
				}
				filteredStreams = append(filteredStreams, ps)
			}

			if len(filteredStreams) < len(providerStreams) {
				log.Printf("[FILTER] Filtered out %d streams (kept %d/%d)",
					len(providerStreams)-len(filteredStreams), len(filteredStreams), len(providerStreams))
			}
			providerStreams = filteredStreams
		}
		
		// Apply release group/language filters if enabled
		if settings.EnableReleaseFilters {
			log.Printf("[FILTER] Applying release filters: excludedGroups=%s, excludedLanguages=%s",
				settings.ExcludedReleaseGroups, settings.ExcludedLanguageTags)

			filteredStreams := make([]providers.TorrentioStream, 0, len(providerStreams))
			for _, ps := range providerStreams {
				ts := models.TorrentStream{
					Title:       ps.Title,
					TorrentName: ps.Name,
					Resolution:  ps.Quality,
					Indexer:     ps.Source,
				}

				if svc, ok := h.streamService.(*streams.StreamService); ok {
					if !svc.ShouldFilterStream(ts, settings.ExcludedReleaseGroups, "", settings.ExcludedLanguageTags) {
						filteredStreams = append(filteredStreams, ps)
					}
				} else {
					filteredStreams = append(filteredStreams, ps)
				}
			}

			if len(filteredStreams) < len(providerStreams) {
				log.Printf("[FILTER] Filtered out %d streams by release filters (kept %d/%d)",
					len(providerStreams)-len(filteredStreams), len(filteredStreams), len(providerStreams))
			}
			providerStreams = filteredStreams
		}
	}

	// Log stream count
	log.Printf("[STREAMS] Returning %d streams for movie %s", len(providerStreams), movie.Title)

	// Log cached vs non-cached breakdown
	cachedCount := 0
	for _, s := range providerStreams {
		if s.Cached {
			cachedCount++
		}
	}
	log.Printf("✅ Found %d streams for movie %d (%s) → %d CACHED, %d UNCACHED",
		len(providerStreams), id, movie.Title, cachedCount, len(providerStreams)-cachedCount)

	// Apply user's sorting preferences to streams
	sortOrder := "quality,size,seeders" // default
	sortPrefer := "best"                // default
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		if settings.StreamSortOrder != "" {
			sortOrder = settings.StreamSortOrder
		}
		if settings.StreamSortPrefer != "" {
			sortPrefer = settings.StreamSortPrefer
		}
	}
	log.Printf("[SORT-UI] Sorting %d streams with order: %s, preference: %s", len(providerStreams), sortOrder, sortPrefer)
	sortStreams(providerStreams, sortOrder, sortPrefer)

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

		// Log individual stream with cached status
		cachedStr := "⚡ CACHED"
		if !ps.Cached {
			cachedStr = "⏳ UNCACHED"
		}
		log.Printf("[STREAM-DETAIL] %s | %s | %s | Size: %.2f GB",
			cachedStr, ps.Quality, ps.Title, float64(ps.Size)/(1024*1024*1024))

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

	// Prepend cached stream if available (for instant playback priority)
	if cachedStreamObj != nil {
		log.Printf("[STREAMS] ⚡ Adding cached stream as first option for instant playback")
		allStreams := make([]interface{}, 0, len(apiStreams)+1)
		allStreams = append(allStreams, cachedStreamObj)
		for _, s := range apiStreams {
			allStreams = append(allStreams, s)
		}
		respondJSON(w, http.StatusOK, allStreams)
		return
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

	// Check if this is Balkan VOD series and return Balkan VOD streams
	ctx := r.Context()
	series, err := h.seriesStore.GetByIMDBID(ctx, imdbID)
	if err == nil && series != nil && isBalkanVODSeries(series) {
		apiStreams := buildBalkanVODSeriesStreams(series, season, episode)
		log.Printf("Returning %d Balkan VOD streams for series %s S%02dE%02d", len(apiStreams), imdbID, season, episode)
		respondJSON(w, http.StatusOK, apiStreams)
		return
	}

	// Check if this is IPTV VOD series and return IPTV streams
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

	log.Printf("Found %d streams for series %s S%02dE%02d", len(providerStreams), imdbID, season, episode)

	// Apply user's sorting preferences to streams
	sortOrder := "quality,size,seeders" // default
	sortPrefer := "best"                // default
	if h.settingsManager != nil {
		settings := h.settingsManager.Get()
		if settings.StreamSortOrder != "" {
			sortOrder = settings.StreamSortOrder
		}
		if settings.StreamSortPrefer != "" {
			sortPrefer = settings.StreamSortPrefer
		}
	}
	log.Printf("[SORT-UI] Sorting %d series streams with order: %s, preference: %s", len(providerStreams), sortOrder, sortPrefer)
	sortStreams(providerStreams, sortOrder, sortPrefer)

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

	// FILTERING DISABLED - All filtering is now handled by Torrentio addon URL configuration
	// Build exclude patterns from settings
	var excludePatterns []string
	// EnableReleaseFilters, ExcludedQualities, etc. have been removed from settings

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

	// Sorting parameters
	sortBy := r.URL.Query().Get("sort") // title, date_added, release_date, rating, monitored, genre
	sortOrder := r.URL.Query().Get("order") // asc, desc
	if sortOrder == "" {
		sortOrder = "asc"
	}

	series, err := h.seriesStore.List(ctx, offset, limit, monitored)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Apply Bollywood filter for library listing
	if h.settingsManager != nil {
		st := h.settingsManager.Get()
		if st.BlockBollywood {
			filtered := make([]*models.Series, 0, len(series))
			for _, s := range series {
				if !services.IsIndianSeries(s) {
					filtered = append(filtered, s)
				}
			}
			series = filtered
		}
	}

	// Apply sorting
	if sortBy != "" {
		sortSeries(series, sortBy, sortOrder)
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
		respondJSON(w, http.StatusOK, existingSeries)
		return
	}

	// Fetch series details from TMDB
	tmdbSeries, err := h.tmdbClient.GetSeries(ctx, int(req.TMDBID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to fetch series from TMDB: %v", err))
		return
	}

	// Bollywood blocking (Indian-origin content)
	if h.settingsManager != nil {
		st := h.settingsManager.Get()
		if st.BlockBollywood && services.IsIndianSeries(tmdbSeries) {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   "content blocked by settings (bollywood)",
			})
			return
		}
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

// CleanupBollywoodLibrary deletes Indian-origin movies and series from the library when blocking is enabled
func (h *Handler) CleanupBollywoodLibrary(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil {
		respondError(w, http.StatusServiceUnavailable, "settings manager not initialized")
		return
	}
	st := h.settingsManager.Get()
	if !st.BlockBollywood {
		respondError(w, http.StatusBadRequest, "enable 'Block Bollywood' in Settings first")
		return
	}

	ctx := r.Context()

	moviesDeleted := 0
	seriesDeleted := 0

	// Paginate through movies
	offset := 0
	limit := 200
	for {
		movies, err := h.movieStore.List(ctx, offset, limit, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list movies: %v", err))
			return
		}
		if len(movies) == 0 {
			break
		}
		for _, m := range movies {
			if services.IsIndianMovie(m) {
				if err := h.movieStore.Delete(ctx, m.ID); err == nil {
					moviesDeleted++
				}
			}
		}
		offset += limit
	}

	// Paginate through series
	offset = 0
	for {
		series, err := h.seriesStore.List(ctx, offset, limit, nil)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list series: %v", err))
			return
		}
		if len(series) == 0 {
			break
		}
		for _, s := range series {
			if services.IsIndianSeries(s) {
				if err := h.seriesStore.Delete(ctx, s.ID); err == nil {
					seriesDeleted++
				}
			}
		}
		offset += limit
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"movies_deleted": moviesDeleted,
		"series_deleted": seriesDeleted,
	})
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

	// Get series details first to check if it's Balkan VOD
	series, err := h.seriesStore.Get(ctx, id)
	if err != nil {
		log.Printf("Error fetching series %d: %v", id, err)
		respondJSON(w, http.StatusOK, []*models.Episode{})
		return
	}

	// Check if this is a Balkan VOD series - if so, build episodes from metadata
	if isBalkanVODSeries(series) {
		episodes := buildBalkanVODEpisodes(series)

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

		log.Printf("Returning %d Balkan VOD episodes for series %d", len(episodes), id)
		respondJSON(w, http.StatusOK, episodes)
		return
	}

	episodes, err := h.episodeStore.ListBySeries(ctx, id)
	if err != nil {
		log.Printf("Error fetching episodes for series %d: %v", id, err)
		respondError(w, http.StatusInternalServerError, "failed to get episodes")
		return
	}

	// If no episodes in database, fetch from TMDB
	if len(episodes) == 0 {

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
	// Filters are now handled by addon configuration

	// Always find best stream
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

// GetChannelCategories handles GET /api/channels/categories - returns list of available categories
func (h *Handler) GetChannelCategories(w http.ResponseWriter, r *http.Request) {
	if h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "channel manager not initialized")
		return
	}

	categories := h.channelManager.GetCategories()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
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

// ProxyChannelStream proxies the channel stream to avoid CORS issues
func (h *Handler) ProxyChannelStream(w http.ResponseWriter, r *http.Request) {
	// Get stream URL from query parameter
	streamURL := r.URL.Query().Get("url")
	if streamURL == "" {
		// Fallback to channel ID lookup
		if h.channelManager == nil {
			http.Error(w, "channel manager not initialized", http.StatusServiceUnavailable)
			return
		}

		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "missing stream URL or channel ID", http.StatusBadRequest)
			return
		}

		channel, err := h.channelManager.GetChannel(id)
		if err != nil {
			log.Printf("Channel not found for ID %s: %v", id, err)
			http.Error(w, "channel not found", http.StatusNotFound)
			return
		}
		streamURL = channel.StreamURL
	}

	log.Printf("Proxying stream: %s", streamURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create request to stream server
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch stream from %s: %v", streamURL, err)
		http.Error(w, fmt.Sprintf("failed to fetch stream: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Ensure proper content type for HLS
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		if strings.HasSuffix(streamURL, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		} else if strings.HasSuffix(streamURL, ".ts") {
			w.Header().Set("Content-Type", "video/mp2t")
		}
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Stream the response
	io.Copy(w, resp.Body)
}

// GetTVGuide handles GET /api/channels/epg/guide - returns EPG data for all channels in a time range
func (h *Handler) GetTVGuide(w http.ResponseWriter, r *http.Request) {
	if h.epgManager == nil || h.channelManager == nil {
		respondError(w, http.StatusServiceUnavailable, "EPG or channel manager not initialized")
		return
	}

	// Parse query parameters
	category := r.URL.Query().Get("category")
	limitStr := r.URL.Query().Get("limit")
	hoursStr := r.URL.Query().Get("hours")

	// Default limit is 20 channels
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Default hours is 4 (4 hours of EPG data)
	hours := 4
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 24 {
			hours = h
		}
	}

	// Get channels (optionally filtered by category)
	var channels []*livetv.Channel
	if category != "" && category != "all" {
		channels = h.channelManager.GetChannelsByCategory(category)
	} else {
		channels = h.channelManager.GetAllChannels()
	}

	// Limit channels
	if len(channels) > limit {
		channels = channels[:limit]
	}

	now := time.Now()
	startTime := now.Add(-30 * time.Minute) // Start 30 minutes before now
	endTime := now.Add(time.Duration(hours) * time.Hour)

	type GuideProgram struct {
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Category    string    `json:"category"`
		IsLive      bool      `json:"is_live"`
	}

	type GuideChannel struct {
		ID       string         `json:"id"`
		Name     string         `json:"name"`
		Logo     string         `json:"logo"`
		Category string         `json:"category"`
		Programs []GuideProgram `json:"programs"`
	}

	type NowPlaying struct {
		ChannelID   string    `json:"channel_id"`
		ChannelName string    `json:"channel_name"`
		ChannelLogo string    `json:"channel_logo"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Progress    float64   `json:"progress"` // 0-100
	}

	guideChannels := make([]GuideChannel, 0, len(channels))
	var nowPlaying *NowPlaying

	for _, ch := range channels {
		// Get EPG programs for this channel
		programs := h.epgManager.GetEPGWithFallback(ch.ID, ch.Name, now)

		guidePrograms := make([]GuideProgram, 0)
		for _, p := range programs {
			// Filter to time range
			if p.EndTime.Before(startTime) || p.StartTime.After(endTime) {
				continue
			}

			isLive := now.After(p.StartTime) && now.Before(p.EndTime)

			gp := GuideProgram{
				Title:       p.Title,
				Description: p.Description,
				StartTime:   p.StartTime,
				EndTime:     p.EndTime,
				Category:    p.Category,
				IsLive:      isLive,
			}
			guidePrograms = append(guidePrograms, gp)

			// Track first "now playing" we find for the hero section
			if isLive && nowPlaying == nil {
				duration := p.EndTime.Sub(p.StartTime).Seconds()
				elapsed := now.Sub(p.StartTime).Seconds()
				progress := 0.0
				if duration > 0 {
					progress = (elapsed / duration) * 100
				}
				nowPlaying = &NowPlaying{
					ChannelID:   ch.ID,
					ChannelName: ch.Name,
					ChannelLogo: ch.Logo,
					Title:       p.Title,
					Description: p.Description,
					StartTime:   p.StartTime,
					EndTime:     p.EndTime,
					Progress:    progress,
				}
			}
		}

		guideChannels = append(guideChannels, GuideChannel{
			ID:       ch.ID,
			Name:     ch.Name,
			Logo:     ch.Logo,
			Category: ch.Category,
			Programs: guidePrograms,
		})
	}

	// Generate time slots for the grid header
	timeSlots := make([]time.Time, 0)
	slotTime := startTime.Truncate(30 * time.Minute)
	for slotTime.Before(endTime) {
		timeSlots = append(timeSlots, slotTime)
		slotTime = slotTime.Add(30 * time.Minute)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"channels":    guideChannels,
		"now_playing": nowPlaying,
		"time_slots":  timeSlots,
		"start_time":  startTime,
		"end_time":    endTime,
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

		// Get old settings to detect changes
		oldSettings := h.settingsManager.Get()

		if err := h.settingsManager.Update(&newSettings); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}

		// Log playlist filter changes
		if oldSettings.OnlyCachedStreams != newSettings.OnlyCachedStreams {
			if newSettings.OnlyCachedStreams {
				log.Printf("[Settings] ✅ Playlist filter enabled: Only cached streams will be included in IPTV playlists")
			} else {
				log.Printf("[Settings] ℹ️ Playlist filter disabled: Full library will be included in IPTV playlists")
			}
			log.Printf("[Settings] 📺 Users should refresh their IPTV apps to see updated playlists")
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
					Name:               s.Name,
					URL:                s.URL,
					Enabled:            s.Enabled,
					EPGURL:             s.EPGURL,
					SelectedCategories: s.SelectedCategories,
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

		// Live TV settings applied

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

		// Trigger MDBList sync in background when MDBList lists are configured
		if h.mdbSyncService != nil && newSettings.MDBListLists != "" && newSettings.MDBListLists != "[]" {
			go func() {
				log.Println("[Settings] MDBList lists updated, triggering sync...")
				h.runService(services.ServiceMDBListSync)
			}()
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

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	collection, err := h.collectionStore.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	// Get full collection details from TMDB including all movies
	_, tmdbMovies, err := h.tmdbClient.GetCollectionWithMovies(ctx, collection.TMDBID)
	if err != nil {
		// Fallback to database movies if TMDB fails
		movies, _ := h.collectionStore.GetMoviesInCollection(ctx, id)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"collection": collection,
			"movies":     movies,
		})
		return
	}

	// Get all library movies with their TMDB IDs to check in_library status
	libraryMovieTMDBIDs := make(map[int]bool)
	libraryMovies, err := h.collectionStore.GetMoviesInCollection(ctx, id)
	if err == nil {
		for _, m := range libraryMovies {
			libraryMovieTMDBIDs[m.TMDBID] = true
		}
	}

	// Mark which movies are in library
	for i := range tmdbMovies {
		if libraryMovieTMDBIDs[tmdbMovies[i].TMDBID] {
			tmdbMovies[i].InLibrary = true
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"collection": collection,
		"movies":     tmdbMovies,
	})
}

// AddCollectionByTMDB handles POST /api/collections/add - adds a collection by TMDB ID
func (h *Handler) AddCollectionByTMDB(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	var req struct {
		TMDBID         int    `json:"tmdb_id"`
		Monitored      bool   `json:"monitored"`
		QualityProfile string `json:"quality_profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TMDBID == 0 {
		respondError(w, http.StatusBadRequest, "tmdb_id is required")
		return
	}

	// Check if collection already exists
	existing, _ := h.collectionStore.GetByTMDBID(ctx, req.TMDBID)
	if existing != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"message":    "Collection already exists",
			"collection": existing,
		})
		return
	}

	// Get collection details from TMDB
	collection, movies, err := h.tmdbClient.GetCollectionWithMovies(ctx, req.TMDBID)
	if err != nil {
		respondError(w, http.StatusNotFound, "collection not found in TMDB")
		return
	}

	// Create the collection in database
	collection.TotalMovies = len(movies)
	if err := h.collectionStore.Create(ctx, collection); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create collection")
		return
	}

	// Start background sync to add all movies (use background context since request context will be canceled)
	go h.addCollectionMovies(context.Background(), collection.TMDBID, req.Monitored, req.QualityProfile)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message":    "Collection added successfully",
		"collection": collection,
	})
}

// SyncCollection handles POST /api/collections/{id}/sync - adds all missing movies from a collection
func (h *Handler) SyncCollection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
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

	// Start background sync (use background context since request context will be canceled after response)
	go h.addCollectionMovies(context.Background(), collection.TMDBID, req.Monitored, req.QualityProfile)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Collection sync started",
		"status":  "syncing",
	})
}

// GetCollectionMovies handles GET /api/collections/{id}/movies
// Returns all movies in the collection from TMDB with in_library status
func (h *Handler) GetCollectionMovies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.collectionStore == nil {
		respondError(w, http.StatusInternalServerError, "collection store not initialized")
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid collection ID")
		return
	}

	// Get the collection to find its TMDB ID
	collection, err := h.collectionStore.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "collection not found")
		return
	}

	// Get full collection details from TMDB including all movies
	_, tmdbMovies, err := h.tmdbClient.GetCollectionWithMovies(ctx, collection.TMDBID)
	if err != nil {
		// Fallback to database movies if TMDB fails
		movies, err := h.collectionStore.GetMoviesInCollection(ctx, id)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to get collection movies")
			return
		}
		respondJSON(w, http.StatusOK, movies)
		return
	}

	// Get all library movies with their TMDB IDs to check in_library status
	libraryMovieTMDBIDs := make(map[int]bool)
	if h.movieStore != nil {
		// Get all movies from library that belong to this collection
		libraryMovies, err := h.collectionStore.GetMoviesInCollection(ctx, id)
		if err == nil {
			for _, m := range libraryMovies {
				libraryMovieTMDBIDs[m.TMDBID] = true
			}
		}
	}

	// Mark which movies are in library
	for i := range tmdbMovies {
		if libraryMovieTMDBIDs[tmdbMovies[i].TMDBID] {
			tmdbMovies[i].InLibrary = true
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"movies": tmdbMovies,
	})
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

// GetTMDBDetails handles GET /api/tmdb/{type}/{id} - get full TMDB details with videos
func (h *Handler) GetTMDBDetails(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	
	mediaType := vars["type"] // "movie" or "tv"
	idStr := vars["id"]
	
	tmdbID, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid TMDB ID")
		return
	}
	
	// Prepare response structure
	response := make(map[string]interface{})
	
	if mediaType == "movie" {
		movie, err := h.tmdbClient.GetMovie(ctx, tmdbID)
		if err != nil {
			respondError(w, http.StatusNotFound, "movie not found")
			return
		}
		response["id"] = movie.TMDBID
		response["title"] = movie.Title
		response["overview"] = movie.Overview
		response["poster_path"] = movie.PosterPath
		response["backdrop_path"] = movie.BackdropPath
		response["release_date"] = movie.ReleaseDate
		response["vote_average"] = movie.VoteAverage
		response["runtime"] = movie.Runtime
		response["genres"] = movie.Genres
	} else if mediaType == "tv" {
		series, err := h.tmdbClient.GetSeries(ctx, tmdbID)
		if err != nil {
			respondError(w, http.StatusNotFound, "series not found")
			return
		}
		response["id"] = series.TMDBID
		response["name"] = series.Title
		response["overview"] = series.Overview
		response["poster_path"] = series.PosterPath
		response["backdrop_path"] = series.BackdropPath
		response["first_air_date"] = series.FirstAirDate
		response["vote_average"] = series.VoteAverage
		response["number_of_seasons"] = series.Seasons
		response["genres"] = series.Genres
	} else {
		respondError(w, http.StatusBadRequest, "invalid media type, must be 'movie' or 'tv'")
		return
	}
	
	// Get videos/trailers
	videos, err := h.tmdbClient.GetVideos(ctx, mediaType, tmdbID)
	if err == nil {
		response["videos"] = map[string]interface{}{
			"results": videos,
		}
	}
	
	respondJSON(w, http.StatusOK, response)
}

// GetPopularCollections handles GET /api/discover/collections - get popular collections from TMDB
func (h *Handler) GetPopularCollections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// TMDB doesn't have a direct "popular collections" endpoint,
	// so we'll search for well-known collection keywords
	popularSearchTerms := []string{
		"Marvel Cinematic Universe",
		"Star Wars",
		"Harry Potter",
		"The Lord of the Rings",
		"Fast & Furious",
		"Jurassic Park",
		"Mission: Impossible",
		"Pirates of the Caribbean",
		"Transformers",
		"James Bond",
		"Batman",
		"Spider-Man",
		"X-Men",
		"The Dark Knight",
		"John Wick",
		"Toy Story",
		"Shrek",
		"Ice Age",
		"Indiana Jones",
		"The Matrix",
	}
	
	var allCollections []*models.Collection
	seen := make(map[int]bool)
	
	for _, term := range popularSearchTerms {
		collections, err := h.tmdbClient.SearchCollections(ctx, term)
		if err != nil {
			continue
		}
		for _, c := range collections {
			if !seen[c.TMDBID] {
				seen[c.TMDBID] = true
				allCollections = append(allCollections, c)
			}
		}
		// Limit total results
		if len(allCollections) >= 30 {
			break
		}
	}
	
	respondJSON(w, http.StatusOK, allCollections)
}

// BrowseCollections handles GET /api/discover/collections/browse - browse all TMDB collections with pagination
func (h *Handler) BrowseCollections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get page parameter (default 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	
	// Get search query if provided
	query := r.URL.Query().Get("query")
	
	// Expanded list of collection search terms for more variety
	allSearchTerms := []string{
		// Superhero/Comic
		"Marvel", "Avengers", "Iron Man", "Captain America", "Thor", "Hulk", "Guardians of the Galaxy",
		"Spider-Man", "X-Men", "Deadpool", "Batman", "Superman", "Justice League", "Wonder Woman",
		"Aquaman", "DC Extended Universe", "The Dark Knight",
		// Sci-Fi/Fantasy
		"Star Wars", "Star Trek", "The Matrix", "Terminator", "Alien", "Predator", "Planet of the Apes",
		"Jurassic", "Back to the Future", "Transformers", "Pacific Rim", "Godzilla", "King Kong",
		"Blade Runner", "Dune", "Avatar",
		// Action/Adventure
		"James Bond", "Mission Impossible", "Fast Furious", "John Wick", "Die Hard", "Rambo",
		"Indiana Jones", "Pirates of the Caribbean", "The Expendables", "Mad Max", "Kingsman",
		"Jack Ryan", "Bourne", "Tomb Raider", "National Treasure",
		// Fantasy
		"Lord of the Rings", "Hobbit", "Harry Potter", "Fantastic Beasts", "Narnia", "The Hunger Games",
		"Twilight", "Maze Runner", "Divergent", "Percy Jackson",
		// Animation
		"Toy Story", "Shrek", "Ice Age", "Madagascar", "Kung Fu Panda", "How to Train Your Dragon",
		"Despicable Me", "Minions", "Finding Nemo", "Cars", "Monsters Inc", "The Incredibles",
		"Frozen", "Moana", "Wreck-It Ralph", "Big Hero", "Zootopia",
		// Horror
		"Conjuring", "Insidious", "Paranormal Activity", "Saw", "Halloween", "Friday the 13th",
		"A Nightmare on Elm Street", "Scream", "The Purge", "Final Destination", "Annabelle",
		// Comedy
		"Hangover", "American Pie", "Rush Hour", "Bad Boys", "Men in Black", "Ghostbusters",
		"Night at the Museum", "Ocean's", "Austin Powers", "Ace Ventura",
		// Drama/Crime
		"Godfather", "Rocky", "Creed", "Ocean", "Now You See Me", "Taken",
	}
	
	var searchTerms []string
	if query != "" {
		// If user provides a search query, use that
		searchTerms = []string{query}
	} else {
		// Calculate which terms to use based on page (5 terms per page for variety)
		termsPerPage := 5
		startIdx := (page - 1) * termsPerPage
		endIdx := startIdx + termsPerPage
		if startIdx >= len(allSearchTerms) {
			startIdx = 0
			endIdx = termsPerPage
		}
		if endIdx > len(allSearchTerms) {
			endIdx = len(allSearchTerms)
		}
		searchTerms = allSearchTerms[startIdx:endIdx]
	}
	
	var allCollections []*models.Collection
	seen := make(map[int]bool)
	
	for _, term := range searchTerms {
		collections, err := h.tmdbClient.SearchCollections(ctx, term)
		if err != nil {
			continue
		}
		for _, c := range collections {
			if !seen[c.TMDBID] {
				seen[c.TMDBID] = true
				allCollections = append(allCollections, c)
			}
		}
	}
	
	// Calculate pagination
	totalCollections := len(allCollections)
	itemsPerPage := 50
	totalPages := (len(allSearchTerms) + 4) / 5 // 5 terms per page
	if query != "" {
		totalPages = 1
	}
	
	// Limit to 50 items per page
	if totalCollections > itemsPerPage {
		allCollections = allCollections[:itemsPerPage]
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"collections": allCollections,
		"page":        page,
		"totalPages":  totalPages,
		"total":       totalCollections,
	})
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
		if h.mdbSyncService != nil {
			services.GlobalScheduler.UpdateProgress(services.ServiceMDBListSync, 0, 0, "Starting MDBList sync...")
			err = h.mdbSyncService.SyncAllLists(ctx)
			if err == nil {
				// Also enrich existing items
				_ = h.mdbSyncService.EnrichExistingItems(ctx)
			}
		} else {
			services.GlobalScheduler.UpdateProgress(services.ServiceMDBListSync, 0, 0, "MDBList sync service not initialized")
		}

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

				// Find incomplete collections; treat unknown totals (0) as incomplete to force refresh
				var incompleteColls []*models.Collection
				for _, coll := range collections {
					if coll.TotalMovies == 0 || coll.MoviesInLibrary < coll.TotalMovies {
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

	case services.ServiceBalkanVODSync:
		interval = 24 * time.Hour
		if h.settingsManager != nil && h.tmdbClient != nil && h.movieStore != nil && h.seriesStore != nil {
			current := h.settingsManager.Get()
			if current.BalkanVODEnabled {
				log.Println("[BalkanVOD] Manual trigger: Starting import...")
				importer := services.NewBalkanVODImporter(h.movieStore, h.seriesStore, h.tmdbClient, current)
				err = importer.ImportBalkanVOD(ctx)
				if err != nil {
					log.Printf("[BalkanVOD] Import error: %v", err)
				} else {
					log.Println("[BalkanVOD] Import completed successfully")
				}
			} else {
				log.Println("[BalkanVOD] Import disabled in settings")
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

	// Count cached streams from media_streams table (Stream Cache Monitor)
	if h.streamCacheStore != nil {
		db := h.streamCacheStore.GetDB()
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_streams").Scan(&streamCount)
		if err != nil {
			log.Printf("Failed to count media_streams: %v", err)
			streamCount = 0
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

	// If branch is "tag", fetch latest release tag instead of commit
	if branch == "tag" {
		h.checkForTagUpdates(w, r)
		return
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
	}
	// Only show update available if commits actually differ, don't assume unknown needs updating

	// Get first line of commit message as changelog
	changelog := commitData.Commit.Message
	if idx := strings.Index(changelog, "\n"); idx > 0 {
		changelog = changelog[:idx]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"current_version":  Version,
		"current_commit":   Commit,
		"build_date":       BuildDate,
		"latest_version":   branch,
		"latest_commit":    commitData.SHA,
		"latest_date":      commitData.Commit.Author.Date,
		"update_available": updateAvailable,
		"changelog":        changelog,
		"update_branch":    branch,
	})
}

// checkForTagUpdates checks GitHub for the latest tag/release
func (h *Handler) checkForTagUpdates(w http.ResponseWriter, r *http.Request) {
	// Fetch latest tags from GitHub API
	resp, err := http.Get("https://api.github.com/repos/Zerr0-C00L/StreamArr_Pro/tags?per_page=1")
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

	var tags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil || len(tags) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"current_version": Version,
			"current_commit":  Commit,
			"build_date":      BuildDate,
			"error":           "Failed to parse update info or no tags found",
		})
		return
	}

	latestTag := tags[0]
	latestVersion := latestTag.Name
	latestCommit := latestTag.Commit.SHA
	latestCommitShort := latestCommit
	if len(latestCommitShort) > 7 {
		latestCommitShort = latestCommitShort[:7]
	}

	// Compare versions - update available if current version != latest tag
	updateAvailable := false
	if Version != "dev" && Version != "" {
		// Compare version strings directly
		updateAvailable = Version != latestVersion
	} else {
		// If version is dev or unknown, compare commits
		if Commit != "unknown" && len(Commit) >= 7 {
			currentShort := Commit
			if len(currentShort) > 7 {
				currentShort = currentShort[:7]
			}
			updateAvailable = currentShort != latestCommitShort
		} else {
			updateAvailable = true
		}
	}

	// Get release info for changelog
	changelog := ""
	releaseResp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/Zerr0-C00L/StreamArr_Pro/releases/tags/%s", latestVersion))
	if err == nil {
		defer releaseResp.Body.Close()
		var releaseData struct {
			Body        string `json:"body"`
			PublishedAt string `json:"published_at"`
		}
		if json.NewDecoder(releaseResp.Body).Decode(&releaseData) == nil {
			changelog = releaseData.Body
			// Truncate changelog if too long
			if len(changelog) > 500 {
				changelog = changelog[:500] + "..."
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"current_version":  Version,
		"current_commit":   Commit,
		"build_date":       BuildDate,
		"latest_version":   latestVersion,
		"latest_commit":    latestCommit,
		"update_available": updateAvailable,
		"changelog":        changelog,
		"update_branch":    "tag",
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

	// If branch is "tag", fetch the latest tag name
	if branch == "tag" {
		resp, err := http.Get("https://api.github.com/repos/Zerr0-C00L/StreamArr_Pro/tags?per_page=1")
		if err == nil {
			defer resp.Body.Close()
			var tags []struct {
				Name string `json:"name"`
			}
			if json.NewDecoder(resp.Body).Decode(&tags); err == nil && len(tags) > 0 {
				branch = tags[0].Name
				log.Printf("[Update] Using script: %s", branch)
			} else {
				log.Println("[Update] Warning: Failed to fetch latest tag, falling back to 'main'")
				branch = "main"
			}
		} else {
			log.Printf("[Update] Warning: Failed to fetch tags: %v, falling back to 'main'", err)
			branch = "main"
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
		if err := exec.Command("chmod", "+x", scriptPath).Run(); err != nil {
			log.Printf("[Update] Warning: Failed to chmod script: %v", err)
		}

		// Check for existing lock file
		lockFile := hostDir + "/logs/update.lock"
		if lockData, err := os.ReadFile(lockFile); err == nil {
			log.Printf("[Update] Lock file exists: %s", string(lockData))
			// Try to remove stale lock
			os.Remove(lockFile)
		}

		// Create a wrapper script that runs the update and handles errors
		wrapperScript := hostDir + "/logs/run-update.sh"
		wrapperContent := fmt.Sprintf(`#!/bin/bash
set -e
cd "%s"
exec /bin/sh "%s" "%s"
`, hostDir, scriptPath, branch)
		
		if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
			log.Printf("[Update] Warning: Failed to create wrapper script: %v", err)
		}

		// Run update script in background (detached) so it survives container stop
		// Using screen/detach properly to ensure process survives
		log.Println("[Update] Executing update script in background...")
		
		cmdStr := fmt.Sprintf("cd %s && nohup /bin/sh -c '/bin/sh %s %s >> %s/logs/update.log 2>&1' > /dev/null 2>&1 &", 
			hostDir, scriptPath, branch, hostDir)
		
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Env = append(os.Environ(),
			"HOME=/root",
			"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		)

		// Start the process
		if err := cmd.Start(); err != nil {
			log.Printf("[Update] Failed to start update: %v", err)
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start update: %v", err))
			return
		}

		// Don't wait for the command, let it run in background
		go cmd.Wait()

		// Give the script a moment to start
		time.Sleep(1 * time.Second)

		// Verify logs were created
		logsFile := hostDir + "/logs/update.log"
		if _, err := os.Stat(logsFile); err == nil {
			log.Println("[Update] Script started successfully (update.log created)")
		} else {
			log.Println("[Update] Note: update.log not yet created, but script was launched")
		}

		log.Println("[Update] Update script started in background")
	} else {
		// Non-Docker environment - use update.sh (already set at the top)
		// updateScript is already "./scripts/update.sh"

		log.Printf("[Update] Using script: %s with branch: %s", updateScript, branch)

		cmd := exec.Command("/bin/bash", "-c",
			fmt.Sprintf("cd %s && nohup setsid /bin/bash %s %s > logs/update.log 2>&1 &",
				".", updateScript, branch))
		cmd.Dir = "."

		if err := cmd.Start(); err != nil {
			log.Printf("[Update] Failed to start update: %v", err)
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start update: %v", err))
			return
		}

		// Detach from the process immediately
		go cmd.Wait()

		log.Println("[Update] Update script started in background")
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

	// Enforce allowed platforms
	allowedPlatforms := []string{"digital", "tv", "streaming", "physical"}
	isAllowedPlatform := false
	for _, platform := range allowedPlatforms {
		if strings.Contains(lowerCategory, platform) {
			isAllowedPlatform = true
			break
		}
	}

	if !isAllowedPlatform {
		return false
	}

	// Check if the content is a series or episode
	isSeriesOrEpisode := strings.Contains(lowerURL, "/series/") ||
		strings.Contains(lowerCategory, "series") ||
		strings.Contains(lowerCategory, "episode")

	if isSeriesOrEpisode {
		// Extract release date from metadata (if available)
		releaseDate := extractReleaseDateFromMetadata(lowerURL, lowerCategory)
		if releaseDate != "" {
			currentDate := time.Now()
			releaseTime, err := time.Parse("2006-01-02", releaseDate)
			if err == nil && releaseTime.After(currentDate) {
				return false // Exclude if release date is in the future
			}
		}
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

// PreviewBalkanCategories handles POST /api/v1/balkan-vod/preview-categories
func (h *Handler) PreviewBalkanCategories(w http.ResponseWriter, r *http.Request) {
	log.Println("[API] PreviewBalkanCategories: Starting request")

	categories, err := services.FetchBalkanCategories()
	if err != nil {
		log.Printf("[API] PreviewBalkanCategories: Error fetching categories: %v", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch categories: %v", err))
		return
	}

	log.Printf("[API] PreviewBalkanCategories: Fetched %d categories", len(categories))

	// Sort by name
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
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

// isBalkanVODMovie checks if a movie is from Balkan VOD source
func isBalkanVODMovie(movie *models.Movie) bool {
	if movie == nil || movie.Metadata == nil {
		return false
	}
	if src, ok := movie.Metadata["source"].(string); ok {
		return strings.EqualFold(src, "balkan_vod")
	}
	return false
}

// buildBalkanVODStreams converts metadata.balkan_vod_streams into API stream entries
func buildBalkanVODStreams(movie *models.Movie) []map[string]interface{} {
	streams := []map[string]interface{}{}
	if movie == nil || movie.Metadata == nil {
		return streams
	}

	if balkanStreams, ok := movie.Metadata["balkan_vod_streams"].([]interface{}); ok {
		for _, s := range balkanStreams {
			m, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			url, _ := m["url"].(string)
			quality, _ := m["quality"].(string)

			name = strings.TrimSpace(name)
			url = strings.TrimSpace(url)
			quality = strings.TrimSpace(quality)

			if name == "" {
				name = "Balkan VOD"
			}
			if quality == "" {
				quality = "HD"
			}
			if url == "" {
				continue
			}

			streams = append(streams, map[string]interface{}{
				"source":   name,
				"quality":  quality,
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

// isBalkanVODSeries checks if a series is from Balkan VOD source
func isBalkanVODSeries(series *models.Series) bool {
	if series == nil || series.Metadata == nil {
		return false
	}
	if src, ok := series.Metadata["source"].(string); ok {
		return strings.EqualFold(src, "balkan_vod")
	}
	return false
}

// buildBalkanVODEpisodes constructs episode objects from metadata.balkan_vod_seasons
func buildBalkanVODEpisodes(series *models.Series) []*models.Episode {
	episodes := []*models.Episode{}
	if series == nil || series.Metadata == nil {
		return episodes
	}

	seasons, ok := series.Metadata["balkan_vod_seasons"].([]interface{})
	if !ok {
		return episodes
	}

	for _, s := range seasons {
		seasonMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		seasonNum, ok := seasonMap["number"].(float64)
		if !ok {
			continue
		}

		episodeList, ok := seasonMap["episodes"].([]interface{})
		if !ok {
			continue
		}

		for _, e := range episodeList {
			episodeMap, ok := e.(map[string]interface{})
			if !ok {
				continue
			}

			episodeNum, ok := episodeMap["episode"].(float64)
			if !ok {
				continue
			}

			title, _ := episodeMap["title"].(string)
			url, _ := episodeMap["url"].(string)
			thumbnail, _ := episodeMap["thumbnail"].(string)

			if title == "" {
				title = fmt.Sprintf("Episode %d", int(episodeNum))
			}

			var streamURL *string
			if url != "" {
				streamURL = &url
			}

			episode := &models.Episode{
				SeriesID:      series.ID,
				SeasonNumber:  int(seasonNum),
				EpisodeNumber: int(episodeNum),
				Title:         title,
				StillPath:     thumbnail,
				Monitored:     true,
				Available:     url != "",
				StreamURL:     streamURL,
				Metadata:      make(map[string]interface{}),
			}

			episode.Metadata["source"] = "balkan_vod"
			episode.Metadata["url"] = url
			episode.Metadata["thumbnail"] = thumbnail

			episodes = append(episodes, episode)
		}
	}

	return episodes
}

// buildBalkanVODSeriesStreams converts metadata.balkan_vod_seasons into API stream entries for specific episode
func buildBalkanVODSeriesStreams(series *models.Series, season, episode int) []map[string]interface{} {
	streams := []map[string]interface{}{}
	if series == nil || series.Metadata == nil {
		return streams
	}

	if seasons, ok := series.Metadata["balkan_vod_seasons"].([]interface{}); ok {
		for _, s := range seasons {
			seasonMap, ok := s.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if this is the season we're looking for
			seasonNum, ok := seasonMap["number"].(float64) // JSON numbers are float64
			if !ok || int(seasonNum) != season {
				continue
			}

			// Find the episode in this season
			episodes, ok := seasonMap["episodes"].([]interface{})
			if !ok {
				continue
			}

			for _, e := range episodes {
				episodeMap, ok := e.(map[string]interface{})
				if !ok {
					continue
				}

				episodeNum, ok := episodeMap["episode"].(float64)
				if !ok || int(episodeNum) != episode {
					continue
				}

				// Found the episode! Extract stream info
				url, _ := episodeMap["url"].(string)
				title, _ := episodeMap["title"].(string)
				url = strings.TrimSpace(url)

				if url == "" {
					continue
				}

				if title == "" {
					title = fmt.Sprintf("%s S%02dE%02d", series.Title, season, episode)
				}

				streams = append(streams, map[string]interface{}{
					"source":   "Balkan VOD",
					"quality":  "HD",
					"codec":    "",
					"url":      url,
					"cached":   false,
					"seeds":    0,
					"size_gb":  0.0,
					"title":    title,
					"name":     "Balkan VOD",
					"filename": "Balkan VOD",
				})

				// Found the episode, no need to continue
				return streams
			}
		}
	}

	return streams
}

// Restart restarts the server
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	log.Println("[Admin] Server restart requested")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Send success response first
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "restarting",
		"message": "Server is restarting... Connection will be restored in 5-10 seconds",
	})

	// Flush the response to client before restarting
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Give the response time to send before restarting
	go func() {
		time.Sleep(1 * time.Second)
		log.Println("[Admin] Exiting process to trigger container restart...")
		os.Exit(0)
	}()
}

// ============================================================================
// Blacklist Handlers
// ============================================================================

// RemoveAndBlacklist removes an item from library and adds to blacklist
func (h *Handler) RemoveAndBlacklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	itemType := vars["type"] // "movie" or "series"
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "Removed by user"
	}

	// Get the item details before deletion
	var tmdbID int
	var title string

	if itemType == "movie" {
		movie, err := h.movieStore.Get(ctx, id)
		if err != nil {
			respondError(w, http.StatusNotFound, "Movie not found")
			return
		}
		tmdbID = movie.TMDBID
		title = movie.Title

		// Delete movie and its streams
		if err := h.streamStore.DeleteByContent(ctx, "movie", id); err != nil {
			log.Printf("Warning: Failed to delete streams for movie %d: %v", id, err)
		}
		if err := h.movieStore.Delete(ctx, id); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete movie: %v", err))
			return
		}
	} else if itemType == "series" {
		series, err := h.seriesStore.Get(ctx, id)
		if err != nil {
			respondError(w, http.StatusNotFound, "Series not found")
			return
		}
		tmdbID = series.TMDBID
		title = series.Title

		// Delete series, episodes, and streams
		episodes, _ := h.episodeStore.ListBySeries(ctx, id)
		for _, ep := range episodes {
			h.streamStore.DeleteByContent(ctx, "episode", ep.ID)
		}
		h.episodeStore.DeleteBySeries(ctx, id)
		if err := h.seriesStore.Delete(ctx, id); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete series: %v", err))
			return
		}
	} else {
		respondError(w, http.StatusBadRequest, "Invalid item type")
		return
	}

	// Add to blacklist
	if err := h.blacklistStore.Add(ctx, tmdbID, itemType, title, req.Reason); err != nil {
		log.Printf("Warning: Failed to add to blacklist: %v", err)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("%s removed and blacklisted", itemType),
		"tmdb_id": tmdbID,
		"title":   title,
	})
}

// GetBlacklist returns paginated blacklist entries
func (h *Handler) GetBlacklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 50
	}

	entries, total, err := h.blacklistStore.List(ctx, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get blacklist: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"blacklist": entries,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// RemoveFromBlacklist removes an entry from the blacklist
func (h *Handler) RemoveFromBlacklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	if err := h.blacklistStore.Remove(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove from blacklist: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Removed from blacklist",
	})
}

// ClearBlacklist clears all blacklist entries
func (h *Handler) ClearBlacklist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.blacklistStore.Clear(ctx); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to clear blacklist: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Blacklist cleared",
	})
}

// validateMovieStreams filters out streams that don't match the requested movie title
// This prevents providers from returning unrelated movies
func validateMovieStreams(streams []providers.TorrentioStream, movieTitle string, releaseYear int) []providers.TorrentioStream {
	if len(streams) == 0 {
		return streams
	}

	// Helper function to normalize text for comparison
	// Removes special characters and extra spaces
	normalizeForMatch := func(text string) string {
		// Convert to lowercase
		text = strings.ToLower(text)
		// Replace & with 'and' to handle both "Coffee & Kareem" and "Coffee and Kareem"
		text = strings.ReplaceAll(text, "&", "and")
		// Replace special chars with spaces: : - . _ etc.
		text = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
				return r
			}
			return ' '
		}, text)
		// Collapse multiple spaces into one
		text = strings.Join(strings.Fields(text), " ")
		return strings.TrimSpace(text)
	}

	// Normalize movie title for comparison
	normalizedTitle := normalizeForMatch(movieTitle)

	validated := []providers.TorrentioStream{}

	for _, stream := range streams {
		// Combine all text fields for matching
		streamText := normalizeForMatch(fmt.Sprintf("%s %s", stream.Title, stream.Name))

		// Check if the stream title contains the movie title as a substring
		// This handles cases like "Django Unchained 2012" containing "Django Unchained"
		if strings.Contains(streamText, normalizedTitle) {
			validated = append(validated, stream)
		} else {
			// Log rejected streams with detailed info for debugging
			log.Printf("[VALIDATION] ❌ Stream rejected (title mismatch): requested='%s' (normalized: '%s') stream='%s' (normalized: '%s')",
				movieTitle, normalizedTitle, stream.Title, normalizeForMatch(stream.Title))
		}
	}

	return validated
}

// extractReleaseDateFromMetadata extracts the release date from the metadata in the URL or category
func extractReleaseDateFromMetadata(url, category string) string {
	// Example logic to extract release date from metadata
	// This should be replaced with actual parsing logic based on your metadata format
	if strings.Contains(url, "release_date=") {
		start := strings.Index(url, "release_date=") + len("release_date=")
		end := strings.Index(url[start:], "&")
		if end == -1 {
			end = len(url)
		} else {
			end += start
		}
		return url[start:end]
	}
	return ""
}

// GetUpdateStatus handles GET /api/debug/update-status
func (h *Handler) GetUpdateStatus(w http.ResponseWriter, r *http.Request) {
	lockFile := "logs/update.lock"
	logFile := "logs/update.log"
	
	// Check if update is running
	isRunning := false
	var lockPID string
	
	if lockData, err := os.ReadFile(lockFile); err == nil {
		lockPID = strings.TrimSpace(string(lockData))
		isRunning = true // Lock exists
	}
	
	// Get log file info
	var logSize int64
	var logModTime string
	var logTail string
	
	if logInfo, err := os.Stat(logFile); err == nil {
		logSize = logInfo.Size()
		logModTime = logInfo.ModTime().Format("2006-01-02 15:04:05")
		
		// Read last 50 lines
		if data, err := os.ReadFile(logFile); err == nil {
			logTail = string(data)
			// Keep only last 2KB for the response
			if len(logTail) > 2000 {
				logTail = "...\n" + logTail[len(logTail)-2000:]
			}
		}
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"is_running":  isRunning,
		"lock_pid":    lockPID,
		"log_size":    logSize,
		"log_mod_time": logModTime,
		"log_tail":    logTail,
	})
}

// GetUpdateLog handles GET /api/debug/update-log
func (h *Handler) GetUpdateLog(w http.ResponseWriter, r *http.Request) {
	logFile := "logs/update.log"
	
	data, err := os.ReadFile(logFile)
	if err != nil {
		respondError(w, http.StatusNotFound, "Update log not found. Update may not have been triggered yet.")
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"log": string(data),
	})
}

// sortMovies sorts a slice of movies based on sort criteria
func sortMovies(movies []*models.Movie, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(movies, func(i, j int) bool {
		switch sortBy {
		case "title":
			if ascending {
				return strings.ToLower(movies[i].Title) < strings.ToLower(movies[j].Title)
			}
			return strings.ToLower(movies[i].Title) > strings.ToLower(movies[j].Title)

		case "date_added":
			if ascending {
				return movies[i].AddedAt.Before(movies[j].AddedAt)
			}
			return movies[i].AddedAt.After(movies[j].AddedAt)

		case "release_date":
			// Handle nil dates
			iDate := time.Time{}
			jDate := time.Time{}
			if movies[i].ReleaseDate != nil {
				iDate = *movies[i].ReleaseDate
			}
			if movies[j].ReleaseDate != nil {
				jDate = *movies[j].ReleaseDate
			}
			if ascending {
				return iDate.Before(jDate)
			}
			return iDate.After(jDate)

		case "rating":
			iRating := 0.0
			jRating := 0.0
			if movies[i].Metadata != nil {
				if r, ok := movies[i].Metadata["vote_average"].(float64); ok {
					iRating = r
				}
			}
			if movies[j].Metadata != nil {
				if r, ok := movies[j].Metadata["vote_average"].(float64); ok {
					jRating = r
				}
			}
			if ascending {
				return iRating < jRating
			}
			return iRating > jRating

		case "runtime":
			if ascending {
				return movies[i].Runtime < movies[j].Runtime
			}
			return movies[i].Runtime > movies[j].Runtime

		case "monitored":
			if ascending {
				if movies[i].Monitored == movies[j].Monitored {
					return strings.ToLower(movies[i].Title) < strings.ToLower(movies[j].Title)
				}
				return movies[i].Monitored
			}
			if movies[i].Monitored == movies[j].Monitored {
				return strings.ToLower(movies[i].Title) < strings.ToLower(movies[j].Title)
			}
			return !movies[i].Monitored

		case "genre":
			iGenre := ""
			jGenre := ""
			if len(movies[i].Genres) > 0 {
				iGenre = movies[i].Genres[0]
			}
			if len(movies[j].Genres) > 0 {
				jGenre = movies[j].Genres[0]
			}
			if ascending {
				return iGenre < jGenre
			}
			return iGenre > jGenre

		case "language":
			iLang := ""
			jLang := ""
			if movies[i].Metadata != nil {
				if lang, ok := movies[i].Metadata["original_language"].(string); ok {
					iLang = lang
				}
			}
			if movies[j].Metadata != nil {
				if lang, ok := movies[j].Metadata["original_language"].(string); ok {
					jLang = lang
				}
			}
			if ascending {
				return iLang < jLang
			}
			return iLang > jLang

		case "country":
			iCountry := ""
			jCountry := ""
			if movies[i].Metadata != nil {
				if countries, ok := movies[i].Metadata["production_countries"].([]interface{}); ok && len(countries) > 0 {
					if c, ok := countries[0].(map[string]interface{}); ok {
						if iso, ok := c["iso_3166_1"].(string); ok {
							iCountry = iso
						}
					}
				}
			}
			if movies[j].Metadata != nil {
				if countries, ok := movies[j].Metadata["production_countries"].([]interface{}); ok && len(countries) > 0 {
					if c, ok := countries[0].(map[string]interface{}); ok {
						if iso, ok := c["iso_3166_1"].(string); ok {
							jCountry = iso
						}
					}
				}
			}
			if ascending {
				return iCountry < jCountry
			}
			return iCountry > jCountry

		default:
			return strings.ToLower(movies[i].Title) < strings.ToLower(movies[j].Title)
		}
	})
}

// sortSeries sorts a slice of series based on sort criteria
func sortSeries(series []*models.Series, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"

	sort.Slice(series, func(i, j int) bool {
		switch sortBy {
		case "title":
			if ascending {
				return strings.ToLower(series[i].Title) < strings.ToLower(series[j].Title)
			}
			return strings.ToLower(series[i].Title) > strings.ToLower(series[j].Title)

		case "date_added":
			if ascending {
				return series[i].AddedAt.Before(series[j].AddedAt)
			}
			return series[i].AddedAt.After(series[j].AddedAt)

		case "release_date":
			iDate := time.Time{}
			jDate := time.Time{}
			if series[i].FirstAirDate != nil {
				iDate = *series[i].FirstAirDate
			}
			if series[j].FirstAirDate != nil {
				jDate = *series[j].FirstAirDate
			}
			if ascending {
				return iDate.Before(jDate)
			}
			return iDate.After(jDate)

		case "rating":
			iRating := 0.0
			jRating := 0.0
			if series[i].Metadata != nil {
				if r, ok := series[i].Metadata["vote_average"].(float64); ok {
					iRating = r
				}
			}
			if series[j].Metadata != nil {
				if r, ok := series[j].Metadata["vote_average"].(float64); ok {
					jRating = r
				}
			}
			if ascending {
				return iRating < jRating
			}
			return iRating > jRating

		case "monitored":
			if ascending {
				if series[i].Monitored == series[j].Monitored {
					return strings.ToLower(series[i].Title) < strings.ToLower(series[j].Title)
				}
				return series[i].Monitored
			}
			if series[i].Monitored == series[j].Monitored {
				return strings.ToLower(series[i].Title) < strings.ToLower(series[j].Title)
			}
			return !series[i].Monitored

		case "genre":
			iGenre := ""
			jGenre := ""
			if len(series[i].Genres) > 0 {
				iGenre = series[i].Genres[0]
			}
			if len(series[j].Genres) > 0 {
				jGenre = series[j].Genres[0]
			}
			if ascending {
				return iGenre < jGenre
			}
			return iGenre > jGenre

		case "language":
			iLang := ""
			jLang := ""
			if series[i].Metadata != nil {
				if lang, ok := series[i].Metadata["original_language"].(string); ok {
					iLang = lang
				}
			}
			if series[j].Metadata != nil {
				if lang, ok := series[j].Metadata["original_language"].(string); ok {
					jLang = lang
				}
			}
			if ascending {
				return iLang < jLang
			}
			return iLang > jLang

		case "country":
			iCountry := ""
			jCountry := ""
			if series[i].Metadata != nil {
				if countries, ok := series[i].Metadata["production_countries"].([]interface{}); ok && len(countries) > 0 {
					if c, ok := countries[0].(map[string]interface{}); ok {
						if iso, ok := c["iso_3166_1"].(string); ok {
							iCountry = iso
						}
					}
				}
			}
			if series[j].Metadata != nil {
				if countries, ok := series[j].Metadata["production_countries"].([]interface{}); ok && len(countries) > 0 {
					if c, ok := countries[0].(map[string]interface{}); ok {
						if iso, ok := c["iso_3166_1"].(string); ok {
							jCountry = iso
						}
					}
				}
			}
			if ascending {
				return iCountry < jCountry
			}
			return iCountry > jCountry

		default:
			return strings.ToLower(series[i].Title) < strings.ToLower(series[j].Title)
		}
	})
}

