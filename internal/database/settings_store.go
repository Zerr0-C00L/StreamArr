package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

// Get retrieves a single setting by key
func (s *SettingsStore) Get(ctx context.Context, key string) (*models.Settings, error) {
	query := `SELECT id, key, value, type, updated_at FROM settings WHERE key = $1`

	setting := &models.Settings{}
	err := s.db.QueryRowContext(ctx, query, key).Scan(
		&setting.ID,
		&setting.Key,
		&setting.Value,
		&setting.Type,
		&setting.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return setting, err
}

// Set updates or inserts a setting
func (s *SettingsStore) Set(ctx context.Context, key, value, typ string) error {
	query := `
		INSERT INTO settings (key, value, type, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) 
		DO UPDATE SET value = $2, type = $3, updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query, key, value, typ)
	return err
}

// GetAll retrieves all settings as a SettingsResponse
func (s *SettingsStore) GetAll(ctx context.Context) (*models.SettingsResponse, error) {
	query := `SELECT key, value, type FROM settings`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value, typ string
		if err := rows.Scan(&key, &value, &typ); err != nil {
			return nil, err
		}
		settings[key] = value
	}

	return s.mapToResponse(settings), nil
}

// SetAll updates multiple settings at once
func (s *SettingsStore) SetAll(ctx context.Context, response *models.SettingsResponse) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	settingsMap := s.responseToMap(response)

	query := `
		INSERT INTO settings (key, value, type, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) 
		DO UPDATE SET value = $2, type = $3, updated_at = NOW()
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for key, value := range settingsMap {
		typ := "string"
		if _, err := strconv.Atoi(value); err == nil {
			typ = "int"
		} else if value == "true" || value == "false" {
			typ = "bool"
		}

		if _, err := stmt.ExecContext(ctx, key, value, typ); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Helper functions to convert between map and struct
func (s *SettingsStore) mapToResponse(m map[string]string) *models.SettingsResponse {
	getBool := func(key string) bool {
		return m[key] == "true"
	}
	getInt := func(key string, def int) int {
		if val, err := strconv.Atoi(m[key]); err == nil {
			return val
		}
		return def
	}
	getString := func(key string) string {
		return m[key]
	}
	getStringWithDefault := func(key, defaultVal string) string {
		if val, ok := m[key]; ok && val != "" {
			return val
		}
		return defaultVal
	}

	return &models.SettingsResponse{
		TMDBAPIKey:                  getString("tmdb_api_key"),
		RealDebridToken:             getString("realdebrid_token"),
		PremiumizeAPIKey:            getString("premiumize_api_key"),
		MDBListAPIKey:               getString("mdblist_api_key"),
		UserCreatePlaylist:          getBool("user_create_playlist"),
		TotalPages:                  getInt("total_pages", 5),
		Language:                    getString("language"),
		MoviesOriginCountry:         getString("movies_origin_country"),
		SeriesOriginCountry:         getString("series_origin_country"),
		M3U8Limit:                   getInt("m3u8_limit", 0),
		IncludeLiveTV:               getBool("include_live_tv"),
		IncludeAdultVOD:             getBool("include_adult_vod"),
		OnlyReleasedContent:         getBool("only_released_content"),
		HideUnavailableContent:      getBool("hide_unavailable_content"),
		EnableQualityVariants:       getBool("enable_quality_variants"),
		ShowFullStreamName:          getBool("show_full_stream_name"),
		UseRealDebrid:               getBool("use_realdebrid"),
		UsePremiumize:               getBool("use_premiumize"),
		MediaFusionEnabled:          getBool("mediafusion_enabled"),
		TorrentioProviders:          getString("torrentio_providers"),
		IncludePopularMovies:        getBool("include_popular_movies"),
		IncludeTopRatedMovies:       getBool("include_top_rated_movies"),
		IncludeNowPlaying:           getBool("include_now_playing"),
		IncludeUpcoming:             getBool("include_upcoming"),
		IncludeLatestReleasesMovies: getBool("include_latest_releases_movies"),
		IncludeCollections:          getBool("include_collections"),
		IncludePopularSeries:        getBool("include_popular_series"),
		IncludeTopRatedSeries:       getBool("include_top_rated_series"),
		IncludeAiringToday:          getBool("include_airing_today"),
		IncludeOnTheAir:             getBool("include_on_the_air"),
		IncludeLatestReleasesSeries: getBool("include_latest_releases_series"),
		UserSetHost:                 getString("user_set_host"),
		ExpirationHours:             getInt("expiration_hours", 3),
		AutoCacheIntervalHours:      getInt("auto_cache_interval_hours", 6),
		Timeout:                     getInt("timeout", 20),
		UseGithubForCache:           getBool("use_github_for_cache"),
		Debug:                       getBool("debug"),
		XtreamUsername:              getStringWithDefault("xtream_username", "streamarr"),
		XtreamPassword:              getStringWithDefault("xtream_password", "streamarr"),
	}
}

func (s *SettingsStore) responseToMap(r *models.SettingsResponse) map[string]string {
	return map[string]string{
		"tmdb_api_key":                   r.TMDBAPIKey,
		"realdebrid_token":               r.RealDebridToken,
		"premiumize_api_key":             r.PremiumizeAPIKey,
		"mdblist_api_key":                r.MDBListAPIKey,
		"user_create_playlist":           fmt.Sprintf("%t", r.UserCreatePlaylist),
		"total_pages":                    fmt.Sprintf("%d", r.TotalPages),
		"language":                       r.Language,
		"movies_origin_country":          r.MoviesOriginCountry,
		"series_origin_country":          r.SeriesOriginCountry,
		"m3u8_limit":                     fmt.Sprintf("%d", r.M3U8Limit),
		"include_live_tv":                fmt.Sprintf("%t", r.IncludeLiveTV),
		"include_adult_vod":              fmt.Sprintf("%t", r.IncludeAdultVOD),
		"only_released_content":          fmt.Sprintf("%t", r.OnlyReleasedContent),
		"hide_unavailable_content":       fmt.Sprintf("%t", r.HideUnavailableContent),
		"enable_quality_variants":        fmt.Sprintf("%t", r.EnableQualityVariants),
		"show_full_stream_name":          fmt.Sprintf("%t", r.ShowFullStreamName),
		"use_realdebrid":                 fmt.Sprintf("%t", r.UseRealDebrid),
		"use_premiumize":                 fmt.Sprintf("%t", r.UsePremiumize),
		"mediafusion_enabled":            fmt.Sprintf("%t", r.MediaFusionEnabled),
		"torrentio_providers":            r.TorrentioProviders,
		"include_popular_movies":         fmt.Sprintf("%t", r.IncludePopularMovies),
		"include_top_rated_movies":       fmt.Sprintf("%t", r.IncludeTopRatedMovies),
		"include_now_playing":            fmt.Sprintf("%t", r.IncludeNowPlaying),
		"include_upcoming":               fmt.Sprintf("%t", r.IncludeUpcoming),
		"include_latest_releases_movies": fmt.Sprintf("%t", r.IncludeLatestReleasesMovies),
		"include_collections":            fmt.Sprintf("%t", r.IncludeCollections),
		"include_popular_series":         fmt.Sprintf("%t", r.IncludePopularSeries),
		"include_top_rated_series":       fmt.Sprintf("%t", r.IncludeTopRatedSeries),
		"include_airing_today":           fmt.Sprintf("%t", r.IncludeAiringToday),
		"include_on_the_air":             fmt.Sprintf("%t", r.IncludeOnTheAir),
		"include_latest_releases_series": fmt.Sprintf("%t", r.IncludeLatestReleasesSeries),
		"user_set_host":                  r.UserSetHost,
		"expiration_hours":               fmt.Sprintf("%d", r.ExpirationHours),
		"auto_cache_interval_hours":      fmt.Sprintf("%d", r.AutoCacheIntervalHours),
		"timeout":                        fmt.Sprintf("%d", r.Timeout),
		"use_github_for_cache":           fmt.Sprintf("%t", r.UseGithubForCache),
		"debug":                          fmt.Sprintf("%t", r.Debug),
		"xtream_username":                r.XtreamUsername,
		"xtream_password":                r.XtreamPassword,
	}
}
