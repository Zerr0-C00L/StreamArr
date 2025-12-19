package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ZileanProvider integrates with Zilean DMM hash database
type ZileanProvider struct {
	BaseURL          string
	APIKey           string
	Client           *http.Client
	Cache            map[string]*ZileanCachedResponse
	RealDebridAPIKey string
}

type ZileanCachedResponse struct {
	Data      []ZileanTorrent
	Timestamp time.Time
}

// ZileanTorrent represents a torrent from Zilean
type ZileanTorrent struct {
	InfoHash string `json:"infoHash"`
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
}

// ZileanSearchResult from /dmm/search endpoint
type ZileanSearchResult struct {
	Torrents []ZileanTorrent `json:"torrents"`
}

// NewZileanProvider creates a new Zilean provider
func NewZileanProvider(baseURL, apiKey, rdAPIKey string) *ZileanProvider {
	if baseURL == "" {
		baseURL = "http://localhost:8181"
	}

	return &ZileanProvider{
		BaseURL:          strings.TrimSuffix(baseURL, "/"),
		APIKey:           apiKey,
		RealDebridAPIKey: rdAPIKey,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		Cache: make(map[string]*ZileanCachedResponse),
	}
}

// SearchByIMDB searches Zilean for torrents by IMDB ID
func (z *ZileanProvider) SearchByIMDB(ctx context.Context, imdbID string) ([]ZileanTorrent, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("imdb_%s", imdbID)
	if cached, ok := z.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 30*time.Minute {
			return cached.Data, nil
		}
	}

	// Build search URL - Zilean uses /imdb/search endpoint with POST
	searchURL := fmt.Sprintf("%s/imdb/search?query=%s", z.BaseURL, imdbID)

	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add API key if provided
	if z.APIKey != "" {
		req.Header.Set("X-Api-Key", z.APIKey)
	}

	resp, err := z.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search zilean: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zilean returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ZileanSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Cache the results
	z.Cache[cacheKey] = &ZileanCachedResponse{
		Data:      result.Torrents,
		Timestamp: time.Now(),
	}

	return result.Torrents, nil
}

// SearchByQuery searches Zilean using a text query
func (z *ZileanProvider) SearchByQuery(ctx context.Context, query string) ([]ZileanTorrent, error) {
	// Check cache
	cacheKey := fmt.Sprintf("query_%s", query)
	if cached, ok := z.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 30*time.Minute {
			return cached.Data, nil
		}
	}

	// URL encode the query
	encodedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("%s/dmm/search?query=%s", z.BaseURL, encodedQuery)

	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if z.APIKey != "" {
		req.Header.Set("X-Api-Key", z.APIKey)
	}

	resp, err := z.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search zilean: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zilean returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ZileanSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Cache the results
	z.Cache[cacheKey] = &ZileanCachedResponse{
		Data:      result.Torrents,
		Timestamp: time.Now(),
	}

	return result.Torrents, nil
}

// GetMovieStreams implements StreamProvider interface
func (z *ZileanProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	ctx := context.Background()
	torrents, err := z.SearchByIMDB(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	return z.convertToStreams(torrents), nil
}

// GetSeriesStreams implements StreamProvider interface
func (z *ZileanProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	ctx := context.Background()
	
	// Search by IMDB ID first
	torrents, err := z.SearchByIMDB(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	// Filter torrents by season/episode in filename
	filtered := z.filterBySeasonEpisode(torrents, season, episode)
	
	return z.convertToStreams(filtered), nil
}

// filterBySeasonEpisode filters torrents that match the season/episode
func (z *ZileanProvider) filterBySeasonEpisode(torrents []ZileanTorrent, season, episode int) []ZileanTorrent {
	filtered := make([]ZileanTorrent, 0)
	
	// Common patterns: S01E01, 1x01, s01e01, etc.
	patterns := []string{
		fmt.Sprintf("S%02dE%02d", season, episode),
		fmt.Sprintf("s%02de%02d", season, episode),
		fmt.Sprintf("%dx%02d", season, episode),
	}

	for _, torrent := range torrents {
		filename := strings.ToLower(torrent.Filename)
		for _, pattern := range patterns {
			if strings.Contains(filename, strings.ToLower(pattern)) {
				filtered = append(filtered, torrent)
				break
			}
		}
	}

	return filtered
}

// convertToStreams converts Zilean torrents to TorrentioStream format
func (z *ZileanProvider) convertToStreams(torrents []ZileanTorrent) []TorrentioStream {
	streams := make([]TorrentioStream, len(torrents))

	for i, torrent := range torrents {
		// Extract quality and other info from filename
		quality := extractQualityFromFilename(torrent.Filename)
		
		stream := TorrentioStream{
			Name:     fmt.Sprintf("📦 Zilean: %s", torrent.Filename),
			Title:    fmt.Sprintf("%s (%.2f GB)", quality, float64(torrent.Filesize)/(1024*1024*1024)),
			InfoHash: torrent.InfoHash,
			Quality:  quality,
			Size:     torrent.Filesize,
			Cached:   true, // Zilean only indexes cached torrents
			Source:   "zilean",
		}

		// If Real-Debrid is configured, create magnet URL
		if z.RealDebridAPIKey != "" {
			stream.URL = fmt.Sprintf("magnet:?xt=urn:btih:%s", torrent.InfoHash)
		}

		streams[i] = stream
	}

	return streams
}

// extractQualityFromFilename extracts quality info from filename
func extractQualityFromFilename(filename string) string {
	upper := strings.ToUpper(filename)
	
	// Check for common quality indicators
	qualities := []string{"2160p", "4K", "1080p", "720p", "480p"}
	for _, q := range qualities {
		if strings.Contains(upper, q) {
			return q
		}
	}
	
	// Check for REMUX
	if strings.Contains(upper, "REMUX") {
		return "REMUX"
	}
	
	// Check for WEB-DL
	if strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL") {
		return "WEB-DL"
	}
	
	// Check for BluRay
	if strings.Contains(upper, "BLURAY") || strings.Contains(upper, "BLU-RAY") {
		return "BluRay"
	}
	
	return "Unknown"
}

// ZileanStats represents statistics from Zilean
type ZileanStats struct {
	TorrentCount int64  `json:"torrent_count"`
	Status       string `json:"status"`
	Healthy      bool   `json:"healthy"`
}

// GetStats retrieves statistics from Zilean
func (z *ZileanProvider) GetStats(ctx context.Context) (*ZileanStats, error) {
	stats := &ZileanStats{
		Status: "unknown",
	}

	// Check health first
	err := z.HealthCheck(ctx)
	stats.Healthy = (err == nil)

	if err != nil {
		stats.Status = "offline"
		return stats, nil
	}

	stats.Status = "online"

	// Zilean doesn't provide a stats API endpoint, so we'll make a test search
	// to verify it's functional and return an indicator that torrents are available
	testReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/dmm/search?query=test", z.BaseURL), nil)
	if err != nil {
		stats.TorrentCount = -1 // -1 indicates "scraping in progress"
		return stats, nil
	}

	if z.APIKey != "" {
		testReq.Header.Set("X-Api-Key", z.APIKey)
	}

	// Set a short timeout
	testCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	testReq = testReq.WithContext(testCtx)

	testResp, err := z.Client.Do(testReq)
	if err != nil {
		stats.TorrentCount = -1 // -1 indicates "scraping in progress"
		return stats, nil
	}
	defer testResp.Body.Close()

	// If search endpoint responds (even with empty results), Zilean is working
	if testResp.StatusCode == http.StatusOK {
		// Return -1 to indicate "scraping in progress" in the UI
		// The actual count would require direct database access
		stats.TorrentCount = -1
	} else {
		stats.TorrentCount = -1
	}

	return stats, nil
}

// HealthCheck checks if Zilean is accessible
func (z *ZileanProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/healthchecks/ping", z.BaseURL), nil)
	if err != nil {
		return err
	}

	resp, err := z.Client.Do(req)
	if err != nil {
		return fmt.Errorf("zilean unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("zilean health check failed: status %d", resp.StatusCode)
	}

	return nil
}
