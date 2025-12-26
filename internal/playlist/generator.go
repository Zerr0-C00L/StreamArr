package playlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/Zerr0-C00L/StreamArr/internal/config"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
)

type PlaylistGenerator struct {
	cfg  *config.Config
	db   *sql.DB
	tmdb *services.TMDBClient
}

type SimplePlaylistEntry struct {
	StreamID    int64  `json:"stream_id"`
	Name        string `json:"name"`
	StreamType  string `json:"stream_type"`
	StreamIcon  string `json:"stream_icon"`
	Rating      float64 `json:"rating"`
	Year        int    `json:"year"`
	CategoryID  string `json:"category_id"`
	ContainerExtension string `json:"container_extension"`
	CustomSID   string `json:"custom_sid,omitempty"`
	DirectSource string `json:"direct_source,omitempty"`
}

func NewPlaylistGenerator(cfg *config.Config, db *sql.DB, tmdb *services.TMDBClient) *PlaylistGenerator {
	return &PlaylistGenerator{
		cfg:  cfg,
		db:   db,
		tmdb: tmdb,
	}
}

func (pg *PlaylistGenerator) GenerateMoviePlaylist(ctx context.Context) ([]SimplePlaylistEntry, error) {
	log.Println("Generating movie playlist...")
	
	if !pg.cfg.UserCreatePlaylist {
		log.Println("User playlist creation disabled, using cached GitHub playlist")
		return nil, fmt.Errorf("GitHub playlist loading not implemented")
	}
	
	entries := make([]SimplePlaylistEntry, 0)
	
	// Fetch movies by discovery (popular, now playing are handled by TMDB's discover)
	// Discover by popularity (descending)
	for page := 1; page <= pg.cfg.TotalPages; page++ {
		movies, err := pg.tmdb.DiscoverMovies(ctx, page, nil, nil)
		if err != nil {
			log.Printf("Error fetching movies page %d: %v", page, err)
			continue
		}
		
		for _, movie := range movies {
			if movie.ReleaseDate != nil && movie.ReleaseDate.Year() >= pg.cfg.MinYear {
				// Bollywood filter based on original language (discover results)
				if pg.cfg.BlockBollywood {
					if lang, ok := movie.Metadata["original_language"].(string); ok && strings.EqualFold(lang, "hi") {
						continue
					}
				}
				// Check if movie has cached stream (if OnlyCachedStreams is enabled)
				if pg.cfg.OnlyCachedStreams {
					hasCachedStream, err := pg.hasCachedStream(ctx, movie.TMDBID)
					if err != nil || !hasCachedStream {
						continue
					}
				}
				
				entry := pg.movieToEntry(*movie, "2", "Popular Movies")
				entries = append(entries, entry)
			}
		}
	}
	
	log.Printf("Generated %d movie entries", len(entries))
	return entries, nil
}

// hasCachedStream checks if a movie has a cached stream in the Stream Cache Monitor
// It queries using TMDB ID and joins with library_movies to find the database ID
func (pg *PlaylistGenerator) hasCachedStream(ctx context.Context, tmdbID int) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM media_streams ms
		JOIN library_movies m ON m.id = ms.movie_id
		WHERE m.tmdb_id = $1
	)`
	var exists bool
	err := pg.db.QueryRowContext(ctx, query, tmdbID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (pg *PlaylistGenerator) GenerateTVPlaylist(ctx context.Context) ([]SimplePlaylistEntry, error) {
	log.Println("Generating TV series playlist...")
	
	entries := make([]SimplePlaylistEntry, 0)
	
	// Note: Series discovery would need to be added to TMDB client
	// For now, this is a placeholder that returns empty list
	log.Println("TV series generation requires additional TMDB client methods")
	
	return entries, nil
}

func (pg *PlaylistGenerator) movieToEntry(movie models.Movie, categoryID, categoryName string) SimplePlaylistEntry {
	year := 0
	if movie.ReleaseDate != nil {
		year = movie.ReleaseDate.Year()
	}
	
	posterPath := movie.PosterPath
	if posterPath != "" && posterPath[0] != 'h' {
		posterPath = "https://image.tmdb.org/t/p/w500" + posterPath
	}
	
	return SimplePlaylistEntry{
		StreamID:    int64(movie.TMDBID),
		Name:        movie.Title,
		StreamType:  "movie",
		StreamIcon:  posterPath,
		Rating:      0, // Would need to extract from metadata
		Year:        year,
		CategoryID:  categoryID,
		ContainerExtension: "mp4",
	}
}

func (pg *PlaylistGenerator) seriesToEntry(series models.Series, categoryID, categoryName string) SimplePlaylistEntry {
	year := 0
	if series.FirstAirDate != nil {
		year = series.FirstAirDate.Year()
	}
	
	posterPath := series.PosterPath
	if posterPath != "" && posterPath[0] != 'h' {
		posterPath = "https://image.tmdb.org/t/p/w500" + posterPath
	}
	
	return SimplePlaylistEntry{
		StreamID:    int64(series.TMDBID),
		Name:        series.Title,
		StreamType:  "series",
		StreamIcon:  posterPath,
		Rating:      0, // Would need to extract from metadata
		Year:        year,
		CategoryID:  categoryID,
		ContainerExtension: "mp4",
	}
}

func (pg *PlaylistGenerator) SavePlaylistJSON(entries []SimplePlaylistEntry, filename string) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	
	// Would write to file here
	log.Printf("Would save %d bytes to %s", len(data), filename)
	return nil
}

func (pg *PlaylistGenerator) SavePlaylistM3U8(entries []SimplePlaylistEntry, filename, baseURL string) error {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("#EXTINF:-1 tvg-id=\"%d\" tvg-name=\"%s\" tvg-logo=\"%s\" group-title=\"%s\",%s\n",
			entry.StreamID, entry.Name, entry.StreamIcon, entry.CategoryID, entry.Name))
		
		if entry.StreamType == "movie" {
			sb.WriteString(fmt.Sprintf("%s/play.php?stream_id=%d\n", baseURL, entry.StreamID))
		} else {
			sb.WriteString(fmt.Sprintf("%s/play.php?series_id=%d\n", baseURL, entry.StreamID))
		}
	}
	
	log.Printf("Would save M3U8 with %d entries to %s", len(entries), filename)
	return nil
}
