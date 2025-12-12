package models

import "time"

// Metadata is a generic map for storing flexible data
type Metadata map[string]interface{}

type Movie struct {
	ID             int64       `json:"id"`
	TMDBID         int         `json:"tmdb_id"`
	Title          string      `json:"title"`
	OriginalTitle  string      `json:"original_title"`
	Overview       string      `json:"overview"`
	PosterPath     string      `json:"poster_path"`
	BackdropPath   string      `json:"backdrop_path"`
	ReleaseDate    *time.Time  `json:"release_date,omitempty"`
	Runtime        int         `json:"runtime"`
	Genres         []string    `json:"genres"`
	Metadata       Metadata    `json:"metadata"`
	Monitored      bool        `json:"monitored"`
	Available      bool        `json:"available"`
	QualityProfile string      `json:"quality_profile"`
	SearchStatus   string      `json:"search_status"`
	LastChecked    *time.Time  `json:"last_checked,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	AddedAt        time.Time   `json:"added_at"`
	// Collection fields
	CollectionID   *int64      `json:"collection_id,omitempty"`
	Collection     *Collection `json:"collection,omitempty"`
}

// Collection represents a movie collection/franchise (e.g., "The Dark Knight Trilogy")
type Collection struct {
	ID           int64     `json:"id"`
	TMDBID       int       `json:"tmdb_id"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	BackdropPath string    `json:"backdrop_path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Computed fields (not stored in DB)
	TotalMovies    int `json:"total_movies,omitempty"`
	MoviesInLibrary int `json:"movies_in_library,omitempty"`
}

type Series struct {
	ID             int64      `json:"id"`
	TMDBID         int        `json:"tmdb_id"`
	Title          string     `json:"title"`
	OriginalTitle  string     `json:"original_title"`
	Overview       string     `json:"overview"`
	PosterPath     string     `json:"poster_path"`
	BackdropPath   string     `json:"backdrop_path"`
	FirstAirDate   *time.Time `json:"first_air_date,omitempty"`
	Status         string     `json:"status"`
	Seasons        int        `json:"seasons"`
	TotalEpisodes  int        `json:"total_episodes"`
	Genres         []string   `json:"genres"`
	Metadata       Metadata   `json:"metadata"`
	Monitored      bool       `json:"monitored"`
	QualityProfile string     `json:"quality_profile"`
	SearchStatus   string     `json:"search_status"`
	LastChecked    *time.Time `json:"last_checked,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	AddedAt        time.Time  `json:"added_at"`
}

type Episode struct {
	ID            int64      `json:"id"`
	SeriesID      int64      `json:"series_id"`
	TMDBID        int        `json:"tmdb_id"`
	SeasonNumber  int        `json:"season_number"`
	EpisodeNumber int        `json:"episode_number"`
	Title         string     `json:"title"`
	Overview      string     `json:"overview"`
	AirDate       *time.Time `json:"air_date,omitempty"`
	StillPath     string     `json:"still_path"`
	Runtime       int        `json:"runtime"`
	Metadata      Metadata   `json:"metadata"`
	Monitored     bool       `json:"monitored"`
	Available     bool       `json:"available"`
	StreamURL     *string    `json:"stream_url,omitempty"`
	LastChecked   *time.Time `json:"last_checked,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Stream struct {
	ID          int64     `json:"id"`
	ContentType string    `json:"content_type"`
	ContentID   int64     `json:"content_id"`
	InfoHash    string    `json:"info_hash"`
	Title       string    `json:"title"`
	SizeBytes   int64     `json:"size_bytes"`
	Quality     string    `json:"quality"`
	Codec       string    `json:"codec"`
	Source      string    `json:"source"`
	Seeders     int       `json:"seeders"`
	Tracker     string    `json:"tracker"`
	Metadata    Metadata  `json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WatchHistory struct {
	ID              int64     `json:"id"`
	MovieID         *int64    `json:"movie_id,omitempty"`
	EpisodeID       *int64    `json:"episode_id,omitempty"`
	StreamID        *int64    `json:"stream_id,omitempty"`
	WatchedAt       time.Time `json:"watched_at"`
	ProgressSeconds int       `json:"progress_seconds"`
	Completed       bool      `json:"completed"`
}

type ActivityLog struct {
	ID          int64     `json:"id"`
	EventType   string    `json:"event_type"`
	ContentType string    `json:"content_type"`
	ContentID   int64     `json:"content_id"`
	Message     string    `json:"message"`
	Data        string    `json:"data"`
	CreatedAt   time.Time `json:"created_at"`
}

type QualityProfile struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Qualities []string `json:"qualities"`
	Cutoff    string   `json:"cutoff"`
}
