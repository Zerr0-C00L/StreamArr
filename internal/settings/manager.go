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
	EPGURL  string `json:"epg_url,omitempty"`
	Enabled bool   `json:"enabled"`
}

// XtreamSource represents an Xtream Codes compatible IPTV provider
type XtreamSource struct {
	Name      string `json:"name"`
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Enabled   bool   `json:"enabled"`
}

// StremioAddon represents a custom Stremio addon for content providers
type StremioAddon struct {
	Name    string `json:"name"`    // Display name (e.g., "Torrentio", "Comet")
	URL     string `json:"url"`     // Base addon URL (e.g., "https://torrentio.strem.fun")
	Enabled bool   `json:"enabled"` // Whether this addon is active
}

// StremioCatalogConfig represents configuration for a single catalog
type StremioCatalogConfig struct {
	ID      string `json:"id"`      // Catalog ID (movies, series, etc.)
	Type    string `json:"type"`    // Content type (movie, series)
	Name    string `json:"name"`    // Display name
	Enabled bool   `json:"enabled"` // Whether this catalog is shown
}

// StremioAddonConfig represents the built-in Stremio addon settings
type StremioAddonConfig struct {
	Enabled          bool                   `json:"enabled"`           // Enable the built-in Stremio addon
	PublicServerURL  string                 `json:"public_server_url"` // Public URL for Stremio to reach (auto-detected)
	AddonName        string                 `json:"addon_name"`        // Display name in Stremio
	SharedToken      string                 `json:"shared_token"`      // Shared authentication token for addon access
	PerUserTokens    bool                   `json:"per_user_tokens"`   // Use per-user tokens instead of shared
	Catalogs         []StremioCatalogConfig `json:"catalogs"`          // Configured catalogs
	CatalogPlacement string                 `json:"catalog_placement"` // "home", "discovery", or "both"
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
	CometURL       string `json:"comet_url"`
	
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
	OnlyReleasedContent    bool   `json:"only_released_content"` // Only include movies/series released on streaming/digital/bluray
	ImportAdultVODFromGitHub bool `json:"import_adult_vod_from_github"` // Import adult VOD content from public-files repo
	
	// Live TV / M3U Sources
	M3USources            []M3USource    `json:"m3u_sources"`
	XtreamSources         []XtreamSource `json:"xtream_sources"`
	LiveTVEnabledSources  []string    `json:"livetv_enabled_sources"`   // Which sources are enabled
	LiveTVEnabledCategories []string  `json:"livetv_enabled_categories"` // Which categories are enabled
	LiveTVShowAllSources  bool        `json:"livetv_show_all_sources"`   // Show all sources by default
	LiveTVShowAllCategories bool      `json:"livetv_show_all_categories"` // Show all categories by default
	LiveTVEnablePlutoTV   bool        `json:"livetv_enable_plutotv"`     // Enable built-in Pluto TV channels
	LiveTVValidateStreams bool        `json:"livetv_validate_streams"`   // Validate stream URLs before loading channels
	
	// IPTV-Org Settings (https://github.com/iptv-org/iptv)
	IPTVOrgEnabled       bool     `json:"iptv_org_enabled"`        // Enable iptv-org.github.io channels
	IPTVOrgCountries     []string `json:"iptv_org_countries"`      // Selected country codes (e.g., ["us", "uk", "ca"])
	IPTVOrgLanguages     []string `json:"iptv_org_languages"`      // Selected language codes (e.g., ["eng", "spa", "fra"])
	IPTVOrgCategories    []string `json:"iptv_org_categories"`     // Selected categories (e.g., ["news", "sports", "movies"])
	
	// Provider Settings
	UseRealDebrid      bool            `json:"use_realdebrid"`
	UsePremiumize      bool            `json:"use_premiumize"`
	StremioAddons      []StremioAddon  `json:"stremio_addons"` // Custom Stremio addons for content providers
	
	// Built-in Stremio Addon Settings
	StremioAddon       StremioAddonConfig `json:"stremio_addon"`
	
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
	
	// Stream Availability Settings
	HideUnavailableContent bool `json:"hide_unavailable_content"` // Don't show movies/episodes with no streams
	
	// Update Settings
	UpdateBranch string `json:"update_branch"` // GitHub branch for updates: main, dev, etc.
	
	// System Settings
	Debug      bool   `json:"debug"`
	ServerPort int    `json:"server_port"`
	Host       string `json:"host"`
	
	// Xtream API Credentials (separate from web app login)
	XtreamUsername string `json:"xtream_username"`
	XtreamPassword string `json:"xtream_password"`
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
		CometURL:               "https://comet.elfhosted.com",
		
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
		ImportAdultVODFromGitHub: false,
		AutoCacheIntervalHours: 6,
		UseRealDebrid:          true,
		UsePremiumize:          false,
		StremioAddons:          []StremioAddon{}, // Empty by default - users should configure their own addons
		StremioAddon: StremioAddonConfig{
			Enabled:         true, // Enabled by default for built-in addon
			PublicServerURL: "",
			AddonName:       "StreamArr Pro",
			SharedToken:     "",
			PerUserTokens:   false,
			Catalogs: []StremioCatalogConfig{
				{ID: "streamarr_recent", Type: "movie", Name: "Recently Added", Enabled: true},
				{ID: "streamarr_recent_movies", Type: "movie", Name: "Recently Added Movies", Enabled: true},
				{ID: "streamarr_recent_series", Type: "series", Name: "Recently Added Series", Enabled: true},
				{ID: "streamarr_trending", Type: "movie", Name: "Trending Now", Enabled: true},
				{ID: "streamarr_trending_movies", Type: "movie", Name: "Trending Now Movies", Enabled: true},
				{ID: "streamarr_trending_series", Type: "series", Name: "Trending Now Series", Enabled: true},
				{ID: "streamarr_popular", Type: "movie", Name: "Popular Now", Enabled: true},
				{ID: "streamarr_popular_movies", Type: "movie", Name: "Popular Movies", Enabled: true},
				{ID: "streamarr_popular_series", Type: "series", Name: "Popular TV Shows", Enabled: true},
				{ID: "streamarr_coming_soon", Type: "movie", Name: "Coming Soon", Enabled: true},
			},
			CatalogPlacement: "both",
		},
		UseHTTPProxy:           false,
		HeadlessVidXAddress:    "localhost:3202",
		UpdateBranch:           "main",
		HeadlessVidXMaxThreads: 5,
		EnableNotifications:    false,
		Debug:                  false,
		ServerPort:             8080,
		Host:                   "0.0.0.0",
		XtreamUsername:         "streamarr",
		XtreamPassword:         "streamarr",
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
	
	// Start with defaults to ensure all fields have proper default values
	// This prevents fields missing from JSON from being left uninitialized
	m.settings = getDefaultSettings()
	
	// Unmarshal JSON over the defaults, overriding only fields present in JSON
	if err := json.Unmarshal([]byte(settingsJSON), m.settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	
	// Also load Xtream credentials from individual keys if they exist (for backward compatibility)
	var xtreamUsername, xtreamPassword string
	err = m.db.QueryRow("SELECT value FROM settings WHERE key = 'xtream_username'").Scan(&xtreamUsername)
	if err == nil && xtreamUsername != "" {
		m.settings.XtreamUsername = xtreamUsername
	}
	
	err = m.db.QueryRow("SELECT value FROM settings WHERE key = 'xtream_password'").Scan(&xtreamPassword)
	if err == nil && xtreamPassword != "" {
		m.settings.XtreamPassword = xtreamPassword
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
	
	// Save main settings as JSON
	_, err = m.db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES ('app_settings', $1, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
	`, string(settingsJSON))
	if err != nil {
		return err
	}
	
	// Also save Xtream credentials as individual keys for backward compatibility
	// (the Xtream handler reads these directly from the database)
	_, err = m.db.Exec(`
		INSERT INTO settings (key, value, type, updated_at)
		VALUES ('xtream_username', $1, 'string', NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
	`, m.settings.XtreamUsername)
	if err != nil {
		return fmt.Errorf("save xtream_username: %w", err)
	}
	
	_, err = m.db.Exec(`
		INSERT INTO settings (key, value, type, updated_at)
		VALUES ('xtream_password', $1, 'string', NOW())
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_at = NOW()
	`, m.settings.XtreamPassword)
	if err != nil {
		return fmt.Errorf("save xtream_password: %w", err)
	}
	
	return nil
}

// Specific getters for commonly accessed settings
func (m *Manager) GetMaxResolution() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.MaxResolution
}

func (m *Manager) GetStremioAddons() []StremioAddon {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	addons := make([]StremioAddon, len(m.settings.StremioAddons))
	copy(addons, m.settings.StremioAddons)
	return addons
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

func (m *Manager) GetCometURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.CometURL == "" {
		return "https://comet.elfhosted.com"
	}
	return m.settings.CometURL
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
		"stremio_addons":               m.settings.StremioAddons,
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
	if v, ok := updates["stremio_addons"].([]interface{}); ok {
		// Convert []interface{} to []StremioAddon
		addons := make([]StremioAddon, 0, len(v))
		for _, item := range v {
			if addonMap, ok := item.(map[string]interface{}); ok {
				addon := StremioAddon{}
				if name, ok := addonMap["name"].(string); ok {
					addon.Name = name
				}
				if url, ok := addonMap["url"].(string); ok {
					addon.URL = url
				}
				if enabled, ok := addonMap["enabled"].(bool); ok {
					addon.Enabled = enabled
				}
				addons = append(addons, addon)
			}
		}
		m.settings.StremioAddons = addons
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
