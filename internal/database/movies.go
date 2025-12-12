package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

type MovieStore struct {
	db *sql.DB
}

func NewMovieStore(db *sql.DB) *MovieStore {
	return &MovieStore{db: db}
}

// Add adds a new movie to the library
func (s *MovieStore) Add(ctx context.Context, movie *models.Movie) error {
	// Build metadata JSON with all movie details
	metadata := map[string]interface{}{
		"title":           movie.Title,
		"original_title":  movie.OriginalTitle,
		"overview":        movie.Overview,
		"poster_path":     movie.PosterPath,
		"backdrop_path":   movie.BackdropPath,
		"release_date":    movie.ReleaseDate,
		"runtime":         movie.Runtime,
		"genres":          movie.Genres,
		"quality_profile": movie.QualityProfile,
	}
	// Merge with existing metadata
	for k, v := range movie.Metadata {
		metadata[k] = v
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Extract year from release date
	var year *int
	if movie.ReleaseDate != nil {
		y := movie.ReleaseDate.Year()
		year = &y
	}

	// Generate clean title for searching
	cleanTitle := movie.Title // TODO: implement proper cleaning

	query := `
		INSERT INTO library_movies (
			tmdb_id, title, year, monitored, clean_title, metadata, added_at, preferred_quality, collection_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, added_at
	`

	err = s.db.QueryRowContext(
		ctx, query,
		movie.TMDBID, movie.Title, year, movie.Monitored,
		cleanTitle, metadataJSON, time.Now(), movie.QualityProfile, movie.CollectionID,
	).Scan(&movie.ID, &movie.AddedAt)

	if err != nil {
		// Check if it's a duplicate entry
		if err.Error() != "" && (sql.ErrNoRows == err || err.Error() == "UNIQUE constraint failed: library_movies.tmdb_id" || err.Error() == "pq: duplicate key value violates unique constraint \"library_movies_tmdb_id_key\"") {
			return fmt.Errorf("movie already exists in library")
		}
		return fmt.Errorf("failed to add movie: %w", err)
	}

	return nil
}

// Helper function to extract movie details from metadata and database columns
func scanMovie(rows *sql.Rows) (*models.Movie, error) {
	movie := &models.Movie{}
	var metadataJSON []byte
	var year *int

	err := rows.Scan(
		&movie.ID, &movie.TMDBID, &movie.Title, &year,
		&movie.Monitored, &movie.Available, &movie.QualityProfile,
		&metadataJSON, &movie.AddedAt, &movie.LastChecked,
	)
	if err != nil {
		return nil, err
	}

	// Parse metadata JSON
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Extract fields from metadata
	if val, ok := metadata["original_title"].(string); ok {
		movie.OriginalTitle = val
	}
	if val, ok := metadata["overview"].(string); ok {
		movie.Overview = val
	}
	if val, ok := metadata["poster_path"].(string); ok {
		movie.PosterPath = val
	}
	if val, ok := metadata["backdrop_path"].(string); ok {
		movie.BackdropPath = val
	}
	if val, ok := metadata["runtime"].(float64); ok {
		movie.Runtime = int(val)
	}
	if val, ok := metadata["genres"].([]interface{}); ok {
		movie.Genres = make([]string, len(val))
		for i, g := range val {
			if gs, ok := g.(string); ok {
				movie.Genres[i] = gs
			}
		}
	}
	if val, ok := metadata["release_date"].(string); ok {
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			movie.ReleaseDate = &t
		}
	}

	movie.Metadata = metadata

	return movie, nil
}

// Get retrieves a movie by ID
func (s *MovieStore) Get(ctx context.Context, id int64) (*models.Movie, error) {
	query := `
		SELECT id, tmdb_id, title, year, monitored, available,
			preferred_quality, metadata, added_at, last_checked
		FROM library_movies
		WHERE id = $1
	`

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query movie: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("movie not found")
	}

	return scanMovie(rows)
}

// GetByTMDBID retrieves a movie by TMDB ID
func (s *MovieStore) GetByTMDBID(ctx context.Context, tmdbID int) (*models.Movie, error) {
	query := `
		SELECT id, tmdb_id, title, year, monitored, available,
			preferred_quality, metadata, added_at, last_checked
		FROM library_movies
		WHERE tmdb_id = $1
	`

	rows, err := s.db.QueryContext(ctx, query, tmdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to query movie: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("movie not found")
	}

	return scanMovie(rows)
}

// List retrieves movies with pagination and optional filtering
func (s *MovieStore) List(ctx context.Context, offset, limit int, monitored *bool) ([]*models.Movie, error) {
	query := `
		SELECT id, tmdb_id, title, year, monitored, available,
			preferred_quality, metadata, added_at, last_checked
		FROM library_movies
	`

	args := []interface{}{}
	if monitored != nil {
		query += " WHERE monitored = $1"
		args = append(args, *monitored)
	}

	query += " ORDER BY added_at DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1) + " OFFSET $" + fmt.Sprintf("%d", len(args)+2)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}
	defer rows.Close()

	movies := []*models.Movie{}
	for rows.Next() {
		movie, err := scanMovie(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}
		movies = append(movies, movie)
	}

	return movies, nil
}

// Search searches movies by title
func (s *MovieStore) Search(ctx context.Context, query string, limit int) ([]*models.Movie, error) {
	sqlQuery := `
		SELECT id, tmdb_id, title, year, monitored, available,
			preferred_quality, metadata, added_at, last_checked
		FROM library_movies
		WHERE title_vector @@ plainto_tsquery('english', $1)
		ORDER BY ts_rank(title_vector, plainto_tsquery('english', $1)) DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search movies: %w", err)
	}
	defer rows.Close()

	movies := []*models.Movie{}
	for rows.Next() {
		movie, err := scanMovie(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}
		movies = append(movies, movie)
	}

	return movies, nil
}

// Update updates movie settings
func (s *MovieStore) Update(ctx context.Context, movie *models.Movie) error {
	metadataJSON, err := json.Marshal(movie.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE library_movies
		SET monitored = $1, preferred_quality = $2, metadata = $3, last_checked = $4
		WHERE id = $5
	`

	_, err = s.db.ExecContext(ctx, query,
		movie.Monitored, movie.QualityProfile, metadataJSON, time.Now(), movie.ID)

	if err != nil {
		return fmt.Errorf("failed to update movie: %w", err)
	}

	return nil
}

// Delete removes a movie from the library
func (s *MovieStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM library_movies WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("movie not found")
	}

	return nil
}

// GetMonitored retrieves all monitored movies
func (s *MovieStore) GetMonitored(ctx context.Context) ([]*models.Movie, error) {
	monitored := true
	return s.List(ctx, 0, 10000, &monitored)
}

// UpdateSearchStatus updates the search status of a movie
func (s *MovieStore) UpdateSearchStatus(ctx context.Context, id int64, status string) error {
	// Store search status in metadata
	query := `
		UPDATE library_movies
		SET metadata = jsonb_set(metadata, '{search_status}', to_jsonb($1::text)),
		    last_checked = $2
		WHERE id = $3
	`

	_, err := s.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update search status: %w", err)
	}

	return nil
}

// Count returns the total number of movies
func (s *MovieStore) Count(ctx context.Context) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM library_movies`

	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count movies: %w", err)
	}

	return count, nil
}

// GetUpcoming returns movies with release dates in the specified date range
// Since library_movies only stores year, we filter by metadata->>release_date
func (s *MovieStore) GetUpcoming(ctx context.Context, start, end string) ([]*models.Movie, error) {
	query := `
		SELECT id, tmdb_id, title, year, monitored, metadata, added_at, last_checked, available, preferred_quality
		FROM library_movies
		WHERE monitored = true
		  AND (
		    (metadata->>'release_date')::date BETWEEN $1::date AND $2::date
		    OR (year::text || '-01-01')::date BETWEEN $1::date AND $2::date
		  )
		ORDER BY COALESCE((metadata->>'release_date')::date, (year::text || '-01-01')::date) ASC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming movies: %w", err)
	}
	defer rows.Close()

	var movies []*models.Movie
	for rows.Next() {
		movie := &models.Movie{}
		var metadataJSON []byte
		var year sql.NullInt32
		var lastChecked sql.NullTime

		err := rows.Scan(
			&movie.ID,
			&movie.TMDBID,
			&movie.Title,
			&year,
			&movie.Monitored,
			&metadataJSON,
			&movie.AddedAt,
			&lastChecked,
			&movie.Available,
			&movie.QualityProfile,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan movie: %w", err)
		}

		// Parse metadata JSON to extract poster_path, overview, etc.
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &movie.Metadata)
			if posterPath, ok := movie.Metadata["poster_path"].(string); ok {
				movie.PosterPath = posterPath
			}
			if overview, ok := movie.Metadata["overview"].(string); ok {
				movie.Overview = overview
			}
			if releaseDate, ok := movie.Metadata["release_date"].(string); ok {
				if t, err := time.Parse("2006-01-02", releaseDate); err == nil {
					movie.ReleaseDate = &t
				}
			}
		}

		// Fallback to year if no release_date in metadata
		if movie.ReleaseDate == nil && year.Valid {
			t := time.Date(int(year.Int32), 1, 1, 0, 0, 0, 0, time.UTC)
			movie.ReleaseDate = &t
		}

		if lastChecked.Valid {
			movie.LastChecked = &lastChecked.Time
		}

		movies = append(movies, movie)
	}

	return movies, nil
}

// CountMonitored returns the number of monitored movies
func (s *MovieStore) CountMonitored(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_movies WHERE monitored = true").Scan(&count)
	return count, err
}

// CountAvailable returns the number of available movies
func (s *MovieStore) CountAvailable(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_movies WHERE available = true").Scan(&count)
	return count, err
}
