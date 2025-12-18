package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/database"
	"github.com/Zerr0-C00L/StreamArr/internal/models"
	"github.com/Zerr0-C00L/StreamArr/internal/settings"
)

// BalkanVODImporter handles importing VOD content from Balkan GitHub repos
type BalkanVODImporter struct {
	movieStore  *database.MovieStore
	seriesStore *database.SeriesStore
	tmdb        *TMDBClient
	cfg         *settings.Settings
}

// BalkanMovieEntry represents a movie from baubau-content.json
type BalkanMovieEntry struct {
	ID          string                   `json:"id"`
	Type        string                   `json:"type"`
	Name        string                   `json:"name"`
	Year        int                      `json:"year"`
	Poster      string                   `json:"poster"`
	Background  string                   `json:"background"`
	Description string                   `json:"description"`
	Runtime     int                      `json:"runtime"`
	Genres      []string                 `json:"genres"`
	Category    string                   `json:"category"`
	Streams     []BalkanStream           `json:"streams"`
}

// BalkanSeriesEntry represents a series from baubau-content.json
type BalkanSeriesEntry struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Year       int            `json:"year"`
	Poster     string         `json:"poster"`
	Background string         `json:"background"`
	Genres     []string       `json:"genres"`
	Category   string         `json:"category"`
	Streams    []BalkanStream `json:"streams"`
}

// BalkanStream represents a stream URL
type BalkanStream struct {
	URL     string `json:"url"`
	Quality string `json:"quality"`
	Source  string `json:"source"`
}

// BalkanContentDatabase represents the full database structure
type BalkanContentDatabase struct {
	Movies []BalkanMovieEntry  `json:"movies"`
	Series []BalkanSeriesEntry `json:"series"`
}

const (
	balkanRepoURL = "https://raw.githubusercontent.com/Zerr0-C00L/Balkan-On-Demand/main/data/baubau-content-full-backup.json"
)

var domesticCategories = []string{
	"EX YU FILMOVI",
	"EX YU SERIJE",
	"EXYU SERIJE",
	"EXYU SERIJE KOJE SE EMITUJU",
	"KLIK PREMIJERA",
	"KLASICI",
	"FILMSKI KLASICI",
	"Bolji Zivot",
	"Bela Ladja",
	"Policajac Sa Petlovog Brda",
	"Slatke Muke",
}

// NewBalkanVODImporter creates a new Balkan VOD importer
func NewBalkanVODImporter(movieStore *database.MovieStore, seriesStore *database.SeriesStore, tmdb *TMDBClient, cfg *settings.Settings) *BalkanVODImporter {
	return &BalkanVODImporter{
		movieStore:  movieStore,
		seriesStore: seriesStore,
		tmdb:        tmdb,
		cfg:         cfg,
	}
}

// FetchBalkanCategories fetches all available categories with counts from GitHub repo
func FetchBalkanCategories() ([]CategoryWithCount, error) {
	log.Println("[BalkanVOD] Fetching categories from GitHub...")
	
	resp, err := http.Get(balkanRepoURL)
	if err != nil {
		log.Printf("[BalkanVOD] Error fetching from GitHub: %v", err)
		return nil, fmt.Errorf("fetch balkan content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[BalkanVOD] GitHub returned status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[BalkanVOD] Error reading response body: %v", err)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	log.Printf("[BalkanVOD] Downloaded %d bytes from GitHub", len(body))

	var content BalkanContentDatabase
	if err := json.Unmarshal(body, &content); err != nil {
		log.Printf("[BalkanVOD] Error parsing JSON: %v", err)
		return nil, fmt.Errorf("parse content database: %w", err)
	}

	log.Printf("[BalkanVOD] Parsed %d movies and %d series", len(content.Movies), len(content.Series))

	// Count items per category
	categoryCounts := make(map[string]int)
	
	// Count movies
	for _, movie := range content.Movies {
		if movie.Category != "" {
			categoryCounts[movie.Category]++
		}
	}
	
	// Count series
	for _, series := range content.Series {
		if series.Category != "" {
			categoryCounts[series.Category]++
		} else {
			// All series without category are domestic
			categoryCounts["EX YU SERIJE"]++
		}
	}
	
	// Convert to slice
	var categories []CategoryWithCount
	for name, count := range categoryCounts {
		categories = append(categories, CategoryWithCount{
			Name:  name,
			Count: count,
		})
	}
	
	log.Printf("[BalkanVOD] Returning %d categories", len(categories))
	return categories, nil
}

// CategoryWithCount represents a category with item count
type CategoryWithCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ImportBalkanVOD imports content from Balkan GitHub repos
func (b *BalkanVODImporter) ImportBalkanVOD(ctx context.Context) error {
	if !b.cfg.BalkanVODEnabled {
		log.Println("[BalkanVOD] Import disabled in settings")
		return nil
	}

	log.Println("[BalkanVOD] Starting import from GitHub repos...")
	
	// Fetch content database
	resp, err := http.Get(balkanRepoURL)
	if err != nil {
		return fmt.Errorf("fetch balkan content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	var content BalkanContentDatabase
	if err := json.Unmarshal(body, &content); err != nil {
		return fmt.Errorf("parse content database: %w", err)
	}

	log.Printf("[BalkanVOD] Fetched %d movies and %d series", len(content.Movies), len(content.Series))

	// Get selected categories from settings
	selectedCategories := b.cfg.BalkanVODSelectedCategories
	useAllCategories := len(selectedCategories) == 0
	
	// Import movies (filter by selected categories)
	imported := 0
	skipped := 0
	failed := 0

	for _, movie := range content.Movies {
		// Filter by category
		if !useAllCategories && !isInSelectedCategories(movie.Category, selectedCategories) {
			skipped++
			continue
		}
		
		// Also filter by domestic categories for safety
		if !isDomesticCategory(movie.Category) {
			skipped++
			continue
		}

		if err := b.importMovie(ctx, movie); err != nil {
			log.Printf("[BalkanVOD] Failed to import movie %s: %v", movie.Name, err)
			failed++
		} else {
			imported++
		}
	}

	// Import series (all series are domestic)
	for _, series := range content.Series {
		if err := b.importSeries(ctx, series); err != nil {
			log.Printf("[BalkanVOD] Failed to import series %s: %v", series.Name, err)
			failed++
		} else {
			imported++
		}
	}

	log.Printf("[BalkanVOD] Import complete: %d imported, %d skipped, %d failed", imported, skipped, failed)
	return nil
}

func (b *BalkanVODImporter) importMovie(ctx context.Context, entry BalkanMovieEntry) error {
	if len(entry.Streams) == 0 {
		return fmt.Errorf("no streams available")
	}

	// Extract TMDB ID from poster URL if available
	tmdbID := extractTMDBFromPoster(entry.Poster)
	
	// Parse release date
	var releaseDate *time.Time
	if entry.Year > 0 {
		t, _ := time.Parse("2006-01-02", fmt.Sprintf("%d-01-01", entry.Year))
		releaseDate = &t
	}
	
	// Create movie entry
	movie := &models.Movie{
		TMDBID:        tmdbID,
		Title:         entry.Name,
		OriginalTitle: entry.Name,
		Overview:      entry.Description,
		PosterPath:    entry.Poster,
		BackdropPath:  entry.Background,
		ReleaseDate:   releaseDate,
		Runtime:       entry.Runtime,
		Monitored:     true,
		Available:     true,
		QualityProfile: "1080p",
		Metadata:      models.Metadata{},
	}

	if movie.Metadata == nil {
		movie.Metadata = models.Metadata{}
	}

	movie.Metadata["source"] = "balkan_vod"
	movie.Metadata["imported_at"] = time.Now().Format(time.RFC3339)
	movie.Metadata["category"] = entry.Category
	
	// Add streams
	streams := make([]map[string]interface{}, len(entry.Streams))
	for i, stream := range entry.Streams {
		streams[i] = map[string]interface{}{
			"name":    "Balkan VOD",
			"url":     stream.URL,
			"quality": stream.Quality,
		}
	}
	movie.Metadata["balkan_vod_streams"] = streams

	// Try to add movie (will skip if already exists)
	if existing, err := b.movieStore.GetByTMDBID(ctx, tmdbID); err == nil && existing != nil {
		// Movie already exists, update metadata
		existing.Metadata = mergeMetadata(existing.Metadata, movie.Metadata)
		return b.movieStore.Update(ctx, existing)
	}

	return b.movieStore.Add(ctx, movie)
}

func (b *BalkanVODImporter) importSeries(ctx context.Context, entry BalkanSeriesEntry) error {
	if len(entry.Streams) == 0 {
		return fmt.Errorf("no streams available")
	}

	// Extract TMDB ID from poster URL if available
	tmdbID := extractTMDBFromPoster(entry.Poster)
	
	// Parse first air date
	var firstAirDate *time.Time
	if entry.Year > 0 {
		t, _ := time.Parse("2006-01-02", fmt.Sprintf("%d-01-01", entry.Year))
		firstAirDate = &t
	}
	
	// Create series entry
	series := &models.Series{
		TMDBID:        tmdbID,
		Title:         entry.Name,
		OriginalTitle: entry.Name,
		PosterPath:    entry.Poster,
		BackdropPath:  entry.Background,
		FirstAirDate:  firstAirDate,
		Monitored:     true,
		QualityProfile: "1080p",
		Metadata:      models.Metadata{},
	}

	if series.Metadata == nil {
		series.Metadata = models.Metadata{}
	}

	series.Metadata["source"] = "balkan_vod"
	series.Metadata["imported_at"] = time.Now().Format(time.RFC3339)
	series.Metadata["category"] = entry.Category
	
	// Add streams
	streams := make([]map[string]interface{}, len(entry.Streams))
	for i, stream := range entry.Streams {
		streams[i] = map[string]interface{}{
			"name":    "Balkan VOD",
			"url":     stream.URL,
			"quality": stream.Quality,
		}
	}
	series.Metadata["balkan_vod_streams"] = streams

	// Try to add series (will skip if already exists)
	if existing, err := b.seriesStore.GetByTMDBID(ctx, tmdbID); err == nil && existing != nil {
		// Series already exists, update metadata
		existing.Metadata = mergeMetadata(existing.Metadata, series.Metadata)
		return b.seriesStore.Update(ctx, existing)
	}

	return b.seriesStore.Add(ctx, series)
}

func isDomesticCategory(category string) bool {
	for _, dc := range domesticCategories {
		if dc == category {
			return true
		}
	}
	return false
}

func isInSelectedCategories(category string, selectedCategories []string) bool {
	for _, sc := range selectedCategories {
		if sc == category {
			return true
		}
	}
	return false
}

func extractTMDBFromPoster(posterURL string) int {
	// Extract TMDB ID from poster URL like: https://image.tmdb.org/t/p/w780/abc123.jpg
	// Try to parse the path and extract the ID
	if posterURL == "" {
		return 0
	}
	
	// For now, return 0 - the poster URL doesn't contain TMDB ID
	// We'll rely on title matching via TMDB API
	return 0
}


