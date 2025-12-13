package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
)

func collectionSyncWorker(ctx context.Context, collectionStore *models.CollectionStore, movieStore *models.MovieStore, tmdbClient *services.TMDBClient, settingsManager *settings.Manager, interval time.Duration) {
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

func runCollectionSync(ctx context.Context, collectionStore *models.CollectionStore, movieStore *models.MovieStore, tmdbClient *services.TMDBClient, settingsManager *settings.Manager) {
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

func episodeScanWorker(ctx context.Context, seriesStore *models.SeriesStore, episodeStore *models.EpisodeStore, tmdbClient *services.TMDBClient, interval time.Duration) {
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

func runEpisodeScan(ctx context.Context, seriesStore *models.SeriesStore, episodeStore *models.EpisodeStore, tmdbClient *services.TMDBClient) {
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
	
	for i, series := range allSeries {
		if i%5 == 0 {
			log.Printf("📺 Progress: %d/%d series scanned\n", i, totalSeries)
		}
		
		tmdbSeries, err := tmdbClient.GetSeries(ctx, series.TMDBID)
		if err != nil {
			continue
		}
		
		// Get all seasons
		for seasonNum := 1; seasonNum <= tmdbSeries.NumberOfSeasons; seasonNum++ {
			season, err := tmdbClient.GetSeason(ctx, series.TMDBID, seasonNum)
			if err != nil {
				continue
			}
			
			// Store each episode
			for _, ep := range season.Episodes {
				episode := &models.Episode{
					SeriesID:      series.ID,
					SeasonNumber:  seasonNum,
					EpisodeNumber: ep.EpisodeNumber,
					Title:         ep.Name,
					Overview:      ep.Overview,
					AirDate:       ep.AirDate,
					StillPath:     ep.StillPath,
				}
				
				if err := episodeStore.Create(ctx, episode); err == nil {
					totalEpisodes++
				}
			}
			
			time.Sleep(100 * time.Millisecond) // Rate limit
		}
	}
	
	log.Printf("✅ Episode Scan complete: %d episodes processed for %d series\n", totalEpisodes, totalSeries)
}

func streamSearchWorker(ctx context.Context, movieStore *models.MovieStore, streamStore *models.StreamStore, multiProvider *providers.MultiProvider, interval time.Duration) {
	log.Printf("🔍 Stream Search Worker: Starting (interval: %v)", interval)
	
	// Don't run immediately - wait for first interval to avoid startup load
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("🔍 Stream Search Worker: Stopping")
			return
		case <-ticker.C:
			runStreamSearch(ctx, movieStore, streamStore, multiProvider)
		}
	}
}

func runStreamSearch(ctx context.Context, movieStore *models.MovieStore, streamStore *models.StreamStore, multiProvider *providers.MultiProvider) {
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
	foundStreams := 0
	
	for i, movie := range movies {
		if i%10 == 0 {
			log.Printf("🔍 Progress: %d/%d movies checked\n", i, total)
		}
		
		// Search for streams
		streams, _ := multiProvider.GetStreams(ctx, movie.IMDBID, "movie", "")
		
		hasStreams := len(streams) > 0
		if hasStreams {
			foundStreams++
		}
		
		// Update movie availability
		updateQuery := `UPDATE library_movies SET available = $1, last_checked = NOW() WHERE id = $2`
		movieStore.GetDB().ExecContext(ctx, updateQuery, hasStreams, movie.ID)
		
		time.Sleep(500 * time.Millisecond) // Rate limit to avoid overwhelming providers
	}
	
	log.Printf("✅ Stream Search complete: %d/%d movies have available streams\n", foundStreams, total)
}
