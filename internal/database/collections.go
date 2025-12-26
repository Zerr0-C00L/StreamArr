package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

// CollectionStore handles database operations for collections
type CollectionStore struct {
	db *sql.DB
}

// NewCollectionStore creates a new CollectionStore
func NewCollectionStore(db *sql.DB) *CollectionStore {
	return &CollectionStore{db: db}
}

// Create creates a new collection
func (s *CollectionStore) Create(ctx context.Context, collection *models.Collection) error {
	query := `
		INSERT INTO collections (tmdb_id, name, overview, poster_path, backdrop_path, total_movies)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tmdb_id) DO UPDATE SET
			name = EXCLUDED.name,
			overview = EXCLUDED.overview,
			poster_path = EXCLUDED.poster_path,
			backdrop_path = EXCLUDED.backdrop_path,
			total_movies = EXCLUDED.total_movies,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	return s.db.QueryRowContext(
		ctx,
		query,
		collection.TMDBID,
		collection.Name,
		collection.Overview,
		collection.PosterPath,
		collection.BackdropPath,
		collection.TotalMovies,
	).Scan(&collection.ID, &collection.CreatedAt, &collection.UpdatedAt)
}

// Count returns the total number of collections
func (s *CollectionStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM collections").Scan(&count)
	return count, err
}

// GetByID retrieves a collection by ID
func (s *CollectionStore) GetByID(ctx context.Context, id int64) (*models.Collection, error) {
	query := `
		SELECT 
			c.id, c.tmdb_id, c.name, c.overview, c.poster_path, c.backdrop_path,
			c.total_movies, c.created_at, c.updated_at,
			COALESCE(COUNT(m.id), 0) as movies_in_library
		FROM collections c
		LEFT JOIN library_movies m ON m.collection_id = c.id
		WHERE c.id = $1
		GROUP BY c.id
	`

	collection := &models.Collection{}
	var posterPath, backdropPath sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&collection.ID,
		&collection.TMDBID,
		&collection.Name,
		&collection.Overview,
		&posterPath,
		&backdropPath,
		&collection.TotalMovies,
		&collection.CreatedAt,
		&collection.UpdatedAt,
		&collection.MoviesInLibrary,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection not found")
	}
	if err != nil {
		return nil, err
	}

	collection.PosterPath = posterPath.String
	collection.BackdropPath = backdropPath.String

	return collection, nil
}

// GetByTMDBID retrieves a collection by TMDB ID
func (s *CollectionStore) GetByTMDBID(ctx context.Context, tmdbID int) (*models.Collection, error) {
	query := `
		SELECT 
			c.id, c.tmdb_id, c.name, c.overview, c.poster_path, c.backdrop_path,
			c.total_movies, c.created_at, c.updated_at,
			COALESCE(COUNT(m.id), 0) as movies_in_library
		FROM collections c
		LEFT JOIN library_movies m ON m.collection_id = c.id
		WHERE c.tmdb_id = $1
		GROUP BY c.id
	`

	collection := &models.Collection{}
	var posterPath, backdropPath sql.NullString

	err := s.db.QueryRowContext(ctx, query, tmdbID).Scan(
		&collection.ID,
		&collection.TMDBID,
		&collection.Name,
		&collection.Overview,
		&posterPath,
		&backdropPath,
		&collection.TotalMovies,
		&collection.CreatedAt,
		&collection.UpdatedAt,
		&collection.MoviesInLibrary,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Return nil, nil for not found
	}
	if err != nil {
		return nil, err
	}

	collection.PosterPath = posterPath.String
	collection.BackdropPath = backdropPath.String

	return collection, nil
}

// List retrieves all collections with pagination
func (s *CollectionStore) List(ctx context.Context, limit, offset int) ([]*models.Collection, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM collections`
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			c.id, c.tmdb_id, c.name, c.overview, c.poster_path, c.backdrop_path,
			c.total_movies, c.created_at, c.updated_at,
			COALESCE(COUNT(m.id), 0) as movies_in_library
		FROM collections c
		LEFT JOIN library_movies m ON m.collection_id = c.id
		GROUP BY c.id
		ORDER BY c.name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		collection := &models.Collection{}
		var posterPath, backdropPath sql.NullString

		err := rows.Scan(
			&collection.ID,
			&collection.TMDBID,
			&collection.Name,
			&collection.Overview,
			&posterPath,
			&backdropPath,
			&collection.TotalMovies,
			&collection.CreatedAt,
			&collection.UpdatedAt,
			&collection.MoviesInLibrary,
		)
		if err != nil {
			return nil, 0, err
		}

		collection.PosterPath = posterPath.String
		collection.BackdropPath = backdropPath.String
		collections = append(collections, collection)
	}

	return collections, total, nil
}

// GetMoviesInCollection retrieves all movies belonging to a collection
func (s *CollectionStore) GetMoviesInCollection(ctx context.Context, collectionID int64) ([]*models.Movie, error) {
	query := `
		SELECT id, tmdb_id, title, year, monitored, available,
			preferred_quality, metadata, added_at, last_checked, collection_id
		FROM library_movies
		WHERE collection_id = $1
		ORDER BY added_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []*models.Movie
	for rows.Next() {
		movie := &models.Movie{}
		var metadataJSON []byte
		var year *int
		var collID sql.NullInt64

		err := rows.Scan(
			&movie.ID, &movie.TMDBID, &movie.Title, &year,
			&movie.Monitored, &movie.Available, &movie.QualityProfile,
			&metadataJSON, &movie.AddedAt, &movie.LastChecked, &collID,
		)
		if err != nil {
			return nil, err
		}

		// Parse metadata JSON
		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
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
		}

		if collID.Valid {
			movie.CollectionID = &collID.Int64
		}

		movies = append(movies, movie)
	}

	return movies, nil
}

// Delete removes a collection
func (s *CollectionStore) Delete(ctx context.Context, id int64) error {
	// First, unlink all movies from this collection
	_, err := s.db.ExecContext(ctx, `UPDATE library_movies SET collection_id = NULL WHERE collection_id = $1`, id)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = $1`, id)
	return err
}

// UpdateMovieCollection links a movie to a collection
func (s *CollectionStore) UpdateMovieCollection(ctx context.Context, movieID, collectionID int64) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE library_movies SET collection_id = $1 WHERE id = $2`,
		collectionID,
		movieID,
	)
	return err
}

// GetCollectionsWithProgress retrieves collections with completion progress
func (s *CollectionStore) GetCollectionsWithProgress(ctx context.Context, limit, offset int) ([]*models.Collection, int, error) {
	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM collections`
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			c.id, c.tmdb_id, c.name, c.overview, c.poster_path, c.backdrop_path,
			c.total_movies, c.created_at, c.updated_at,
			COALESCE(COUNT(m.id), 0) as movies_in_library
		FROM collections c
		LEFT JOIN library_movies m ON m.collection_id = c.id
		GROUP BY c.id
		ORDER BY 
			CASE WHEN COUNT(m.id) = c.total_movies THEN 1 ELSE 0 END DESC,
			COUNT(m.id)::float / GREATEST(c.total_movies, 1) DESC,
			c.name ASC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var collections []*models.Collection
	for rows.Next() {
		collection := &models.Collection{}
		var posterPath, backdropPath sql.NullString

		err := rows.Scan(
			&collection.ID,
			&collection.TMDBID,
			&collection.Name,
			&collection.Overview,
			&posterPath,
			&backdropPath,
			&collection.TotalMovies,
			&collection.CreatedAt,
			&collection.UpdatedAt,
			&collection.MoviesInLibrary,
		)
		if err != nil {
			return nil, 0, err
		}

		collection.PosterPath = posterPath.String
		collection.BackdropPath = backdropPath.String
		collections = append(collections, collection)
	}

	return collections, total, nil
}

// DeleteAll removes all collections from the database
func (s *CollectionStore) DeleteAll(ctx context.Context) error {
	// First, unlink all movies from collections
	_, err := s.db.ExecContext(ctx, "UPDATE library_movies SET collection_id = NULL")
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM collections")
	return err
}

// ListAll returns all collections
func (s *CollectionStore) ListAll(ctx context.Context) ([]*models.Collection, error) {
	collections, _, err := s.GetCollectionsWithProgress(ctx, 10000, 0)
	return collections, err
}
