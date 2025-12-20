package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DMMDirectProvider queries DMM API for pre-scraped torrents
type DMMDirectProvider struct {
	DMMURL           string // DMM instance URL (e.g., http://dmm:8080)
	RealDebridAPIKey string
	Client           *http.Client
	Cache            map[string]*DMMCachedResponse
}

type DMMCachedResponse struct {
	Data      []TorrentioStream
	Timestamp time.Time
}

// DMM API response format
type DMMSearchResult struct {
	Title    string  `json:"title"`
	FileSize float64 `json:"fileSize"` // Size in MB
	Hash     string  `json:"hash"`
}

type DMMAPIResponse struct {
	Results      []DMMSearchResult `json:"results,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
}

// NewDMMDirectProvider creates a new DMM API provider
func NewDMMDirectProvider(rdAPIKey string) *DMMDirectProvider {
	// Default to DMM container on same network (DMM listens on port 8181)
	dmmURL := "http://dmm:8181"
	
	return &DMMDirectProvider{
		DMMURL:           dmmURL,
		RealDebridAPIKey: rdAPIKey,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Cache: make(map[string]*DMMCachedResponse),
	}
}

// GetMovieStreams queries DMM API for movie torrents
func (d *DMMDirectProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	cacheKey := fmt.Sprintf("movie_%s", imdbID)
	
	// Check cache first (10 minute cache)
	if cached, ok := d.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 10*time.Minute {
			log.Printf("[DMM API] Cache hit for movie %s (%d streams)", imdbID, len(cached.Data))
			return cached.Data, nil
		}
	}

	log.Printf("[DMM API] Fetching streams for movie %s from %s", imdbID, d.DMMURL)
	
	// Generate DMM authentication token (timestamp + hash)
	tokenWithTimestamp, tokenHash := d.generateDMMToken()
	
	// Query DMM API
	url := fmt.Sprintf("%s/api/torrents/movie?imdbId=%s&dmmProblemKey=%s&solution=%s&onlyTrusted=false",
		d.DMMURL, imdbID, tokenWithTimestamp, tokenHash)
	
	results, err := d.queryDMMAPI(url, "movie")
	if err != nil {
		log.Printf("[DMM API] Error querying DMM for movie %s: %v", imdbID, err)
		return nil, err
	}

	log.Printf("[DMM API] Found %d streams for movie %s", len(results), imdbID)

	// Cache results
	d.Cache[cacheKey] = &DMMCachedResponse{
		Data:      results,
		Timestamp: time.Now(),
	}

	return results, nil
}

// GetSeriesStreams queries DMM API for series torrents
func (d *DMMDirectProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	cacheKey := fmt.Sprintf("series_%s_s%de%d", imdbID, season, episode)
	
	// Check cache first (10 minute cache)
	if cached, ok := d.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 10*time.Minute {
			log.Printf("[DMM API] Cache hit for series %s S%dE%d (%d streams)", imdbID, season, episode, len(cached.Data))
			return cached.Data, nil
		}
	}

	log.Printf("[DMM API] Fetching streams for series %s S%dE%d from %s", imdbID, season, episode, d.DMMURL)
	
	// Generate DMM authentication token
	tokenWithTimestamp, tokenHash := d.generateDMMToken()
	
	// Query DMM API for TV show season
	url := fmt.Sprintf("%s/api/torrents/tv?imdbId=%s&seasonNum=%d&dmmProblemKey=%s&solution=%s&onlyTrusted=false",
		d.DMMURL, imdbID, season, tokenWithTimestamp, tokenHash)
	
	results, err := d.queryDMMAPI(url, "series")
	if err != nil {
		log.Printf("[DMM API] Error querying DMM for series %s S%d: %v", imdbID, season, err)
		return nil, err
	}

	log.Printf("[DMM API] Found %d streams for series %s S%dE%d", len(results), imdbID, season, episode)

	// Cache results
	d.Cache[cacheKey] = &DMMCachedResponse{
		Data:      results,
		Timestamp: time.Now(),
	}

	return results, nil
}

// queryDMMAPI queries DMM API and converts results to TorrentioStream format
func (d *DMMDirectProvider) queryDMMAPI(url, mediaType string) ([]TorrentioStream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// DMM returns 204 with status header when processing or requested
	if resp.StatusCode == 204 {
		status := resp.Header.Get("status")
		if status == "processing" {
			return nil, fmt.Errorf("DMM is still scraping this content, please try again in 1-2 minutes")
		}
		if status == "requested" {
			return nil, fmt.Errorf("content requested for scraping, please try again later")
		}
		return nil, fmt.Errorf("no results available (status: %s)", status)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("DMM API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp DMMAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode DMM response: %v", err)
	}

	if apiResp.ErrorMessage != "" {
		return nil, fmt.Errorf("DMM error: %s", apiResp.ErrorMessage)
	}

	// Convert DMM results to TorrentioStream format
	streams := make([]TorrentioStream, 0, len(apiResp.Results))
	for _, result := range apiResp.Results {
		stream := TorrentioStream{
			Name:     result.Title,
			Title:    result.Title,
			InfoHash: result.Hash,
			Cached:   true, // DMM only returns cached torrents
			Source:   "DMM",
			Size:     int64(result.FileSize * 1024 * 1024), // Convert MB to bytes
		}
		
		// Extract quality from title
		stream.Quality = extractQualityFromTitle(result.Title)

		streams = append(streams, stream)
	}

	return streams, nil
}

// generateDMMToken generates authentication token for DMM API
func (d *DMMDirectProvider) generateDMMToken() (string, string) {
	// DMM expects: timestamp + hash of timestamp
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	
	// Generate hash
	hash := sha256.Sum256([]byte(timestamp))
	hashStr := hex.EncodeToString(hash[:])
	
	return timestamp, hashStr
}

// extractQualityFromTitle extracts quality info from title
func extractQualityFromTitle(title string) string {
	title = strings.ToUpper(title)
	
	qualities := []string{"2160P", "4K", "UHD", "1080P", "720P", "480P"}
	for _, q := range qualities {
		if strings.Contains(title, q) {
			return q
		}
	}
	
	return "Unknown"
}
