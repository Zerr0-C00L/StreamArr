package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
)

// M3USource represents a custom M3U playlist source for Live TV
type M3USource struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type Settings struct {
	// Database Configuration (required, set via DATABASE_URL env var as fallback)
	DatabaseURL string `json:"database_url"`
	RedisURL    string `json:"redis_url"`
	
	// API Keys
	TMDBAPIKey       string `json:"tmdb_api_key"`
	RealDebridAPIKey string `json:"realdebrid_api_key"`
	PremiumizeAPIKey string `json:"premiumize_api_key"`
	MDBListAPIKey    string `json:"mdblist_api_key"`
	MDBListLists     string `json:"mdblist_lists"`
	
	// Service URLs
	TorrentioURL   string `json:"torrentio_url"`
	CometURL       string `json:"comet_url"`
	MediaFusionURL string `json:"mediafusion_url"`
	
	// Quality Settings
	MaxResolution         int    `json:"max_resolution"`
	MaxFileSize           int    `json:"max_file_size"`
	EnableQualityVariants bool   `json:"enable_quality_variants"`
	ShowFullStreamName    bool   `json:"show_full_stream_name"`
	AutoAddCollections    bool   `json:"auto_add_collections"` // Automatically add entire collection when adding a movie
	
	// Playlist Settings
	TotalPages             int    `json:"total_pages"`
	MinYear                int    `json:"min_year"`
	MinRuntime             int    `json:"min_runtime"`
	Language               string `json:"language"`
	SeriesOriginCountry    string `json:"series_origin_country"`
	MoviesOriginCountry    string `json:"movies_origin_country"`
	UserCreatePlaylist     bool   `json:"user_create_playlist"`
	IncludeAdultVOD        bool   `json:"include_adult_vod"`
	IncludeLiveTV          bool   `json:"include_live_tv"`
	AutoCacheIntervalHours int    `json:"auto_cache_interval_hours"`
	
	// Live TV / M3U Sources
	M3USources            []M3USource `json:"m3u_sources"`
	LiveTVEnabledSources  []string    `json:"livetv_enabled_sources"`   // Which sources are enabled
	LiveTVEnabledCategories []string  `json:"livetv_enabled_categories"` // Which categories are enabled
	LiveTVShowAllSources  bool        `json:"livetv_show_all_sources"`   // Show all sources by default
	LiveTVShowAllCategories bool      `json:"livetv_show_all_categories"` // Show all categories by default
	LiveTVEnablePlutoTV   bool        `json:"livetv_enable_plutotv"`     // Enable built-in Pluto TV channels
	
	// Provider Settings
	UseRealDebrid      bool     `json:"use_realdebrid"`
	UsePremiumize      bool     `json:"use_premiumize"`
	StreamProviders    []string `json:"stream_providers"`
	TorrentioProviders string   `json:"torrentio_providers"`
	CometIndexers      []string `json:"comet_indexers"`
	
	// Proxy Settings
	HTTPProxy    string `json:"http_proxy"`
	UseHTTPProxy bool   `json:"use_http_proxy"`
	
	// HeadlessVidX Settings
	HeadlessVidXAddress    string `json:"headless_vidx_address"`
	HeadlessVidXMaxThreads int    `json:"headless_vidx_max_threads"`
	
	// Notification Settings
	EnableNotifications bool   `json:"enable_notifications"`
	DiscordWebhookURL   string `json:"discord_webhook_url"`
	TelegramBotToken    string `json:"telegram_bot_token"`
	TelegramChatID      string `json:"telegram_chat_id"`
	
	// Release Filter Settings
	EnableReleaseFilters  bool   `json:"enable_release_filters"`   // Enable/disable release filtering
	ExcludedReleaseGroups string `json:"excluded_release_groups"` // Pipe-separated patterns
	ExcludedLanguageTags  string `json:"excluded_language_tags"`  // Pipe-separated patterns
	ExcludedQualities     string `json:"excluded_qualities"`      // Pipe-separated patterns
	CustomExcludePatterns string `json:"custom_exclude_patterns"` // Pipe-separated regex patterns
	
	// Stream Sorting Settings
	StreamSortOrder  string `json:"stream_sort_order"`  // e.g., "quality,size,seeders"
	StreamSortPrefer string `json:"stream_sort_prefer"` // "best", "smallest", or "balanced"
	
	// System Settings
	Debug      bool   `json:"debug"`
	ServerPort int    `json:"server_port"`
	Host       string `json:"host"`
}

type Manager struct {
	db       *sql.DB
	settings *Settings
	mu       sync.RWMutex
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:       db,
		settings: getDefaultSettings(),
	}
}

func getDefaultSettings() *Settings {
	return &Settings{
		// Database defaults (can be overridden by environment variables for initial setup)
		DatabaseURL:            "postgres://streamarr:streamarr_password@localhost:5432/streamarr?sslmode=disable",
		RedisURL:               "redis://localhost:6379/0",
		
		// Service URLs
		TorrentioURL:           "https://torrentio.strem.fun",
		CometURL:               "https://comet.elfhosted.com",
		MediaFusionURL:         "https://mediafusion.elfhosted.com",
		
		// Quality defaults
		MaxResolution:          2160,
		MaxFileSize:            50000,
		EnableQualityVariants:  false,
		ShowFullStreamName:     false,
		TotalPages:             5,
		MinYear:                1970,
		MinRuntime:             30,
		Language:               "en-US",
		SeriesOriginCountry:    "US",
		MoviesOriginCountry:    "US",
		UserCreatePlaylist:     true,
		IncludeAdultVOD:        false,
		AutoCacheIntervalHours: 6,
		UseRealDebrid:          true,
		UsePremiumize:          false,
		StreamProviders:        []string{"comet", "mediafusion", "torrentio"},
		TorrentioProviders:     "yts,eztv,rarbg,1337x,thepiratebay",
		CometIndexers:          []string{"bktorrent", "thepiratebay", "yts", "eztv"},
		UseHTTPProxy:           false,
		HeadlessVidXAddress:    "localhost:3202",
		HeadlessVidXMaxThreads: 5,
		EnableNotifications:    false,
		Debug:                  false,
		ServerPort:             8080,
		Host:                   "0.0.0.0",
	}
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Try to load from database
	var settingsJSON string
	err := m.db.QueryRow("SELECT value FROM settings WHERE key = 'app_settings'").Scan(&settingsJSON)
	if err == sql.ErrNoRows {
		// No settings in DB, use defaults and save them
		return m.saveToDBLocked()
	} else if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	
	if err := json.Unmarshal([]byte(settingsJSON), m.settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	
	return nil
}

func (m *Manager) Get() *Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a copy
	settingsCopy := *m.settings
	return &settingsCopy
}

func (m *Manager) Update(newSettings *Settings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.settings = newSettings
	return m.saveToDBLocked()
}

func (m *Manager) UpdatePartial(updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Convert current settings to map
	settingsJSON, err := json.Marshal(m.settings)
	if err != nil {
		return err
	}
	
	var settingsMap map[string]interface{}
	if err := json.Unmarshal(settingsJSON, &settingsMap); err != nil {
		return err
	}
	
	// Apply updates
	for key, value := range updates {
		settingsMap[key] = value
	}
	
	// Convert back to struct
	updatedJSON, err := json.Marshal(settingsMap)
	if err != nil {
		return err
	}
	
	if err := json.Unmarshal(updatedJSON, m.settings); err != nil {
		return err
	}
	
	return m.saveToDBLocked()
}

func (m *Manager) saveToDBLocked() error {
	settingsJSON, err := json.Marshal(m.settings)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	
	_, err = m.db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES ('app_settings', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
	`, string(settingsJSON))
	
	return err
}

// Specific getters for commonly accessed settings
func (m *Manager) GetMaxResolution() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.MaxResolution
}

func (m *Manager) GetStreamProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	providers := make([]string, len(m.settings.StreamProviders))
	copy(providers, m.settings.StreamProviders)
	return providers
}

func (m *Manager) GetTMDBAPIKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.TMDBAPIKey
}

func (m *Manager) GetRealDebridAPIKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.RealDebridAPIKey
}

func (m *Manager) GetMDBListAPIKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.MDBListAPIKey
}

func (m *Manager) GetPremiumizeAPIKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.PremiumizeAPIKey
}

func (m *Manager) GetTorrentioURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.TorrentioURL == "" {
		return "https://torrentio.strem.fun"
	}
	return m.settings.TorrentioURL
}

func (m *Manager) GetCometURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.CometURL == "" {
		return "https://comet.elfhosted.com"
	}
	return m.settings.CometURL
}

func (m *Manager) GetMediaFusionURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.MediaFusionURL == "" {
		return "https://mediafusion.elfhosted.com"
	}
	return m.settings.MediaFusionURL
}

func (m *Manager) GetServerPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.ServerPort == 0 {
		return 8080
	}
	return m.settings.ServerPort
}

func (m *Manager) GetHost() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.Host == "" {
		return "0.0.0.0"
	}
	return m.settings.Host
}

func (m *Manager) GetTorrentioProviders() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.TorrentioProviders == "" {
		return "yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy,magnetdl"
	}
	return m.settings.TorrentioProviders
}

func (m *Manager) GetCometIndexers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.settings.CometIndexers) == 0 {
		return []string{"bktorrent", "thepiratebay", "yts", "eztv"}
	}
	indexers := make([]string, len(m.settings.CometIndexers))
	copy(indexers, m.settings.CometIndexers)
	return indexers
}

func (m *Manager) UseRealDebrid() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.UseRealDebrid
}

func (m *Manager) UsePremiumize() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.UsePremiumize
}

func (m *Manager) GetDiscordWebhookURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.DiscordWebhookURL
}

func (m *Manager) GetTelegramBotToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.TelegramBotToken
}

func (m *Manager) GetTelegramChatID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.TelegramChatID
}

func (m *Manager) IsNotificationsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.EnableNotifications
}

func (m *Manager) IsDebugEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.Debug
}

// GetAll returns all settings as a map
func (m *Manager) GetAll() (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]interface{}{
		"tmdb_api_key":                 m.settings.TMDBAPIKey,
		"realdebrid_token":             m.settings.RealDebridAPIKey,
		"premiumize_api_key":           m.settings.PremiumizeAPIKey,
		"mdblist_api_key":              m.settings.MDBListAPIKey,
		"use_realdebrid":               m.settings.UseRealDebrid,
		"use_premiumize":               m.settings.UsePremiumize,
		"total_pages":                  m.settings.TotalPages,
		"max_resolution":               m.settings.MaxResolution,
		"auto_cache_interval_hours":    m.settings.AutoCacheIntervalHours,
		"user_create_playlist":         m.settings.UserCreatePlaylist,
		"include_adult_vod":            m.settings.IncludeAdultVOD,
		"debug":                        m.settings.Debug,
		"language":                     m.settings.Language,
		"series_origin_country":        m.settings.SeriesOriginCountry,
		"movies_origin_country":        m.settings.MoviesOriginCountry,
		"torrentio_providers":          m.settings.TorrentioProviders,
		"torrentio_url":                m.settings.TorrentioURL,
		"comet_url":                    m.settings.CometURL,
		"mediafusion_url":              m.settings.MediaFusionURL,
		"server_port":                  m.settings.ServerPort,
		"host":                         m.settings.Host,
		"enable_notifications":         m.settings.EnableNotifications,
		"discord_webhook_url":          m.settings.DiscordWebhookURL,
		"telegram_bot_token":           m.settings.TelegramBotToken,
		"telegram_chat_id":             m.settings.TelegramChatID,
	}, nil
}

// SetAll updates all settings from a map
func (m *Manager) SetAll(updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update settings fields from map
	if v, ok := updates["tmdb_api_key"].(string); ok {
		m.settings.TMDBAPIKey = v
	}
	if v, ok := updates["realdebrid_token"].(string); ok {
		m.settings.RealDebridAPIKey = v
	}
	if v, ok := updates["premiumize_api_key"].(string); ok {
		m.settings.PremiumizeAPIKey = v
	}
	if v, ok := updates["mdblist_api_key"].(string); ok {
		m.settings.MDBListAPIKey = v
	}
	if v, ok := updates["use_realdebrid"].(bool); ok {
		m.settings.UseRealDebrid = v
	}
	if v, ok := updates["use_premiumize"].(bool); ok {
		m.settings.UsePremiumize = v
	}
	if v, ok := updates["total_pages"].(float64); ok {
		m.settings.TotalPages = int(v)
	}
	if v, ok := updates["max_resolution"].(float64); ok {
		m.settings.MaxResolution = int(v)
	}
	if v, ok := updates["auto_cache_interval_hours"].(float64); ok {
		m.settings.AutoCacheIntervalHours = int(v)
	}
	if v, ok := updates["user_create_playlist"].(bool); ok {
		m.settings.UserCreatePlaylist = v
	}
	if v, ok := updates["include_adult_vod"].(bool); ok {
		m.settings.IncludeAdultVOD = v
	}
	if v, ok := updates["debug"].(bool); ok {
		m.settings.Debug = v
	}
	if v, ok := updates["language"].(string); ok {
		m.settings.Language = v
	}
	if v, ok := updates["series_origin_country"].(string); ok {
		m.settings.SeriesOriginCountry = v
	}
	if v, ok := updates["movies_origin_country"].(string); ok {
		m.settings.MoviesOriginCountry = v
	}
	if v, ok := updates["host"].(string); ok {
		m.settings.Host = v
	}
	if v, ok := updates["torrentio_providers"].(string); ok {
		m.settings.TorrentioProviders = v
	}
	if v, ok := updates["torrentio_url"].(string); ok {
		m.settings.TorrentioURL = v
	}
	if v, ok := updates["comet_url"].(string); ok {
		m.settings.CometURL = v
	}
	if v, ok := updates["mediafusion_url"].(string); ok {
		m.settings.MediaFusionURL = v
	}
	if v, ok := updates["server_port"].(float64); ok {
		m.settings.ServerPort = int(v)
	}
	if v, ok := updates["enable_notifications"].(bool); ok {
		m.settings.EnableNotifications = v
	}
	if v, ok := updates["discord_webhook_url"].(string); ok {
		m.settings.DiscordWebhookURL = v
	}
	if v, ok := updates["telegram_bot_token"].(string); ok {
		m.settings.TelegramBotToken = v
	}
	if v, ok := updates["telegram_chat_id"].(string); ok {
		m.settings.TelegramChatID = v
	}
	
	return m.saveToDBLocked()
}
