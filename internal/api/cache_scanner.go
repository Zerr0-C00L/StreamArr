package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
	"github.com/Zerr0-C00L/StreamArr/internal/services/debrid"
	"github.com/Zerr0-C00L/StreamArr/internal/services/streams"
)

// CacheScanner handles automatic cache maintenance and upgrades
type CacheScanner struct {
	movieStore    *database.MovieStore
	seriesStore   *database.SeriesStore
	cacheStore    *database.StreamCacheStore
	streamService *streams.StreamService
	provider      *providers.MultiProvider
	debridService debrid.DebridService
	ticker        *time.Ticker
	stopChan      chan bool
}

// NewCacheScanner creates a new cache scanner
func NewCacheScanner(
	movieStore *database.MovieStore,
	seriesStore *database.SeriesStore,
	cacheStore *database.StreamCacheStore,
	streamService *streams.StreamService,
	provider *providers.MultiProvider,
	debridService debrid.DebridService,
) *CacheScanner {
	return &CacheScanner{
		movieStore:    movieStore,
		seriesStore:   seriesStore,
		cacheStore:    cacheStore,
		streamService: streamService,
		provider:      provider,
		debridService: debridService,
		stopChan:      make(chan bool),
	}
}

// Start begins the automatic 7-day scan cycle
func (cs *CacheScanner) Start() {
	cs.ticker = time.NewTicker(7 * 24 * time.Hour)
	go func() {
		// Run once on startup after 5 minutes
		time.Sleep(5 * time.Minute)
		log.Println("[CACHE-SCANNER] Running initial scan...")
		cs.ScanAndUpgrade(context.Background())

		// Then run every 7 days
		for {
			select {
			case <-cs.ticker.C:
				log.Println("[CACHE-SCANNER] Running scheduled 7-day scan...")
				cs.ScanAndUpgrade(context.Background())
			case <-cs.stopChan:
				return
			}
		}
	}()
}

// Stop stops the automatic scanning
func (cs *CacheScanner) Stop() {
	if cs.ticker != nil {
		cs.ticker.Stop()
	}
	close(cs.stopChan)
}

// ScanAndUpgrade scans all movies for cache upgrades and empty entries
func (cs *CacheScanner) ScanAndUpgrade(ctx context.Context) error {
	log.Println("[CACHE-SCANNER] Starting library scan for upgrades and empty cache...")
	
	// Mark service as running
	services.GlobalScheduler.MarkRunning(services.ServiceStreamSearch)
	defer func() {
		services.GlobalScheduler.MarkComplete(services.ServiceStreamSearch, nil, 7*24*time.Hour)
	}()
	
	upgraded := 0
	cached := 0
	skipped := 0
	errors := 0
	totalProcessed := 0
	
	// Process in batches of 5000 to handle large libraries (100k+ movies)
	batchSize := 5000
	offset := 0
	
	for {
		// Get batch of movies
		movies, err := cs.movieStore.List(ctx, offset, batchSize, nil)
		if err != nil {
			log.Printf("[CACHE-SCANNER] Error getting movies at offset %d: %v", offset, err)
			return err
		}
		
		if len(movies) == 0 {
			break // No more movies
		}
		
		log.Printf("[CACHE-SCANNER] Processing batch %d-%d (%d movies)...", offset, offset+len(movies), len(movies))

	for _, movie := range movies {
		// Log progress every 100 movies
		if totalProcessed > 0 && totalProcessed%100 == 0 {
			log.Printf("[CACHE-SCANNER] Progress: %d movies scanned (%d cached, %d upgraded, %d skipped)", 
				totalProcessed, cached, upgraded, skipped)
			// Update service progress
			services.GlobalScheduler.UpdateProgress(
				services.ServiceStreamSearch,
				totalProcessed,
				0, // We don't know total yet, will update at end
				fmt.Sprintf("Scanned %d movies (%d cached, %d upgraded)", totalProcessed, cached, upgraded),
			)
		}
		totalProcessed++
		
		// Get IMDB ID
		imdbID, ok := movie.Metadata["imdb_id"].(string)
		if !ok || imdbID == "" {
			skipped++
			continue
		}

		// Get release year
		releaseYear := 0
		if movie.ReleaseDate != nil && !movie.ReleaseDate.IsZero() {
			releaseYear = movie.ReleaseDate.Year()
		}

		// Check existing cache
		existingCache, err := cs.cacheStore.GetCachedStream(ctx, int(movie.ID))
		if err != nil {
			log.Printf("[CACHE-SCANNER] Error checking cache for movie %d: %v", movie.ID, err)
			errors++
			continue
		}

		// Fetch available streams from provider
		providerStreams, err := cs.provider.GetMovieStreamsWithYear(imdbID, releaseYear)
		if err != nil {
			log.Printf("[CACHE-SCANNER] Error fetching streams for %s (%s): %v", movie.Title, imdbID, err)
			// Add long delay on error to avoid rate limiting
			time.Sleep(30 * time.Second)
			errors++
			continue
		}
		if len(providerStreams) == 0 {
			if totalProcessed%50 == 0 {
				log.Printf("[CACHE-SCANNER] No streams found for %s (%s)", movie.Title, imdbID)
			}
			continue
		}
		
		log.Printf("[CACHE-SCANNER] Found %d streams for %s (%s)", len(providerStreams), movie.Title, imdbID)
		
		// Add delay between successful requests to avoid rate limiting
		time.Sleep(5 * time.Second)

		// Check which streams are cached in RD
		// Extract hashes from URL if InfoHash is empty (happens with Torrentio+RD)
		hashes := make([]string, 0)
		for i := range providerStreams {
			hash := providerStreams[i].InfoHash
			if hash == "" && providerStreams[i].URL != "" {
				// Extract hash from URL (40-character hex string)
				parts := []rune(providerStreams[i].URL)
				for j := 0; j < len(parts)-40; j++ {
					candidate := string(parts[j : j+40])
					// Check if it's a valid hex hash (40 chars, alphanumeric)
					if len(candidate) == 40 {
						hash = candidate
						providerStreams[i].InfoHash = hash // Update the stream
						break
					}
				}
			}
			if hash != "" {
				hashes = append(hashes, hash)
			}
		}
		
		log.Printf("[CACHE-SCANNER] Extracted %d hashes from %d streams for %s", len(hashes), len(providerStreams), movie.Title)
		
		// Note: Torrentio with RD configured already filters to cached-only streams
		// No need to call CheckCache again - all returned streams are pre-filtered as cached
		log.Printf("[CACHE-SCANNER] All %d streams are pre-filtered as cached by Torrentio+RD for %s", len(providerStreams), movie.Title)

		// Find best cached stream
		var bestStream *providers.TorrentioStream
		bestScore := 0
		hasExistingCache := false
		if existingCache != nil {
			bestScore = existingCache.QualityScore
			hasExistingCache = true
		}

		for i := range providerStreams {
			// All streams from Torrentio+RD are already cached, no need to check again

			// Parse and score
			parsed := cs.streamService.ParseStreamFromTorrentName(
				providerStreams[i].Title,
				providerStreams[i].InfoHash,
				providerStreams[i].Source,
				0,
			)
			quality := streams.StreamQuality{
				Resolution:  parsed.Resolution,
				HDRType:     parsed.HDRType,
				AudioFormat: parsed.AudioFormat,
				Source:      parsed.Source,
				Codec:       parsed.Codec,
				SizeGB:      parsed.SizeGB,
			}
			score := streams.CalculateScore(quality).TotalScore

			// For movies with no cache, accept any stream (score >= 0)
			// For movies with cache, only upgrade if better (score > bestScore)
			if (!hasExistingCache && score >= 0) || (hasExistingCache && score > bestScore) {
				bestScore = score
				bestStream = &providerStreams[i]
			}
		}

		// Cache or upgrade if we found a better stream
		if bestStream != nil {
			// Extract hash from URL if needed
			hash := bestStream.InfoHash
			if hash == "" && bestStream.URL != "" {
				parts := []rune(bestStream.URL)
				for i := 0; i < len(parts)-40; i++ {
					candidate := string(parts[i : i+40])
					if len(candidate) == 40 {
						hash = candidate
						break
					}
				}
			}

			stream := models.TorrentStream{
				Hash:        hash,
				Title:       bestStream.Name,
				TorrentName: bestStream.Title,
				Resolution:  bestStream.Quality,
				SizeGB:      float64(bestStream.Size) / (1024 * 1024 * 1024),
				Indexer:     bestStream.Source,
			}

			// Parse for quality details
			parsed := cs.streamService.ParseStreamFromTorrentName(stream.TorrentName, stream.Hash, stream.Indexer, 0)
			quality := streams.StreamQuality{
				Resolution:  parsed.Resolution,
				HDRType:     parsed.HDRType,
				AudioFormat: parsed.AudioFormat,
				Source:      parsed.Source,
				Codec:       parsed.Codec,
				SizeGB:      parsed.SizeGB,
			}
			stream.QualityScore = streams.CalculateScore(quality).TotalScore
			stream.Resolution = parsed.Resolution
			stream.HDRType = parsed.HDRType
			stream.AudioFormat = parsed.AudioFormat
			stream.Source = parsed.Source
			stream.Codec = parsed.Codec

			// Save to cache
			if err := cs.cacheStore.CacheStream(ctx, int(movie.ID), stream, bestStream.URL); err != nil {
				log.Printf("[CACHE-SCANNER] ❌ Error caching stream for movie %d (%s): %v", movie.ID, movie.Title, err)
				errors++
			} else {
				if existingCache == nil {
					cached++
					log.Printf("[CACHE-SCANNER] ✅ Cached: %s | %s | Score: %d", movie.Title, stream.Resolution, stream.QualityScore)
				} else {
					upgraded++
					log.Printf("[CACHE-SCANNER] ⬆️  Upgraded: %s | %s → %s | Score: %d → %d", 
						movie.Title, existingCache.Resolution, stream.Resolution, existingCache.QualityScore, stream.QualityScore)
				}
			}
		}
	}
	
		// Move to next batch
		offset += batchSize
		
		// Short break between batches to avoid overwhelming the system
		time.Sleep(2 * time.Second)
	}

	log.Printf("[CACHE-SCANNER] Movies scan complete: %d total movies processed, %d upgraded, %d newly cached, %d skipped, %d errors", 
		totalProcessed, upgraded, cached, skipped, errors)
	
	// Now scan series (scan first episode of first season for each series as a sample)
	log.Println("[CACHE-SCANNER] Starting series scan...")
	services.GlobalScheduler.UpdateProgress(
		services.ServiceStreamSearch,
		totalProcessed,
		totalProcessed,
		"Starting series scan...",
	)
	seriesScanned, seriesCached, seriesErrors := cs.scanSeries(ctx)
	log.Printf("[CACHE-SCANNER] Series scan complete: %d series scanned, %d cached, %d errors", 
		seriesScanned, seriesCached, seriesErrors)
	
	log.Printf("[CACHE-SCANNER] === FULL SCAN COMPLETE ===")
	log.Printf("[CACHE-SCANNER] Movies: %d processed, %d upgraded, %d cached", totalProcessed, upgraded, cached)
	log.Printf("[CACHE-SCANNER] Series: %d scanned, %d cached", seriesScanned, seriesCached)
	log.Printf("[CACHE-SCANNER] Total errors: %d", errors+seriesErrors)
	
	return nil
}

// scanSeries scans all series and caches first episode of first season
func (cs *CacheScanner) scanSeries(ctx context.Context) (int, int, int) {
	scanned := 0
	cached := 0
	errors := 0
	
	batchSize := 5000
	offset := 0
	
	for {
		series, err := cs.seriesStore.List(ctx, offset, batchSize, nil)
		if err != nil {
			log.Printf("[CACHE-SCANNER] Error getting series at offset %d: %v", offset, err)
			return scanned, cached, errors + 1
		}
		
		if len(series) == 0 {
			break
		}
		
		log.Printf("[CACHE-SCANNER] Processing series batch %d-%d (%d series)...", offset, offset+len(series), len(series))
		
		for _, s := range series {
			scanned++
			
			if scanned > 0 && scanned%100 == 0 {
				log.Printf("[CACHE-SCANNER] Series progress: %d scanned, %d cached", scanned, cached)
				services.GlobalScheduler.UpdateProgress(
					services.ServiceStreamSearch,
					0,
					0,
					fmt.Sprintf("Series: %d scanned, %d cached", scanned, cached),
				)
			}
			
			// Get IMDB ID
			imdbID, ok := s.Metadata["imdb_id"].(string)
			if !ok || imdbID == "" {
				continue
			}
			
			// For now, cache S01E01 as a sample (in future: scan all episodes)
			season, episode := 1, 1
			
			// Check if already cached
			existsQuery := `SELECT COUNT(*) FROM media_streams WHERE series_id = $1 AND season = $2 AND episode = $3`
			var count int
			if err := cs.cacheStore.GetDB().QueryRowContext(ctx, existsQuery, s.ID, season, episode).Scan(&count); err == nil && count > 0 {
				continue // Already cached
			}
			
			// Fetch streams for this episode
			providerStreams, err := cs.provider.GetSeriesStreams(imdbID, season, episode)
			if err != nil || len(providerStreams) == 0 {
				continue
			}
			
			// Extract hashes from URL if InfoHash is empty (happens with Torrentio+RD)
			hashes := make([]string, 0)
			for i := range providerStreams {
				hash := providerStreams[i].InfoHash
				if hash == "" && providerStreams[i].URL != "" {
					// Extract hash from URL (40-character hex string)
					parts := []rune(providerStreams[i].URL)
					for j := 0; j < len(parts)-40; j++ {
						candidate := string(parts[j : j+40])
						if len(candidate) == 40 {
							hash = candidate
							providerStreams[i].InfoHash = hash
							break
						}
					}
				}
				if hash != "" {
					hashes = append(hashes, hash)
				}
			}
			
			if len(hashes) == 0 {
				continue
			}
			
			// Note: Torrentio with RD configured already filters to cached-only streams
			// All returned streams are pre-filtered as cached
			
			// Find best cached stream
			var bestStream *providers.TorrentioStream
			bestScore := 0
			
			for i := range providerStreams {
				// All streams from Torrentio+RD are already cached
				
				parsed := cs.streamService.ParseStreamFromTorrentName(
					providerStreams[i].Title,
					providerStreams[i].InfoHash,
					providerStreams[i].Source,
					0,
				)
				quality := streams.StreamQuality{
					Resolution:  parsed.Resolution,
					HDRType:     parsed.HDRType,
					AudioFormat: parsed.AudioFormat,
					Source:      parsed.Source,
					Codec:       parsed.Codec,
					SizeGB:      parsed.SizeGB,
				}
				score := streams.CalculateScore(quality).TotalScore
				
				if score > bestScore {
					bestScore = score
					bestStream = &providerStreams[i]
				}
			}
			
			if bestStream == nil {
				continue
			}
			
			// Extract hash
			hash := bestStream.InfoHash
			if hash == "" && bestStream.URL != "" {
				parts := []rune(bestStream.URL)
				for i := 0; i < len(parts)-40; i++ {
					candidate := string(parts[i : i+40])
					if len(candidate) == 40 {
						hash = candidate
						break
					}
				}
			}
			
			// Parse quality details
			parsed := cs.streamService.ParseStreamFromTorrentName(bestStream.Title, hash, bestStream.Source, 0)
			quality := streams.StreamQuality{
				Resolution:  parsed.Resolution,
				HDRType:     parsed.HDRType,
				AudioFormat: parsed.AudioFormat,
				Source:      parsed.Source,
				Codec:       parsed.Codec,
				SizeGB:      parsed.SizeGB,
			}
			qualityScore := streams.CalculateScore(quality).TotalScore
			
			// Insert into media_streams for series
			insertQuery := `
				INSERT INTO media_streams (
					series_id, season, episode, stream_url, stream_hash, 
					quality_score, resolution, hdr_type, audio_format, 
					source_type, file_size_gb, codec, indexer
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				ON CONFLICT (series_id, season, episode) 
				DO UPDATE SET 
					stream_url = EXCLUDED.stream_url,
					stream_hash = EXCLUDED.stream_hash,
					quality_score = EXCLUDED.quality_score,
					resolution = EXCLUDED.resolution,
					hdr_type = EXCLUDED.hdr_type,
					audio_format = EXCLUDED.audio_format,
					source_type = EXCLUDED.source_type,
					file_size_gb = EXCLUDED.file_size_gb,
					codec = EXCLUDED.codec,
					indexer = EXCLUDED.indexer,
					updated_at = NOW()
			`
			
			_, err = cs.cacheStore.GetDB().ExecContext(ctx, insertQuery,
				s.ID, season, episode, bestStream.URL, hash,
				qualityScore, parsed.Resolution, parsed.HDRType, parsed.AudioFormat,
				parsed.Source, parsed.SizeGB, parsed.Codec, bestStream.Source,
			)
			
			if err != nil {
				log.Printf("[CACHE-SCANNER] ❌ Error caching series %s S%02dE%02d: %v", s.Title, season, episode, err)
				errors++
			} else {
				cached++
				log.Printf("[CACHE-SCANNER] ✅ Cached series: %s S%02dE%02d | %s | Score: %d", 
					s.Title, season, episode, parsed.Resolution, qualityScore)
			}
		}
		
		offset += batchSize
		time.Sleep(2 * time.Second)
	}
	
	return scanned, cached, errors
}

