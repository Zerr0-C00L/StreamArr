package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

// StreamCacheStore manages media_streams table operations
type StreamCacheStore struct {
	db *sql.DB
}

// NewStreamCacheStore creates a new stream cache store
func NewStreamCacheStore(db *sql.DB) *StreamCacheStore {
	return &StreamCacheStore{db: db}
}

// GetDB returns the database connection for advanced queries
func (s *StreamCacheStore) GetDB() *sql.DB {
	return s.db
}

// GetCachedStream retrieves the cached stream for a movie
// Returns nil if no cached stream exists
func (s *StreamCacheStore) GetCachedStream(ctx context.Context, movieID int) (*models.CachedStream, error) {
	query := `
		SELECT id, movie_id, stream_url, stream_hash, quality_score,
		       resolution, hdr_type, audio_format, source_type, file_size_gb,
		       codec, indexer, cached_at, last_checked, check_count,
		       is_available, upgrade_available, next_check_at, created_at, updated_at
		FROM media_streams
		WHERE movie_id = $1
	`
	
	cached := &models.CachedStream{}
	err := s.db.QueryRowContext(ctx, query, movieID).Scan(
		&cached.ID,
		&cached.MovieID,
		&cached.StreamURL,
		&cached.StreamHash,
		&cached.QualityScore,
		&cached.Resolution,
		&cached.HDRType,
		&cached.AudioFormat,
		&cached.SourceType,
		&cached.FileSizeGB,
		&cached.Codec,
		&cached.Indexer,
		&cached.CachedAt,
		&cached.LastChecked,
		&cached.CheckCount,
		&cached.IsAvailable,
		&cached.UpgradeAvailable,
		&cached.NextCheckAt,
		&cached.CreatedAt,
		&cached.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil // No cached stream
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cached stream: %w", err)
	}
	
	return cached, nil
}

// CacheStream stores or updates the cached stream for a media item
// Replaces existing stream if one exists (one stream per media)
func (s *StreamCacheStore) CacheStream(ctx context.Context, movieID int, stream models.TorrentStream, streamURL string) error {
	query := `
		INSERT INTO media_streams (
			movie_id, stream_url, stream_hash, quality_score,
			resolution, hdr_type, audio_format, source_type, file_size_gb,
			codec, indexer, cached_at, last_checked, check_count,
			is_available, upgrade_available, next_check_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			NOW(), NOW(), 0, true, false, NOW() + INTERVAL '7 days', NOW(), NOW()
		)
		ON CONFLICT (movie_id) DO UPDATE SET
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
			cached_at = NOW(),
			last_checked = NOW(),
			check_count = 0,
			is_available = true,
			upgrade_available = false,
			next_check_at = NOW() + INTERVAL '7 days',
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query,
		movieID,
		streamURL,
		stream.Hash,
		stream.QualityScore,
		stream.Resolution,
		stream.HDRType,
		stream.AudioFormat,
		stream.Source,
		stream.SizeGB,
		stream.Codec,
		stream.Indexer,
	)
	
	if err != nil {
		return fmt.Errorf("failed to cache stream: %w", err)
	}
	
	return nil
}

// MarkUnavailable marks a stream as unavailable (debrid cache expired)
func (s *StreamCacheStore) MarkUnavailable(ctx context.Context, movieID int) error {
	query := `
		UPDATE media_streams
		SET is_available = false,
		    last_checked = NOW(),
		    check_count = check_count + 1,
		    next_check_at = NOW() + INTERVAL '1 day',
		    updated_at = NOW()
		WHERE movie_id = $1
	`
	
	result, err := s.db.ExecContext(ctx, query, movieID)
	if err != nil {
		return fmt.Errorf("failed to mark unavailable: %w", err)
	}
	
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no stream found for movie_id %d", movieID)
	}
	
	return nil
}

// MarkUpgradeAvailable marks that a better quality stream is available
func (s *StreamCacheStore) MarkUpgradeAvailable(ctx context.Context, movieID int, available bool) error {
	query := `
		UPDATE media_streams
		SET upgrade_available = $1,
		    updated_at = NOW()
		WHERE movie_id = $2
	`
	
	_, err := s.db.ExecContext(ctx, query, available, movieID)
	if err != nil {
		return fmt.Errorf("failed to mark upgrade available: %w", err)
	}
	
	return nil
}

// UpdateNextCheck schedules the next availability check
func (s *StreamCacheStore) UpdateNextCheck(ctx context.Context, movieID int, daysUntilCheck int) error {
	query := `
		UPDATE media_streams
		SET last_checked = NOW(),
		    check_count = check_count + 1,
		    next_check_at = NOW() + INTERVAL '1 day' * $1,
		    updated_at = NOW()
		WHERE movie_id = $2
	`
	
	_, err := s.db.ExecContext(ctx, query, daysUntilCheck, movieID)
	if err != nil {
		return fmt.Errorf("failed to update next check: %w", err)
	}
	
	return nil
}

// GetStreamsDueForCheck retrieves streams that need availability checking
func (s *StreamCacheStore) GetStreamsDueForCheck(ctx context.Context, limit int) ([]*models.CachedStream, error) {
	query := `
		SELECT id, movie_id, stream_url, stream_hash, quality_score,
		       resolution, hdr_type, audio_format, source_type, file_size_gb,
		       codec, indexer, cached_at, last_checked, check_count,
		       is_available, upgrade_available, next_check_at, created_at, updated_at
		FROM media_streams
		WHERE next_check_at <= NOW()
		  AND is_available = true
		ORDER BY last_checked ASC
		LIMIT $1
	`
	
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get streams for check: %w", err)
	}
	defer rows.Close()
	
	var streams []*models.CachedStream
	for rows.Next() {
		cached := &models.CachedStream{}
		err := rows.Scan(
			&cached.ID,
			&cached.MovieID,
			&cached.StreamURL,
			&cached.StreamHash,
			&cached.QualityScore,
			&cached.Resolution,
			&cached.HDRType,
			&cached.AudioFormat,
			&cached.SourceType,
			&cached.FileSizeGB,
			&cached.Codec,
			&cached.Indexer,
			&cached.CachedAt,
			&cached.LastChecked,
			&cached.CheckCount,
			&cached.IsAvailable,
			&cached.UpgradeAvailable,
			&cached.NextCheckAt,
			&cached.CreatedAt,
			&cached.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream: %w", err)
		}
		streams = append(streams, cached)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating streams: %w", err)
	}
	
	return streams, nil
}

// GetStreamsByQualityScore retrieves streams with quality score below threshold
// Useful for finding upgrade candidates
func (s *StreamCacheStore) GetStreamsByQualityScore(ctx context.Context, maxScore int, limit int) ([]*models.CachedStream, error) {
	query := `
		SELECT id, movie_id, stream_url, stream_hash, quality_score,
		       resolution, hdr_type, audio_format, source_type, file_size_gb,
		       codec, indexer, cached_at, last_checked, check_count,
		       is_available, upgrade_available, next_check_at, created_at, updated_at
		FROM media_streams
		WHERE quality_score <= $1
		  AND is_available = true
		ORDER BY quality_score ASC
		LIMIT $2
	`
	
	rows, err := s.db.QueryContext(ctx, query, maxScore, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get streams by score: %w", err)
	}
	defer rows.Close()
	
	var streams []*models.CachedStream
	for rows.Next() {
		cached := &models.CachedStream{}
		err := rows.Scan(
			&cached.ID,
			&cached.MovieID,
			&cached.StreamURL,
			&cached.StreamHash,
			&cached.QualityScore,
			&cached.Resolution,
			&cached.HDRType,
			&cached.AudioFormat,
			&cached.SourceType,
			&cached.FileSizeGB,
			&cached.Codec,
			&cached.Indexer,
			&cached.CachedAt,
			&cached.LastChecked,
			&cached.CheckCount,
			&cached.IsAvailable,
			&cached.UpgradeAvailable,
			&cached.NextCheckAt,
			&cached.CreatedAt,
			&cached.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream: %w", err)
		}
		streams = append(streams, cached)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating streams: %w", err)
	}
	
	return streams, nil
}

// GetUnavailableStreams retrieves streams marked as unavailable
func (s *StreamCacheStore) GetUnavailableStreams(ctx context.Context, limit int) ([]*models.CachedStream, error) {
	query := `
		SELECT id, movie_id, stream_url, stream_hash, quality_score,
		       resolution, hdr_type, audio_format, source_type, file_size_gb,
		       codec, indexer, cached_at, last_checked, check_count,
		       is_available, upgrade_available, next_check_at, created_at, updated_at
		FROM media_streams
		WHERE is_available = false
		ORDER BY last_checked ASC
		LIMIT $1
	`
	
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unavailable streams: %w", err)
	}
	defer rows.Close()
	
	var streams []*models.CachedStream
	for rows.Next() {
		cached := &models.CachedStream{}
		err := rows.Scan(
			&cached.ID,
			&cached.MovieID,
			&cached.StreamURL,
			&cached.StreamHash,
			&cached.QualityScore,
			&cached.Resolution,
			&cached.HDRType,
			&cached.AudioFormat,
			&cached.SourceType,
			&cached.FileSizeGB,
			&cached.Codec,
			&cached.Indexer,
			&cached.CachedAt,
			&cached.LastChecked,
			&cached.CheckCount,
			&cached.IsAvailable,
			&cached.UpgradeAvailable,
			&cached.NextCheckAt,
			&cached.CreatedAt,
			&cached.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream: %w", err)
		}
		streams = append(streams, cached)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating streams: %w", err)
	}
	
	return streams, nil
}

// GetCacheStats returns statistics about cached streams
func (s *StreamCacheStore) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE is_available = true) as available,
			COUNT(*) FILTER (WHERE is_available = false) as unavailable,
			COUNT(*) FILTER (WHERE upgrade_available = true) as upgrades_available,
			AVG(quality_score) as avg_score,
			COUNT(*) FILTER (WHERE resolution = '2160p' OR resolution = '4K') as count_4k,
			COUNT(*) FILTER (WHERE resolution = '1080p') as count_1080p,
			COUNT(*) FILTER (WHERE resolution = '720p') as count_720p,
			COUNT(*) FILTER (WHERE hdr_type = 'DV') as count_dolby_vision,
			COUNT(*) FILTER (WHERE source_type = 'REMUX') as count_remux
		FROM media_streams
	`
	
	var stats struct {
		Total              int
		Available          int
		Unavailable        int
		UpgradesAvailable  int
		AvgScore           sql.NullFloat64
		Count4K            int
		Count1080p         int
		Count720p          int
		CountDolbyVision   int
		CountRemux         int
	}
	
	err := s.db.QueryRowContext(ctx, query).Scan(
		&stats.Total,
		&stats.Available,
		&stats.Unavailable,
		&stats.UpgradesAvailable,
		&stats.AvgScore,
		&stats.Count4K,
		&stats.Count1080p,
		&stats.Count720p,
		&stats.CountDolbyVision,
		&stats.CountRemux,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get cache stats: %w", err)
	}
	
	avgScore := 0.0
	if stats.AvgScore.Valid {
		avgScore = stats.AvgScore.Float64
	}
	
	return map[string]interface{}{
		"total":               stats.Total,
		"available":           stats.Available,
		"unavailable":         stats.Unavailable,
		"upgrades_available":  stats.UpgradesAvailable,
		"avg_quality_score":   avgScore,
		"4k_streams":          stats.Count4K,
		"1080p_streams":       stats.Count1080p,
		"720p_streams":        stats.Count720p,
		"dolby_vision_streams": stats.CountDolbyVision,
		"remux_streams":       stats.CountRemux,
	}, nil
}

// DeleteCachedStream removes a cached stream
func (s *StreamCacheStore) DeleteCachedStream(ctx context.Context, movieID int) error {
	query := `DELETE FROM media_streams WHERE movie_id = $1`
	
	_, err := s.db.ExecContext(ctx, query, movieID)
	if err != nil {
		return fmt.Errorf("failed to delete cached stream: %w", err)
	}
	
	return nil
}

// GetStreamsWithUpgradesAvailable retrieves streams that have upgrades available
func (s *StreamCacheStore) GetStreamsWithUpgradesAvailable(ctx context.Context, limit int) ([]*models.CachedStream, error) {
	query := `
		SELECT id, movie_id, stream_url, stream_hash, quality_score,
		       resolution, hdr_type, audio_format, source_type, file_size_gb,
		       codec, indexer, cached_at, last_checked, check_count,
		       is_available, upgrade_available, next_check_at, created_at, updated_at
		FROM media_streams
		WHERE upgrade_available = true
		  AND is_available = true
		ORDER BY quality_score ASC
		LIMIT $1
	`
	
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get streams with upgrades: %w", err)
	}
	defer rows.Close()
	
	var streams []*models.CachedStream
	for rows.Next() {
		cached := &models.CachedStream{}
		err := rows.Scan(
			&cached.ID,
			&cached.MovieID,
			&cached.StreamURL,
			&cached.StreamHash,
			&cached.QualityScore,
			&cached.Resolution,
			&cached.HDRType,
			&cached.AudioFormat,
			&cached.SourceType,
			&cached.FileSizeGB,
			&cached.Codec,
			&cached.Indexer,
			&cached.CachedAt,
			&cached.LastChecked,
			&cached.CheckCount,
			&cached.IsAvailable,
			&cached.UpgradeAvailable,
			&cached.NextCheckAt,
			&cached.CreatedAt,
			&cached.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream: %w", err)
		}
		streams = append(streams, cached)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating streams: %w", err)
	}
	
	return streams, nil
}
