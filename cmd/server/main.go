package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
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
	blacklistStore := database.NewBlacklistStore(db)
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

	// Set up callback for when Balkan VOD is disabled - clean up all Balkan VOD content
	settingsManager.SetOnBalkanVODDisabledCallback(func() error {
		ctx := context.Background()
		movieCount, err := movieStore.DeleteBySource(ctx, "balkan_vod")
		if err != nil {
			return fmt.Errorf("failed to delete Balkan VOD movies: %w", err)
		}
		
		seriesCount, err := seriesStore.DeleteBySource(ctx, "balkan_vod")
		if err != nil {
			return fmt.Errorf("failed to delete Balkan VOD series: %w", err)
		}
		
		log.Printf("✓ Balkan VOD disabled - Removed %d movies and %d series from library", movieCount, seriesCount)
		return nil
	})

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
	
	// Provider settings
	cfg.UseRealDebrid = appSettings.UseRealDebrid
	cfg.UsePremiumize = appSettings.UsePremiumize
	if len(appSettings.StremioAddons) > 0 {
		// Convert settings.StremioAddon to config.StremioAddon
		cfg.StremioAddons = make([]config.StremioAddon, len(appSettings.StremioAddons))
		for i, addon := range appSettings.StremioAddons {
			cfg.StremioAddons[i] = config.StremioAddon{
				Name:    addon.Name,
				URL:     addon.URL,
				Enabled: addon.Enabled,
			}
		}
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
	cfg.OnlyReleasedContent = appSettings.OnlyReleasedContent
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

	// Initialize Live TV channel manager
	channelManager := livetv.NewChannelManager()
	
	// Load M3U sources from settings
	currentSettings := settingsManager.Get()
	// Set Live TV enabled/disabled from settings
	channelManager.SetIncludeLiveTV(currentSettings.IncludeLiveTV)
	// Set IPTV import mode (live_only/vod_only/both) BEFORE loading channels
	channelManager.SetIPTVImportMode(currentSettings.IPTVImportMode)
	if len(currentSettings.M3USources) > 0 {
		m3uSources := make([]livetv.M3USource, len(currentSettings.M3USources))
		for i, s := range currentSettings.M3USources {
			m3uSources[i] = livetv.M3USource{
				Name:               s.Name,
				URL:                s.URL,
				Enabled:            s.Enabled,
				SelectedCategories: s.SelectedCategories,
			}
		}
		channelManager.SetM3USources(m3uSources)
		log.Printf("Live TV: Configured %d custom M3U sources", len(m3uSources))
	}
	
	// Load Xtream sources from settings
	if len(currentSettings.XtreamSources) > 0 {
		xtreamSources := make([]livetv.XtreamSource, len(currentSettings.XtreamSources))
		for i, s := range currentSettings.XtreamSources {
			xtreamSources[i] = livetv.XtreamSource{
				Name:      s.Name,
				ServerURL: s.ServerURL,
				Username:  s.Username,
				Password:  s.Password,
				Enabled:   s.Enabled,
			}
		}
		channelManager.SetXtreamSources(xtreamSources)
		log.Printf("Live TV: Configured %d custom Xtream sources", len(xtreamSources))
	}
	
	// Set stream validation enabled/disabled from settings (default false)
	channelManager.SetStreamValidation(currentSettings.LiveTVValidateStreams)
	if currentSettings.LiveTVValidateStreams {
		log.Println("Live TV: Stream validation enabled - broken streams will be filtered")
	}
	
	
	if err := channelManager.LoadChannels(); err != nil {
		log.Printf("Warning: Could not load channels: %v", err)
	} else {
		log.Printf("Live TV: Loaded %d channels", len(channelManager.GetAllChannels()))
	}

	// Auto-import IPTV VOD when mode includes VOD
	if strings.EqualFold(currentSettings.IPTVImportMode, "vod_only") || strings.EqualFold(currentSettings.IPTVImportMode, "both") {
		if cfg.TMDBAPIKey != "" {
			go func() {
				ctx := context.Background()
				summary, err := services.ImportIPTVVOD(ctx, currentSettings, tmdbClient, movieStore, seriesStore)
				if err != nil {
					log.Printf("IPTV VOD import error: %v", err)
				} else if summary != nil {
					log.Printf("IPTV VOD import: sources=%d items=%d movies=%d series=%d skipped=%d errors=%d",
						summary.SourcesChecked, summary.ItemsFound, summary.MoviesImported, summary.SeriesImported, summary.Skipped, summary.Errors)
				}
				// Cleanup removed providers after import
				_ = services.CleanupIPTVVOD(ctx, currentSettings, movieStore, seriesStore)
			}()
		} else {
			log.Printf("IPTV VOD auto-import skipped: TMDB API key missing")
		}
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
	settings := settingsManager.Get()
	epgManager := epg.NewEPGManager()
	
	// Add custom EPG URLs from M3U sources
	log.Printf("Live TV: Checking %d M3U sources for EPG URLs", len(settings.M3USources))
	if len(settings.M3USources) > 0 {
		var customEPGURLs []string
		for _, s := range settings.M3USources {
			log.Printf("Live TV: M3U source '%s' - enabled=%v, epg_url='%s'", s.Name, s.Enabled, s.EPGURL)
			if s.Enabled {
				// If EPGURL is already set, use it
				if s.EPGURL != "" {
					customEPGURLs = append(customEPGURLs, s.EPGURL)
				} else {
					// Try to extract EPG URL from M3U file header
					extractedURL := livetv.FetchAndExtractEPGURL(s.URL)
					if extractedURL != "" {
						log.Printf("Live TV: Extracted EPG URL from '%s': %s", s.Name, extractedURL)
						customEPGURLs = append(customEPGURLs, extractedURL)
					}
				}
			}
		}
		// Deduplicate EPG URLs
		seen := make(map[string]bool)
		uniqueURLs := make([]string, 0)
		for _, url := range customEPGURLs {
			if !seen[url] {
				seen[url] = true
				uniqueURLs = append(uniqueURLs, url)
			}
		}
		if len(uniqueURLs) > 0 {
			epgManager.AddCustomEPGURLs(uniqueURLs)
			log.Printf("Live TV: Added %d unique custom EPG URLs from M3U sources", len(uniqueURLs))
		}
	}
	
	// Initialize Xtream Codes API handler
	// Convert config.StremioAddon to providers.StremioAddon
	stremioAddons := make([]providers.StremioAddon, len(cfg.StremioAddons))
	for i, addon := range cfg.StremioAddons {
		stremioAddons[i] = providers.StremioAddon{
			Name:    addon.Name,
			URL:     addon.URL,
			Enabled: addon.Enabled,
		}
	}
	
	// Create MultiProvider
	multiProvider := providers.NewMultiProviderWithConfig(cfg.RealDebridAPIKey, stremioAddons, tmdbClient)
	log.Printf("✓ Stream providers enabled: %v", multiProvider.ProviderNames)

	// Create Xtream handler
	xtreamHandler := xtream.NewXtreamHandlerWithProvider(cfg, db, tmdbClient, rdClient, channelManager, epgManager, multiProvider)

	// Wire up settings for hiding unavailable content
	xtreamHandler.SetHideUnavailable(func() bool {
		s := settingsManager.Get()
		return s.HideUnavailableContent
	})

	// Wire up optional duplication of VOD entries per provider for broader IPTV client compatibility
	xtreamHandler.SetDuplicateVODPerProvider(func() bool {
		s := settingsManager.Get()
		return s.DuplicateVODPerProvider
	})

	// Wire up stream sorting settings
	xtreamHandler.SetSortSettings(
		func() string {
			s := settingsManager.Get()
			if s.StreamSortOrder != "" {
				return s.StreamSortOrder
			}
			return "quality,size,seeders"
		},
		func() string {
			s := settingsManager.Get()
			if s.StreamSortPrefer != "" {
				return s.StreamSortPrefer
			}
			return "best"
		},
	)
	
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

	// Periodic IPTV VOD sync (import + cleanup) using configurable interval when mode includes VOD
	go func() {
		for {
			current := settingsManager.Get()
			mode := strings.ToLower(current.IPTVImportMode)
			includesVOD := mode == "vod_only" || mode == "both"
			intervalHours := current.IPTVVODSyncIntervalHours
			if intervalHours <= 0 {
				intervalHours = 6
			}
			interval := time.Duration(intervalHours) * time.Hour
			if includesVOD && cfg.TMDBAPIKey != "" {
				services.GlobalScheduler.MarkRunning(services.ServiceIPTVVODSync)
				ctx := context.Background()
				_, err := services.ImportIPTVVOD(ctx, current, tmdbClient, movieStore, seriesStore)
				if err != nil {
					log.Printf("[Scheduler] IPTV VOD import error: %v", err)
				}
				_ = services.CleanupIPTVVOD(ctx, current, movieStore, seriesStore)
				services.GlobalScheduler.MarkComplete(services.ServiceIPTVVODSync, err, interval)
			}
			time.Sleep(interval)
		}
	}()

	// Initialize MDBList sync service
	mdbSyncService := services.NewMDBListSyncService(db, cfg.MDBListAPIKey, cfg.TMDBAPIKey)
	log.Println("✓ MDBList sync service initialized")

	// Initialize API handler with all components
	handler := api.NewHandlerWithComponents(
		movieStore,
		seriesStore,
		episodeStore,
		streamStore,
		settingsStore,
		userStore,
		collectionStore,
		blacklistStore,
		tmdbClient,
		rdClient,
		channelManager,
		settingsManager,
		epgManager,
		multiProvider,
		mdbSyncService,
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
	
	// Log enabled addons
	enabledAddons := []string{}
	for _, addon := range cfg.StremioAddons {
		if addon.Enabled {
			enabledAddons = append(enabledAddons, addon.Name)
		}
	}
	log.Println("✓ Stremio Addons enabled:", enabledAddons)

	// Create HTTP server with extended timeouts for stream fetching
	// Stremio addons can take up to 120 seconds to respond
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  180 * time.Second, // 3 minutes for slow clients
		WriteTimeout: 180 * time.Second, // 3 minutes to fetch and redirect streams
		IdleTimeout:  120 * time.Second, // 2 minutes idle
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
