package database

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// UserStore handles user database operations
type UserStore struct {
	db *sql.DB
}

// User represents a system user
type User struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Password   string    `json:"-"` // Never expose in JSON
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
}

// WatchlistItem represents a user's watchlist entry
type WatchlistItem struct {
	ID       int       `json:"id"`
	UserID   int       `json:"user_id"`
	StreamID int       `json:"stream_id"`
	Title    string    `json:"title"`
	Type     string    `json:"type"`
	AddedAt  time.Time `json:"added_at"`
}

// WatchHistory represents viewing history
type WatchHistory struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	StreamID  int       `json:"stream_id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Progress  int       `json:"progress"`
	Duration  int       `json:"duration"`
	WatchedAt time.Time `json:"watched_at"`
}

// UserPlaylist represents a custom playlist
type UserPlaylist struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	ItemCount   int       `json:"item_count"`
}

// PlaylistItem represents an item in a playlist
type PlaylistItem struct {
	ID         int `json:"id"`
	PlaylistID int `json:"playlist_id"`
	StreamID   int `json:"stream_id"`
	Position   int `json:"position"`
}

// NewUserStore creates a new user store
func NewUserStore(db *sql.DB) (*UserStore, error) {
	store := &UserStore{db: db}
	if err := store.initTables(); err != nil {
		return nil, err
	}
	return store, nil
}

// initTables creates user-related tables
func (s *UserStore) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			role VARCHAR(50) DEFAULT 'user',
			status VARCHAR(50) DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_watchlist (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			stream_id INTEGER NOT NULL,
			title VARCHAR(500) NOT NULL,
			type VARCHAR(50) NOT NULL,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, stream_id)
		)`,
		`CREATE TABLE IF NOT EXISTS user_history (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			stream_id INTEGER NOT NULL,
			title VARCHAR(500) NOT NULL,
			type VARCHAR(50) NOT NULL,
			progress INTEGER DEFAULT 0,
			duration INTEGER DEFAULT 0,
			watched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS user_playlists (
			playlist_id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_items (
			id SERIAL PRIMARY KEY,
			playlist_id INTEGER NOT NULL REFERENCES user_playlists(playlist_id) ON DELETE CASCADE,
			stream_id INTEGER NOT NULL,
			position INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_user_watchlist_user ON user_watchlist(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_history_user ON user_history(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_playlists_user ON user_playlists(user_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// CreateUser creates a new user with hashed password
func (s *UserStore) CreateUser(username, email, password, role string) (int, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	var userID int
	err = s.db.QueryRow(`
		INSERT INTO users (username, email, password, role, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING user_id
	`, username, email, string(hashedPassword), role).Scan(&userID)

	return userID, err
}

// GetUserByID retrieves a user by ID
func (s *UserStore) GetUserByID(userID int) (*User, error) {
	var user User
	err := s.db.QueryRow(`
		SELECT user_id, username, email, password, role, status, created_at, last_active
		FROM users WHERE user_id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.LastActive)

	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *UserStore) GetUserByUsername(username string) (*User, error) {
	var user User
	err := s.db.QueryRow(`
		SELECT user_id, username, email, password, role, status, created_at, last_active
		FROM users WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.LastActive)

	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserStore) GetUserByEmail(email string) (*User, error) {
	var user User
	err := s.db.QueryRow(`
		SELECT id, username, email, password, role, status, created_at, last_active
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Status, &user.CreatedAt, &user.LastActive)

	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAllUsers retrieves all users with statistics
func (s *UserStore) GetAllUsers() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT 
			u.user_id, u.username, u.email, u.role, u.status, u.created_at, u.last_active,
			COUNT(DISTINCT w.id) as watchlist_count,
			COUNT(DISTINCT p.playlist_id) as playlist_count,
			MAX(wh.watched_at) as last_watch
		FROM users u
		LEFT JOIN user_watchlist w ON u.user_id = w.user_id
		LEFT JOIN user_playlists p ON u.user_id = p.user_id
		LEFT JOIN user_history wh ON u.user_id = wh.user_id
		GROUP BY u.user_id
		ORDER BY u.last_active DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, watchlistCount, playlistCount int
		var username, email, role, status string
		var createdAt, lastActive time.Time
		var lastWatch sql.NullTime

		err := rows.Scan(&id, &username, &email, &role, &status, &createdAt, &lastActive,
			&watchlistCount, &playlistCount, &lastWatch)
		if err != nil {
			continue
		}

		user := map[string]interface{}{
			"id":              id,
			"username":        username,
			"email":           email,
			"role":            role,
			"status":          status,
			"created_at":      createdAt,
			"last_active":     lastActive,
			"watchlist_count": watchlistCount,
			"playlist_count":  playlistCount,
		}
		if lastWatch.Valid {
			user["last_watch"] = lastWatch.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// UpdateUser updates user fields
func (s *UserStore) UpdateUser(userID int, updates map[string]interface{}) error {
	allowedFields := []string{"username", "email", "role", "status"}
	
	query := "UPDATE users SET "
	args := []interface{}{}
	argPos := 1

	for _, field := range allowedFields {
		if value, ok := updates[field]; ok {
			if argPos > 1 {
				query += ", "
			}
			query += fmt.Sprintf("%s = $%d", field, argPos)
			args = append(args, value)
			argPos++
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("no valid fields to update")
	}

	query += fmt.Sprintf(" WHERE user_id = $%d", argPos)
	args = append(args, userID)

	_, err := s.db.Exec(query, args...)
	return err
}

// DeleteUser deletes a user
func (s *UserStore) DeleteUser(userID int) error {
	_, err := s.db.Exec("DELETE FROM users WHERE user_id = $1", userID)
	return err
}

// UpdateLastActive updates user's last active timestamp
func (s *UserStore) UpdateLastActive(userID int) error {
	_, err := s.db.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE user_id = $1", userID)
	return err
}

// VerifyPassword checks if password matches user's hashed password
func (s *UserStore) VerifyPassword(username, password string) (*User, error) {
	user, err := s.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	return user, nil
}

// Watchlist Management
func (s *UserStore) AddToWatchlist(userID, streamID int, title, mediaType string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_watchlist (user_id, stream_id, title, type)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, stream_id) DO NOTHING
	`, userID, streamID, title, mediaType)
	return err
}

func (s *UserStore) RemoveFromWatchlist(userID, streamID int) error {
	_, err := s.db.Exec("DELETE FROM user_watchlist WHERE user_id = $1 AND stream_id = $2", userID, streamID)
	return err
}

func (s *UserStore) GetWatchlist(userID int) ([]WatchlistItem, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, stream_id, title, type, added_at
		FROM user_watchlist
		WHERE user_id = $1
		ORDER BY added_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WatchlistItem
	for rows.Next() {
		var item WatchlistItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.StreamID, &item.Title, &item.Type, &item.AddedAt); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// Watch History Management
func (s *UserStore) AddWatchHistory(userID, streamID int, title, mediaType string, progress, duration int) error {
	_, err := s.db.Exec(`
		INSERT INTO user_history (user_id, stream_id, title, type, progress, duration)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, userID, streamID, title, mediaType, progress, duration)
	return err
}

func (s *UserStore) GetWatchHistory(userID, limit int) ([]WatchHistory, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, stream_id, title, type, progress, duration, watched_at
		FROM user_history
		WHERE user_id = $1
		ORDER BY watched_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []WatchHistory
	for rows.Next() {
		var h WatchHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.StreamID, &h.Title, &h.Type, &h.Progress, &h.Duration, &h.WatchedAt); err != nil {
			continue
		}
		history = append(history, h)
	}

	return history, nil
}

// Playlist Management
func (s *UserStore) CreatePlaylist(userID int, name, description string) (int, error) {
	var playlistID int
	err := s.db.QueryRow(`
		INSERT INTO user_playlists (user_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING playlist_id
	`, userID, name, description).Scan(&playlistID)

	return playlistID, err
}

func (s *UserStore) GetUserPlaylists(userID int) ([]UserPlaylist, error) {
	rows, err := s.db.Query(`
		SELECT 
			p.playlist_id, p.user_id, p.name, p.description, p.created_at,
			COUNT(pi.id) as item_count
		FROM user_playlists p
		LEFT JOIN playlist_items pi ON p.playlist_id = pi.playlist_id
		WHERE p.user_id = $1
		GROUP BY p.playlist_id
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []UserPlaylist
	for rows.Next() {
		var p UserPlaylist
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.CreatedAt, &p.ItemCount); err != nil {
			continue
		}
		playlists = append(playlists, p)
	}

	return playlists, nil
}

func (s *UserStore) AddToPlaylist(playlistID, streamID int) error {
	// Get next position
	var maxPos sql.NullInt64
	s.db.QueryRow("SELECT MAX(position) FROM playlist_items WHERE playlist_id = $1", playlistID).Scan(&maxPos)
	
	nextPos := 0
	if maxPos.Valid {
		nextPos = int(maxPos.Int64) + 1
	}

	_, err := s.db.Exec(`
		INSERT INTO playlist_items (playlist_id, stream_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING
	`, playlistID, streamID, nextPos)

	return err
}

// Statistics
func (s *UserStore) GetUserStats() (map[string]int, error) {
	var totalUsers, activeUsers, onlineUsers int
	err := s.db.QueryRow(`
		SELECT 
			COUNT(*) as total_users,
			SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END) as active_users,
			SUM(CASE WHEN last_active > NOW() - INTERVAL '1 hour' THEN 1 ELSE 0 END) as online_users
		FROM users
	`).Scan(&totalUsers, &activeUsers, &onlineUsers)

	if err != nil {
		return nil, err
	}

	return map[string]int{
		"total_users":  totalUsers,
		"active_users": activeUsers,
		"online_users": onlineUsers,
	}, nil
}
