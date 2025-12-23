package api

import (
	"context"
	"log"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services/debrid"
	"github.com/Zerr0-C00L/StreamArr/internal/services/streams"
)

// CacheScanner handles automatic cache maintenance and upgrades
type CacheScanner struct {
	movieStore    *database.MovieStore
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
	cacheStore *database.StreamCacheStore,
	streamService *streams.StreamService,
	provider *providers.MultiProvider,
	debridService debrid.DebridService,
) *CacheScanner {
	return &CacheScanner{
		movieStore:    movieStore,
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
	
	// Get all movies (offset=0, limit=10000, monitored=nil for all)
	movies, err := cs.movieStore.List(ctx, 0, 10000, nil)
	if err != nil {
		log.Printf("[CACHE-SCANNER] Error getting movies: %v", err)
		return err
	}

	upgraded := 0
	cached := 0
	skipped := 0
	errors := 0
	
	totalMovies := len(movies)
	log.Printf("[CACHE-SCANNER] Scanning %d movies for upgrade opportunities...", totalMovies)

	for i, movie := range movies {
		// Log progress every 100 movies
		if i > 0 && i%100 == 0 {
			log.Printf("[CACHE-SCANNER] Progress: %d/%d movies scanned (%d cached, %d upgraded, %d skipped)", 
				i, totalMovies, cached, upgraded, skipped)
		}
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
		if err != nil || len(providerStreams) == 0 {
			continue
		}

		// Check which streams are cached in RD
		hashes := make([]string, 0)
		for _, s := range providerStreams {
			if s.InfoHash != "" {
				hashes = append(hashes, s.InfoHash)
			}
		}
		
		cachedHashes := make(map[string]bool)
		if len(hashes) > 0 {
			cachedHashes, _ = cs.debridService.CheckCache(ctx, hashes)
		}

		// Find best cached stream
		var bestStream *providers.TorrentioStream
		bestScore := 0
		hasExistingCache := false
		if existingCache != nil {
			bestScore = existingCache.QualityScore
			hasExistingCache = true
		}

		for i := range providerStreams {
			// Check if cached in debrid
			if !cachedHashes[providerStreams[i].InfoHash] {
				continue
			}

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

	log.Printf("[CACHE-SCANNER] Scan complete: %d upgraded, %d newly cached, %d skipped, %d errors (total movies: %d)", 
		upgraded, cached, skipped, errors, len(movies))
	return nil
}
