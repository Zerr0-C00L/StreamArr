package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Zerr0-C00L/StreamArr/internal/auth"
	"github.com/gorilla/mux"
)

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

	// Add aggressive no-cache headers for all static files
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

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

// cleanupOldAssets removes orphaned asset files (old builds) from the UI dist folder
func cleanupOldAssets(uiPath string) {
	assetsDir := filepath.Join(uiPath, "assets")
	indexPath := filepath.Join(uiPath, "index.html")

	// Read index.html to find currently referenced assets
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		log.Printf("[Cleanup] Cannot read index.html: %v", err)
		return
	}

	indexHTML := string(indexContent)

	// Read all files in assets directory
	files, err := os.ReadDir(assetsDir)
	if err != nil {
		// Assets directory might not exist or be accessible
		return
	}

	cleaned := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		// Only clean up index-*.js and index-*.css files (built assets)
		if strings.HasPrefix(filename, "index-") && (strings.HasSuffix(filename, ".js") || strings.HasSuffix(filename, ".css")) {
			// Check if this file is referenced in index.html
			if !strings.Contains(indexHTML, filename) {
				// Orphaned file - delete it
				filePath := filepath.Join(assetsDir, filename)
				if err := os.Remove(filePath); err == nil {
					log.Printf("[Cleanup] Removed old asset: %s", filename)
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		log.Printf("[Cleanup] Cleaned up %d old asset file(s)", cleaned)
	}
}

// getUIPath returns the path to the UI dist folder
func getUIPath() string {
	// Prefer host-mounted UI for hot-reload if enabled
	hostFlag := "/app/host/.hotreload"
	hostUI := "/app/host/streamarr-pro-ui/dist"
	if _, err := os.Stat(hostFlag); err == nil {
		if _, err := os.Stat(filepath.Join(hostUI, "index.html")); err == nil {
			cleanupOldAssets(hostUI)
			return hostUI
		}
	}

	// Fallbacks: container-built UI and common install paths
	paths := []string{
		"./streamarr-pro-ui/dist",    // resolves to /app/streamarr-pro-ui/dist in container
		"/app/streamarr-pro-ui/dist", // explicit container path
		"/opt/StreamArr/streamarr-pro-ui/dist",
		"/opt/streamarr/streamarr-pro-ui/dist",
	}

	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "index.html")); err == nil {
			cleanupOldAssets(p)
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-API-Key")
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
		log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// SetupRoutesWithXtream configures API routes with Xtream Codes handler integration
func SetupRoutesWithXtream(handler *Handler, xtreamHandler interface{ RegisterRoutes(*mux.Router) }) http.Handler {
	r := mux.NewRouter()

	// Security middleware - Use Session-based auth instead of Basic Auth
	r.Use(IPWhitelistMiddleware)
	r.Use(auth.SessionMiddleware)
	r.Use(loggingMiddleware)

	// Register Stremio poster proxy FIRST (before Xtream generic routes)
	// This prevents it from matching Xtream's /{username}/{password}/{id}.{ext} pattern
	r.HandleFunc("/stremio/poster/{path:.+}", handler.StremioPostersProxyHandler).Methods("GET", "HEAD")

	// Register Xtream Codes API routes
	xtreamHandler.RegisterRoutes(r)

	// API v1 routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Authentication endpoints (public)
	api.HandleFunc("/auth/login", handler.Login).Methods("POST")
	api.HandleFunc("/auth/logout", handler.Logout).Methods("POST")
	api.HandleFunc("/auth/verify", handler.VerifyToken).Methods("GET")
	api.HandleFunc("/auth/status", handler.AuthStatus).Methods("GET")
	api.HandleFunc("/auth/setup", handler.CreateFirstUser).Methods("POST")

	// Protected auth endpoints (require authentication via token in context)
	api.HandleFunc("/auth/profile", handler.GetCurrentUser).Methods("GET")
	api.HandleFunc("/auth/profile", handler.UpdateProfile).Methods("PUT")
	api.HandleFunc("/auth/password", handler.ChangePassword).Methods("PUT")

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
	api.HandleFunc("/movies/{id}/videos", handler.GetMediaVideos).Methods("GET")

	// Series
	api.HandleFunc("/series", handler.ListSeries).Methods("GET")
	api.HandleFunc("/series", handler.AddSeries).Methods("POST")
	api.HandleFunc("/series/{id}", handler.GetSeries).Methods("GET")
	api.HandleFunc("/series/{id}", handler.UpdateSeries).Methods("PUT")
	api.HandleFunc("/series/{id}", handler.DeleteSeries).Methods("DELETE")
	api.HandleFunc("/series/{id}/episodes", handler.GetSeriesEpisodes).Methods("GET")
	api.HandleFunc("/series/{id}/videos", handler.GetMediaVideos).Methods("GET")

	// Episodes
	api.HandleFunc("/episodes/{id}/play", handler.PlayEpisode).Methods("GET")
	api.HandleFunc("/stream/series/{stream_id}", handler.GetEpisodeStreams).Methods("GET")

	// Channels (Live TV)
	api.HandleFunc("/channels", handler.ListChannels).Methods("GET")
	api.HandleFunc("/channels/categories", handler.GetChannelCategories).Methods("GET")
	api.HandleFunc("/channels/stats", handler.GetChannelStats).Methods("GET")
	api.HandleFunc("/channels/check-source", handler.CheckM3USourceStatus).Methods("POST")
	api.HandleFunc("/channels/epg/guide", handler.GetTVGuide).Methods("GET")
	api.HandleFunc("/channels/{id}", handler.GetChannel).Methods("GET")
	api.HandleFunc("/channels/{id}/stream", handler.GetChannelStream).Methods("GET")
	api.HandleFunc("/channels/proxy", handler.ProxyChannelStream).Methods("GET")

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

	// Blacklist
	api.HandleFunc("/blacklist", handler.GetBlacklist).Methods("GET")
	api.HandleFunc("/blacklist/clear", handler.ClearBlacklist).Methods("POST")
	api.HandleFunc("/blacklist/{id}", handler.RemoveFromBlacklist).Methods("DELETE")
	api.HandleFunc("/{type}/{id}/remove-and-blacklist", handler.RemoveAndBlacklist).Methods("POST")

	// Services
	api.HandleFunc("/services", handler.GetServices).Methods("GET")
	api.HandleFunc("/services/{name}", handler.UpdateServiceEnabled).Methods("PUT")
	api.HandleFunc("/services/{name}/trigger", handler.TriggerService).Methods("POST")

	// Settings
	api.HandleFunc("/settings", handler.GetSettings).Methods("GET")
	api.HandleFunc("/settings", handler.UpdateSettings).Methods("PUT")

	// Admin - System control
	api.HandleFunc("/admin/restart", handler.Restart).Methods("POST")

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

	// Balkan VOD Import
	api.HandleFunc("/balkan-vod/preview-categories", handler.PreviewBalkanCategories).Methods("POST")
	api.HandleFunc("/adult-vod/import", handler.ImportAdultVOD).Methods("POST")
	api.HandleFunc("/adult-vod/stats", handler.GetAdultVODStats).Methods("GET")

	// IPTV VOD Import (from configured M3U/Xtream)
	api.HandleFunc("/iptv-vod/preview-categories", handler.PreviewM3UCategories).Methods("POST")
	api.HandleFunc("/iptv-vod/preview-xtream-categories", handler.PreviewXtreamCategories).Methods("POST")
	api.HandleFunc("/iptv-vod/import", handler.ImportIPTVVOD).Methods("POST")
	// Fallbacks for clients hitting /api/iptv-vod/import or trailing slash
	r.HandleFunc("/api/iptv-vod/import", handler.ImportIPTVVOD).Methods("POST")
	api.HandleFunc("/iptv-vod/import/", handler.ImportIPTVVOD).Methods("POST")

	// Maintenance
	api.HandleFunc("/maintenance/cleanup-bollywood", handler.CleanupBollywoodLibrary).Methods("POST")

	// Stremio Addon Management
	api.HandleFunc("/stremio/generate-token", handler.GenerateStremioToken).Methods("POST")
	api.HandleFunc("/stremio/manifest-url", handler.GetStremioManifestURL).Methods("GET")

	// Debug endpoints (for troubleshooting updates)
	api.HandleFunc("/debug/update-status", handler.GetUpdateStatus).Methods("GET")
	api.HandleFunc("/debug/update-log", handler.GetUpdateLog).Methods("GET")

	// Stremio Addon Endpoints (public with token auth)
	r.HandleFunc("/stremio/manifest.json", handler.StremioManifestHandler).Methods("GET")
	r.HandleFunc("/stremio/catalog/{type}/{id}.json", handler.StremioCatalogHandler).Methods("GET")
	r.HandleFunc("/stremio/stream/{type}/{id}.json", handler.StremioStreamHandler).Methods("GET")
	r.HandleFunc("/stremio/poster/{path:.+}", handler.StremioPostersProxyHandler).Methods("GET", "HEAD")

	// Serve static UI files (SPA)
	uiPath := getUIPath()
	if uiPath != "" {
		spa := spaHandler{staticPath: uiPath, indexPath: "index.html"}
		r.PathPrefix("/").Handler(spa)
	}

	// Return with CORS middleware wrapping at top level (handles OPTIONS before routing)
	return corsMiddleware(r)
}
