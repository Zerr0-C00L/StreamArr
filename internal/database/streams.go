package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

type StreamStore struct {
	db *sql.DB
}

func NewStreamStore(db *sql.DB) *StreamStore {
	return &StreamStore{db: db}
}

// Add adds a new stream to the available streams
func (s *StreamStore) Add(ctx context.Context, stream *models.Stream) error {
	metadataJSON, err := json.Marshal(stream.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO available_streams (
			content_type, content_id, info_hash, title, size_bytes,
			quality, codec, source, seeders, tracker, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err = s.db.QueryRowContext(
		ctx, query,
		stream.ContentType, stream.ContentID, stream.InfoHash, stream.Title,
		stream.SizeBytes, stream.Quality, stream.Codec, stream.Source,
		stream.Seeders, stream.Tracker, metadataJSON,
	).Scan(&stream.ID, &stream.CreatedAt, &stream.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to add stream: %w", err)
	}

	return nil
}

// AddBatch adds multiple streams in a single transaction
func (s *StreamStore) AddBatch(ctx context.Context, streams []*models.Stream) error {
	if len(streams) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO available_streams (
			content_type, content_id, info_hash, title, size_bytes,
			quality, codec, source, seeders, tracker, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (content_type, content_id, info_hash)
		DO UPDATE SET
			title = EXCLUDED.title,
			size_bytes = EXCLUDED.size_bytes,
			quality = EXCLUDED.quality,
			codec = EXCLUDED.codec,
			seeders = EXCLUDED.seeders,
			tracker = EXCLUDED.tracker,
			metadata = EXCLUDED.metadata,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, stream := range streams {
		metadataJSON, err := json.Marshal(stream.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		err = stmt.QueryRowContext(
			ctx,
			stream.ContentType, stream.ContentID, stream.InfoHash, stream.Title,
			stream.SizeBytes, stream.Quality, stream.Codec, stream.Source,
			stream.Seeders, stream.Tracker, metadataJSON,
		).Scan(&stream.ID, &stream.CreatedAt, &stream.UpdatedAt)

		if err != nil {
			return fmt.Errorf("failed to add stream: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Get retrieves a stream by ID
func (s *StreamStore) Get(ctx context.Context, id int64) (*models.Stream, error) {
	query := `
		SELECT id, content_type, content_id, info_hash, title, size_bytes,
			quality, codec, source, seeders, tracker, metadata,
			created_at, updated_at
		FROM available_streams
		WHERE id = $1
	`

	var stream models.Stream
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&stream.ID, &stream.ContentType, &stream.ContentID, &stream.InfoHash,
		&stream.Title, &stream.SizeBytes, &stream.Quality, &stream.Codec,
		&stream.Source, &stream.Seeders, &stream.Tracker, &metadataJSON,
		&stream.CreatedAt, &stream.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("stream not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stream: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &stream.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &stream, nil
}

// ListByContent returns all streams for a specific content item
func (s *StreamStore) ListByContent(ctx context.Context, contentType string, contentID int64) ([]*models.Stream, error) {
	query := `
		SELECT id, content_type, content_id, info_hash, title, size_bytes,
			quality, codec, source, seeders, tracker, metadata,
			created_at, updated_at
		FROM available_streams
		WHERE content_type = $1 AND content_id = $2
		ORDER BY 
			CASE quality
				WHEN '2160p' THEN 1
				WHEN '1080p' THEN 2
				WHEN '720p' THEN 3
				WHEN '480p' THEN 4
				ELSE 5
			END,
			seeders DESC
	`

	rows, err := s.db.QueryContext(ctx, query, contentType, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list streams: %w", err)
	}
	defer rows.Close()

	var streams []*models.Stream
	for rows.Next() {
		var stream models.Stream
		var metadataJSON []byte

		err := rows.Scan(
			&stream.ID, &stream.ContentType, &stream.ContentID, &stream.InfoHash,
			&stream.Title, &stream.SizeBytes, &stream.Quality, &stream.Codec,
			&stream.Source, &stream.Seeders, &stream.Tracker, &metadataJSON,
			&stream.CreatedAt, &stream.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stream: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &stream.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		streams = append(streams, &stream)
	}

	return streams, nil
}

// FindBestStream returns the best available stream based on quality profile
func (s *StreamStore) FindBestStream(ctx context.Context, contentType string, contentID int64, preferredQuality string) (*models.Stream, error) {
	// Build quality priority order
	qualityOrder := buildQualityOrder(preferredQuality)

	query := `
		SELECT id, content_type, content_id, info_hash, title, size_bytes,
			quality, codec, source, seeders, tracker, metadata,
			created_at, updated_at
		FROM available_streams
		WHERE content_type = $1 AND content_id = $2
		ORDER BY 
			CASE quality
				` + qualityOrder + `
			END,
			seeders DESC
		LIMIT 1
	`

	var stream models.Stream
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, contentType, contentID).Scan(
		&stream.ID, &stream.ContentType, &stream.ContentID, &stream.InfoHash,
		&stream.Title, &stream.SizeBytes, &stream.Quality, &stream.Codec,
		&stream.Source, &stream.Seeders, &stream.Tracker, &metadataJSON,
		&stream.CreatedAt, &stream.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no streams available")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find best stream: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &stream.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &stream, nil
}

// Update updates an existing stream
func (s *StreamStore) Update(ctx context.Context, stream *models.Stream) error {
	metadataJSON, err := json.Marshal(stream.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE available_streams
		SET title = $1, size_bytes = $2, quality = $3, codec = $4,
			seeders = $5, tracker = $6, metadata = $7, updated_at = $8
		WHERE id = $9
	`

	result, err := s.db.ExecContext(
		ctx, query,
		stream.Title, stream.SizeBytes, stream.Quality, stream.Codec,
		stream.Seeders, stream.Tracker, metadataJSON, time.Now(), stream.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update stream: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// Delete removes a stream
func (s *StreamStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM available_streams WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete stream: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// DeleteByContent removes all streams for a specific content item
func (s *StreamStore) DeleteByContent(ctx context.Context, contentType string, contentID int64) error {
	query := `DELETE FROM available_streams WHERE content_type = $1 AND content_id = $2`

	_, err := s.db.ExecContext(ctx, query, contentType, contentID)
	if err != nil {
		return fmt.Errorf("failed to delete streams: %w", err)
	}

	return nil
}

// CleanupOld removes streams older than the specified duration
func (s *StreamStore) CleanupOld(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM available_streams
		WHERE updated_at < $1
	`

	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old streams: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

// Count returns the total number of streams
func (s *StreamStore) Count(ctx context.Context, contentType *string, contentID *int64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM available_streams
		WHERE ($1::text IS NULL OR content_type = $1)
		  AND ($2::bigint IS NULL OR content_id = $2)
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, contentType, contentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count streams: %w", err)
	}

	return count, nil
}

// buildQualityOrder builds the CASE statement for quality ordering
func buildQualityOrder(preferredQuality string) string {
	switch preferredQuality {
	case "2160p":
		return `
			WHEN '2160p' THEN 1
			WHEN '1080p' THEN 2
			WHEN '720p' THEN 3
			WHEN '480p' THEN 4
			ELSE 5
		`
	case "1080p":
		return `
			WHEN '1080p' THEN 1
			WHEN '2160p' THEN 2
			WHEN '720p' THEN 3
			WHEN '480p' THEN 4
			ELSE 5
		`
	case "720p":
		return `
			WHEN '720p' THEN 1
			WHEN '1080p' THEN 2
			WHEN '2160p' THEN 3
			WHEN '480p' THEN 4
			ELSE 5
		`
	default: // Default to 1080p preference
		return `
			WHEN '1080p' THEN 1
			WHEN '720p' THEN 2
			WHEN '2160p' THEN 3
			WHEN '480p' THEN 4
			ELSE 5
		`
	}
}

// DeleteAll removes all streams from the database
func (s *StreamStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM available_streams")
	return err
}

// DeleteStale removes streams older than the specified number of days
func (s *StreamStore) DeleteStale(ctx context.Context, days int) error {
	_, err := s.db.ExecContext(ctx, 
		"DELETE FROM available_streams WHERE updated_at < NOW() - INTERVAL '1 day' * $1",
		days)
	return err
}

// CountAll returns the total count of all streams
func (s *StreamStore) CountAll(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM available_streams").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// List returns all streams (count only for stats)
func (s *StreamStore) List(ctx context.Context) ([]*models.Stream, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM available_streams").Scan(&count)
	if err != nil {
		return nil, err
	}
	result := make([]*models.Stream, count)
	return result, nil
}
