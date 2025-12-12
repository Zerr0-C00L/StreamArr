package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"github.com/Zerr0-C00L/StreamArr/internal/api"
	"github.com/Zerr0-C00L/StreamArr/internal/config"
	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/epg"
	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
	"github.com/Zerr0-C00L/StreamArr/internal/playlist"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
	"github.com/Zerr0-C00L/StreamArr/internal/xtream"
)

func main() {
	log.Println("Starting StreamArr API Server...")

	// Load initial configuration (uses DATABASE_URL from environment if set)
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connection established")

	// Initialize stores
	movieStore := database.NewMovieStore(db)
	seriesStore := database.NewSeriesStore(db)
	episodeStore := database.NewEpisodeStore(db)
	streamStore := database.NewStreamStore(db)
	settingsStore := database.NewSettingsStore(db)
	collectionStore := database.NewCollectionStore(db)
	userStore, err := database.NewUserStore(db)
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}
	log.Println("Database stores initialized")

	// Initialize settings manager and load from database
	settingsManager := settings.NewManager(db)
	if err := settingsManager.Load(); err != nil {
		log.Printf("Warning: Could not load settings: %v, using defaults", err)
	}
	log.Println("Settings manager initialized")

	// Override config with ALL settings from database
	appSettings := settingsManager.Get()
	
	// API Keys
	if appSettings.TMDBAPIKey != "" {
		cfg.TMDBAPIKey = appSettings.TMDBAPIKey
		log.Println("✓ TMDB API key loaded from settings")
	}
	if appSettings.RealDebridAPIKey != "" {
		cfg.RealDebridAPIKey = appSettings.RealDebridAPIKey
		cfg.UseRealDebrid = true
		log.Println("✓ Real-Debrid API key loaded from settings")
	}
	if appSettings.PremiumizeAPIKey != "" {
		cfg.PremiumizeAPIKey = appSettings.PremiumizeAPIKey
		cfg.UsePremiumize = true
		log.Println("✓ Premiumize API key loaded from settings")
	}
	if appSettings.MDBListAPIKey != "" {
		cfg.MDBListAPIKey = appSettings.MDBListAPIKey
		log.Println("✓ MDBList API key loaded from settings")
	}
	
	// Service URLs
	if appSettings.TorrentioURL != "" {
		cfg.TorrentioURL = appSettings.TorrentioURL
	}
	if appSettings.CometURL != "" {
		cfg.CometURL = appSettings.CometURL
	}
	if appSettings.MediaFusionURL != "" {
		cfg.MediaFusionURL = appSettings.MediaFusionURL
	}
	
	// Provider settings
	cfg.UseRealDebrid = appSettings.UseRealDebrid
	cfg.UsePremiumize = appSettings.UsePremiumize
	if len(appSettings.StreamProviders) > 0 {
		cfg.StreamProviders = appSettings.StreamProviders
	}
	if appSettings.TorrentioProviders != "" {
		cfg.TorrentioProviders = appSettings.TorrentioProviders
	}
	if len(appSettings.CometIndexers) > 0 {
		cfg.CometIndexers = appSettings.CometIndexers
	}
	
	// Quality settings
	if appSettings.MaxResolution > 0 {
		cfg.MaxResolution = appSettings.MaxResolution
	}
	if appSettings.MaxFileSize > 0 {
		cfg.MaxFileSize = appSettings.MaxFileSize
	}
	cfg.EnableQualityVariants = appSettings.EnableQualityVariants
	cfg.ShowFullStreamName = appSettings.ShowFullStreamName
	
	// Playlist settings
	if appSettings.TotalPages > 0 {
		cfg.TotalPages = appSettings.TotalPages
	}
	if appSettings.MinYear > 0 {
		cfg.MinYear = appSettings.MinYear
	}
	if appSettings.MinRuntime > 0 {
		cfg.MinRuntime = appSettings.MinRuntime
	}
	if appSettings.Language != "" {
		cfg.Language = appSettings.Language
	}
	if appSettings.SeriesOriginCountry != "" {
		cfg.SeriesOriginCountry = appSettings.SeriesOriginCountry
	}
	if appSettings.MoviesOriginCountry != "" {
		cfg.MoviesOriginCountry = appSettings.MoviesOriginCountry
	}
	cfg.UserCreatePlaylist = appSettings.UserCreatePlaylist
	cfg.IncludeAdultVOD = appSettings.IncludeAdultVOD
	if appSettings.AutoCacheIntervalHours > 0 {
		cfg.AutoCacheIntervalHours = appSettings.AutoCacheIntervalHours
	}
	
	// Notification settings
	cfg.EnableNotifications = appSettings.EnableNotifications
	if appSettings.DiscordWebhookURL != "" {
		cfg.DiscordWebhookURL = appSettings.DiscordWebhookURL
	}
	if appSettings.TelegramBotToken != "" {
		cfg.TelegramBotToken = appSettings.TelegramBotToken
	}
	if appSettings.TelegramChatID != "" {
		cfg.TelegramChatID = appSettings.TelegramChatID
	}
	
	// Proxy settings
	cfg.UseHTTPProxy = appSettings.UseHTTPProxy
	if appSettings.HTTPProxy != "" {
		cfg.HTTPProxy = appSettings.HTTPProxy
	}
	
	// Server settings
	if appSettings.ServerPort > 0 {
		cfg.ServerPort = appSettings.ServerPort
	}
	if appSettings.Host != "" {
		cfg.Host = appSettings.Host
	}
	cfg.Debug = appSettings.Debug
	
	log.Println("✓ All settings loaded from database")

	// Initialize service scheduler
	services.InitializeDefaultServices()
	log.Println("Service scheduler initialized")

	// Initialize service clients
	tmdbClient := services.NewTMDBClient(cfg.TMDBAPIKey)
	rdClient := services.NewRealDebridClient(cfg.RealDebridAPIKey)
	torrentioClient := services.NewTorrentioClient()

	// Initialize Live TV channel manager
	channelManager := livetv.NewChannelManager()
	
	// Load M3U sources from settings
	currentSettings := settingsManager.Get()
	if len(currentSettings.M3USources) > 0 {
		m3uSources := make([]livetv.M3USource, len(currentSettings.M3USources))
		for i, s := range currentSettings.M3USources {
			m3uSources[i] = livetv.M3USource{
				Name:    s.Name,
				URL:     s.URL,
				Enabled: s.Enabled,
			}
		}
		channelManager.SetM3USources(m3uSources)
		log.Printf("Live TV: Configured %d custom M3U sources", len(m3uSources))
	}
	
	// Set Pluto TV enabled/disabled from settings (default true if not set)
	channelManager.SetPlutoTVEnabled(currentSettings.LiveTVEnablePlutoTV)
	
	if err := channelManager.LoadChannels(); err != nil {
		log.Printf("Warning: Could not load channels: %v", err)
	} else {
		log.Printf("Live TV: Loaded %d channels", len(channelManager.GetAllChannels()))
	}

	// Test Real-Debrid connection
	ctx := context.Background()
	if cfg.UseRealDebrid && cfg.RealDebridAPIKey != "" {
		if err := rdClient.TestConnection(ctx); err != nil {
			log.Printf("Warning: Real-Debrid connection test failed: %v", err)
		} else {
			log.Println("Real-Debrid connection verified")
		}
	}

	// Initialize EPG manager
	epgManager := epg.NewEPGManager()
	
	// Initialize Xtream Codes API handler
	xtreamHandler := xtream.NewXtreamHandler(cfg, db, tmdbClient, rdClient, channelManager, epgManager)
	
	// Initialize playlist generator
	playlistGen := playlist.NewPlaylistGenerator(cfg, db, tmdbClient)
	_ = playlistGen // Use in background worker or on-demand

	// Update EPG data in background
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		
		for {
			channels := channelManager.GetAllChannels()
			channelList := make([]livetv.Channel, len(channels))
			for i, ch := range channels {
				channelList[i] = *ch
			}
			if err := epgManager.UpdateEPG(channelList); err != nil {
				log.Printf("EPG update error: %v", err)
			} else {
				log.Println("EPG data updated successfully")
			}
			<-ticker.C
		}
	}()

	// Initialize API handler with all components
	handler := api.NewHandlerWithComponents(
		movieStore,
		seriesStore,
		episodeStore,
		streamStore,
		settingsStore,
		userStore,
		collectionStore,
		tmdbClient,
		rdClient,
		torrentioClient,
		channelManager,
		settingsManager,
		epgManager,
	)

	// Create router and setup REST API routes
	router := api.SetupRoutesWithXtream(handler, xtreamHandler)
	
	// Register admin routes
	adminHandler := api.NewAdminHandler(handler)
	if muxRouter, ok := router.(*mux.Router); ok {
		adminHandler.RegisterAdminRoutes(muxRouter)
		log.Println("✓ Admin API enabled at /api/admin")
	}
	
	log.Println("✓ Xtream Codes API enabled at /player_api.php")
	log.Println("✓ REST API enabled at /api/v1")
	log.Println("✓ Multi-provider streaming enabled:", cfg.StreamProviders)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on port %d", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}
