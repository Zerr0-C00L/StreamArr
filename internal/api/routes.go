package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) http.Handler {
	r := mux.NewRouter()

	// API v1 routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Health check
	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Movies
	api.HandleFunc("/movies", handler.ListMovies).Methods("GET")
	api.HandleFunc("/movies", handler.AddMovie).Methods("POST")
	api.HandleFunc("/movies/{id}", handler.GetMovie).Methods("GET")
	api.HandleFunc("/movies/{id}", handler.UpdateMovie).Methods("PUT")
	api.HandleFunc("/movies/{id}", handler.DeleteMovie).Methods("DELETE")
	api.HandleFunc("/movies/{id}/streams", handler.GetMovieStreams).Methods("GET")
	api.HandleFunc("/movies/{id}/play", handler.PlayMovie).Methods("GET")

	// Series
	api.HandleFunc("/series", handler.ListSeries).Methods("GET")
	api.HandleFunc("/series", handler.AddSeries).Methods("POST")
	api.HandleFunc("/series/{id}", handler.GetSeries).Methods("GET")
	api.HandleFunc("/series/{id}", handler.UpdateSeries).Methods("PUT")
	api.HandleFunc("/series/{id}", handler.DeleteSeries).Methods("DELETE")
	api.HandleFunc("/series/{id}/episodes", handler.GetSeriesEpisodes).Methods("GET")

	// Episodes
	api.HandleFunc("/episodes/{id}/play", handler.PlayEpisode).Methods("GET")

	// Channels (Live TV)
	api.HandleFunc("/channels", handler.ListChannels).Methods("GET")
	api.HandleFunc("/channels/stats", handler.GetChannelStats).Methods("GET")
	api.HandleFunc("/channels/{id}", handler.GetChannel).Methods("GET")
	api.HandleFunc("/channels/{id}/stream", handler.GetChannelStream).Methods("GET")

	// Search
	api.HandleFunc("/search/movies", handler.SearchMovies).Methods("GET")
	api.HandleFunc("/search/series", handler.SearchSeries).Methods("GET")
	api.HandleFunc("/search/collections", handler.SearchCollections).Methods("GET")

        // Discover / Trending
        api.HandleFunc("/discover/trending", handler.GetTrending).Methods("GET")
        api.HandleFunc("/discover/popular", handler.GetPopular).Methods("GET")
        api.HandleFunc("/discover/now-playing", handler.GetNowPlaying).Methods("GET")

	// Collections
	api.HandleFunc("/collections", handler.ListCollections).Methods("GET")
	api.HandleFunc("/collections/{id}", handler.GetCollection).Methods("GET")
	api.HandleFunc("/collections/{id}/sync", handler.SyncCollection).Methods("POST")
	api.HandleFunc("/collections/{id}/movies", handler.GetCollectionMovies).Methods("GET")

	// Services
	api.HandleFunc("/services", handler.GetServices).Methods("GET")
	api.HandleFunc("/services/{name}", handler.UpdateServiceEnabled).Methods("PUT")
	api.HandleFunc("/services/{name}/trigger", handler.TriggerService).Methods("POST")

	// Settings
	api.HandleFunc("/settings", handler.GetSettings).Methods("GET")
	api.HandleFunc("/settings", handler.UpdateSettings).Methods("PUT")

	// Calendar
	api.HandleFunc("/calendar", handler.GetCalendar).Methods("GET")

	// MDBList
	api.HandleFunc("/mdblist/user-lists", handler.GetMDBListUserLists).Methods("GET")

	// Stats (for dashboard)
	api.HandleFunc("/stats", handler.GetStats).Methods("GET")

	// Database management
	api.HandleFunc("/database/stats", handler.GetDatabaseStats).Methods("GET")
	api.HandleFunc("/database/{action}", handler.ExecuteDatabaseAction).Methods("POST")

	// Version / Updates
	api.HandleFunc("/version", handler.GetVersion).Methods("GET")
	api.HandleFunc("/version/check", handler.CheckForUpdates).Methods("GET")
	api.HandleFunc("/update/install", handler.InstallUpdate).Methods("POST")

	// Adult VOD Import
	api.HandleFunc("/adult-vod/import", handler.ImportAdultVOD).Methods("POST")
	api.HandleFunc("/adult-vod/stats", handler.GetAdultVODStats).Methods("GET")

	// Serve static UI files (SPA)
	uiPath := getUIPath()
	if uiPath != "" {
		spa := spaHandler{staticPath: uiPath, indexPath: "index.html"}
		r.PathPrefix("/").Handler(spa)
	}

	// Logging middleware
	r.Use(loggingMiddleware)

	// Return with CORS middleware wrapping at top level (handles OPTIONS before routing)
	return corsMiddleware(r)
}

// spaHandler implements the http.Handler interface for serving Single Page Applications
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip API routes - let them 404 if not handled
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/player_api") {
		http.NotFound(w, r)
		return
	}
	
	// Get the absolute path to prevent directory traversal
	path := filepath.Join(h.staticPath, filepath.Clean(r.URL.Path))

	// Check if file exists
	fi, err := os.Stat(path)
	if os.IsNotExist(err) || fi.IsDir() {
		// File doesn't exist or is a directory, serve index.html for SPA routing
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set proper content type for JS and CSS files
	if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css")
	}

	// File exists, serve it
	http.ServeFile(w, r, path)
}

// getUIPath returns the path to the UI dist folder
func getUIPath() string {
	paths := []string{
		"./streamarr-pro-ui/dist",
		"/opt/StreamArr/streamarr-pro-ui/dist",
		"/opt/streamarr/streamarr-pro-ui/dist",
	}
	
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
			return p
		}
	}
	
	return ""
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs all HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// You can add proper logging here
		next.ServeHTTP(w, r)
	})
}

// SetupRoutesWithXtream configures API routes with Xtream Codes handler integration
func SetupRoutesWithXtream(handler *Handler, xtreamHandler interface{ RegisterRoutes(*mux.Router) }) http.Handler {
	r := mux.NewRouter()

	// Register Xtream Codes API routes first
	xtreamHandler.RegisterRoutes(r)

	// API v1 routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Health check
	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Movies
	api.HandleFunc("/movies", handler.ListMovies).Methods("GET")
	api.HandleFunc("/movies", handler.AddMovie).Methods("POST")
	api.HandleFunc("/movies/{id}", handler.GetMovie).Methods("GET")
	api.HandleFunc("/movies/{id}", handler.UpdateMovie).Methods("PUT")
	api.HandleFunc("/movies/{id}", handler.DeleteMovie).Methods("DELETE")
	api.HandleFunc("/movies/{id}/streams", handler.GetMovieStreams).Methods("GET")
	api.HandleFunc("/movies/{id}/play", handler.PlayMovie).Methods("GET")

	// Series
	api.HandleFunc("/series", handler.ListSeries).Methods("GET")
	api.HandleFunc("/series", handler.AddSeries).Methods("POST")
	api.HandleFunc("/series/{id}", handler.GetSeries).Methods("GET")
	api.HandleFunc("/series/{id}", handler.UpdateSeries).Methods("PUT")
	api.HandleFunc("/series/{id}", handler.DeleteSeries).Methods("DELETE")
	api.HandleFunc("/series/{id}/episodes", handler.GetSeriesEpisodes).Methods("GET")

	// Episodes
	api.HandleFunc("/episodes/{id}/play", handler.PlayEpisode).Methods("GET")

	// Channels (Live TV)
	api.HandleFunc("/channels", handler.ListChannels).Methods("GET")
	api.HandleFunc("/channels/stats", handler.GetChannelStats).Methods("GET")
	api.HandleFunc("/channels/{id}", handler.GetChannel).Methods("GET")
	api.HandleFunc("/channels/{id}/stream", handler.GetChannelStream).Methods("GET")

	// Search
	api.HandleFunc("/search/movies", handler.SearchMovies).Methods("GET")
	api.HandleFunc("/search/series", handler.SearchSeries).Methods("GET")
	api.HandleFunc("/search/collections", handler.SearchCollections).Methods("GET")

        // Discover / Trending
        api.HandleFunc("/discover/trending", handler.GetTrending).Methods("GET")
        api.HandleFunc("/discover/popular", handler.GetPopular).Methods("GET")
        api.HandleFunc("/discover/now-playing", handler.GetNowPlaying).Methods("GET")

	// Collections
	api.HandleFunc("/collections", handler.ListCollections).Methods("GET")
	api.HandleFunc("/collections/{id}", handler.GetCollection).Methods("GET")
	api.HandleFunc("/collections/{id}/sync", handler.SyncCollection).Methods("POST")
	api.HandleFunc("/collections/{id}/movies", handler.GetCollectionMovies).Methods("GET")

	// Services
	api.HandleFunc("/services", handler.GetServices).Methods("GET")
	api.HandleFunc("/services/{name}", handler.UpdateServiceEnabled).Methods("PUT")
	api.HandleFunc("/services/{name}/trigger", handler.TriggerService).Methods("POST")

	// Settings
	api.HandleFunc("/settings", handler.GetSettings).Methods("GET")
	api.HandleFunc("/settings", handler.UpdateSettings).Methods("PUT")

	// Calendar
	api.HandleFunc("/calendar", handler.GetCalendar).Methods("GET")

	// MDBList
	api.HandleFunc("/mdblist/user-lists", handler.GetMDBListUserLists).Methods("GET")

	// Stats (for dashboard)
	api.HandleFunc("/stats", handler.GetStats).Methods("GET")

	// Database management
	api.HandleFunc("/database/stats", handler.GetDatabaseStats).Methods("GET")
	api.HandleFunc("/database/{action}", handler.ExecuteDatabaseAction).Methods("POST")

	// Version / Updates
	api.HandleFunc("/version", handler.GetVersion).Methods("GET")
	api.HandleFunc("/version/check", handler.CheckForUpdates).Methods("GET")
	api.HandleFunc("/update/install", handler.InstallUpdate).Methods("POST")

	// Adult VOD Import
	api.HandleFunc("/adult-vod/import", handler.ImportAdultVOD).Methods("POST")
	api.HandleFunc("/adult-vod/stats", handler.GetAdultVODStats).Methods("GET")

	// Serve static UI files (SPA)
	uiPath := getUIPath()
	if uiPath != "" {
		spa := spaHandler{staticPath: uiPath, indexPath: "index.html"}
		r.PathPrefix("/").Handler(spa)
	}

	// Logging middleware
	r.Use(loggingMiddleware)

	// Return with CORS middleware wrapping at top level (handles OPTIONS before routing)
	return corsMiddleware(r)
}
