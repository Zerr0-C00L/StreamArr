package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

type SeriesStore struct {
	db *sql.DB
}

func NewSeriesStore(db *sql.DB) *SeriesStore {
	return &SeriesStore{db: db}
}

// GetDB returns the underlying database connection
func (s *SeriesStore) GetDB() *sql.DB {
	return s.db
}

// Add adds a new series to the library
func (s *SeriesStore) Add(ctx context.Context, series *models.Series) error {
	// Build metadata JSON with all series details
	metadata := map[string]interface{}{
		"original_title":  series.OriginalTitle,
		"overview":        series.Overview,
		"poster_path":     series.PosterPath,
		"backdrop_path":   series.BackdropPath,
		"first_air_date":  series.FirstAirDate,
		"status":          series.Status,
		"seasons":         series.Seasons,
		"total_episodes":  series.TotalEpisodes,
		"genres":          series.Genres,
		"quality_profile": series.QualityProfile,
	}
	// Merge with existing metadata
	for k, v := range series.Metadata {
		metadata[k] = v
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Extract year from first air date
	var year *int
	if series.FirstAirDate != nil {
		y := series.FirstAirDate.Year()
		year = &y
	}

	// Generate clean title for searching
	cleanTitle := series.Title

	query := `
		INSERT INTO library_series (
			tmdb_id, imdb_id, title, year, monitored, clean_title, metadata, added_at, preferred_quality
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, added_at
	`

	// Use NULL for empty IMDB ID
	var imdbID *string
	if series.IMDBID != "" {
		imdbID = &series.IMDBID
	}

	err = s.db.QueryRowContext(
		ctx, query,
		series.TMDBID, imdbID, series.Title, year, series.Monitored,
		cleanTitle, metadataJSON, time.Now(), series.QualityProfile,
	).Scan(&series.ID, &series.AddedAt)

	if err != nil {
		return fmt.Errorf("failed to add series: %w", err)
	}

	return nil
}

// Get retrieves a series by ID
func (s *SeriesStore) Get(ctx context.Context, id int64) (*models.Series, error) {
	query := `
		SELECT id, tmdb_id, imdb_id, title, year, monitored, clean_title,
			metadata, added_at, last_checked, preferred_quality
		FROM library_series
		WHERE id = $1
	`

	var series models.Series
	var metadataJSON []byte
	var imdbID sql.NullString
	var year sql.NullInt32
	var lastChecked sql.NullTime
	var preferredQuality sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&series.ID, &series.TMDBID, &imdbID, &series.Title, &year,
		&series.Monitored, &series.CleanTitle, &metadataJSON,
		&series.AddedAt, &lastChecked, &preferredQuality,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("series not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &series.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Set IMDB ID field from database column
	if imdbID.Valid && imdbID.String != "" {
		series.IMDBID = imdbID.String
		series.Metadata["imdb_id"] = imdbID.String
	}

	// Extract additional fields from metadata if available
	if title, ok := series.Metadata["title"].(string); ok {
		series.Title = title
	}
	if overview, ok := series.Metadata["overview"].(string); ok {
		series.Overview = overview
	}
	if posterPath, ok := series.Metadata["poster_path"].(string); ok {
		series.PosterPath = posterPath
	}
	if backdropPath, ok := series.Metadata["backdrop_path"].(string); ok {
		series.BackdropPath = backdropPath
	}
	if seasons, ok := series.Metadata["seasons"].(float64); ok {
		series.Seasons = int(seasons)
	}

	if lastChecked.Valid {
		series.LastChecked = &lastChecked.Time
	}
	if preferredQuality.Valid {
		series.QualityProfile = preferredQuality.String
	}

	return &series, nil
}

// GetByTMDBID retrieves a series by TMDB ID
func (s *SeriesStore) GetByTMDBID(ctx context.Context, tmdbID int) (*models.Series, error) {
	query := `
		SELECT id, tmdb_id, title, original_title, overview, poster_path, backdrop_path,
			first_air_date, status, seasons, total_episodes, genres, metadata,
			monitored, quality_profile, search_status, last_checked,
			created_at, updated_at, added_at
		FROM library_series
		WHERE tmdb_id = $1
	`

	var series models.Series
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, tmdbID).Scan(
		&series.ID, &series.TMDBID, &series.Title, &series.OriginalTitle,
		&series.Overview, &series.PosterPath, &series.BackdropPath,
		&series.FirstAirDate, &series.Status, &series.Seasons, &series.TotalEpisodes,
		&series.Genres, &metadataJSON, &series.Monitored, &series.QualityProfile,
		&series.SearchStatus, &series.LastChecked, &series.CreatedAt,
		&series.UpdatedAt, &series.AddedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("series not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &series.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &series, nil
}

// GetByIMDBID retrieves a series by IMDB ID
func (s *SeriesStore) GetByIMDBID(ctx context.Context, imdbID string) (*models.Series, error) {
	query := `
		SELECT id, tmdb_id, imdb_id, title, year, monitored, clean_title,
			metadata, added_at, last_checked, preferred_quality
		FROM library_series
		WHERE imdb_id = $1
	`

	var series models.Series
	var metadataJSON []byte
	var dbImdbID sql.NullString
	var year sql.NullInt32
	var lastChecked sql.NullTime
	var preferredQuality sql.NullString

	err := s.db.QueryRowContext(ctx, query, imdbID).Scan(
		&series.ID, &series.TMDBID, &dbImdbID, &series.Title, &year,
		&series.Monitored, &series.CleanTitle, &metadataJSON,
		&series.AddedAt, &lastChecked, &preferredQuality,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("series not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &series.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Set IMDB ID field from database column
	if dbImdbID.Valid && dbImdbID.String != "" {
		series.IMDBID = dbImdbID.String
	}

	// Parse metadata fields
	if series.Metadata != nil {
		if ot, ok := series.Metadata["original_title"].(string); ok {
			series.OriginalTitle = ot
		}
		if ov, ok := series.Metadata["overview"].(string); ok {
			series.Overview = ov
		}
		if pp, ok := series.Metadata["poster_path"].(string); ok {
			series.PosterPath = pp
		}
		if bp, ok := series.Metadata["backdrop_path"].(string); ok {
			series.BackdropPath = bp
		}
		if st, ok := series.Metadata["status"].(string); ok {
			series.Status = st
		}
		if pq, ok := series.Metadata["quality_profile"].(string); ok {
			series.QualityProfile = pq
		}
	}

	if dbImdbID.Valid {
		imdbStr := dbImdbID.String
		series.Metadata["imdb_id"] = imdbStr
	}
	if year.Valid {
		series.Metadata["year"] = int(year.Int32)
	}
	if lastChecked.Valid {
		series.LastChecked = &lastChecked.Time
	}
	if preferredQuality.Valid {
		series.QualityProfile = preferredQuality.String
	}

	return &series, nil
}

// List returns paginated series with optional filtering
func (s *SeriesStore) List(ctx context.Context, offset, limit int, monitored *bool) ([]*models.Series, error) {
	query := `
		SELECT id, tmdb_id, imdb_id, title, year, monitored, metadata, added_at, last_checked, preferred_quality
		FROM library_series
		WHERE ($1::boolean IS NULL OR monitored = $1)
		ORDER BY added_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.QueryContext(ctx, query, monitored, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list series: %w", err)
	}
	defer rows.Close()

	var seriesList []*models.Series
	for rows.Next() {
		var series models.Series
		var metadataJSON []byte
		var imdbID sql.NullString
		var year sql.NullInt32
		var lastChecked sql.NullTime

		err := rows.Scan(
			&series.ID, &series.TMDBID, &imdbID, &series.Title, &year,
			&series.Monitored, &metadataJSON, &series.AddedAt, &lastChecked, &series.QualityProfile,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan series: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &series.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		// Set IMDB ID field from database column
		if imdbID.Valid && imdbID.String != "" {
			series.IMDBID = imdbID.String
		}

		// Extract fields from metadata if present
		if posterPath, ok := series.Metadata["poster_path"].(string); ok {
			series.PosterPath = posterPath
		}
		if backdropPath, ok := series.Metadata["backdrop_path"].(string); ok {
			series.BackdropPath = backdropPath
		}
		if overview, ok := series.Metadata["overview"].(string); ok {
			series.Overview = overview
		}

		seriesList = append(seriesList, &series)
	}

	return seriesList, nil
}

// Search performs full-text search on series
func (s *SeriesStore) Search(ctx context.Context, query string, limit int) ([]*models.Series, error) {
	searchQuery := `
		SELECT id, tmdb_id, title, original_title, overview, poster_path, backdrop_path,
			first_air_date, status, seasons, total_episodes, genres, metadata,
			monitored, quality_profile, search_status, last_checked,
			created_at, updated_at, added_at
		FROM library_series
		WHERE search_vector @@ plainto_tsquery('english', $1)
		ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, searchQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search series: %w", err)
	}
	defer rows.Close()

	var seriesList []*models.Series
	for rows.Next() {
		var series models.Series
		var metadataJSON []byte

		err := rows.Scan(
			&series.ID, &series.TMDBID, &series.Title, &series.OriginalTitle,
			&series.Overview, &series.PosterPath, &series.BackdropPath,
			&series.FirstAirDate, &series.Status, &series.Seasons, &series.TotalEpisodes,
			&series.Genres, &metadataJSON, &series.Monitored, &series.QualityProfile,
			&series.SearchStatus, &series.LastChecked, &series.CreatedAt,
			&series.UpdatedAt, &series.AddedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan series: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &series.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		seriesList = append(seriesList, &series)
	}

	return seriesList, nil
}

// Update updates an existing series
func (s *SeriesStore) Update(ctx context.Context, series *models.Series) error {
	metadataJSON, err := json.Marshal(series.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE library_series
		SET title = $1, original_title = $2, overview = $3, poster_path = $4,
			backdrop_path = $5, first_air_date = $6, status = $7, seasons = $8,
			total_episodes = $9, genres = $10, metadata = $11, monitored = $12,
			quality_profile = $13, search_status = $14, last_checked = $15,
			updated_at = $16
		WHERE id = $17
	`

	result, err := s.db.ExecContext(
		ctx, query,
		series.Title, series.OriginalTitle, series.Overview, series.PosterPath,
		series.BackdropPath, series.FirstAirDate, series.Status, series.Seasons,
		series.TotalEpisodes, series.Genres, metadataJSON, series.Monitored,
		series.QualityProfile, series.SearchStatus, series.LastChecked,
		time.Now(), series.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update series: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("series not found")
	}

	return nil
}

// Delete removes a series from the library
func (s *SeriesStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM library_series WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete series: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("series not found")
	}

	return nil
}

// GetMonitored returns all monitored series
func (s *SeriesStore) GetMonitored(ctx context.Context) ([]*models.Series, error) {
	monitored := true
	return s.List(ctx, 0, 10000, &monitored)
}

// UpdateSearchStatus updates the search status for a series
func (s *SeriesStore) UpdateSearchStatus(ctx context.Context, id int64, status string) error {
	query := `
		UPDATE library_series
		SET search_status = $1, last_checked = $2, updated_at = $3
		WHERE id = $4
	`

	now := time.Now()
	result, err := s.db.ExecContext(ctx, query, status, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update search status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("series not found")
	}

	return nil
}

// Count returns the total number of series
func (s *SeriesStore) Count(ctx context.Context, monitored *bool) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM library_series
		WHERE ($1::boolean IS NULL OR monitored = $1)
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, monitored).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count series: %w", err)
	}

	return count, nil
}

// CountEpisodes returns the total number of episodes across all series
func (s *SeriesStore) CountEpisodes(ctx context.Context) (int, error) {
	var count sql.NullInt64
	err := s.db.QueryRowContext(ctx, "SELECT SUM(total_episodes) FROM library_series").Scan(&count)
	if err != nil {
		return 0, err
	}
	if !count.Valid {
		return 0, nil
	}
	return int(count.Int64), nil
}

// DeleteAll removes all series from the library
func (s *SeriesStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM library_series")
	return err
}

// ResetStatus resets the search status for all series
func (s *SeriesStore) ResetStatus(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE library_series 
		SET search_status = '',
		    last_checked = NULL
	`)
	return err
}

// DeleteBySource removes all series from a specific source
func (s *SeriesStore) DeleteBySource(ctx context.Context, source string) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM library_series WHERE metadata->>'source' = $1",
		source)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
