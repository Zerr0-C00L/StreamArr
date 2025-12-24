package config

import (
	"os"
)

// StremioAddon represents a Stremio addon configuration
type StremioAddon struct {
	Name    string
	URL     string
	Enabled bool
}

type Config struct {
	// Server
	ServerPort int
	Host       string

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// API Keys
	TMDBAPIKey       string
	RealDebridAPIKey string
	PremiumizeAPIKey string
	MDBListAPIKey    string

	// Services
	CometURL string

	// Features
	EnableNotifications bool
	DiscordWebhookURL   string
	TelegramBotToken    string
	TelegramChatID      string

	// Playlist Settings
	TotalPages             int
	MinYear                int
	MinRuntime             int
	Language               string
	SeriesOriginCountry    string
	MoviesOriginCountry    string
	MaxResolution          int
	MaxFileSize            int
	AutoCacheIntervalHours int
	UserCreatePlaylist     bool
	IncludeAdultVOD        bool
	EnableQualityVariants  bool
	ShowFullStreamName     bool
	OnlyCachedStreams      bool // Only include media with cached streams

	// Provider Settings
	UseRealDebrid bool
	UsePremiumize bool
	StremioAddons []StremioAddon

	// Proxy Settings
	HTTPProxy    string
	UseHTTPProxy bool

	// HeadlessVidX Settings
	HeadlessVidXAddress    string
	HeadlessVidXMaxThreads int

	// Debug
	Debug bool
}

// Load returns initial configuration with hardcoded defaults.
// Only DATABASE_URL is read from environment variable for initial database connection.
// All other settings are loaded from the database after connection is established.
func Load() *Config {
	return &Config{
		// Server defaults
		ServerPort: 8080,
		Host:       "0.0.0.0",

		// Database URL can be set via environment for initial connection
		// After that, all settings come from the database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://streamarr:streamarr_password@localhost:5432/streamarr?sslmode=disable"),
		RedisURL:    "redis://localhost:6379/0",

		// API Keys - empty by default, set via Web UI
		TMDBAPIKey:       "",
		RealDebridAPIKey: "",
		PremiumizeAPIKey: "",
		MDBListAPIKey:    "",

		// Notifications - disabled by default
		EnableNotifications: false,
		DiscordWebhookURL:   "",
		TelegramBotToken:    "",
		TelegramChatID:      "",

		// Playlist defaults
		TotalPages:             5,
		MinYear:                1970,
		MinRuntime:             30,
		Language:               "en-US",
		SeriesOriginCountry:    "US",
		MoviesOriginCountry:    "US",
		MaxResolution:          2160,
		MaxFileSize:            50000,
		AutoCacheIntervalHours: 6,
		UserCreatePlaylist:     true,
		IncludeAdultVOD:        false,
		EnableQualityVariants:  false,
		ShowFullStreamName:     false,
		OnlyCachedStreams:      false,

		// Provider defaults
		UseRealDebrid: true,
		UsePremiumize: false,
		StremioAddons: []StremioAddon{}, // Empty by default - users must configure their own addons

		// Proxy - disabled by default
		HTTPProxy:    "",
		UseHTTPProxy: false,

		// HeadlessVidX defaults
		HeadlessVidXAddress:    "localhost:3202",
		HeadlessVidXMaxThreads: 5,

		// Debug - disabled by default
		Debug: false,
	}
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
