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
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Year        int            `json:"year"`
	Poster      string         `json:"poster"`
	Background  string         `json:"background"`
	Description string         `json:"description"`
	Runtime     interface{}    `json:"runtime"` // Can be int or string
	Genres      []string       `json:"genres"`
	Category    string         `json:"category"`
	Streams     []BalkanStream `json:"streams"`
}

// GetRuntime returns runtime as int regardless of JSON type
func (b *BalkanMovieEntry) GetRuntime() int {
	switch v := b.Runtime.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		// Try to parse string to int
		if v == "" {
			return 0
		}
		var runtime int
		fmt.Sscanf(v, "%d", &runtime)
		return runtime
	default:
		return 0
	}
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

// ImportResult represents the result of importing a single item
type ImportResult struct {
	Updated bool
	Error   error
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
	
	log.Printf("[BalkanVOD] Selected categories: %v (useAll: %v)", selectedCategories, useAllCategories)
	
	// Import movies (filter by selected categories)
	imported := 0
	skipped := 0
	failed := 0
	updated := 0
	skippedByCategory := 0
	skippedByDomestic := 0

	for _, movie := range content.Movies {
		// Filter by category
		if !useAllCategories && !isInSelectedCategories(movie.Category, selectedCategories) {
			skippedByCategory++
			skipped++
			continue
		}
		
		// If categories are selected, trust the selection; otherwise check domestic list
		if useAllCategories && !isDomesticCategory(movie.Category) {
			log.Printf("[BalkanVOD] Skipping movie '%s' - category '%s' not in domestic list", movie.Name, movie.Category)
			skippedByDomestic++
			skipped++
			continue
		}

		result := b.importMovie(ctx, movie)
		if result.Error != nil {
			log.Printf("[BalkanVOD] Failed to import movie %s: %v", movie.Name, result.Error)
			failed++
		} else if result.Updated {
			updated++
		} else {
			imported++
		}
	}

	// Import series (all series are domestic)
	for _, series := range content.Series {
		result := b.importSeries(ctx, series)
		if result.Error != nil {
			log.Printf("[BalkanVOD] Failed to import series %s: %v", series.Name, result.Error)
			failed++
		} else if result.Updated {
			updated++
		} else {
			imported++
		}
	}

	log.Printf("[BalkanVOD] Import complete: %d new, %d updated, %d skipped (%d by category filter, %d not domestic), %d failed", imported, updated, skipped, skippedByCategory, skippedByDomestic, failed)
	return nil
}

func (b *BalkanVODImporter) importMovie(ctx context.Context, entry BalkanMovieEntry) ImportResult {
	if len(entry.Streams) == 0 {
		return ImportResult{Error: fmt.Errorf("no streams available")}
	}

	// Generate a unique TMDB ID based on Balkan ID to avoid constraint violations
	// Use negative IDs to distinguish from real TMDB IDs
	tmdbID := generateUniqueTMDBID(entry.ID)
	
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
		Runtime:       entry.GetRuntime(),
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

	// Add movie directly - no duplicate checking
	log.Printf("[BalkanVOD] Adding movie '%s' (TMDB: %d, Category: %s)", entry.Name, tmdbID, entry.Category)
	err := b.movieStore.Add(ctx, movie)
	if err != nil {
		log.Printf("[BalkanVOD] Error adding movie '%s': %v", entry.Name, err)
	}
	return ImportResult{Updated: false, Error: err}
}

func (b *BalkanVODImporter) importSeries(ctx context.Context, entry BalkanSeriesEntry) ImportResult {
	if len(entry.Streams) == 0 {
		return ImportResult{Error: fmt.Errorf("no streams available")}
	}

	// Generate a unique TMDB ID based on Balkan ID to avoid constraint violations
	// Use negative IDs to distinguish from real TMDB IDs
	tmdbID := generateUniqueTMDBID(entry.ID)
	
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

	// Add series directly - no duplicate checking
	log.Printf("[BalkanVOD] Adding series '%s' (TMDB: %d, Category: %s)", entry.Name, tmdbID, entry.Category)
	err := b.seriesStore.Add(ctx, series)
	if err != nil {
		log.Printf("[BalkanVOD] Error adding series '%s': %v", entry.Name, err)
	}
	return ImportResult{Updated: false, Error: err}
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

// mergeStreams combines streams from existing and new sources, removing duplicates by URL
func mergeStreams(existingStreams, newStreams interface{}) []map[string]interface{} {
	var result []map[string]interface{}
	seenURLs := make(map[string]bool)
	
	// Add existing streams
	if existing, ok := existingStreams.([]map[string]interface{}); ok {
		for _, stream := range existing {
			if url, ok := stream["url"].(string); ok && url != "" {
				if !seenURLs[url] {
					result = append(result, stream)
					seenURLs[url] = true
				}
			}
		}
	} else if existing, ok := existingStreams.([]interface{}); ok {
		for _, s := range existing {
			if stream, ok := s.(map[string]interface{}); ok {
				if url, ok := stream["url"].(string); ok && url != "" {
					if !seenURLs[url] {
						result = append(result, stream)
						seenURLs[url] = true
					}
				}
			}
		}
	}
	
	// Add new streams (avoiding duplicates)
	if newStreamsSlice, ok := newStreams.([]map[string]interface{}); ok {
		for _, stream := range newStreamsSlice {
			if url, ok := stream["url"].(string); ok && url != "" {
				if !seenURLs[url] {
					result = append(result, stream)
					seenURLs[url] = true
				}
			}
		}
	}
	
	log.Printf("[BalkanVOD] Merged streams: %d existing + new = %d total unique", len(seenURLs)-len(result), len(result))
	return result
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

// generateUniqueTMDBID creates a unique negative TMDB ID from a Balkan ID
// Negative IDs distinguish Balkan content from real TMDB content
// Must fit in PostgreSQL integer type (32-bit signed: -2147483648 to 2147483647)
func generateUniqueTMDBID(balkanID string) int {
	// Simple hash: sum of character codes modulo to fit in 31 bits
	hash := uint32(0)
	for _, char := range balkanID {
		hash = hash*31 + uint32(char)
	}
	// Keep in range for 32-bit signed int, use modulo to ensure it fits
	// Range: -2147483647 to -1
	positiveHash := int(hash%2147483647) + 1
	return -positiveHash
}


