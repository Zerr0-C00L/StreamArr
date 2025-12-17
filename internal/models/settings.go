package models

import "time"

// Settings represents application settings stored in the database
type Settings struct {
	ID        int64     `json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Type      string    `json:"type"` // string, int, bool, json
	UpdatedAt time.Time `json:"updated_at"`
}

// SettingsResponse is the full settings object returned to the frontend
type SettingsResponse struct {
	// API Keys
	TMDBAPIKey        string `json:"tmdb_api_key"`
	RealDebridToken   string `json:"realdebrid_token"`
	PremiumizeAPIKey  string `json:"premiumize_api_key"`
	MDBListAPIKey     string `json:"mdblist_api_key"`

	// Playlist Settings
	UserCreatePlaylist   bool   `json:"user_create_playlist"`
	TotalPages           int    `json:"total_pages"`
	Language             string `json:"language"`
	MoviesOriginCountry  string `json:"movies_origin_country"`
	SeriesOriginCountry  string `json:"series_origin_country"`
	M3U8Limit            int    `json:"m3u8_limit"`
	IncludeLiveTV        bool   `json:"include_live_tv"`
	IncludeAdultVOD      bool   `json:"include_adult_vod"`
	IPTVImportMode       string `json:"iptv_import_mode"` // live_only | vod_only | both
	DuplicateVODPerProvider bool `json:"duplicate_vod_per_provider"`
	IPTVVODSyncIntervalHours int `json:"iptv_vod_sync_interval_hours"`

	// IPTV-Org Settings
	IPTVOrgEnabled      bool     `json:"iptv_org_enabled"`
	IPTVOrgCountries    []string `json:"iptv_org_countries"`
	IPTVOrgLanguages    []string `json:"iptv_org_languages"`
	IPTVOrgCategories   []string `json:"iptv_org_categories"`

	// Quality Settings
	MaxResolution          int  `json:"max_resolution"`
	MaxFileSize            int  `json:"max_file_size"`
	EnableQualityVariants  bool `json:"enable_quality_variants"`
	ShowFullStreamName     bool `json:"show_full_stream_name"`

	// Collection Settings
	AutoAddCollections     bool `json:"auto_add_collections"`

	// Providers
	UseRealDebrid       bool   `json:"use_realdebrid"`
	UsePremiumize       bool   `json:"use_premiumize"`
	MediaFusionEnabled  bool   `json:"mediafusion_enabled"`
	TorrentioProviders  string `json:"torrentio_providers"`

	// Content Sources (GitHub lists)
	IncludePopularMovies         bool `json:"include_popular_movies"`
	IncludeTopRatedMovies        bool `json:"include_top_rated_movies"`
	IncludeNowPlaying            bool `json:"include_now_playing"`
	IncludeUpcoming              bool `json:"include_upcoming"`
	IncludeLatestReleasesMovies  bool `json:"include_latest_releases_movies"`
	IncludeCollections           bool `json:"include_collections"`
	IncludePopularSeries         bool `json:"include_popular_series"`
	IncludeTopRatedSeries        bool `json:"include_top_rated_series"`
	IncludeAiringToday           bool `json:"include_airing_today"`
	IncludeOnTheAir              bool `json:"include_on_the_air"`
	IncludeLatestReleasesSeries  bool `json:"include_latest_releases_series"`

	// Filters
	EnableReleaseFilters  bool   `json:"enable_release_filters"`
	ExcludedReleaseGroups string `json:"excluded_release_groups"`
	ExcludedLanguages     string `json:"excluded_languages"`
	ExcludedQualities     string `json:"excluded_qualities"`

	// Advanced
	UserSetHost             string `json:"user_set_host"`
	ExpirationHours         int    `json:"expiration_hours"`
	AutoCacheIntervalHours  int    `json:"auto_cache_interval_hours"`
	Timeout                 int    `json:"timeout"`
	UseGithubForCache       bool   `json:"use_github_for_cache"`
	Debug                   bool   `json:"debug"`

	// Xtream API Credentials (separate from web app login)
	XtreamUsername string `json:"xtream_username"`
	XtreamPassword string `json:"xtream_password"`
}

// CalendarEntry represents a movie or episode air date
type CalendarEntry struct {
	ID          int64      `json:"id"`
	Type        string     `json:"type"` // "movie" or "episode"
	Title       string     `json:"title"`
	Date        *time.Time `json:"date"`
	PosterPath  string     `json:"poster_path,omitempty"`
	Overview    string     `json:"overview,omitempty"`
	VoteAverage float64    `json:"vote_average,omitempty"`
	Year        int        `json:"year,omitempty"`
	
	// For episodes
	SeriesID      *int64 `json:"series_id,omitempty"`
	SeriesTitle   string `json:"series_title,omitempty"`
	SeasonNumber  *int   `json:"season_number,omitempty"`
	EpisodeNumber *int   `json:"episode_number,omitempty"`
}
