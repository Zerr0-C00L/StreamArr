package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) http.Handler {
	r := mux.NewRouter()

	// Root handler
	r.HandleFunc("/", handler.RootHandler).Methods("GET")

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

	// Enable CORS
	r.Use(corsMiddleware)

	// Logging middleware
	r.Use(loggingMiddleware)

	return r
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
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

	// Root handler
	r.HandleFunc("/", handler.RootHandler).Methods("GET")

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

	// Enable CORS
	r.Use(corsMiddleware)

	// Logging middleware
	r.Use(loggingMiddleware)

	return r
}
