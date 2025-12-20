package providers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type CometProvider struct {
	BaseURL          string
	RealDebridAPIKey string
	Indexers         []string
	OnlyShowCached   bool
	Client           *http.Client
	Cache            map[string]*CometCachedResponse
}

type CometConfig struct {
	Indexers       []string `json:"indexers"`
	DebridService  string   `json:"debridService"`
	DebridAPIKey   string   `json:"debridApiKey"`
	MaxResults     int      `json:"maxResults,omitempty"`
}

type CometStream struct {
	Name     string `json:"name"`
	InfoHash string `json:"infoHash"`
	FileIdx  int    `json:"fileIdx,omitempty"`
	URL      string `json:"url"`
}

type CometResponse struct {
	Streams []CometStream `json:"streams"`
}

type CometCachedResponse struct {
	Data      *CometResponse
	Timestamp time.Time
}

func NewCometProvider(rdAPIKey string, indexers []string, onlyShowCached bool) *CometProvider {
	if len(indexers) == 0 {
		indexers = []string{"bktorrent", "thepiratebay", "yts", "eztv"}
	}
	
	return &CometProvider{
		BaseURL:          "https://comet.elfhosted.com",
		RealDebridAPIKey: rdAPIKey,
		Indexers:         indexers,
		OnlyShowCached:   onlyShowCached,
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
		Cache: make(map[string]*CometCachedResponse),
	}
}

func (c *CometProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	config := CometConfig{
		Indexers:      c.Indexers,
		DebridService: "realdebrid",
		DebridAPIKey:  c.RealDebridAPIKey,
	}
	
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	
	configBase64 := base64.StdEncoding.EncodeToString(configJSON)
	url := fmt.Sprintf("%s/%s/stream/movie/%s.json", c.BaseURL, configBase64, imdbID)
	
	return c.fetchStreams(url, fmt.Sprintf("movie_%s", imdbID))
}

func (c *CometProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	config := CometConfig{
		Indexers:      c.Indexers,
		DebridService: "realdebrid",
		DebridAPIKey:  c.RealDebridAPIKey,
	}
	
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	
	configBase64 := base64.StdEncoding.EncodeToString(configJSON)
	url := fmt.Sprintf("%s/%s/stream/series/%s:%d:%d.json", c.BaseURL, configBase64, imdbID, season, episode)
	
	return c.fetchStreams(url, fmt.Sprintf("series_%s_%d_%d", imdbID, season, episode))
}

func (c *CometProvider) fetchStreams(url, cacheKey string) ([]TorrentioStream, error) {
	// Check cache
	if cached, ok := c.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 30*time.Minute {
			return c.convertToTorrentioStreams(cached.Data.Streams), nil
		}
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch streams: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	
	var response CometResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	
	// Cache the response
	c.Cache[cacheKey] = &CometCachedResponse{
		Data:      &response,
		Timestamp: time.Now(),
	}
	
	return c.convertToTorrentioStreams(response.Streams), nil
}

func (c *CometProvider) convertToTorrentioStreams(cometStreams []CometStream) []TorrentioStream {
	streams := make([]TorrentioStream, 0, len(cometStreams))
	
	for _, cs := range cometStreams {
		// Normalize RD indicator
		name := cs.Name
		if strings.Contains(name, "[RD⚡]") {
			name = strings.ReplaceAll(name, "[RD⚡]", "[RD+]")
		}
		
		stream := TorrentioStream{
			Name:     name,
			Title:    name,
			InfoHash: cs.InfoHash,
			FileIdx:  cs.FileIdx,
			URL:      cs.URL,
			Source:   "comet",
		}
		
		// Parse stream info
		parseStreamInfoForComet(&stream)
		
		// Filter: if OnlyShowCached is enabled, skip non-cached streams
		if c.OnlyShowCached && !stream.Cached {
			continue
		}
		
		streams = append(streams, stream)
	}
	
	return streams
}

func parseStreamInfoForComet(stream *TorrentioStream) {
	name := stream.Name
	
	// Extract quality
	qualityPatterns := []string{"2160p", "4K", "1080p", "720p", "480p"}
	for _, q := range qualityPatterns {
		if strings.Contains(name, q) {
			stream.Quality = strings.ToLower(strings.TrimSuffix(q, "p"))
			if q == "4K" {
				stream.Quality = "2160"
			}
			break
		}
	}
	
	// Extract file size
	if strings.Contains(name, "GB") || strings.Contains(name, "MB") {
		parts := strings.Split(name, " ")
		for i, part := range parts {
			if strings.Contains(part, "GB") {
				if i > 0 {
					var size float64
					fmt.Sscanf(parts[i-1], "%f", &size)
					stream.Size = int64(size * 1024 * 1024 * 1024)
				}
			} else if strings.Contains(part, "MB") {
				if i > 0 {
					var size float64
					fmt.Sscanf(parts[i-1], "%f", &size)
					stream.Size = int64(size * 1024 * 1024)
				}
			}
		}
	}
	
	// Check if cached
	stream.Cached = strings.Contains(name, "[RD+]") || 
	                strings.Contains(name, "[RD⚡]") ||
	                strings.Contains(name, "⚡")
}
