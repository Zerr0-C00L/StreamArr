package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
	Seasons    []BalkanSeason `json:"seasons"`
}

// BalkanSeason represents a season with episodes
type BalkanSeason struct {
	Number   int              `json:"number"`
	Episodes []BalkanEpisode `json:"episodes"`
}

// BalkanEpisode represents an episode
type BalkanEpisode struct {
	Episode   int    `json:"episode"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
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

// extractTMDBPath extracts just the path from a TMDB image URL
// Converts: https://image.tmdb.org/t/p/w780/abc123.jpg -> /abc123.jpg
// If already a path (starts with /), returns as-is
// If empty, returns empty string
func extractTMDBPath(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	
	// If already a path (starts with /), return as-is
	if strings.HasPrefix(imageURL, "/") {
		return imageURL
	}
	
	// Extract path from full TMDB URL
	// Example: https://image.tmdb.org/t/p/w780/abc123.jpg -> /abc123.jpg
	if idx := strings.Index(imageURL, "/t/p/"); idx != -1 {
		// Find the next / after /t/p/
		if idx2 := strings.Index(imageURL[idx+5:], "/"); idx2 != -1 {
			return imageURL[idx+5+idx2:]
		}
	}
	
	// If no TMDB URL pattern found, return empty
	return ""
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

// fetchBalkanData fetches content from Balkan On Demand GitHub repo
func fetchBalkanData() (*BalkanContentDatabase, error) {
	log.Printf("[BalkanVOD] Fetching from GitHub: %s", balkanRepoURL)
	
	resp, err := http.Get(balkanRepoURL)
	if err != nil {
		return nil, fmt.Errorf("fetch balkan content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	log.Printf("[BalkanVOD] Downloaded %d bytes", len(body))

	var content BalkanContentDatabase
	if err := json.Unmarshal(body, &content); err != nil {
		return nil, fmt.Errorf("parse content database: %w", err)
	}

	log.Printf("[BalkanVOD] Fetched %d movies and %d series", len(content.Movies), len(content.Series))
	return &content, nil
}

// FetchBalkanCategories fetches all available categories with counts from GitHub repo
func FetchBalkanCategories() ([]CategoryWithCount, error) {
	content, err := fetchBalkanData()
	if err != nil {
		return nil, err
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

	log.Println("[BalkanVOD] Starting import from GitHub repo...")
	
	// Fetch content from Balkan On Demand repo
	content, err := fetchBalkanData()
	if err != nil {
		return fmt.Errorf("fetch balkan content: %w", err)
	}

	log.Printf("[BalkanVOD] Fetched %d movies and %d series", len(content.Movies), len(content.Series))

	// Get selected categories from settings
	selectedCategories := b.cfg.BalkanVODSelectedCategories
	useAllCategories := len(selectedCategories) == 0
	
	log.Printf("[BalkanVOD] Selected categories: %v (useAll: %v)", selectedCategories, useAllCategories)
	
	// Import movies (filter by selected categories if specified)
	imported := 0
	skipped := 0
	failed := 0
	updated := 0
	skippedByCategory := 0
	skippedByDomestic := 0

	for _, movie := range content.Movies {
		// Filter by category ONLY if categories are explicitly selected
		if !useAllCategories && !isInSelectedCategories(movie.Category, selectedCategories) {
			skippedByCategory++
			skipped++
			continue
		}
		
		// Import ALL content when no categories selected (removed domestic-only filter)

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
		PosterPath:    extractTMDBPath(entry.Poster),
		BackdropPath:  extractTMDBPath(entry.Background),
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
	
	// Prepare new streams
	newStreams := make([]map[string]interface{}, len(entry.Streams))
	for i, stream := range entry.Streams {
		newStreams[i] = map[string]interface{}{
			"name":    "Balkan VOD",
			"url":     stream.URL,
			"quality": stream.Quality,
			"source":  stream.Source,
		}
	}
	
	// Try to add the movie
	err := b.movieStore.Add(ctx, movie)
	if err != nil && (err.Error() == "movie already exists in library") {
		// Movie exists with same TMDB ID - this means it's a duplicate from the same source
		// Try to find it and merge streams
		existingMovie, getErr := b.movieStore.GetByTMDBID(ctx, tmdbID)
		if getErr == nil && existingMovie != nil {
			log.Printf("[BalkanVOD] Duplicate movie '%s' (TMDB: %d) - merging streams", entry.Name, tmdbID)
			
			// Get existing streams
			existingStreams := []map[string]interface{}{}
			if streams, ok := existingMovie.Metadata["balkan_vod_streams"].([]interface{}); ok {
				for _, s := range streams {
					if streamMap, ok := s.(map[string]interface{}); ok {
						existingStreams = append(existingStreams, streamMap)
					}
				}
			}
			
			// Merge streams (remove duplicates by URL)
			streamURLs := make(map[string]bool)
			for _, s := range existingStreams {
				if url, ok := s["url"].(string); ok {
					streamURLs[url] = true
				}
			}
			
			for _, newStream := range newStreams {
				if url, ok := newStream["url"].(string); ok {
					if !streamURLs[url] {
						existingStreams = append(existingStreams, newStream)
						streamURLs[url] = true
					}
				}
			}
			
			// Update metadata with merged streams
			existingMovie.Metadata["balkan_vod_streams"] = existingStreams
			existingMovie.Metadata["last_updated"] = time.Now().Format(time.RFC3339)
			
			// Update the movie in database
			updateErr := b.movieStore.Update(ctx, existingMovie)
			if updateErr != nil {
				log.Printf("[BalkanVOD] Failed to update movie '%s': %v", entry.Name, updateErr)
				return ImportResult{Error: updateErr}
			}
			
			return ImportResult{Updated: true, Error: nil}
		}
		// If we can't find it, just return the error
		return ImportResult{Error: err}
	} else if err != nil {
		// Some other error
		log.Printf("[BalkanVOD] Error adding movie '%s': %v", entry.Name, err)
		return ImportResult{Error: err}
	}
	
	// Movie added successfully
	log.Printf("[BalkanVOD] Added new movie '%s' (TMDB: %d, Category: %s)", entry.Name, tmdbID, entry.Category)
	return ImportResult{Updated: false, Error: nil}
}

func (b *BalkanVODImporter) importSeries(ctx context.Context, entry BalkanSeriesEntry) ImportResult {
	// Check if we have seasons with episodes
	if len(entry.Seasons) == 0 {
		return ImportResult{Error: fmt.Errorf("no seasons available")}
	}

	// Count total episodes
	totalEpisodes := 0
	for _, season := range entry.Seasons {
		totalEpisodes += len(season.Episodes)
	}
	if totalEpisodes == 0 {
		return ImportResult{Error: fmt.Errorf("no episodes available")}
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
		PosterPath:    extractTMDBPath(entry.Poster),
		BackdropPath:  extractTMDBPath(entry.Background),
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
	series.Metadata["total_seasons"] = len(entry.Seasons)
	series.Metadata["total_episodes"] = totalEpisodes
	
	// Store season and episode structure
	seasons := make([]map[string]interface{}, len(entry.Seasons))
	for i, season := range entry.Seasons {
		episodes := make([]map[string]interface{}, len(season.Episodes))
		for j, ep := range season.Episodes {
			episodes[j] = map[string]interface{}{
				"episode":   ep.Episode,
				"title":     ep.Title,
				"url":       ep.URL,
				"thumbnail": ep.Thumbnail,
			}
		}
		seasons[i] = map[string]interface{}{
			"number":   season.Number,
			"episodes": episodes,
		}
	}
	
	// Try to add the series
	series.Metadata["balkan_vod_seasons"] = seasons
	err := b.seriesStore.Add(ctx, series)
	if err != nil && (err.Error() == "failed to add series: pq: duplicate key value violates unique constraint \"library_series_tmdb_id_key\"" || err.Error() == "series already exists in library") {
		// Series exists - merge seasons/episodes
		existingSeries, getErr := b.seriesStore.GetByTMDBID(ctx, tmdbID)
		if getErr == nil && existingSeries != nil {
			log.Printf("[BalkanVOD] Duplicate series '%s' (TMDB: %d) - merging episodes", entry.Name, tmdbID)
			
			// Get existing seasons
			existingSeasons := []map[string]interface{}{}
			if sData, ok := existingSeries.Metadata["balkan_vod_seasons"].([]interface{}); ok {
				for _, s := range sData {
					if seasonMap, ok := s.(map[string]interface{}); ok {
						existingSeasons = append(existingSeasons, seasonMap)
					}
				}
			}
			
			// Merge logic: combine episodes from matching seasons, avoiding duplicates by episode number
			seasonMap := make(map[int]map[int]map[string]interface{}) // [seasonNum][episodeNum]episodeData
			
			// Add existing episodes to map
			for _, season := range existingSeasons {
				seasonNum := int(season["number"].(float64))
				if seasonMap[seasonNum] == nil {
					seasonMap[seasonNum] = make(map[int]map[string]interface{})
				}
				if eps, ok := season["episodes"].([]interface{}); ok {
					for _, ep := range eps {
						if epMap, ok := ep.(map[string]interface{}); ok {
							epNum := int(epMap["episode"].(float64))
							seasonMap[seasonNum][epNum] = epMap
						}
					}
				}
			}
			
			// Add new episodes to map (will overwrite if same episode number)
			for _, season := range seasons {
				seasonNum := int(season["number"].(float64))
				if seasonMap[seasonNum] == nil {
					seasonMap[seasonNum] = make(map[int]map[string]interface{})
				}
				if eps, ok := season["episodes"].([]interface{}); ok {
					for _, ep := range eps {
						if epMap, ok := ep.(map[string]interface{}); ok {
							epNum := int(epMap["episode"].(float64))
							seasonMap[seasonNum][epNum] = epMap
						}
					}
				}
			}
			
			// Convert map back to season array
			mergedSeasons := []map[string]interface{}{}
			for seasonNum, episodes := range seasonMap {
				episodeList := []map[string]interface{}{}
				for _, epData := range episodes {
					episodeList = append(episodeList, epData)
				}
				mergedSeasons = append(mergedSeasons, map[string]interface{}{
					"number":   seasonNum,
					"episodes": episodeList,
				})
			}
			
			// Update metadata with merged seasons
			existingSeries.Metadata["balkan_vod_seasons"] = mergedSeasons
			existingSeries.Metadata["last_updated"] = time.Now().Format(time.RFC3339)
			existingSeries.Metadata["total_episodes"] = len(mergedSeasons) // Update total
			
			// Update the series in database
			updateErr := b.seriesStore.Update(ctx, existingSeries)
			if updateErr != nil {
				log.Printf("[BalkanVOD] Failed to update series '%s': %v", entry.Name, updateErr)
				return ImportResult{Error: updateErr}
			}
			
			return ImportResult{Updated: true, Error: nil}
		}
		// If we can't find it, return the error
		return ImportResult{Error: err}
	} else if err != nil {
		// Some other error
		log.Printf("[BalkanVOD] Error adding series '%s': %v", entry.Name, err)
		return ImportResult{Error: err}
	}
	
	// Series added successfully
	log.Printf("[BalkanVOD] Added new series '%s' (TMDB: %d, Category: %s)", entry.Name, tmdbID, entry.Category)
	return ImportResult{Updated: false, Error: nil}
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


