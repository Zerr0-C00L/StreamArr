package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"errors"
	"regexp"
	"strings"
	"time"
)

// MDBListSyncService handles syncing MDBList lists to the database
type MDBListSyncService struct {
	db         *sql.DB
	mdbClient  *MDBListClient
	tmdbClient *TMDBClient
}

// MDBListConfigEntry represents a configured MDBList list
type MDBListConfigEntry struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// NewMDBListSyncService creates a new sync service
func NewMDBListSyncService(db *sql.DB, mdbAPIKey, tmdbAPIKey string) *MDBListSyncService {
	return &MDBListSyncService{
		db:         db,
		mdbClient:  NewMDBListClient(mdbAPIKey, "./cache/mdblist"),
		tmdbClient: NewTMDBClient(tmdbAPIKey),
	}
}

// SyncAllLists fetches all enabled MDBList lists and imports movies/series to database
func (s *MDBListSyncService) SyncAllLists(ctx context.Context) error {
	// Get mdblist_lists from settings
	listsJSON, err := s.getSettingValue("mdblist_lists")
	if err != nil {
		return fmt.Errorf("failed to get mdblist_lists setting: %w", err)
	}

	if listsJSON == "" {
		log.Println("📋 No MDBList lists configured")
		return nil
	}

	// Parse the lists JSON
	var lists []MDBListConfigEntry
	if err := json.Unmarshal([]byte(listsJSON), &lists); err != nil {
		return fmt.Errorf("failed to parse mdblist_lists: %w", err)
	}

	// Filter enabled lists
	var enabledLists []MDBListConfigEntry
	for _, list := range lists {
		if list.Enabled {
			enabledLists = append(enabledLists, list)
		}
	}

	if len(enabledLists) == 0 {
		log.Println("📋 No enabled MDBList lists found")
		return nil
	}

	log.Printf("📋 Syncing %d MDBList lists...", len(enabledLists))
	GlobalScheduler.UpdateProgress(ServiceMDBListSync, 0, len(enabledLists), "Starting MDBList sync...")

	totalMovies := 0
	totalSeries := 0

	for listIdx, listConfig := range enabledLists {
		log.Printf("  → Fetching: %s", listConfig.Name)
		GlobalScheduler.UpdateProgress(ServiceMDBListSync, listIdx, len(enabledLists), 
			fmt.Sprintf("Fetching: %s", listConfig.Name))

		// Parse username and slug from URL
		username, slug := parseListURL(listConfig.URL)
		if username == "" || slug == "" {
			log.Printf("    ⚠️ Invalid URL format: %s", listConfig.URL)
			continue
		}

		// Fetch list items
		result, err := s.mdbClient.FetchPublicList(username, slug)
		if err != nil {
			log.Printf("    ❌ Error fetching list: %v", err)
			continue
		}

		log.Printf("    📊 Found %d movies, %d series", len(result.Movies), len(result.Series))
		
		totalItems := len(result.Movies) + len(result.Series)
		processedItems := 0

		// Import movies
		moviesAdded := 0
		for _, item := range result.Movies {
			processedItems++
			GlobalScheduler.UpdateProgress(ServiceMDBListSync, listIdx, len(enabledLists), 
				fmt.Sprintf("%s: Importing %s (%d/%d)", listConfig.Name, item.Title, processedItems, totalItems))

			if err := s.importMovie(ctx, item, listConfig.Name); err != nil {
				if errors.Is(err, ErrBlockedBollywood) {
					// treat as skip without warning
					continue
				}
				// Silently skip duplicates
				if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate") {
					log.Printf("    ⚠️ Error importing movie %s: %v", item.Title, err)
				}
			} else {
				moviesAdded++
			}
		}
		totalMovies += moviesAdded

		// Import series
		seriesAdded := 0
		for _, item := range result.Series {
			processedItems++
			GlobalScheduler.UpdateProgress(ServiceMDBListSync, listIdx, len(enabledLists), 
				fmt.Sprintf("%s: Importing %s (%d/%d)", listConfig.Name, item.Title, processedItems, totalItems))

			if err := s.importSeries(ctx, item, listConfig.Name); err != nil {
				if errors.Is(err, ErrBlockedBollywood) {
					// treat as skip without warning
					continue
				}
				// Silently skip duplicates
				if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "duplicate") {
					log.Printf("    ⚠️ Error importing series %s: %v", item.Title, err)
				}
			} else {
				seriesAdded++
			}
		}
		totalSeries += seriesAdded
		
		GlobalScheduler.UpdateProgress(ServiceMDBListSync, listIdx+1, len(enabledLists), 
			fmt.Sprintf("Completed: %s (+%d movies, +%d series)", listConfig.Name, moviesAdded, seriesAdded))

		log.Printf("    ✅ Added %d movies, %d series", moviesAdded, seriesAdded)
	}

	log.Printf("📋 MDBList sync complete: %d movies, %d series imported", totalMovies, totalSeries)
	return nil
}

// importMovie imports a single movie from MDBList to the database
func (s *MDBListSyncService) importMovie(ctx context.Context, item MDBListItem, listName string) error {
	// Check if movie already exists
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM library_movies WHERE tmdb_id = $1)", item.TMDBID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	if exists {
		return fmt.Errorf("movie already exists")
	}

	// Fetch full details from TMDB
	var posterPath, backdropPath, overview string
	var year int
	title := item.Title

	if s.tmdbClient != nil {
		tmdbMovie, err := s.tmdbClient.GetMovie(ctx, item.TMDBID)
		if err == nil && tmdbMovie != nil {
			title = tmdbMovie.Title
			overview = tmdbMovie.Overview
			posterPath = tmdbMovie.PosterPath
			backdropPath = tmdbMovie.BackdropPath
			if tmdbMovie.ReleaseDate != nil {
				year = tmdbMovie.ReleaseDate.Year()
			}
			// Bollywood blocking via settings
			blockStr, _ := s.getSettingValue("block_bollywood")
			if strings.EqualFold(blockStr, "true") && IsIndianMovie(tmdbMovie) {
				return ErrBlockedBollywood
			}
		}
	}

	// Fallback to MDBList data if TMDB failed
	if posterPath == "" {
		posterPath = item.PosterPath
	}
	if backdropPath == "" {
		backdropPath = item.BackdropPath
	}
	if overview == "" {
		overview = item.Overview
	}
	if year == 0 {
		year = item.Year
	}
	if year == 0 {
		year = time.Now().Year()
	}

	// Build metadata with full artwork
	metadata := map[string]interface{}{
		"title":         title,
		"overview":      overview,
		"poster_path":   posterPath,
		"backdrop_path": backdropPath,
		"imdb_id":       item.IMDBID,
		"source":        item.Source,
		"mdblist":       listName,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Insert movie
	query := `
		INSERT INTO library_movies (
			tmdb_id, title, year, monitored, clean_title, metadata, added_at, preferred_quality
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	cleanTitle := strings.ToLower(title)

	_, err = s.db.ExecContext(ctx, query,
		item.TMDBID, title, year, true,
		cleanTitle, metadataJSON, time.Now(), "1080p",
	)
	if err != nil {
		return fmt.Errorf("insert movie: %w", err)
	}

	return nil
}

// importSeries imports a single series from MDBList to the database
func (s *MDBListSyncService) importSeries(ctx context.Context, item MDBListItem, listName string) error {
	// Check if series already exists
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM library_series WHERE tmdb_id = $1)", item.TMDBID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	if exists {
		return fmt.Errorf("series already exists")
	}

	// Fetch full details from TMDB
	var posterPath, backdropPath, overview string
	var year int
	title := item.Title

	if s.tmdbClient != nil {
		tmdbSeries, err := s.tmdbClient.GetSeries(ctx, item.TMDBID)
		if err == nil && tmdbSeries != nil {
			title = tmdbSeries.Title
			overview = tmdbSeries.Overview
			posterPath = tmdbSeries.PosterPath
			backdropPath = tmdbSeries.BackdropPath
			if tmdbSeries.FirstAirDate != nil {
				year = tmdbSeries.FirstAirDate.Year()
			}
			// Bollywood blocking via settings
			blockStr, _ := s.getSettingValue("block_bollywood")
			if strings.EqualFold(blockStr, "true") && IsIndianSeries(tmdbSeries) {
				return ErrBlockedBollywood
			}
		}
	}

	// Fallback to MDBList data if TMDB failed
	if posterPath == "" {
		posterPath = item.PosterPath
	}
	if backdropPath == "" {
		backdropPath = item.BackdropPath
	}
	if overview == "" {
		overview = item.Overview
	}
	if year == 0 {
		year = item.Year
	}
	if year == 0 {
		year = time.Now().Year()
	}

	// Build metadata with full artwork
	metadata := map[string]interface{}{
		"title":         title,
		"overview":      overview,
		"poster_path":   posterPath,
		"backdrop_path": backdropPath,
		"source":        item.Source,
		"mdblist":       listName,
		"media_type":    "tv",
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// Insert series with columns that actually exist in the table
	query := `
		INSERT INTO library_series (
			tmdb_id, imdb_id, title, year, monitored, clean_title, metadata, added_at, preferred_quality
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	cleanTitle := strings.ToLower(title)

	_, err = s.db.ExecContext(ctx, query,
		item.TMDBID, item.IMDBID, title, year, true,
		cleanTitle, metadataJSON, time.Now(), "1080p",
	)
	if err != nil {
		return fmt.Errorf("insert series: %w", err)
	}

	return nil
}

// getSettingValue retrieves a setting value from app_settings JSON
func (s *MDBListSyncService) getSettingValue(key string) (string, error) {
	var appSettingsJSON string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = 'app_settings'").Scan(&appSettingsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(appSettingsJSON), &settings); err != nil {
		return "", err
	}

	if val, ok := settings[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal, nil
		}
	}

	return "", nil
}

// parseListURL extracts username and slug from MDBList URL
// Format: https://mdblist.com/lists/username/slug
func parseListURL(url string) (username, slug string) {
	re := regexp.MustCompile(`mdblist\.com/lists/([^/]+)/([^/\s?]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	return "", ""
}

// GetSyncStats returns current sync statistics
func (s *MDBListSyncService) GetSyncStats(ctx context.Context) (movies int, series int, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_movies").Scan(&movies)
	if err != nil {
		return 0, 0, err
	}
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM library_series").Scan(&series)
	if err != nil {
		return 0, 0, err
	}
	return movies, series, nil
}

// EnrichExistingItems fetches TMDB details for items that are missing artwork
func (s *MDBListSyncService) EnrichExistingItems(ctx context.Context) error {
	if s.tmdbClient == nil {
		return fmt.Errorf("TMDB client not initialized")
	}

	log.Println("[MDBListSync] Enriching existing items with TMDB artwork...")
	log.Printf("[MDBListSync] TMDB API Key configured: %v", s.tmdbClient != nil)

	// Enrich movies missing artwork
	moviesEnriched, err := s.enrichMovies(ctx)
	if err != nil {
		log.Printf("[MDBListSync] Error enriching movies: %v", err)
	}

	// Enrich series missing artwork
	seriesEnriched, err := s.enrichSeries(ctx)
	if err != nil {
		log.Printf("[MDBListSync] Error enriching series: %v", err)
	}

	log.Printf("[MDBListSync] Enrichment complete: %d movies, %d series updated", moviesEnriched, seriesEnriched)
	return nil
}

// enrichMovies updates movies that are missing poster_path in metadata
func (s *MDBListSyncService) enrichMovies(ctx context.Context) (int, error) {
	// Find movies with empty or missing poster_path in metadata
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tmdb_id, metadata 
		FROM library_movies 
		WHERE metadata IS NULL 
		   OR metadata->>'poster_path' IS NULL 
		   OR metadata->>'poster_path' = ''
		LIMIT 500
	`)
	if err != nil {
		log.Printf("[MDBListSync] enrichMovies query error: %v", err)
		return 0, err
	}
	defer rows.Close()

	var enriched int
	var count int
	for rows.Next() {
		count++
		var id int
		var tmdbID int
		var metadataJSON sql.NullString

		if err := rows.Scan(&id, &tmdbID, &metadataJSON); err != nil {
			log.Printf("[MDBListSync] Error scanning movie row: %v", err)
			continue
		}

		// Fetch from TMDB
		tmdbMovie, err := s.tmdbClient.GetMovie(ctx, tmdbID)
		if err != nil || tmdbMovie == nil {
			if count <= 5 {
				log.Printf("[MDBListSync] Could not fetch TMDB movie %d: %v", tmdbID, err)
			}
			continue
		}

		// Build updated metadata
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}

		metadata["poster_path"] = tmdbMovie.PosterPath
		metadata["backdrop_path"] = tmdbMovie.BackdropPath
		metadata["overview"] = tmdbMovie.Overview
		metadata["title"] = tmdbMovie.Title

		newMetadataJSON, err := json.Marshal(metadata)
		if err != nil {
			continue
		}

		// Update in database
		_, err = s.db.ExecContext(ctx, `
			UPDATE library_movies 
			SET metadata = $1, title = $2
			WHERE id = $3
		`, newMetadataJSON, tmdbMovie.Title, id)

		if err != nil {
			log.Printf("[MDBListSync] Error updating movie %d: %v", id, err)
			continue
		}

		enriched++
		
		// Rate limit TMDB calls
		time.Sleep(100 * time.Millisecond)
	}

	return enriched, rows.Err()
}

// enrichSeries updates series that are missing poster_path in metadata
func (s *MDBListSyncService) enrichSeries(ctx context.Context) (int, error) {
	// Find series with empty or missing poster_path in metadata
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tmdb_id, metadata 
		FROM library_series 
		WHERE (metadata IS NULL 
		   OR metadata->>'poster_path' IS NULL 
		   OR metadata->>'poster_path' = '')
		   AND (metadata->>'tmdb_not_found' IS NULL OR metadata->>'tmdb_not_found' != 'true')
		LIMIT 500
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var enriched int
	for rows.Next() {
		var id int
		var tmdbID int
		var metadataJSON sql.NullString

		if err := rows.Scan(&id, &tmdbID, &metadataJSON); err != nil {
			log.Printf("[MDBListSync] Error scanning series row: %v", err)
			continue
		}

		// Build base metadata
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}
		if metadata == nil {
			metadata = make(map[string]interface{})
		}

		// Fetch from TMDB
		tmdbSeries, err := s.tmdbClient.GetSeries(ctx, tmdbID)
		if err != nil || tmdbSeries == nil {
			// Mark as not found to avoid retrying
			metadata["tmdb_not_found"] = "true"
			notFoundJSON, _ := json.Marshal(metadata)
			if _, err := s.db.ExecContext(ctx, `UPDATE library_series SET metadata = $1 WHERE id = $2`, notFoundJSON, id); err != nil {
				log.Printf("[MDBList] Error marking series as not found: %v", err)
			}
			continue
		}

		metadata["poster_path"] = tmdbSeries.PosterPath
		metadata["backdrop_path"] = tmdbSeries.BackdropPath
		metadata["overview"] = tmdbSeries.Overview
		metadata["title"] = tmdbSeries.Title

		newMetadataJSON, err := json.Marshal(metadata)
		if err != nil {
			continue
		}

		// Update in database
		_, err = s.db.ExecContext(ctx, `
			UPDATE library_series 
			SET metadata = $1, title = $2
			WHERE id = $3
		`, newMetadataJSON, tmdbSeries.Title, id)

		if err != nil {
			log.Printf("[MDBListSync] Error updating series %d: %v", id, err)
			continue
		}

		enriched++
		
		// Rate limit TMDB calls
		time.Sleep(100 * time.Millisecond)
	}

	return enriched, rows.Err()
}
