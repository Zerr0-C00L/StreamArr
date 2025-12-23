package api

import (
	"context"
	"log"
	"net/http"
	
	"github.com/Zerr0-C00L/StreamArr/internal/services/streams"
)

// ============ PHASE 1: STREAM CACHE MONITORING ENDPOINTS ============

// GetCacheStats returns statistics about the stream cache
func (h *Handler) GetCacheStats(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] GetCacheStats called")
	ctx := r.Context()
	
	if h.streamCacheStore == nil {
		log.Printf("[DEBUG] streamCacheStore is nil")
		respondError(w, http.StatusServiceUnavailable, "stream cache not enabled")
		return
	}
	
	log.Printf("[DEBUG] Getting cache stats from store")
	stats, err := h.streamCacheStore.GetCacheStats(ctx)
	if err != nil {
		log.Printf("Error getting cache stats: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to get cache stats")
		return
	}
	
	log.Printf("[DEBUG] Returning cache stats: %+v", stats)
	respondJSON(w, http.StatusOK, stats)
}

// GetLibraryHealth returns health analysis of the cached streams
func (h *Handler) GetLibraryHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	if h.streamCacheStore == nil {
		respondError(w, http.StatusServiceUnavailable, "stream cache not enabled")
		return
	}
	
	// Get the database connection from the store
	healthMonitor := streams.NewHealthMonitor(h.streamCacheStore.GetDB())
	report, err := healthMonitor.GenerateHealthReport(ctx)
	if err != nil {
		log.Printf("Error generating health report: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to generate health report")
		return
	}
	
	respondJSON(w, http.StatusOK, report)
}

// GetDuplicates returns potential duplicate streams in the cache
func (h *Handler) GetDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	if h.streamCacheStore == nil {
		respondError(w, http.StatusServiceUnavailable, "stream cache not enabled")
		return
	}
	
	// Get the database connection from the store
	detector := streams.NewDuplicateDetector(h.streamCacheStore.GetDB())
	duplicates, err := detector.FindDuplicates(ctx, 0.85)
	if err != nil {
		log.Printf("Error finding duplicates: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to find duplicates")
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_duplicates": len(duplicates),
		"duplicates":       duplicates,
	})
}

// GetUpgradesAvailable returns streams with better quality available
func (h *Handler) GetUpgradesAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	if h.streamCacheStore == nil {
		respondError(w, http.StatusServiceUnavailable, "stream cache not enabled")
		return
	}
	
	upgrades, err := h.streamCacheStore.GetStreamsWithUpgradesAvailable(ctx, 100)
	if err != nil {
		log.Printf("Error getting upgrades: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to get upgrades")
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_upgrades": len(upgrades),
		"upgrades":       upgrades,
	})
}

// GetCachedMoviesList returns a list of all cached movies with details
func (h *Handler) GetCachedMoviesList(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] GetCachedMoviesList called")
	ctx := r.Context()
	
	if h.streamCacheStore == nil {
		log.Printf("[DEBUG] streamCacheStore is nil in GetCachedMoviesList")
		respondError(w, http.StatusServiceUnavailable, "stream cache not enabled")
		return
	}
	
	log.Printf("[DEBUG] Querying cached movies from database")
	// Query all cached streams from database with movie details
	query := `
		SELECT 
			ms.movie_id,
			m.title,
			m.year,
			ms.quality_score,
			ms.resolution,
			ms.source_type,
			ms.hdr_type,
			ms.audio_format,
			ms.file_size_gb,
			ms.is_available,
			ms.upgrade_available,
			ms.last_checked,
			ms.created_at as cached_at,
			ms.indexer
		FROM media_streams ms
		LEFT JOIN library_movies m ON m.id = ms.movie_id
		ORDER BY ms.created_at DESC
		LIMIT 1000
	`
	
	rows, err := h.streamCacheStore.GetDB().QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error querying cached movies: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to query cached movies")
		return
	}
	defer rows.Close()
	
	type CachedMovie struct {
		MovieID          int     `json:"movie_id"`
		Title            string  `json:"title"`
		Year             int     `json:"year"`
		QualityScore     int     `json:"quality_score"`
		Resolution       string  `json:"resolution"`
		SourceType       string  `json:"source_type"`
		HDRType          string  `json:"hdr_type"`
		AudioFormat      string  `json:"audio_format"`
		FileSizeGB       float64 `json:"file_size_gb"`
		IsAvailable      bool    `json:"is_available"`
		UpgradeAvailable bool    `json:"upgrade_available"`
		LastChecked      string  `json:"last_checked"`
		CachedAt         string  `json:"cached_at"`
		Indexer          string  `json:"indexer"`
	}
	
	var movies []CachedMovie
	for rows.Next() {
		var m CachedMovie
		err := rows.Scan(
			&m.MovieID,
			&m.Title,
			&m.Year,
			&m.QualityScore,
			&m.Resolution,
			&m.SourceType,
			&m.HDRType,
			&m.AudioFormat,
			&m.FileSizeGB,
			&m.IsAvailable,
			&m.UpgradeAvailable,
			&m.LastChecked,
			&m.CachedAt,
			&m.Indexer,
		)
		if err != nil {
			log.Printf("Error scanning cached movie: %v", err)
			continue
		}
		movies = append(movies, m)
	}
	
	respondJSON(w, http.StatusOK, movies)
}

// TriggerCacheScan manually triggers the cache scanner to find upgrades and cache empty movies
func (h *Handler) TriggerCacheScan(w http.ResponseWriter, r *http.Request) {
	if h.cacheScanner == nil {
		respondError(w, http.StatusServiceUnavailable, "cache scanner not enabled")
		return
	}

	// Run scan in background with independent context
	go func() {
		ctx := context.Background() // Use background context, not request context
		if err := h.cacheScanner.ScanAndUpgrade(ctx); err != nil {
			log.Printf("[CACHE-SCANNER] Manual scan failed: %v", err)
		}
	}()

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "started",
		"message": "Cache scan started in background. Check logs for progress.",
	})
}
