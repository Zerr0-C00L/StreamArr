package streams

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// HealthReport contains library health metrics
type HealthReport struct {
	GeneratedAt          time.Time
	TotalMedia           int
	MediaWithStreams     int
	MediaWithoutStreams  int
	AvailableStreams     int
	UnavailableStreams   int
	UpgradesAvailable    int
	AverageQualityScore  float64
	QualityDistribution  map[string]int
	SourceDistribution   map[string]int
	HDRDistribution      map[string]int
	StaleStreams         int // Not checked in 14+ days
	LowQualityStreams    int // Score < 50
	Issues               []HealthIssue
}

// HealthIssue represents a specific library health problem
type HealthIssue struct {
	Type        string
	Severity    string // "critical", "warning", "info"
	Count       int
	Description string
	MediaIDs    []int
}

// HealthMonitor provides library health analysis using pure SQL
type HealthMonitor struct {
	db *sql.DB
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(db *sql.DB) *HealthMonitor {
	return &HealthMonitor{db: db}
}

// GenerateHealthReport creates a comprehensive health report
func (h *HealthMonitor) GenerateHealthReport(ctx context.Context) (*HealthReport, error) {
	report := &HealthReport{
		GeneratedAt:         time.Now(),
		QualityDistribution: make(map[string]int),
		SourceDistribution:  make(map[string]int),
		HDRDistribution:     make(map[string]int),
		Issues:              []HealthIssue{},
	}
	
	// Get basic counts
	if err := h.getBasicCounts(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to get basic counts: %w", err)
	}
	
	// Get quality distribution
	if err := h.getQualityDistribution(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to get quality distribution: %w", err)
	}
	
	// Get source distribution
	if err := h.getSourceDistribution(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to get source distribution: %w", err)
	}
	
	// Get HDR distribution
	if err := h.getHDRDistribution(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to get HDR distribution: %w", err)
	}
	
	// Identify issues
	if err := h.identifyIssues(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to identify issues: %w", err)
	}
	
	return report, nil
}

// getBasicCounts retrieves basic stream statistics
func (h *HealthMonitor) getBasicCounts(ctx context.Context, report *HealthReport) error {
	query := `
		SELECT 
			COUNT(DISTINCT m.id) as total_media,
			COUNT(DISTINCT ms.media_id) as media_with_streams,
			COUNT(*) FILTER (WHERE ms.is_available = true) as available_streams,
			COUNT(*) FILTER (WHERE ms.is_available = false) as unavailable_streams,
			COUNT(*) FILTER (WHERE ms.upgrade_available = true) as upgrades_available,
			AVG(ms.quality_score) as avg_score,
			COUNT(*) FILTER (WHERE ms.last_checked < NOW() - INTERVAL '14 days') as stale_streams,
			COUNT(*) FILTER (WHERE ms.quality_score < 50) as low_quality_streams
		FROM media m
		LEFT JOIN media_streams ms ON m.id = ms.media_id
	`
	
	var avgScore sql.NullFloat64
	err := h.db.QueryRowContext(ctx, query).Scan(
		&report.TotalMedia,
		&report.MediaWithStreams,
		&report.AvailableStreams,
		&report.UnavailableStreams,
		&report.UpgradesAvailable,
		&avgScore,
		&report.StaleStreams,
		&report.LowQualityStreams,
	)
	
	if err != nil {
		return err
	}
	
	if avgScore.Valid {
		report.AverageQualityScore = avgScore.Float64
	}
	
	report.MediaWithoutStreams = report.TotalMedia - report.MediaWithStreams
	
	return nil
}

// getQualityDistribution retrieves resolution distribution
func (h *HealthMonitor) getQualityDistribution(ctx context.Context, report *HealthReport) error {
	query := `
		SELECT 
			COALESCE(resolution, 'Unknown') as resolution,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY resolution
		ORDER BY count DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var resolution string
		var count int
		if err := rows.Scan(&resolution, &count); err != nil {
			return err
		}
		report.QualityDistribution[resolution] = count
	}
	
	return rows.Err()
}

// getSourceDistribution retrieves source type distribution
func (h *HealthMonitor) getSourceDistribution(ctx context.Context, report *HealthReport) error {
	query := `
		SELECT 
			COALESCE(source_type, 'Unknown') as source,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY source_type
		ORDER BY count DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return err
		}
		report.SourceDistribution[source] = count
	}
	
	return rows.Err()
}

// getHDRDistribution retrieves HDR type distribution
func (h *HealthMonitor) getHDRDistribution(ctx context.Context, report *HealthReport) error {
	query := `
		SELECT 
			COALESCE(hdr_type, 'SDR') as hdr,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY hdr_type
		ORDER BY count DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var hdr string
		var count int
		if err := rows.Scan(&hdr, &count); err != nil {
			return err
		}
		report.HDRDistribution[hdr] = count
	}
	
	return rows.Err()
}

// identifyIssues finds library health problems
func (h *HealthMonitor) identifyIssues(ctx context.Context, report *HealthReport) error {
	// Issue 1: Media without cached streams
	if report.MediaWithoutStreams > 0 {
		mediaIDs, err := h.getMediaWithoutStreams(ctx, 100)
		if err != nil {
			return err
		}
		
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "missing_streams",
			Severity:    "warning",
			Count:       report.MediaWithoutStreams,
			Description: fmt.Sprintf("%d media items have no cached streams", report.MediaWithoutStreams),
			MediaIDs:    mediaIDs,
		})
	}
	
	// Issue 2: Unavailable streams
	if report.UnavailableStreams > 0 {
		mediaIDs, err := h.getUnavailableStreamMediaIDs(ctx, 100)
		if err != nil {
			return err
		}
		
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "unavailable_streams",
			Severity:    "critical",
			Count:       report.UnavailableStreams,
			Description: fmt.Sprintf("%d streams are unavailable (debrid cache expired)", report.UnavailableStreams),
			MediaIDs:    mediaIDs,
		})
	}
	
	// Issue 3: Low quality streams
	if report.LowQualityStreams > 0 {
		mediaIDs, err := h.getLowQualityStreamMediaIDs(ctx, 100)
		if err != nil {
			return err
		}
		
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "low_quality",
			Severity:    "info",
			Count:       report.LowQualityStreams,
			Description: fmt.Sprintf("%d streams have quality score below 50", report.LowQualityStreams),
			MediaIDs:    mediaIDs,
		})
	}
	
	// Issue 4: Stale streams (not checked in 14+ days)
	if report.StaleStreams > 0 {
		mediaIDs, err := h.getStaleStreamMediaIDs(ctx, 100)
		if err != nil {
			return err
		}
		
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "stale_streams",
			Severity:    "warning",
			Count:       report.StaleStreams,
			Description: fmt.Sprintf("%d streams haven't been checked in 14+ days", report.StaleStreams),
			MediaIDs:    mediaIDs,
		})
	}
	
	// Issue 5: Upgrades available
	if report.UpgradesAvailable > 0 {
		mediaIDs, err := h.getUpgradeAvailableMediaIDs(ctx, 100)
		if err != nil {
			return err
		}
		
		report.Issues = append(report.Issues, HealthIssue{
			Type:        "upgrades_available",
			Severity:    "info",
			Count:       report.UpgradesAvailable,
			Description: fmt.Sprintf("%d streams have better quality versions available", report.UpgradesAvailable),
			MediaIDs:    mediaIDs,
		})
	}
	
	return nil
}

// getMediaWithoutStreams returns media IDs without cached streams
func (h *HealthMonitor) getMediaWithoutStreams(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT m.id
		FROM media m
		LEFT JOIN media_streams ms ON m.id = ms.media_id
		WHERE ms.id IS NULL
		LIMIT $1
	`
	
	rows, err := h.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// getUnavailableStreamMediaIDs returns media IDs with unavailable streams
func (h *HealthMonitor) getUnavailableStreamMediaIDs(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE is_available = false
		ORDER BY last_checked ASC
		LIMIT $1
	`
	
	return h.queryMediaIDs(ctx, query, limit)
}

// getLowQualityStreamMediaIDs returns media IDs with low quality scores
func (h *HealthMonitor) getLowQualityStreamMediaIDs(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE quality_score < 50
		  AND is_available = true
		ORDER BY quality_score ASC
		LIMIT $1
	`
	
	return h.queryMediaIDs(ctx, query, limit)
}

// getStaleStreamMediaIDs returns media IDs with stale streams
func (h *HealthMonitor) getStaleStreamMediaIDs(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE last_checked < NOW() - INTERVAL '14 days'
		ORDER BY last_checked ASC
		LIMIT $1
	`
	
	return h.queryMediaIDs(ctx, query, limit)
}

// getUpgradeAvailableMediaIDs returns media IDs with upgrades available
func (h *HealthMonitor) getUpgradeAvailableMediaIDs(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE upgrade_available = true
		  AND is_available = true
		ORDER BY quality_score ASC
		LIMIT $1
	`
	
	return h.queryMediaIDs(ctx, query, limit)
}

// queryMediaIDs executes a query and returns media IDs
func (h *HealthMonitor) queryMediaIDs(ctx context.Context, query string, limit int) ([]int, error) {
	rows, err := h.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// GetMediaMissingMetadata finds streams with incomplete metadata
func (h *HealthMonitor) GetMediaMissingMetadata(ctx context.Context, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE (resolution IS NULL OR resolution = '')
		   OR (hdr_type IS NULL OR hdr_type = '')
		   OR (audio_format IS NULL OR audio_format = '')
		   OR (source_type IS NULL OR source_type = '')
		   OR codec IS NULL
		LIMIT $1
	`
	
	return h.queryMediaIDs(ctx, query, limit)
}

// GetStreamsByResolution returns counts grouped by resolution
func (h *HealthMonitor) GetStreamsByResolution(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(resolution, 'Unknown') as res,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY resolution
		ORDER BY 
			CASE resolution
				WHEN '2160p' THEN 1
				WHEN '4K' THEN 1
				WHEN '1080p' THEN 2
				WHEN '720p' THEN 3
				WHEN '576p' THEN 4
				WHEN '480p' THEN 5
				ELSE 6
			END
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	counts := make(map[string]int)
	for rows.Next() {
		var res string
		var count int
		if err := rows.Scan(&res, &count); err != nil {
			return nil, err
		}
		counts[res] = count
	}
	
	return counts, rows.Err()
}

// GetQualityScoreDistribution returns quality score histogram
func (h *HealthMonitor) GetQualityScoreDistribution(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT 
			CASE 
				WHEN quality_score >= 90 THEN '90-100'
				WHEN quality_score >= 80 THEN '80-89'
				WHEN quality_score >= 70 THEN '70-79'
				WHEN quality_score >= 60 THEN '60-69'
				WHEN quality_score >= 50 THEN '50-59'
				WHEN quality_score >= 40 THEN '40-49'
				WHEN quality_score >= 30 THEN '30-39'
				ELSE '0-29'
			END as score_range,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY score_range
		ORDER BY score_range DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	dist := make(map[string]int)
	for rows.Next() {
		var scoreRange string
		var count int
		if err := rows.Scan(&scoreRange, &count); err != nil {
			return nil, err
		}
		dist[scoreRange] = count
	}
	
	return dist, rows.Err()
}

// GetIndexerPerformance returns stream counts by indexer
func (h *HealthMonitor) GetIndexerPerformance(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT 
			COALESCE(indexer, 'Unknown') as idx,
			COUNT(*) as count
		FROM media_streams
		WHERE is_available = true
		GROUP BY indexer
		ORDER BY count DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	perf := make(map[string]int)
	for rows.Next() {
		var indexer string
		var count int
		if err := rows.Scan(&indexer, &count); err != nil {
			return nil, err
		}
		perf[indexer] = count
	}
	
	return perf, rows.Err()
}

// GetRecentlyCachedStreams returns recently cached streams
func (h *HealthMonitor) GetRecentlyCachedStreams(ctx context.Context, hours int, limit int) ([]int, error) {
	query := `
		SELECT media_id
		FROM media_streams
		WHERE cached_at > NOW() - INTERVAL '1 hour' * $1
		ORDER BY cached_at DESC
		LIMIT $2
	`
	
	rows, err := h.db.QueryContext(ctx, query, hours, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	
	return ids, rows.Err()
}

// GetAverageScoreBySource returns average quality score per source type
func (h *HealthMonitor) GetAverageScoreBySource(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT 
			COALESCE(source_type, 'Unknown') as source,
			AVG(quality_score) as avg_score
		FROM media_streams
		WHERE is_available = true
		GROUP BY source_type
		ORDER BY avg_score DESC
	`
	
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	scores := make(map[string]float64)
	for rows.Next() {
		var source string
		var avgScore float64
		if err := rows.Scan(&source, &avgScore); err != nil {
			return nil, err
		}
		scores[source] = avgScore
	}
	
	return scores, rows.Err()
}
