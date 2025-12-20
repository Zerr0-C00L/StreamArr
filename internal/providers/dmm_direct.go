package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DMMDirectProvider queries DMM sources directly on-demand
type DMMDirectProvider struct {
	RealDebridAPIKey string
	Client           *http.Client
	Cache            map[string]*DMMCachedResponse
}

type DMMCachedResponse struct {
	Data      []TorrentioStream
	Timestamp time.Time
}

// NewDMMDirectProvider creates a new direct DMM provider
func NewDMMDirectProvider(rdAPIKey string) *DMMDirectProvider {
	return &DMMDirectProvider{
		RealDebridAPIKey: rdAPIKey,
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		Cache: make(map[string]*DMMCachedResponse),
	}
}

// GetMovieStreams queries DMM sources directly for a movie
func (d *DMMDirectProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	cacheKey := fmt.Sprintf("movie_%s", imdbID)
	
	// Check cache first (5 minute cache)
	if cached, ok := d.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 5*time.Minute {
			log.Printf("[DMM Direct] Cache hit for movie %s (%d streams)", imdbID, len(cached.Data))
			return cached.Data, nil
		}
	}

	log.Printf("[DMM Direct] Fetching streams for movie %s", imdbID)
	
	// Query multiple DMM sources in parallel
	sources := []string{
		fmt.Sprintf("https://torrentio.strem.fun/%s/stream/movie/%s.json", d.getRDConfig(), imdbID),
		fmt.Sprintf("https://comet.elfhosted.com/c/realdebrid=%s/stream/movie/%s.json", url.QueryEscape(d.RealDebridAPIKey), imdbID),
	}

	allStreams := make([]TorrentioStream, 0)
	seenHashes := make(map[string]bool)

	for _, sourceURL := range sources {
		streams, err := d.querySource(sourceURL, "movie")
		if err != nil {
			log.Printf("[DMM Direct] Error querying %s: %v", sourceURL, err)
			continue
		}

		// Deduplicate by info hash
		for _, stream := range streams {
			if stream.InfoHash != "" && !seenHashes[stream.InfoHash] {
				seenHashes[stream.InfoHash] = true
				allStreams = append(allStreams, stream)
			}
		}
	}

	log.Printf("[DMM Direct] Found %d unique streams for movie %s", len(allStreams), imdbID)

	// Cache results
	d.Cache[cacheKey] = &DMMCachedResponse{
		Data:      allStreams,
		Timestamp: time.Now(),
	}

	return allStreams, nil
}

// GetSeriesStreams queries DMM sources directly for a series episode
func (d *DMMDirectProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	cacheKey := fmt.Sprintf("series_%s_s%de%d", imdbID, season, episode)
	
	// Check cache first (5 minute cache)
	if cached, ok := d.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 5*time.Minute {
			log.Printf("[DMM Direct] Cache hit for series %s S%dE%d (%d streams)", imdbID, season, episode, len(cached.Data))
			return cached.Data, nil
		}
	}

	log.Printf("[DMM Direct] Fetching streams for series %s S%dE%d", imdbID, season, episode)
	
	// Query multiple DMM sources
	sources := []string{
		fmt.Sprintf("https://torrentio.strem.fun/%s/stream/series/%s:%d:%d.json", d.getRDConfig(), imdbID, season, episode),
		fmt.Sprintf("https://comet.elfhosted.com/c/realdebrid=%s/stream/series/%s:%d:%d.json", url.QueryEscape(d.RealDebridAPIKey), imdbID, season, episode),
	}

	allStreams := make([]TorrentioStream, 0)
	seenHashes := make(map[string]bool)

	for _, sourceURL := range sources {
		streams, err := d.querySource(sourceURL, "series")
		if err != nil {
			log.Printf("[DMM Direct] Error querying %s: %v", sourceURL, err)
			continue
		}

		// Deduplicate by info hash
		for _, stream := range streams {
			if stream.InfoHash != "" && !seenHashes[stream.InfoHash] {
				seenHashes[stream.InfoHash] = true
				allStreams = append(allStreams, stream)
			}
		}
	}

	log.Printf("[DMM Direct] Found %d unique streams for series %s S%dE%d", len(allStreams), imdbID, season, episode)

	// Cache results
	d.Cache[cacheKey] = &DMMCachedResponse{
		Data:      allStreams,
		Timestamp: time.Now(),
	}

	return allStreams, nil
}

// querySource queries a single DMM source
func (d *DMMDirectProvider) querySource(sourceURL, mediaType string) ([]TorrentioStream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Streams []struct {
			Name          string `json:"name"`
			Title         string `json:"title"`
			InfoHash      string `json:"infoHash"`
			FileIdx       int    `json:"fileIdx"`
			URL           string `json:"url"`
			BehaviorHints struct {
				Filename  string `json:"filename"`
				VideoSize int64  `json:"videoSize"`
			} `json:"behaviorHints"`
		} `json:"streams"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	streams := make([]TorrentioStream, 0)
	for _, s := range result.Streams {
		stream := TorrentioStream{
			Name:     s.Name,
			Title:    s.Title,
			InfoHash: s.InfoHash,
			FileIdx:  s.FileIdx,
			URL:      s.URL,
			Cached:   true, // DMM sources only return cached torrents
			Source:   d.getSourceName(sourceURL),
		}
		stream.BehaviorHints.Filename = s.BehaviorHints.Filename
		stream.BehaviorHints.VideoSize = s.BehaviorHints.VideoSize
		stream.Size = s.BehaviorHints.VideoSize

		// Extract quality from title
		stream.Quality = extractQuality(s.Title)

		streams = append(streams, stream)
	}

	return streams, nil
}

// getRDConfig returns Real-Debrid configuration for Torrentio
func (d *DMMDirectProvider) getRDConfig() string {
	if d.RealDebridAPIKey == "" {
		return "realdebrid"
	}
	return fmt.Sprintf("realdebrid=%s", url.QueryEscape(d.RealDebridAPIKey))
}

// getSourceName extracts source name from URL
func (d *DMMDirectProvider) getSourceName(sourceURL string) string {
	if strings.Contains(sourceURL, "torrentio") {
		return "Torrentio"
	}
	if strings.Contains(sourceURL, "comet") {
		return "Comet"
	}
	return "DMM"
}

// extractQuality extracts quality info from title
func extractQuality(title string) string {
	title = strings.ToUpper(title)
	
	qualities := []string{"2160P", "4K", "UHD", "1080P", "720P", "480P"}
	for _, q := range qualities {
		if strings.Contains(title, q) {
			return q
		}
	}
	
	return "Unknown"
}
