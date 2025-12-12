package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

type EpisodeStore struct {
	db *sql.DB
}

func NewEpisodeStore(db *sql.DB) *EpisodeStore {
	return &EpisodeStore{db: db}
}

// Add adds a new episode to the library
func (e *EpisodeStore) Add(ctx context.Context, episode *models.Episode) error {
	metadataJSON, err := json.Marshal(episode.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO library_episodes (
			series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at
	`

	err = e.db.QueryRowContext(
		ctx, query,
		episode.SeriesID, episode.TMDBID, episode.SeasonNumber, episode.EpisodeNumber,
		episode.Title, episode.Overview, episode.AirDate, episode.StillPath,
		episode.Runtime, metadataJSON, episode.Monitored, episode.Available,
		episode.StreamURL, episode.LastChecked,
	).Scan(&episode.ID, &episode.CreatedAt, &episode.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to add episode: %w", err)
	}

	return nil
}

// AddBatch adds multiple episodes in a single transaction
func (e *EpisodeStore) AddBatch(ctx context.Context, episodes []*models.Episode) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO library_episodes (
			series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, episode := range episodes {
		metadataJSON, err := json.Marshal(episode.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		err = stmt.QueryRowContext(
			ctx,
			episode.SeriesID, episode.TMDBID, episode.SeasonNumber, episode.EpisodeNumber,
			episode.Title, episode.Overview, episode.AirDate, episode.StillPath,
			episode.Runtime, metadataJSON, episode.Monitored, episode.Available,
			episode.StreamURL, episode.LastChecked,
		).Scan(&episode.ID, &episode.CreatedAt, &episode.UpdatedAt)

		if err != nil {
			return fmt.Errorf("failed to add episode: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Get retrieves an episode by ID
func (e *EpisodeStore) Get(ctx context.Context, id int64) (*models.Episode, error) {
	query := `
		SELECT id, series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked,
			created_at, updated_at
		FROM library_episodes
		WHERE id = $1
	`

	var episode models.Episode
	var metadataJSON []byte

	err := e.db.QueryRowContext(ctx, query, id).Scan(
		&episode.ID, &episode.SeriesID, &episode.TMDBID, &episode.SeasonNumber,
		&episode.EpisodeNumber, &episode.Title, &episode.Overview, &episode.AirDate,
		&episode.StillPath, &episode.Runtime, &metadataJSON, &episode.Monitored,
		&episode.Available, &episode.StreamURL, &episode.LastChecked,
		&episode.CreatedAt, &episode.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("episode not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get episode: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &episode.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &episode, nil
}

// GetBySeriesAndNumber retrieves an episode by series, season, and episode number
func (e *EpisodeStore) GetBySeriesAndNumber(ctx context.Context, seriesID int64, season, episode int) (*models.Episode, error) {
	query := `
		SELECT id, series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked,
			created_at, updated_at
		FROM library_episodes
		WHERE series_id = $1 AND season_number = $2 AND episode_number = $3
	`

	var ep models.Episode
	var metadataJSON []byte

	err := e.db.QueryRowContext(ctx, query, seriesID, season, episode).Scan(
		&ep.ID, &ep.SeriesID, &ep.TMDBID, &ep.SeasonNumber,
		&ep.EpisodeNumber, &ep.Title, &ep.Overview, &ep.AirDate,
		&ep.StillPath, &ep.Runtime, &metadataJSON, &ep.Monitored,
		&ep.Available, &ep.StreamURL, &ep.LastChecked,
		&ep.CreatedAt, &ep.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("episode not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get episode: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &ep.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &ep, nil
}

// ListBySeries returns all episodes for a series
func (e *EpisodeStore) ListBySeries(ctx context.Context, seriesID int64) ([]*models.Episode, error) {
	query := `
		SELECT id, series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked,
			created_at, updated_at
		FROM library_episodes
		WHERE series_id = $1
		ORDER BY season_number ASC, episode_number ASC
	`

	rows, err := e.db.QueryContext(ctx, query, seriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*models.Episode
	for rows.Next() {
		var episode models.Episode
		var metadataJSON []byte

		err := rows.Scan(
			&episode.ID, &episode.SeriesID, &episode.TMDBID, &episode.SeasonNumber,
			&episode.EpisodeNumber, &episode.Title, &episode.Overview, &episode.AirDate,
			&episode.StillPath, &episode.Runtime, &metadataJSON, &episode.Monitored,
			&episode.Available, &episode.StreamURL, &episode.LastChecked,
			&episode.CreatedAt, &episode.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan episode: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &episode.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		episodes = append(episodes, &episode)
	}

	return episodes, nil
}

// ListBySeason returns all episodes for a specific season
func (e *EpisodeStore) ListBySeason(ctx context.Context, seriesID int64, season int) ([]*models.Episode, error) {
	query := `
		SELECT id, series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked,
			created_at, updated_at
		FROM library_episodes
		WHERE series_id = $1 AND season_number = $2
		ORDER BY episode_number ASC
	`

	rows, err := e.db.QueryContext(ctx, query, seriesID, season)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*models.Episode
	for rows.Next() {
		var episode models.Episode
		var metadataJSON []byte

		err := rows.Scan(
			&episode.ID, &episode.SeriesID, &episode.TMDBID, &episode.SeasonNumber,
			&episode.EpisodeNumber, &episode.Title, &episode.Overview, &episode.AirDate,
			&episode.StillPath, &episode.Runtime, &metadataJSON, &episode.Monitored,
			&episode.Available, &episode.StreamURL, &episode.LastChecked,
			&episode.CreatedAt, &episode.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan episode: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &episode.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		episodes = append(episodes, &episode)
	}

	return episodes, nil
}

// ListAiringToday returns episodes airing today (based on air_date)
func (e *EpisodeStore) ListAiringToday(ctx context.Context) ([]*models.Episode, error) {
	query := `
		SELECT id, series_id, tmdb_id, season_number, episode_number, title,
			overview, air_date, still_path, runtime, metadata,
			monitored, available, stream_url, last_checked,
			created_at, updated_at
		FROM library_episodes
		WHERE air_date = CURRENT_DATE AND monitored = true
		ORDER BY air_date ASC
	`

	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list airing episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*models.Episode
	for rows.Next() {
		var episode models.Episode
		var metadataJSON []byte

		err := rows.Scan(
			&episode.ID, &episode.SeriesID, &episode.TMDBID, &episode.SeasonNumber,
			&episode.EpisodeNumber, &episode.Title, &episode.Overview, &episode.AirDate,
			&episode.StillPath, &episode.Runtime, &metadataJSON, &episode.Monitored,
			&episode.Available, &episode.StreamURL, &episode.LastChecked,
			&episode.CreatedAt, &episode.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan episode: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &episode.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		episodes = append(episodes, &episode)
	}

	return episodes, nil
}

// Update updates an existing episode
func (e *EpisodeStore) Update(ctx context.Context, episode *models.Episode) error {
	metadataJSON, err := json.Marshal(episode.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE library_episodes
		SET title = $1, overview = $2, air_date = $3, still_path = $4,
			runtime = $5, metadata = $6, monitored = $7, available = $8,
			stream_url = $9, last_checked = $10, updated_at = $11
		WHERE id = $12
	`

	result, err := e.db.ExecContext(
		ctx, query,
		episode.Title, episode.Overview, episode.AirDate, episode.StillPath,
		episode.Runtime, metadataJSON, episode.Monitored, episode.Available,
		episode.StreamURL, episode.LastChecked, time.Now(), episode.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update episode: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("episode not found")
	}

	return nil
}

// UpdateAvailability updates the availability status and stream URL
func (e *EpisodeStore) UpdateAvailability(ctx context.Context, id int64, available bool, streamURL *string) error {
	query := `
		UPDATE library_episodes
		SET available = $1, stream_url = $2, last_checked = $3, updated_at = $4
		WHERE id = $5
	`

	now := time.Now()
	result, err := e.db.ExecContext(ctx, query, available, streamURL, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to update availability: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("episode not found")
	}

	return nil
}

// Delete removes an episode
func (e *EpisodeStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM library_episodes WHERE id = $1`

	result, err := e.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete episode: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("episode not found")
	}

	return nil
}

// DeleteBySeries removes all episodes for a series
func (e *EpisodeStore) DeleteBySeries(ctx context.Context, seriesID int64) error {
	query := `DELETE FROM library_episodes WHERE series_id = $1`

	_, err := e.db.ExecContext(ctx, query, seriesID)
	if err != nil {
		return fmt.Errorf("failed to delete episodes: %w", err)
	}

	return nil
}

// Count returns the total number of episodes
func (e *EpisodeStore) Count(ctx context.Context, seriesID *int64, monitored *bool) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM library_episodes
		WHERE ($1::bigint IS NULL OR series_id = $1)
		  AND ($2::boolean IS NULL OR monitored = $2)
	`

	var count int
	err := e.db.QueryRowContext(ctx, query, seriesID, monitored).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count episodes: %w", err)
	}

	return count, nil
}

// GetUpcoming returns episodes with air dates in the specified date range
func (e *EpisodeStore) GetUpcoming(ctx context.Context, start, end string) ([]*models.Episode, error) {
	query := `
		SELECT id, series_id, season_number, episode_number,
		       title, overview, air_date, still_path,
		       monitored, available, last_checked
		FROM library_episodes
		WHERE air_date IS NOT NULL
		  AND air_date BETWEEN $1::date AND $2::date
		  AND monitored = true
		ORDER BY air_date ASC
		LIMIT 100
	`

	rows, err := e.db.QueryContext(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*models.Episode
	for rows.Next() {
		episode := &models.Episode{}
		var stillPath sql.NullString
		var lastChecked sql.NullTime

		err := rows.Scan(
			&episode.ID,
			&episode.SeriesID,
			&episode.SeasonNumber,
			&episode.EpisodeNumber,
			&episode.Title,
			&episode.Overview,
			&episode.AirDate,
			&stillPath,
			&episode.Monitored,
			&episode.Available,
			&lastChecked,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan episode: %w", err)
		}

		if stillPath.Valid {
			episode.StillPath = stillPath.String
		}
		if lastChecked.Valid {
			episode.LastChecked = &lastChecked.Time
		}

		episodes = append(episodes, episode)
	}

	return episodes, rows.Err()
}

// DeleteAll removes all episodes from the database
func (e *EpisodeStore) DeleteAll(ctx context.Context) error {
	_, err := e.db.ExecContext(ctx, "DELETE FROM library_episodes")
	return err
}

// CountAll returns the total count of all episodes
func (e *EpisodeStore) CountAll(ctx context.Context) (int, error) {
	var count int
	err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_episodes").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListAll returns all episodes (just counts for stats)
func (e *EpisodeStore) ListAll(ctx context.Context) ([]*models.Episode, error) {
	var count int
	err := e.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_episodes").Scan(&count)
	if err != nil {
		return nil, err
	}
	// Return empty slice with length info encoded
	result := make([]*models.Episode, count)
	return result, nil
}

