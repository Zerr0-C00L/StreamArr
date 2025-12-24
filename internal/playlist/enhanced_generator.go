package playlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/config"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/providers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
)

// EnhancedGenerator with quality variants and full feature support
type EnhancedGenerator struct {
	cfg             *config.Config
	db              *sql.DB
	tmdb            *services.TMDBClient
	multiProvider   *providers.MultiProvider
	enableVariants  bool
	qualityVariants []string
	streamCache     map[string][]StreamInfo // IMDB ID -> streams
}

type StreamInfo struct {
	Hash     string
	Title    string
	Quality  string
	Size     int64
	FileIdx  int
	FileName string
	Cached   bool
}

type MoviePlaylistEntry struct {
	Num                int     `json:"num"`
	Name               string  `json:"name"`
	StreamType         string  `json:"stream_type"`
	StreamID           int64   `json:"stream_id"`
	StreamIcon         string  `json:"stream_icon"`
	Rating             float64 `json:"rating"`
	Rating5Based       float64 `json:"rating_5based"`
	Added              int64   `json:"added"`
	CategoryID         string  `json:"category_id"`
	ContainerExtension string  `json:"container_extension"`
	CustomSID          string  `json:"custom_sid,omitempty"`
	DirectSource       string  `json:"direct_source"`
	Plot               string  `json:"plot"`
	BackdropPath       string  `json:"backdrop_path,omitempty"`
	Group              string  `json:"group"`
	Quality            string  `json:"quality,omitempty"`
	Year               int     `json:"year,omitempty"`
}

type SeriesEntry struct {
	Num                int              `json:"num"`
	Name               string           `json:"name"`
	SeriesID           int64            `json:"series_id"`
	Cover              string           `json:"cover"`
	Plot               string           `json:"plot"`
	Cast               string           `json:"cast"`
	Director           string           `json:"director"`
	Genre              string           `json:"genre"`
	ReleaseDate        string           `json:"releaseDate"`
	Rating             float64          `json:"rating"`
	Rating5Based       float64          `json:"rating_5based"`
	BackdropPath       []string         `json:"backdrop_path"`
	YouTubeTrailer     string           `json:"youtube_trailer"`
	EpisodeRunTime     string           `json:"episode_run_time"`
	CategoryID         string           `json:"category_id"`
	Seasons            []SeasonInfo     `json:"seasons"`
	Episodes           map[string][]Episode `json:"episodes"`
}

type SeasonInfo struct {
	AirDate      string `json:"air_date"`
	EpisodeCount int    `json:"episode_count"`
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	SeasonNumber int    `json:"season_number"`
	Cover        string `json:"cover"`
}

type Episode struct {
	ID             string  `json:"id"`
	EpisodeNum     int     `json:"episode_num"`
	Title          string  `json:"title"`
	ContainerExtension string `json:"container_extension"`
	Info           EpisodeInfo `json:"info"`
	CustomSID      string  `json:"custom_sid"`
	Added          string  `json:"added"`
	Season         int     `json:"season"`
	DirectSource   string  `json:"direct_source"`
}

type EpisodeInfo struct {
	ReleaseDate  string  `json:"releasedate"`
	Plot         string  `json:"plot"`
	Duration     string  `json:"duration"`
	Video        Video   `json:"video"`
	Audio        Audio   `json:"audio"`
	Rating       float64 `json:"rating"`
	Name         string  `json:"name"`
	EpisodeNum   int     `json:"episode_num"`
	Season       int     `json:"season"`
	Cover        string  `json:"cover_big"`
}

type Video struct {
	Aspect  string `json:"aspect"`
	Codec   string `json:"codec"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type Audio struct {
	Channels int    `json:"channels"`
	Codec    string `json:"codec"`
}

func NewEnhancedGenerator(cfg *config.Config, db *sql.DB, tmdb *services.TMDBClient, mp *providers.MultiProvider) *EnhancedGenerator {
	enableVariants := cfg.EnableQualityVariants
	qualityVariants := []string{"4k", "1080p", "720p"}
	
	return &EnhancedGenerator{
		cfg:             cfg,
		db:              db,
		tmdb:            tmdb,
		multiProvider:   mp,
		enableVariants:  enableVariants,
		qualityVariants: qualityVariants,
		streamCache:     make(map[string][]StreamInfo),
	}
}

// GenerateComplete generates all playlists (movies + TV + live)
func (eg *EnhancedGenerator) GenerateComplete(ctx context.Context) error {
	log.Println("🎬 Starting complete playlist generation...")
	
	// Generate movie playlist
	movieEntries, err := eg.GenerateMoviePlaylistEnhanced(ctx)
	if err != nil {
		log.Printf("Warning: Movie playlist generation error: %v", err)
	} else {
		if err := eg.SaveMoviePlaylist(movieEntries); err != nil {
			log.Printf("Error saving movie playlist: %v", err)
		} else {
			log.Printf("✅ Saved %d movie entries", len(movieEntries))
		}
	}
	
	// Generate TV series playlist
	seriesEntries, err := eg.GenerateTVPlaylistEnhanced(ctx)
	if err != nil {
		log.Printf("Warning: TV playlist generation error: %v", err)
	} else {
		if err := eg.SaveTVPlaylist(seriesEntries); err != nil {
			log.Printf("Error saving TV playlist: %v", err)
		} else {
			log.Printf("✅ Saved %d series entries", len(seriesEntries))
		}
	}
	
	log.Println("🎉 Playlist generation complete!")
	return nil
}

// hasCachedStream checks if a movie has a cached stream in the Stream Cache Monitor
// It queries using TMDB ID and joins with library_movies to find the database ID
func (eg *EnhancedGenerator) hasCachedStream(ctx context.Context, tmdbID int) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM media_streams ms
		JOIN library_movies m ON m.id = ms.movie_id
		WHERE m.tmdb_id = $1
	)`
	log.Printf("DEBUG: Executing query: %s with tmdbID=%d", query, tmdbID)
	var exists bool
	err := eg.db.QueryRowContext(ctx, query, tmdbID).Scan(&exists)
	if err != nil {
		log.Printf("DEBUG: Query error: %v", err)
		return false, err
	}
	log.Printf("DEBUG: Query result for tmdbID=%d: exists=%v", tmdbID, exists)
	return exists, nil
}

func (eg *EnhancedGenerator) GenerateMoviePlaylistEnhanced(ctx context.Context) ([]MoviePlaylistEntry, error) {
	log.Println("Generating enhanced movie playlist with quality variants...")
	log.Printf("DEBUG: OnlyCachedStreams setting: %v", eg.cfg.OnlyCachedStreams)
	
	entries := []MoviePlaylistEntry{}
	addedIDs := make(map[int]bool)
	num := 0
	skippedCount := 0
	
	// Fetch Popular Movies
	log.Println("Fetching popular movies...")
	for page := 1; page <= min(eg.cfg.TotalPages, 15); page++ {
		movies, err := eg.tmdb.DiscoverMovies(ctx, page, nil, nil)
		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			continue
		}
		
		for _, movie := range movies {
			log.Printf("DEBUG: Processing movie: %s (TMDB ID: %d, movie.ID: %d)", movie.Title, movie.TMDBID, movie.ID)
			
			if addedIDs[movie.TMDBID] {
				log.Printf("DEBUG: Movie %s already added, skipping", movie.Title)
				skippedCount++
				continue
			}
			if movie.ReleaseDate == nil || movie.ReleaseDate.Year() < eg.cfg.MinYear {
				log.Printf("DEBUG: Movie %s has no release date or year < %d, skipping", movie.Title, eg.cfg.MinYear)
				skippedCount++
				continue
			}
			
			// Added stricter filtering for unreleased movies
			if movie.ReleaseDate.After(time.Now()) {
				log.Printf("DEBUG: Skipping unreleased movie: %s (Release Date: %v)", movie.Title, movie.ReleaseDate)
				skippedCount++
				continue
			}
			
			// Check if movie has cached stream (if OnlyCachedStreams is enabled)
			if eg.cfg.OnlyCachedStreams {
				imdbID, ok := movie.Metadata["imdb_id"].(string)
				if !ok || imdbID == "" {
					log.Printf("DEBUG: Movie %s (TMDB %d) has no IMDB ID in metadata", movie.Title, movie.TMDBID)
					skippedCount++
					continue
				}
				log.Printf("DEBUG: Checking cache for movie %s (TMDB %d, IMDB %s)", movie.Title, movie.TMDBID, imdbID)
				
				// NOTE: movie.ID from TMDB API is actually the TMDB ID, not database ID
				// We need to use TMDBID which is the actual TMDB ID
				hasCachedStream, err := eg.hasCachedStream(ctx, movie.TMDBID)
				log.Printf("DEBUG: hasCachedStream result: exists=%v, err=%v (using TMDBID=%d)", hasCachedStream, err, movie.TMDBID)
				if err != nil || !hasCachedStream {
					log.Printf("DEBUG: Skipping %s - no cached streams found", movie.Title)
					skippedCount++
					continue
				}
			}
			
			movieEntries := eg.createMovieEntries(*movie, "Popular Movies", &num)
			entries = append(entries, movieEntries...)
			addedIDs[movie.TMDBID] = true
		}
	}
	
	// Fetch movies by genres
	log.Println("Fetching movies by genre...")
	genres := []struct{ id int; name string }{
		{28, "Action"}, {12, "Adventure"}, {16, "Animation"}, {35, "Comedy"},
		{80, "Crime"}, {99, "Documentary"}, {18, "Drama"}, {10751, "Family"},
		{14, "Fantasy"}, {36, "History"}, {27, "Horror"}, {10402, "Music"},
		{9648, "Mystery"}, {10749, "Romance"}, {878, "Science Fiction"},
		{10770, "TV Movie"}, {53, "Thriller"}, {10752, "War"}, {37, "Western"},
	}
	
	for _, genre := range genres {
		for page := 1; page <= min(eg.cfg.TotalPages, 3); page++ {
			movies, err := eg.tmdb.DiscoverMovies(ctx, page, &genre.id, nil)
			if err != nil {
				continue
			}
			
			for _, movie := range movies {
				if addedIDs[movie.TMDBID] {
					continue
				}
				if movie.ReleaseDate == nil || movie.ReleaseDate.Year() < eg.cfg.MinYear {
					continue
				}
				
				// Check if movie has cached stream (if OnlyCachedStreams is enabled)
				if eg.cfg.OnlyCachedStreams {
					imdbID, ok := movie.Metadata["imdb_id"].(string)
					if !ok || imdbID == "" {
						continue
					}
					hasCachedStream, err := eg.hasCachedStream(ctx, int(movie.ID))
					if err != nil || !hasCachedStream {
						continue
					}
				}
				
				movieEntries := eg.createMovieEntries(*movie, genre.name, &num)
				entries = append(entries, movieEntries...)
				addedIDs[movie.TMDBID] = true
			}
		}
	}
	
	log.Printf("DEBUG: Final stats - Total entries generated: %d, Total movies processed: %d, Skipped: %d", len(entries), len(addedIDs), skippedCount)
	log.Printf("Generated %d total movie entries (with variants)", len(entries))
	return entries, nil
}

func (eg *EnhancedGenerator) createMovieEntries(movie models.Movie, group string, num *int) []MoviePlaylistEntry {
	entries := []MoviePlaylistEntry{}
	
	year := 0
	timestamp := time.Now().Unix()
	if movie.ReleaseDate != nil {
		year = movie.ReleaseDate.Year()
		timestamp = movie.ReleaseDate.Unix()
	}
	
	posterPath := movie.PosterPath
	if posterPath != "" && posterPath[0] != 'h' {
		posterPath = "https://image.tmdb.org/t/p/w500" + posterPath
	}
	
	backdropPath := movie.BackdropPath
	if backdropPath != "" && backdropPath[0] != 'h' {
		backdropPath = "https://image.tmdb.org/t/p/original" + backdropPath
	}
	
	baseURL := fmt.Sprintf("http://localhost:%d/api/v1/movies/%d/play", eg.cfg.ServerPort, movie.TMDBID)
	
	if eg.enableVariants {
		// Generate entry for each quality variant
		for idx, quality := range eg.qualityVariants {
			*num++
			qualityLabel := strings.ToUpper(quality)
			streamID := int64(movie.TMDBID*10 + idx)
			
			entry := MoviePlaylistEntry{
				Num:                *num,
				Name:               fmt.Sprintf("%s (%d) [%s]", movie.Title, year, qualityLabel),
				StreamType:         "movie",
				StreamID:           streamID,
				StreamIcon:         posterPath,
				Rating:             7.0, // Default rating
				Rating5Based:       3.5,
				Added:              timestamp,
				CategoryID:         "999992",
				ContainerExtension: "mp4",
				DirectSource:       fmt.Sprintf("%s?quality=%s", baseURL, quality),
				Plot:               movie.Overview,
				BackdropPath:       backdropPath,
				Group:              group,
				Quality:            quality,
				Year:               year,
			}
			entries = append(entries, entry)
		}
	} else {
		// Single entry
		*num++
		entry := MoviePlaylistEntry{
			Num:                *num,
			Name:               fmt.Sprintf("%s (%d)", movie.Title, year),
			StreamType:         "movie",
			StreamID:           int64(movie.TMDBID),
			StreamIcon:         posterPath,
			Rating:             7.0, // Default rating
			Rating5Based:       3.5,
			Added:              timestamp,
			CategoryID:         "999992",
			ContainerExtension: "mp4",
			DirectSource:       baseURL,
			Plot:               movie.Overview,
			BackdropPath:       backdropPath,
			Group:              group,
			Year:               year,
		}
		entries = append(entries, entry)
	}
	
	return entries
}

// GenerateTVPlaylistEnhanced generates TV series with all seasons/episodes
func (eg *EnhancedGenerator) GenerateTVPlaylistEnhanced(ctx context.Context) ([]SeriesEntry, error) {
	log.Println("Generating enhanced TV series playlist...")
	
	entries := []SeriesEntry{}
	// addedIDs := make(map[int]bool)  // TODO: Use when implementing series discovery
	// num := 0  // TODO: Use when implementing series discovery
	
	// Fetch popular series (placeholder - needs TMDB series discovery)
	// For now, return empty list with log message
	log.Println("TV series discovery requires additional TMDB client methods")
	log.Println("This will be populated when series endpoints are added")
	
	return entries, nil
}

// SaveMoviePlaylist saves to JSON and M3U8 formats
func (eg *EnhancedGenerator) SaveMoviePlaylist(entries []MoviePlaylistEntry) error {
	// Save JSON
	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	if err := os.WriteFile("playlist.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	
	// Generate M3U8
	m3u8Content := "#EXTM3U\n"
	for _, entry := range entries {
		m3u8Content += fmt.Sprintf("#EXTINF:-1 group-title=\"%s\" tvg-id=\"%s\" tvg-logo=\"%s\",%s\n%s\n\n",
			entry.Group, entry.Name, entry.StreamIcon, entry.Name, entry.DirectSource)
	}
	
	if err := os.WriteFile("playlist.m3u8", []byte(m3u8Content), 0644); err != nil {
		return fmt.Errorf("failed to write M3U8: %w", err)
	}
	
	return nil
}

// SaveTVPlaylist saves TV series to JSON
func (eg *EnhancedGenerator) SaveTVPlaylist(entries []SeriesEntry) error {
	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	if err := os.WriteFile("tv_playlist.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}
	
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
