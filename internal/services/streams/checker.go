package streams

import (
"github.com/Zerr0-C00L/StreamArr/internal/models"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/services/debrid"
)

// CheckerConfig holds configuration for stream checker
type CheckerConfig struct {
	CheckIntervalMinutes int  // How often to run checks (default: 60)
	BatchSize            int  // How many streams to check per batch (default: 100)
	AutoUpgrade          bool // Automatically upgrade to better streams (default: true)
	MinUpgradePoints     int  // Minimum score improvement for auto-upgrade (default: 20)
	MaxUpgradeSizeGB     int  // Max size increase for upgrade (default: 30)
}

// DefaultCheckerConfig returns default configuration
func DefaultCheckerConfig() CheckerConfig {
	return CheckerConfig{
		CheckIntervalMinutes: 60,
		BatchSize:            100,
		AutoUpgrade:          true,
		MinUpgradePoints:     20,
		MaxUpgradeSizeGB:     30,
	}
}

// StreamChecker manages background availability checks and upgrades
type StreamChecker struct {
	config              CheckerConfig
	cacheStore          *database.StreamCacheStore
	streamSvc           *StreamService
	debrid              debrid.DebridService
	logger              *slog.Logger
	stopChan            chan struct{}
	indexerFunc         func(ctx context.Context, mediaID int) ([]models.TorrentStream, error) // Function to search indexers
	settingsGetter      func() (excludedGroups, excludedQualities, excludedLanguages string, filtersEnabled bool) // Get filter settings
}

// NewStreamChecker creates a new stream checker
func NewStreamChecker(
	config CheckerConfig,
	cacheStore *database.StreamCacheStore,
	streamSvc *StreamService,
	debridSvc debrid.DebridService,
	indexerFunc func(ctx context.Context, mediaID int) ([]models.TorrentStream, error),
	logger *slog.Logger,
) *StreamChecker {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &StreamChecker{
		config:              config,
		cacheStore:          cacheStore,
		streamSvc:           streamSvc,
		debrid:              debridSvc,
		indexerFunc:         indexerFunc,
		logger:              logger,
		stopChan:            make(chan struct{}),
		settingsGetter:      nil, // Will be set by SetSettingsGetter
	}
}

// SetSettingsGetter sets the function to retrieve filter settings
func (c *StreamChecker) SetSettingsGetter(getter func() (string, string, string, bool)) {
	c.settingsGetter = getter
}

// GetConfig returns the checker configuration
func (c *StreamChecker) GetConfig() CheckerConfig {
	return c.config
}

// Start begins the background checker loop
func (c *StreamChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.config.CheckIntervalMinutes) * time.Minute)
	defer ticker.Stop()
	
	c.logger.Info("Stream checker started",
		"interval_minutes", c.config.CheckIntervalMinutes,
		"batch_size", c.config.BatchSize,
		"auto_upgrade", c.config.AutoUpgrade)
	
	// Run initial check immediately
	if err := c.RunCheck(ctx); err != nil {
		c.logger.Error("Initial stream check failed", "error", err)
	}
	
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stream checker stopped (context cancelled)")
			return
		case <-c.stopChan:
			c.logger.Info("Stream checker stopped (stop signal)")
			return
		case <-ticker.C:
			if err := c.RunCheck(ctx); err != nil {
				c.logger.Error("Stream check failed", "error", err)
			}
		}
	}
}

// Stop gracefully stops the checker
func (c *StreamChecker) Stop() {
	close(c.stopChan)
}

// RunCheck performs one check cycle
func (c *StreamChecker) RunCheck(ctx context.Context) error {
	startTime := time.Now()
	c.logger.Info("Starting stream availability check")
	
	// Get streams due for checking
	streams, err := c.cacheStore.GetStreamsDueForCheck(ctx, c.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get streams for check: %w", err)
	}
	
	if len(streams) == 0 {
		c.logger.Info("No streams due for checking")
		return nil
	}
	
	c.logger.Info("Checking stream availability",
		"count", len(streams),
		"batch_size", c.config.BatchSize)
	
	// Extract hashes for batch debrid check
	hashes := make([]string, len(streams))
	hashToStream := make(map[string]*models.CachedStream)
	for i, stream := range streams {
		hashes[i] = stream.StreamHash
		hashToStream[stream.StreamHash] = stream
	}
	
	// Batch check debrid cache status
	cached, err := c.debrid.CheckCache(ctx, hashes)
	if err != nil {
		c.logger.Error("Debrid cache check failed", "error", err)
		return fmt.Errorf("debrid cache check failed: %w", err)
	}
	
	// Process each stream
	var stillCached, expired, upgraded, upgradeAvailable int
	
	for hash, isCached := range cached {
		stream := hashToStream[hash]
		
		if isCached {
			// Stream still cached on debrid
			stillCached++
			
			// Check for better quality version
			if c.config.AutoUpgrade {
				if err := c.checkForUpgrade(ctx, stream); err != nil {
					c.logger.Error("Upgrade check failed",
						"media_id", stream.MovieID,
						"error", err)
				} else if stream.UpgradeAvailable {
					upgradeAvailable++
				}
			}
			
			// Schedule next check in 7 days
			if err := c.cacheStore.UpdateNextCheck(ctx, stream.MovieID, 7); err != nil {
				c.logger.Error("Failed to update next check",
					"media_id", stream.MovieID,
					"error", err)
			}
			
		} else {
			// Stream expired from debrid cache
			expired++
			c.logger.Warn("Stream expired from debrid cache",
				"media_id", stream.MovieID,
				"title", stream.StreamURL)
			
			// Try to find replacement
			replaced, err := c.findReplacement(ctx, stream)
			if err != nil {
				c.logger.Error("Failed to find replacement",
					"media_id", stream.MovieID,
					"error", err)
				// Mark as unavailable, retry tomorrow
				if err := c.cacheStore.MarkUnavailable(ctx, stream.MovieID); err != nil {
					c.logger.Error("Failed to mark unavailable",
						"media_id", stream.MovieID,
						"error", err)
				}
			} else if replaced {
				upgraded++
				c.logger.Info("Found replacement stream",
					"media_id", stream.MovieID)
			} else {
				// No replacement available
				if err := c.cacheStore.MarkUnavailable(ctx, stream.MovieID); err != nil {
					c.logger.Error("Failed to mark unavailable",
						"media_id", stream.MovieID,
						"error", err)
				}
			}
		}
	}
	
	duration := time.Since(startTime)
	c.logger.Info("Stream check completed",
		"duration_seconds", duration.Seconds(),
		"checked", len(streams),
		"still_cached", stillCached,
		"expired", expired,
		"upgraded", upgraded,
		"upgrade_available", upgradeAvailable)
	
	return nil
}

// checkForUpgrade checks if a better quality stream is available
func (c *StreamChecker) checkForUpgrade(ctx context.Context, current *models.CachedStream) error {
	// Skip if indexer function not provided
	if c.indexerFunc == nil {
		return nil
	}
	
	// Search indexers for this media
	results, err := c.indexerFunc(ctx, current.MovieID)
	if err != nil {
		return fmt.Errorf("indexer search failed: %w", err)
	}
	
	if len(results) == 0 {
		return nil // No results
	}
	
	// Apply user filters if configured
	if c.settingsGetter != nil {
		excludedGroups, excludedQualities, excludedLanguages, filtersEnabled := c.settingsGetter()
		if filtersEnabled {
			var filteredResults []models.TorrentStream
			for _, stream := range results {
				if !c.streamSvc.ShouldFilterStream(stream, excludedGroups, excludedQualities, excludedLanguages) {
					filteredResults = append(filteredResults, stream)
				}
			}
			results = filteredResults
			
			c.logger.Info("Applied quality filters to upgrade candidates",
				"original_count", len(results),
				"filtered_count", len(filteredResults))
		}
	}
	
	if len(results) == 0 {
		return nil // All streams filtered out
	}
	
	// Find best cached stream
	best, err := c.streamSvc.FindBestCachedStream(ctx, results)
	if err != nil {
		return fmt.Errorf("failed to find best stream: %w", err)
	}
	
	if best == nil {
		return nil // No cached streams available
	}
	
	// Check if upgrade is worthwhile
	currentStream := models.TorrentStream{
		Hash:         current.StreamHash,
		QualityScore: current.QualityScore,
		Resolution:   current.Resolution,
		HDRType:      current.HDRType,
		Source:       current.SourceType,
		SizeGB:       current.FileSizeGB,
	}
	
	shouldUpgrade := c.streamSvc.ShouldUpgrade(currentStream, *best, c.config.MinUpgradePoints)
	
	if shouldUpgrade {
		// Check size increase limit
		sizeIncrease := best.SizeGB - current.FileSizeGB
		if sizeIncrease > float64(c.config.MaxUpgradeSizeGB) {
			c.logger.Info("Upgrade available but size increase too large",
				"media_id", current.MovieID,
				"current_size_gb", current.FileSizeGB,
				"new_size_gb", best.SizeGB,
				"increase_gb", sizeIncrease)
			
			// Mark upgrade available but don't auto-upgrade
			return c.cacheStore.MarkUpgradeAvailable(ctx, current.MovieID, true)
		}
		
		// Get stream URL from debrid
		streamURL, err := c.debrid.GetStreamURL(ctx, best.Hash, 0)
		if err != nil {
			return fmt.Errorf("failed to get stream URL: %w", err)
		}
		
		// Upgrade to better stream
		if err := c.cacheStore.CacheStream(ctx, current.MovieID, *best, streamURL); err != nil {
			return fmt.Errorf("failed to cache upgraded stream: %w", err)
		}
		
		c.logger.Info("Auto-upgraded to better stream",
			"media_id", current.MovieID,
			"old_score", current.QualityScore,
			"new_score", best.QualityScore,
			"improvement", best.QualityScore-current.QualityScore,
			"old_resolution", current.Resolution,
			"new_resolution", best.Resolution)
		
		return nil
	}
	
	// Check if there's a slight improvement worth flagging
	if best.QualityScore > current.QualityScore+10 {
		return c.cacheStore.MarkUpgradeAvailable(ctx, current.MovieID, true)
	}
	
	return nil
}

// findReplacement tries to find a replacement stream when current expires
func (c *StreamChecker) findReplacement(ctx context.Context, expired *models.CachedStream) (bool, error) {
	// Skip if indexer function not provided
	if c.indexerFunc == nil {
		return false, nil
	}
	
	// Search indexers for this media
	results, err := c.indexerFunc(ctx, expired.MovieID)
	if err != nil {
		return false, fmt.Errorf("indexer search failed: %w", err)
	}
	
	if len(results) == 0 {
		c.logger.Warn("No replacement streams found",
			"media_id", expired.MovieID)
		return false, nil
	}
	
	// Find best cached stream
	best, err := c.streamSvc.FindBestCachedStream(ctx, results)
	if err != nil {
		return false, fmt.Errorf("failed to find best stream: %w", err)
	}
	
	if best == nil {
		c.logger.Warn("No cached replacement available",
			"media_id", expired.MovieID)
		return false, nil
	}
	
	// Get stream URL from debrid
	streamURL, err := c.debrid.GetStreamURL(ctx, best.Hash, 0)
	if err != nil {
		return false, fmt.Errorf("failed to get stream URL: %w", err)
	}
	
	// Cache replacement stream
	if err := c.cacheStore.CacheStream(ctx, expired.MovieID, *best, streamURL); err != nil {
		return false, fmt.Errorf("failed to cache replacement: %w", err)
	}
	
	c.logger.Info("Cached replacement stream",
		"media_id", expired.MovieID,
		"old_score", expired.QualityScore,
		"new_score", best.QualityScore,
		"resolution", best.Resolution)
	
	return true, nil
}

// CheckSpecificStream forces a check for a specific media item
func (c *StreamChecker) CheckSpecificStream(ctx context.Context, mediaID int) error {
	cached, err := c.cacheStore.GetCachedStream(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("failed to get cached stream: %w", err)
	}
	
	if cached == nil {
		return fmt.Errorf("no cached stream for media_id %d", mediaID)
	}
	
	// Check if still cached on debrid
	cacheStatus, err := c.debrid.CheckCache(ctx, []string{cached.StreamHash})
	if err != nil {
		return fmt.Errorf("debrid cache check failed: %w", err)
	}
	
	isCached := cacheStatus[cached.StreamHash]
	
	if isCached {
		c.logger.Info("Stream still cached on debrid",
			"media_id", mediaID)
		
		// Check for upgrade
		if c.config.AutoUpgrade {
			if err := c.checkForUpgrade(ctx, cached); err != nil {
				return fmt.Errorf("upgrade check failed: %w", err)
			}
		}
		
		// Update next check
		return c.cacheStore.UpdateNextCheck(ctx, mediaID, 7)
	}
	
	// Stream expired, find replacement
	c.logger.Warn("Stream expired, finding replacement",
		"media_id", mediaID)
	
	replaced, err := c.findReplacement(ctx, cached)
	if err != nil {
		// Mark unavailable
		if markErr := c.cacheStore.MarkUnavailable(ctx, mediaID); markErr != nil {
			c.logger.Error("Failed to mark unavailable",
				"media_id", mediaID,
				"error", markErr)
		}
		return fmt.Errorf("failed to find replacement: %w", err)
	}
	
	if !replaced {
		// No replacement available
		if err := c.cacheStore.MarkUnavailable(ctx, mediaID); err != nil {
			return fmt.Errorf("failed to mark unavailable: %w", err)
		}
		return fmt.Errorf("no replacement available for media_id %d", mediaID)
	}
	
	return nil
}

// GetStats returns current checker statistics
func (c *StreamChecker) GetStats(ctx context.Context) (map[string]interface{}, error) {
	cacheStats, err := c.cacheStore.GetCacheStats(ctx)
	if err != nil {
		return nil, err
	}
	
	// Add checker config to stats
	stats := make(map[string]interface{})
	for k, v := range cacheStats {
		stats[k] = v
	}
	
	stats["checker_config"] = map[string]interface{}{
		"check_interval_minutes": c.config.CheckIntervalMinutes,
		"batch_size":             c.config.BatchSize,
		"auto_upgrade":           c.config.AutoUpgrade,
		"min_upgrade_points":     c.config.MinUpgradePoints,
		"max_upgrade_size_gb":    c.config.MaxUpgradeSizeGB,
	}
	
	return stats, nil
}
