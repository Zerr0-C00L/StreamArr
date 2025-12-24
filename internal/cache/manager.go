package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Manager handles all caching operations
type Manager struct {
	db           *sql.DB
	episodeCache *EpisodeCache
	rdURLCache   *RDURLCache
	requestCache *RequestCache
	mu           sync.RWMutex
}

// EpisodeCache stores episode metadata and availability
type EpisodeCache struct {
	db     *sql.DB
	mu     sync.RWMutex
	memory map[string]*EpisodeData
}

// EpisodeData represents cached episode information
type EpisodeData struct {
	IMDBID      string                 `json:"imdb_id"`
	TMDBID      int                    `json:"tmdb_id"`
	Season      int                    `json:"season"`
	Episode     int                    `json:"episode"`
	Title       string                 `json:"title"`
	Streams     []StreamData           `json:"streams"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// StreamData represents a single stream source
type StreamData struct {
	Hash     string `json:"hash"`
	Title    string `json:"title"`
	Quality  string `json:"quality"`
	Size     int64  `json:"size"`
	Cached   bool   `json:"cached"`
	Provider string `json:"provider"`
	FileIdx  int    `json:"file_idx"`
	FileName string `json:"file_name"`
}

// RDURLCache stores Real-Debrid resolved URLs
type RDURLCache struct {
	db     *sql.DB
	mu     sync.RWMutex
	memory map[string]*RDURLData
}

// RDURLData represents a cached RD URL
type RDURLData struct {
	Hash      string    `json:"hash"`
	FileID    string    `json:"file_id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Quality   string    `json:"quality"`
	Size      int64     `json:"size"`
}

// RequestCache stores generic HTTP request results
type RequestCache struct {
	db     *sql.DB
	mu     sync.RWMutex
	memory map[string]*RequestData
}

// RequestData represents a cached request
type RequestData struct {
	Key       string    `json:"key"`
	Data      []byte    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewManager creates a new cache manager
func NewManager(db *sql.DB) *Manager {
	manager := &Manager{
		db:           db,
		episodeCache: NewEpisodeCache(db),
		rdURLCache:   NewRDURLCache(db),
		requestCache: NewRequestCache(db),
	}

	// Initialize tables
	if err := manager.initTables(); err != nil {
		panic(fmt.Sprintf("Failed to initialize cache tables: %v", err))
	}

	// Start cleanup worker
	go manager.cleanupWorker()

	return manager
}

func (m *Manager) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS episode_cache (
			imdb_id TEXT NOT NULL,
			season INTEGER NOT NULL,
			episode INTEGER NOT NULL,
			data JSONB NOT NULL,
			last_updated TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (imdb_id, season, episode)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_episode_cache_imdb ON episode_cache(imdb_id)`,
		`CREATE INDEX IF NOT EXISTS idx_episode_cache_updated ON episode_cache(last_updated)`,

		`CREATE TABLE IF NOT EXISTS rd_url_cache (
			hash TEXT NOT NULL,
			file_id TEXT NOT NULL,
			url TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			quality TEXT,
			size BIGINT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (hash, file_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rd_url_expires ON rd_url_cache(expires_at)`,

		`CREATE TABLE IF NOT EXISTS request_cache (
			key TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_expires ON request_cache(expires_at)`,
	}

	for _, query := range queries {
		if _, err := m.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

func (m *Manager) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.Cleanup()
	}
}

// Cleanup removes expired entries
func (m *Manager) Cleanup() {
	ctx := context.Background()

	// Clean RD URLs
	if _, err := m.db.ExecContext(ctx, "DELETE FROM rd_url_cache WHERE expires_at < NOW()"); err != nil {
		fmt.Printf("Error cleaning RD URL cache: %v\n", err)
	}

	// Clean requests
	if _, err := m.db.ExecContext(ctx, "DELETE FROM request_cache WHERE expires_at < NOW()"); err != nil {
		fmt.Printf("Error cleaning request cache: %v\n", err)
	}

	// Clean old episode cache (>30 days)
	if _, err := m.db.ExecContext(ctx, "DELETE FROM episode_cache WHERE last_updated < NOW() - INTERVAL '30 days'"); err != nil {
		fmt.Printf("Error cleaning episode cache: %v\n", err)
	}
}

// EpisodeCache implementation

func NewEpisodeCache(db *sql.DB) *EpisodeCache {
	return &EpisodeCache{
		db:     db,
		memory: make(map[string]*EpisodeData),
	}
}

func (ec *EpisodeCache) Get(imdbID string, season, episode int) (*EpisodeData, error) {
	ec.mu.RLock()
	key := fmt.Sprintf("%s:%d:%d", imdbID, season, episode)
	if data, ok := ec.memory[key]; ok {
		ec.mu.RUnlock()
		return data, nil
	}
	ec.mu.RUnlock()

	// Load from database
	var dataJSON []byte
	var lastUpdated time.Time

	err := ec.db.QueryRow(
		"SELECT data, last_updated FROM episode_cache WHERE imdb_id = $1 AND season = $2 AND episode = $3",
		imdbID, season, episode,
	).Scan(&dataJSON, &lastUpdated)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var data EpisodeData
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		return nil, err
	}
	data.LastUpdated = lastUpdated

	// Cache in memory
	ec.mu.Lock()
	ec.memory[key] = &data
	ec.mu.Unlock()

	return &data, nil
}

func (ec *EpisodeCache) Set(data *EpisodeData) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = ec.db.Exec(
		`INSERT INTO episode_cache (imdb_id, season, episode, data, last_updated)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (imdb_id, season, episode)
		 DO UPDATE SET data = $4, last_updated = NOW()`,
		data.IMDBID, data.Season, data.Episode, dataJSON,
	)

	if err != nil {
		return err
	}

	// Update memory cache
	key := fmt.Sprintf("%s:%d:%d", data.IMDBID, data.Season, data.Episode)
	ec.mu.Lock()
	data.LastUpdated = time.Now()
	ec.memory[key] = data
	ec.mu.Unlock()

	return nil
}

func (ec *EpisodeCache) GetBySeries(imdbID string) ([]*EpisodeData, error) {
	rows, err := ec.db.Query(
		"SELECT data, last_updated FROM episode_cache WHERE imdb_id = $1 ORDER BY season, episode",
		imdbID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []*EpisodeData
	for rows.Next() {
		var dataJSON []byte
		var lastUpdated time.Time

		if err := rows.Scan(&dataJSON, &lastUpdated); err != nil {
			continue
		}

		var data EpisodeData
		if err := json.Unmarshal(dataJSON, &data); err != nil {
			continue
		}
		data.LastUpdated = lastUpdated
		episodes = append(episodes, &data)
	}

	return episodes, nil
}

// RDURLCache implementation

func NewRDURLCache(db *sql.DB) *RDURLCache {
	return &RDURLCache{
		db:     db,
		memory: make(map[string]*RDURLData),
	}
}

func (rc *RDURLCache) Get(hash, fileID string) (*RDURLData, error) {
	rc.mu.RLock()
	key := fmt.Sprintf("%s:%s", hash, fileID)
	if data, ok := rc.memory[key]; ok {
		if time.Now().Before(data.ExpiresAt) {
			rc.mu.RUnlock()
			return data, nil
		}
	}
	rc.mu.RUnlock()

	// Load from database
	var data RDURLData
	err := rc.db.QueryRow(
		"SELECT hash, file_id, url, expires_at, quality, size FROM rd_url_cache WHERE hash = $1 AND file_id = $2 AND expires_at > NOW()",
		hash, fileID,
	).Scan(&data.Hash, &data.FileID, &data.URL, &data.ExpiresAt, &data.Quality, &data.Size)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache in memory
	rc.mu.Lock()
	rc.memory[key] = &data
	rc.mu.Unlock()

	return &data, nil
}

func (rc *RDURLCache) Set(data *RDURLData) error {
	_, err := rc.db.Exec(
		`INSERT INTO rd_url_cache (hash, file_id, url, expires_at, quality, size)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (hash, file_id)
		 DO UPDATE SET url = $3, expires_at = $4, quality = $5, size = $6`,
		data.Hash, data.FileID, data.URL, data.ExpiresAt, data.Quality, data.Size,
	)

	if err != nil {
		return err
	}

	// Update memory cache
	key := fmt.Sprintf("%s:%s", data.Hash, data.FileID)
	rc.mu.Lock()
	rc.memory[key] = data
	rc.mu.Unlock()

	return nil
}

// RequestCache implementation

func NewRequestCache(db *sql.DB) *RequestCache {
	return &RequestCache{
		db:     db,
		memory: make(map[string]*RequestData),
	}
}

func (rc *RequestCache) Get(key string) (*RequestData, error) {
	rc.mu.RLock()
	if data, ok := rc.memory[key]; ok {
		if time.Now().Before(data.ExpiresAt) {
			rc.mu.RUnlock()
			return data, nil
		}
	}
	rc.mu.RUnlock()

	// Load from database
	var data RequestData
	err := rc.db.QueryRow(
		"SELECT key, data, expires_at FROM request_cache WHERE key = $1 AND expires_at > NOW()",
		key,
	).Scan(&data.Key, &data.Data, &data.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Cache in memory
	rc.mu.Lock()
	rc.memory[key] = &data
	rc.mu.Unlock()

	return &data, nil
}

func (rc *RequestCache) Set(key string, data []byte, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)

	_, err := rc.db.Exec(
		`INSERT INTO request_cache (key, data, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (key)
		 DO UPDATE SET data = $2, expires_at = $3`,
		key, data, expiresAt,
	)

	if err != nil {
		return err
	}

	// Update memory cache
	rc.mu.Lock()
	rc.memory[key] = &RequestData{
		Key:       key,
		Data:      data,
		ExpiresAt: expiresAt,
	}
	rc.mu.Unlock()

	return nil
}

// GetEpisodeCache returns the episode cache
func (m *Manager) GetEpisodeCache() *EpisodeCache {
	return m.episodeCache
}

// GetRDURLCache returns the RD URL cache
func (m *Manager) GetRDURLCache() *RDURLCache {
	return m.rdURLCache
}

// GetRequestCache returns the request cache
func (m *Manager) GetRequestCache() *RequestCache {
	return m.requestCache
}
