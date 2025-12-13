package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/Zerr0-C00L/StreamArr/internal/cache"
	"github.com/Zerr0-C00L/StreamArr/internal/config"
	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/epg"
	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/playlist"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
)

func main() {
	// Load initial config (uses DATABASE_URL from environment if set)
	cfg := config.Load()

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("🤖 StreamArr Background Workers Starting...")
	log.Println("========================================")

	// Initialize settings manager and load from database
	settingsManager := settings.NewManager(db)
	if err := settingsManager.Load(); err != nil {
		log.Printf("Warning: Could not load settings: %v, using defaults", err)
	}
	
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
	
	// Playlist settings
	if appSettings.TotalPages > 0 {
		cfg.TotalPages = appSettings.TotalPages
	}
	if appSettings.MinYear > 0 {
		cfg.MinYear = appSettings.MinYear
	}
	cfg.OnlyReleasedContent = appSettings.OnlyReleasedContent
	
	log.Println("✓ All settings loaded from database")

	// Initialize components
	tmdbClient := services.NewTMDBClient(cfg.TMDBAPIKey)
	
	// Initialize providers
	multiProvider := providers.NewMultiProvider(
		cfg.RealDebridAPIKey,
		cfg.StreamProviders,
		cfg.TorrentioProviders,
		cfg.CometIndexers,
		tmdbClient,
	)
	
	// Initialize cache manager
	cacheManager := cache.NewManager(db)
	
	// Initialize playlist generator
	playlistGen := playlist.NewEnhancedGenerator(cfg, db, tmdbClient, multiProvider)
	
	// Initialize channel manager
	channelManager := livetv.NewChannelManager()
	
	// Initialize EPG manager
	epgManager := epg.NewEPGManager()

	// Initialize MDBList sync service
	mdbSyncService := services.NewMDBListSyncService(db, cfg.MDBListAPIKey, cfg.TMDBAPIKey)

	// Initialize stores for collection and episode workers
	movieStore := models.NewMovieStore(db)
	seriesStore := models.NewSeriesStore(db)
	episodeStore := models.NewEpisodeStore(db)
	streamStore := models.NewStreamStore(db)
	collectionStore := models.NewCollectionStore(db)

	// Create context for workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start workers
	log.Println("Starting workers...")
	
	// Worker 1: Playlist Regeneration (every 12 hours)
	go playlistWorker(ctx, playlistGen, 12*time.Hour)
	
	// Worker 2: Cache Cleanup (every hour)
	go cacheCleanupWorker(ctx, cacheManager, 1*time.Hour)
	
	// Worker 3: EPG Update (every 6 hours)
	go epgUpdateWorker(ctx, epgManager, channelManager, 6*time.Hour)
	
	// Worker 4: Channel Refresh (every hour)
	go channelRefreshWorker(ctx, channelManager, 1*time.Hour)

	// Worker 5: MDBList Sync (every 6 hours)
	go mdblistSyncWorker(ctx, mdbSyncService, db, 6*time.Hour)

	// Worker 6: Collection Sync (every 24 hours)
	go collectionSyncWorker(ctx, collectionStore, movieStore, tmdbClient, settingsManager, 24*time.Hour)

	// Worker 7: Episode Scan (every 24 hours)
	go episodeScanWorker(ctx, seriesStore, episodeStore, tmdbClient, 24*time.Hour)

	// Worker 8: Stream Search (every 30 minutes)
	go streamSearchWorker(ctx, movieStore, streamStore, multiProvider, 30*time.Minute)

	log.Println("✅ All workers started successfully")
	log.Println("========================================")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n🛑 Shutting down workers...")
	cancel()
	time.Sleep(2 * time.Second)
	log.Println("✅ Shutdown complete")
}

func playlistWorker(ctx context.Context, gen *playlist.EnhancedGenerator, interval time.Duration) {
	log.Printf("📋 Playlist Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	if err := gen.GenerateComplete(ctx); err != nil {
		log.Printf("❌ Playlist generation error: %v", err)
	} else {
		log.Println("✅ Initial playlist generation complete")
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📋 Playlist Worker: Stopping")
			return
		case <-ticker.C:
			log.Println("📋 Playlist Worker: Starting playlist regeneration...")
			if err := gen.GenerateComplete(ctx); err != nil {
				log.Printf("❌ Playlist generation error: %v", err)
			} else {
				log.Println("✅ Playlist regeneration complete")
			}
		}
	}
}

func cacheCleanupWorker(ctx context.Context, manager *cache.Manager, interval time.Duration) {
	log.Printf("🧹 Cache Cleanup Worker: Starting (interval: %v)", interval)
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("🧹 Cache Cleanup Worker: Stopping")
			return
		case <-ticker.C:
			log.Println("🧹 Cache Cleanup Worker: Running cleanup...")
			manager.Cleanup()
			log.Println("✅ Cache cleanup complete")
		}
	}
}

func epgUpdateWorker(ctx context.Context, epgManager *epg.Manager, channelManager *livetv.ChannelManager, interval time.Duration) {
	log.Printf("📺 EPG Update Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	updateEPG(epgManager, channelManager)
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📺 EPG Update Worker: Stopping")
			return
		case <-ticker.C:
			updateEPG(epgManager, channelManager)
		}
	}
}

func updateEPG(epgManager *epg.Manager, channelManager *livetv.ChannelManager) {
	log.Println("📺 EPG Update Worker: Updating EPG data...")
	channels := channelManager.GetAllChannels()
	channelList := make([]livetv.Channel, len(channels))
	for i, ch := range channels {
		channelList[i] = *ch
	}
	
	if err := epgManager.UpdateEPG(channelList); err != nil {
		log.Printf("❌ EPG update error: %v", err)
	} else {
		log.Println("✅ EPG update complete")
	}
}

func channelRefreshWorker(ctx context.Context, manager *livetv.ChannelManager, interval time.Duration) {
	log.Printf("📡 Channel Refresh Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	if err := manager.LoadChannels(); err != nil {
		log.Printf("❌ Initial channel load error: %v", err)
	} else {
		log.Println("✅ Initial channel load complete")
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📡 Channel Refresh Worker: Stopping")
			return
		case <-ticker.C:
			log.Println("📡 Channel Refresh Worker: Refreshing channels...")
			if err := manager.LoadChannels(); err != nil {
				log.Printf("❌ Channel refresh error: %v", err)
			} else {
				log.Println("✅ Channel refresh complete")
			}
		}
	}
}

func mdblistSyncWorker(ctx context.Context, syncService *services.MDBListSyncService, db *sql.DB, interval time.Duration) {
	log.Printf("📋 MDBList Sync Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	log.Println("📋 MDBList Sync Worker: Running initial sync...")
	if err := syncService.SyncAllLists(ctx); err != nil {
		log.Printf("❌ Initial MDBList sync error: %v", err)
	} else {
		movies, series, _ := syncService.GetSyncStats(ctx)
		log.Printf("✅ Initial MDBList sync complete - Library: %d movies, %d series", movies, series)
	}

	// Enrich any existing items missing artwork
	log.Println("📋 MDBList Sync Worker: Enriching items with TMDB artwork...")
	if err := syncService.EnrichExistingItems(ctx); err != nil {
		log.Printf("⚠️ MDBList enrichment error: %v", err)
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📋 MDBList Sync Worker: Stopping")
			return
		case <-ticker.C:
			log.Println("📋 MDBList Sync Worker: Syncing MDBList lists...")
			if err := syncService.SyncAllLists(ctx); err != nil {
				log.Printf("❌ MDBList sync error: %v", err)
			} else {
				movies, series, _ := syncService.GetSyncStats(ctx)
				log.Printf("✅ MDBList sync complete - Library: %d movies, %d series", movies, series)
			}
			// Enrich any new items missing artwork
			if err := syncService.EnrichExistingItems(ctx); err != nil {
				log.Printf("⚠️ MDBList enrichment error: %v", err)
			}
		}
	}
}
