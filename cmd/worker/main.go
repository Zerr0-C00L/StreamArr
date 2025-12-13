package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	// Initialize stores for collection worker
	movieStore := database.NewMovieStore(db)
	seriesStore := database.NewSeriesStore(db)
	episodeStore := database.NewEpisodeStore(db)
	collectionStore := database.NewCollectionStore(db)

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

	// Worker 8: Stream Search (every 6 hours)
	go streamSearchWorker(ctx, movieStore, settingsManager, 6*time.Hour)

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

func collectionSyncWorker(ctx context.Context, collectionStore *database.CollectionStore, movieStore *database.MovieStore, tmdbClient *services.TMDBClient, settingsManager *settings.Manager, interval time.Duration) {
	log.Printf("📦 Collection Sync Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	runCollectionSync(ctx, collectionStore, movieStore, tmdbClient, settingsManager)
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📦 Collection Sync Worker: Stopping")
			return
		case <-ticker.C:
			runCollectionSync(ctx, collectionStore, movieStore, tmdbClient, settingsManager)
		}
	}
}

func runCollectionSync(ctx context.Context, collectionStore *database.CollectionStore, movieStore *database.MovieStore, tmdbClient *services.TMDBClient, settingsManager *settings.Manager) {
	log.Println("📦 Collection Sync Worker: Phase 1 - Scanning movies for collections...")
	
	// Phase 1: Scan and link movies to collections
	movies, err := movieStore.ListUncheckedForCollection(ctx)
	if err != nil {
		log.Printf("❌ Collection Sync Phase 1 error: %v", err)
		return
	}
	
	totalMovies := len(movies)
	if totalMovies == 0 {
		log.Println("✅ Collection Sync Phase 1: All movies already checked")
	} else {
		log.Printf("📦 Scanning %d unchecked movies...\n", totalMovies)
		linked := 0
		
		for i, movie := range movies {
			if i%10 == 0 {
				log.Printf("📦 Progress: %d/%d movies scanned\n", i, totalMovies)
			}
			
			_, collection, err := tmdbClient.GetMovieWithCollection(ctx, movie.TMDBID)
			if err != nil {
				movieStore.MarkCollectionChecked(ctx, movie.ID)
				continue
			}
			
			if collection != nil {
				fullCollection, _, err := tmdbClient.GetCollection(ctx, collection.TMDBID)
				if err != nil {
					movieStore.MarkCollectionChecked(ctx, movie.ID)
					continue
				}
				
				if err := collectionStore.Create(ctx, fullCollection); err != nil {
					movieStore.MarkCollectionChecked(ctx, movie.ID)
					continue
				}
				
				if err := collectionStore.UpdateMovieCollection(ctx, movie.ID, fullCollection.ID); err != nil {
					movieStore.MarkCollectionChecked(ctx, movie.ID)
					continue
				}
				
				linked++
			}
			
			movieStore.MarkCollectionChecked(ctx, movie.ID)
		}
		
		log.Printf("✅ Collection Sync Phase 1 complete: %d movies linked to collections\n", linked)
	}
	
	// Phase 2: Sync incomplete collections if auto-add is enabled
	settings := settingsManager.Get()
	if settings.AutoAddCollections {
		log.Println("📦 Collection Sync Phase 2: Adding missing movies from incomplete collections...")
		
		collections, _, _ := collectionStore.GetCollectionsWithProgress(ctx, 1000, 0)
		var incompleteColls []*models.Collection
		for _, coll := range collections {
			if coll.MoviesInLibrary < coll.TotalMovies {
				incompleteColls = append(incompleteColls, coll)
			}
		}
		
		if len(incompleteColls) == 0 {
			log.Println("✅ Collection Sync Phase 2: All collections complete!")
		} else {
			log.Printf("📦 Found %d incomplete collections - skipping auto-add (requires stream search)\n", len(incompleteColls))
			log.Println("ℹ️  Use 'Add Collection' button in UI to manually add missing movies")
		}
	} else {
		log.Println("📦 Collection Sync Phase 2 skipped: AutoAddCollections is disabled")
	}
}
func episodeScanWorker(ctx context.Context, seriesStore *database.SeriesStore, episodeStore *database.EpisodeStore, tmdbClient *services.TMDBClient, interval time.Duration) {
	log.Printf("📺 Episode Scan Worker: Starting (interval: %v)", interval)
	
	// Run immediately on startup
	runEpisodeScan(ctx, seriesStore, episodeStore, tmdbClient)
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("📺 Episode Scan Worker: Stopping")
			return
		case <-ticker.C:
			runEpisodeScan(ctx, seriesStore, episodeStore, tmdbClient)
		}
	}
}

func runEpisodeScan(ctx context.Context, seriesStore *database.SeriesStore, episodeStore *database.EpisodeStore, tmdbClient *services.TMDBClient) {
	log.Println("📺 Episode Scan Worker: Scanning episodes for all series...")
	
	allSeries, err := seriesStore.List(ctx, 0, 10000, nil)
	if err != nil {
		log.Printf("❌ Episode Scan error: %v", err)
		return
	}
	
	totalSeries := len(allSeries)
	if totalSeries == 0 {
		log.Println("✅ Episode Scan: No series in library")
		return
	}
	
	log.Printf("📺 Found %d series to scan\n", totalSeries)
	totalEpisodes := 0
	errors := 0
	
	for i, series := range allSeries {
		if i%5 == 0 {
			log.Printf("📺 Progress: %d/%d series scanned\n", i, totalSeries)
		}
		
		// Get series details from TMDB
		tmdbSeries, err := tmdbClient.GetSeries(ctx, series.TMDBID)
		if err != nil {
			errors++
			continue
		}
		
		numSeasons := tmdbSeries.Seasons
		if numSeasons == 0 {
			continue
		}
		
		// Get all episodes for this series
		episodes, err := tmdbClient.GetEpisodes(ctx, series.ID, series.TMDBID, numSeasons)
		if err != nil {
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
			if err := episodeStore.AddBatch(ctx, episodes); err == nil {
				totalEpisodes += len(episodes)
			}
		}
		
		time.Sleep(200 * time.Millisecond) // Rate limit
	}
	
	log.Printf("✅ Episode Scan complete: %d episodes for %d series (%d errors)\n", totalEpisodes, totalSeries, errors)
}

func streamSearchWorker(ctx context.Context, movieStore *database.MovieStore, settingsManager *settings.Manager, interval time.Duration) {
	log.Printf("🔍 Stream Search Worker: Starting (interval: %v)", interval)
	
	// Wait one interval before first run to avoid startup load
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("🔍 Stream Search Worker: Stopping")
			return
		case <-ticker.C:
			runStreamSearch(ctx, movieStore, settingsManager)
		}
	}
}

func runStreamSearch(ctx context.Context, movieStore *database.MovieStore, settingsManager *settings.Manager) {
	log.Println("🔍 Stream Search Worker: Checking stream availability...")
	
	// Query for monitored movies that need checking
	query := `
		SELECT id, tmdb_id, imdb_id, title 
		FROM library_movies 
		WHERE monitored = true 
		AND imdb_id IS NOT NULL 
		AND (last_checked IS NULL OR last_checked < NOW() - INTERVAL '7 days')
		ORDER BY added_at DESC
		LIMIT 50
	`
	
	rows, err := movieStore.GetDB().QueryContext(ctx, query)
	if err != nil {
		log.Printf("❌ Stream Search error: %v", err)
		return
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
		if err := rows.Scan(&m.ID, &m.TMDBID, &m.IMDBID, &m.Title); err != nil {
			continue
		}
		if m.IMDBID != "" {
			movies = append(movies, m)
		}
	}
	
	total := len(movies)
	if total == 0 {
		log.Println("✅ Stream Search: No movies to scan")
		return
	}
	
	log.Printf("🔍 Found %d movies to check\n", total)
	
	// Get provider settings
	appSettings := settingsManager.Get()
	providerNames := appSettings.StreamProviders
	if len(providerNames) == 0 {
		providerNames = []string{"comet", "mediafusion"}
	}
	
	foundStreams := 0
	
	for i, movie := range movies {
		if i%10 == 0 {
			log.Printf("🔍 Progress: %d/%d movies checked\n", i, total)
		}
		
		hasStreams := false
		
		// Check each provider for streams
		for _, providerName := range providerNames {
			var url string
			switch providerName {
			case "comet":
				url = fmt.Sprintf("https://comet.elfhosted.com/stream/movie/%s.json", movie.IMDBID)
			case "mediafusion":
				url = fmt.Sprintf("https://mediafusion.elfhosted.com/stream/movie/%s.json", movie.IMDBID)
			default:
				continue
			}
			
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(url)
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
		}
		
		if hasStreams {
			foundStreams++
		}
		
		// Update movie availability
		updateQuery := `UPDATE library_movies SET available = $1, last_checked = NOW() WHERE id = $2`
		movieStore.GetDB().ExecContext(ctx, updateQuery, hasStreams, movie.ID)
		
		time.Sleep(500 * time.Millisecond) // Rate limit
	}
	
	log.Printf("✅ Stream Search complete: %d/%d movies have available streams\n", foundStreams, total)
}
