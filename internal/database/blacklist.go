package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// BlacklistEntry represents an entry in the blacklist
type BlacklistEntry struct {
	ID        int64     `json:"id"`
	TMDBID    int       `json:"tmdb_id"`
	ItemType  string    `json:"item_type"` // "movie" or "series"
	Title     string    `json:"title"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// BlacklistStore handles blacklist database operations
type BlacklistStore struct {
	db *sql.DB
}

// NewBlacklistStore creates a new blacklist store
func NewBlacklistStore(db *sql.DB) *BlacklistStore {
	return &BlacklistStore{db: db}
}

// Add adds an item to the blacklist
func (s *BlacklistStore) Add(ctx context.Context, tmdbID int, itemType, title, reason string) error {
	query := `
		INSERT INTO blacklist (tmdb_id, item_type, title, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tmdb_id, item_type) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, tmdbID, itemType, title, reason)
	if err != nil {
		return fmt.Errorf("failed to add to blacklist: %w", err)
	}
	return nil
}

// IsBlacklisted checks if an item is blacklisted
func (s *BlacklistStore) IsBlacklisted(ctx context.Context, tmdbID int, itemType string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM blacklist WHERE tmdb_id = $1 AND item_type = $2)`
	err := s.db.QueryRowContext(ctx, query, tmdbID, itemType).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return exists, nil
}

// List returns all blacklisted items with pagination
func (s *BlacklistStore) List(ctx context.Context, limit, offset int) ([]*BlacklistEntry, int, error) {
	// Get total count
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM blacklist").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count blacklist: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, tmdb_id, item_type, title, reason, created_at
		FROM blacklist
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list blacklist: %w", err)
	}
	defer rows.Close()

	var entries []*BlacklistEntry
	for rows.Next() {
		entry := &BlacklistEntry{}
		err := rows.Scan(&entry.ID, &entry.TMDBID, &entry.ItemType, &entry.Title, &entry.Reason, &entry.CreatedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan blacklist entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// Remove removes an item from the blacklist
func (s *BlacklistStore) Remove(ctx context.Context, id int64) error {
	query := `DELETE FROM blacklist WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to remove from blacklist: %w", err)
	}
	return nil
}

// RemoveByTMDB removes an item from blacklist by TMDB ID
func (s *BlacklistStore) RemoveByTMDB(ctx context.Context, tmdbID int, itemType string) error {
	query := `DELETE FROM blacklist WHERE tmdb_id = $1 AND item_type = $2`
	_, err := s.db.ExecContext(ctx, query, tmdbID, itemType)
	if err != nil {
		return fmt.Errorf("failed to remove from blacklist: %w", err)
	}
	return nil
}

// Clear removes all entries from the blacklist
func (s *BlacklistStore) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM blacklist")
	return err
}
