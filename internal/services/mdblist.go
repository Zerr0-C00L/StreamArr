package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// MDBListClient handles MDBList.com API integration
type MDBListClient struct {
	apiKey     string
	cacheDir   string
	client     *http.Client
	cacheExpiry time.Duration
}

// MDBListItem represents a media item from MDBList
type MDBListItem struct {
	ID           int     `json:"id"`
	TMDBID       int     `json:"tmdb_id"`
	IMDBID       string  `json:"imdb_id,omitempty"`
	Title        string  `json:"title"`
	Year         int     `json:"year,omitempty"`
	Rating       float64 `json:"rating,omitempty"`
	PosterPath   string  `json:"poster_path,omitempty"`
	BackdropPath string  `json:"backdrop_path,omitempty"`
	Overview     string  `json:"overview"`
	MediaType    string  `json:"media_type"` // "movie" or "tv"
	Source       string  `json:"source"`
}

// MDBListResult represents the result of fetching a list
type MDBListResult struct {
	Success     bool          `json:"success"`
	Source      string        `json:"source"`
	Movies      []MDBListItem `json:"movies"`
	Series      []MDBListItem `json:"series"`
	Total       int           `json:"total"`
	MovieCount  int           `json:"movie_count"`
	SeriesCount int           `json:"series_count"`
	FetchedAt   string        `json:"fetched_at"`
}

// MDBListConfig represents a configured list
type MDBListConfig struct {
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"`
}

// NewMDBListClient creates a new MDBList client
func NewMDBListClient(apiKey, cacheDir string) *MDBListClient {
	if cacheDir == "" {
		cacheDir = "./cache/mdblist"
	}

	os.MkdirAll(cacheDir, 0755)

	return &MDBListClient{
		apiKey:      apiKey,
		cacheDir:    cacheDir,
		cacheExpiry: 1 * time.Hour,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// HasAPIKey checks if API key is configured
func (m *MDBListClient) HasAPIKey() bool {
	return m.apiKey != ""
}

// TestConnection tests the MDBList API connection
func (m *MDBListClient) TestConnection() (map[string]interface{}, error) {
	if !m.HasAPIKey() {
		return map[string]interface{}{
			"success": false,
			"error":   "API key not configured",
		}, nil
	}

	apiURL := fmt.Sprintf("https://mdblist.com/api/user/?apikey=%s", url.QueryEscape(m.apiKey))
	resp, err := m.client.Get(apiURL)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Failed to connect to MDBList API",
		}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal(body, &data)

	if errMsg, ok := data["error"]; ok {
		return map[string]interface{}{
			"success": false,
			"error":   errMsg,
		}, nil
	}

	return map[string]interface{}{
		"success":               true,
		"user":                  data["name"],
		"api_requests_remaining": data["api_requests"],
	}, nil
}

// GetUserLists retrieves user's MDBList lists
func (m *MDBListClient) GetUserLists() (map[string]interface{}, error) {
	if !m.HasAPIKey() {
		return map[string]interface{}{
			"success": false,
			"error":   "API key not configured",
		}, nil
	}

	apiURL := fmt.Sprintf("https://mdblist.com/api/lists/user/?apikey=%s", url.QueryEscape(m.apiKey))
	resp, err := m.client.Get(apiURL)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Failed to fetch user lists",
		}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var lists []map[string]interface{}
	json.Unmarshal(body, &lists)

	// Get username from first list's details
	username := ""
	if len(lists) > 0 {
		if listID, ok := lists[0]["id"].(float64); ok {
			detailURL := fmt.Sprintf("https://mdblist.com/api/lists/%d/?apikey=%s", int(listID), url.QueryEscape(m.apiKey))
			detailResp, err := m.client.Get(detailURL)
			if err == nil {
				defer detailResp.Body.Close()
				detailBody, _ := io.ReadAll(detailResp.Body)
				var details []map[string]interface{}
				if json.Unmarshal(detailBody, &details) == nil && len(details) > 0 {
					if un, ok := details[0]["user_name"].(string); ok {
						username = un
					}
				}
			}
		}
	}

	// Add username to each list
	for i := range lists {
		lists[i]["user_name"] = username
	}

	return map[string]interface{}{
		"success":  true,
		"lists":    lists,
		"username": username,
	}, nil
}

// FetchListByURL fetches items from a public MDBList URL
func (m *MDBListClient) FetchListByURL(listURL string) (*MDBListResult, error) {
	// Parse URL to extract username and list slug
	// Format: https://mdblist.com/lists/username/listname
	var username, listSlug string
	
	if _, err := fmt.Sscanf(listURL, "https://mdblist.com/lists/%s/%s", &username, &listSlug); err == nil {
		return m.FetchPublicList(username, listSlug)
	}

	return nil, fmt.Errorf("invalid MDBList URL format")
}

// FetchPublicList fetches a public list by username and slug
func (m *MDBListClient) FetchPublicList(username, listSlug string) (*MDBListResult, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("list_%s_%s", username, listSlug)
	if cached := m.getCache(cacheKey); cached != nil {
		return cached, nil
	}

	// Fetch from MDBList public JSON endpoint
	apiURL := fmt.Sprintf("https://mdblist.com/lists/%s/%s/json", username, listSlug)
	resp, err := m.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	// Process items
	result := m.processListItems(items, fmt.Sprintf("%s/%s", username, listSlug))
	m.setCache(cacheKey, result)

	return result, nil
}

// FetchListByID fetches a list by ID (requires API key)
func (m *MDBListClient) FetchListByID(listID string) (*MDBListResult, error) {
	if !m.HasAPIKey() {
		return nil, fmt.Errorf("API key required for list ID lookup")
	}

	// Check cache
	cacheKey := fmt.Sprintf("list_id_%s", listID)
	if cached := m.getCache(cacheKey); cached != nil {
		return cached, nil
	}

	apiURL := fmt.Sprintf("https://mdblist.com/api/lists/%s/items/?apikey=%s", listID, url.QueryEscape(m.apiKey))
	resp, err := m.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var items []map[string]interface{}
	json.Unmarshal(body, &items)

	result := m.processListItems(items, fmt.Sprintf("list_%s", listID))
	m.setCache(cacheKey, result)

	return result, nil
}

// SearchLists searches MDBList for lists
func (m *MDBListClient) SearchLists(query string) ([]interface{}, error) {
	apiURL := fmt.Sprintf("https://mdblist.com/api/lists/search/?query=%s", url.QueryEscape(query))
	if m.HasAPIKey() {
		apiURL += fmt.Sprintf("&apikey=%s", url.QueryEscape(m.apiKey))
	}

	resp, err := m.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var lists []interface{}
	json.Unmarshal(body, &lists)

	return lists, nil
}

// GetTopLists gets top/popular MDBList lists
func (m *MDBListClient) GetTopLists(mediaType string) ([]interface{}, error) {
	apiURL := fmt.Sprintf("https://mdblist.com/api/lists/top/?mediatype=%s", url.QueryEscape(mediaType))
	if m.HasAPIKey() {
		apiURL += fmt.Sprintf("&apikey=%s", url.QueryEscape(m.apiKey))
	}

	resp, err := m.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var lists []interface{}
	json.Unmarshal(body, &lists)

	return lists, nil
}

// processListItems processes raw list items into structured format
func (m *MDBListClient) processListItems(items []map[string]interface{}, sourceName string) *MDBListResult {
	var movies, series []MDBListItem

	for _, item := range items {
		tmdbID, ok := item["tmdb_id"].(float64)
		if !ok {
			if id, ok := item["id"].(float64); ok {
				tmdbID = id
			} else {
				continue
			}
		}

		mediaType := "movie"
		if mt, ok := item["mediatype"].(string); ok {
			mediaType = mt
		} else if mt, ok := item["media_type"].(string); ok {
			mediaType = mt
		}

		title := ""
		if t, ok := item["title"].(string); ok {
			title = t
		} else if n, ok := item["name"].(string); ok {
			title = n
		}

		processed := MDBListItem{
			ID:           int(tmdbID),
			TMDBID:       int(tmdbID),
			Title:        title,
			MediaType:    mediaType,
			Source:       "mdblist:" + sourceName,
		}

		if imdb, ok := item["imdb_id"].(string); ok {
			processed.IMDBID = imdb
		}
		if year, ok := item["year"].(float64); ok {
			processed.Year = int(year)
		} else if year, ok := item["release_year"].(float64); ok {
			processed.Year = int(year)
		}
		if rating, ok := item["rating"].(float64); ok {
			processed.Rating = rating
		} else if score, ok := item["score"].(float64); ok {
			processed.Rating = score
		}
		if poster, ok := item["poster"].(string); ok {
			processed.PosterPath = poster
		}
		if backdrop, ok := item["backdrop"].(string); ok {
			processed.BackdropPath = backdrop
		}
		if overview, ok := item["overview"].(string); ok {
			processed.Overview = overview
		} else if desc, ok := item["description"].(string); ok {
			processed.Overview = desc
		}

		if mediaType == "movie" {
			movies = append(movies, processed)
		} else {
			series = append(series, processed)
		}
	}

	return &MDBListResult{
		Success:     true,
		Source:      sourceName,
		Movies:      movies,
		Series:      series,
		Total:       len(movies) + len(series),
		MovieCount:  len(movies),
		SeriesCount: len(series),
		FetchedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}
}

// getCache retrieves cached list data
func (m *MDBListClient) getCache(key string) *MDBListResult {
	cachePath := filepath.Join(m.cacheDir, key+".json")
	
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil
	}

	// Check if cache is expired
	if time.Since(info.ModTime()) > m.cacheExpiry {
		os.Remove(cachePath)
		return nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var result MDBListResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return &result
}

// setCache stores list data in cache
func (m *MDBListClient) setCache(key string, result *MDBListResult) {
	cachePath := filepath.Join(m.cacheDir, key+".json")
	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(cachePath, data, 0644)
}

// FetchAllConfiguredLists fetches all configured MDBList lists
func (m *MDBListClient) FetchAllConfiguredLists(configPath string) (*MDBListResult, error) {
	// Load saved lists from config
	configs, err := m.loadListConfigs(configPath)
	if err != nil {
		return &MDBListResult{
			Success: true,
			Movies:  []MDBListItem{},
			Series:  []MDBListItem{},
			Total:   0,
		}, nil
	}

	// Filter enabled lists
	var enabledLists []MDBListConfig
	for _, config := range configs {
		if config.Enabled {
			enabledLists = append(enabledLists, config)
		}
	}

	if len(enabledLists) == 0 {
		return &MDBListResult{
			Success: true,
			Movies:  []MDBListItem{},
			Series:  []MDBListItem{},
			Total:   0,
		}, nil
	}

	// Fetch all lists and merge
	allMovies := make(map[int]MDBListItem)
	allSeries := make(map[int]MDBListItem)

	for _, listConfig := range enabledLists {
		result, err := m.FetchListByURL(listConfig.URL)
		if err != nil {
			continue
		}

		// Deduplicate by TMDB ID
		for _, movie := range result.Movies {
			allMovies[movie.TMDBID] = movie
		}
		for _, show := range result.Series {
			allSeries[show.TMDBID] = show
		}
	}

	// Convert maps to slices
	movies := make([]MDBListItem, 0, len(allMovies))
	for _, m := range allMovies {
		movies = append(movies, m)
	}
	series := make([]MDBListItem, 0, len(allSeries))
	for _, s := range allSeries {
		series = append(series, s)
	}

	return &MDBListResult{
		Success:     true,
		Source:      "merged",
		Movies:      movies,
		Series:      series,
		Total:       len(movies) + len(series),
		MovieCount:  len(movies),
		SeriesCount: len(series),
		FetchedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// loadListConfigs loads list configurations from file
func (m *MDBListClient) loadListConfigs(configPath string) ([]MDBListConfig, error) {
	if configPath == "" {
		configPath = "./mdblist_config.json"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var configs []MDBListConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}
