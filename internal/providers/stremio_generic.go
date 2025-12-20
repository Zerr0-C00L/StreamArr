package providers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GenericStremioProvider works with any Stremio addon that follows the standard protocol
type GenericStremioProvider struct {
	Name             string
	BaseURL          string
	RealDebridAPIKey string
	Client           *http.Client
	Cache            map[string]*GenericStreamCachedResponse
	rateLimiter      chan struct{} // Semaphore to limit concurrent requests
}

type GenericStreamCachedResponse struct {
	Data      *GenericStreamResponse
	Timestamp time.Time
}

type GenericStreamResponse struct {
	Streams []GenericStream `json:"streams"`
}

type GenericBehaviorHints struct {
	Filename  string `json:"filename"`
	VideoSize int64  `json:"videoSize"`
}

type GenericStream struct {
	Name          string               `json:"name"`
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	InfoHash      string               `json:"infoHash"`
	FileIdx       int                  `json:"fileIdx,omitempty"`
	URL           string               `json:"url"`
	BehaviorHints GenericBehaviorHints `json:"behaviorHints"`
}

func NewGenericStremioProvider(name, baseURL, rdAPIKey string) *GenericStremioProvider {
	return &GenericStremioProvider{
		Name:             name,
		BaseURL:          baseURL,
		RealDebridAPIKey: rdAPIKey,
		Client: &http.Client{
			Timeout: 120 * time.Second,
		},
		Cache:       make(map[string]*GenericStreamCachedResponse),
		rateLimiter: make(chan struct{}, 2), // Max 2 concurrent requests to avoid overwhelming addon
	}
}

// buildConfigURL builds the appropriate URL based on addon type
func (g *GenericStremioProvider) buildConfigURL(contentType, imdbID string, season, episode *int) string {
	var contentPath string
	if season != nil && episode != nil {
		contentPath = fmt.Sprintf("stream/series/%s:%d:%d.json", imdbID, *season, *episode)
	} else {
		contentPath = fmt.Sprintf("stream/movie/%s.json", imdbID)
	}
	
	baseURL := g.BaseURL
	
	// If the URL already contains manifest.json, replace it with the stream path
	// This handles pre-configured addon URLs like Torrentio, Autostream, Sootio, etc.
	if strings.Contains(baseURL, "/manifest.json") {
		return strings.Replace(baseURL, "/manifest.json", "/"+contentPath, 1)
	}
	if strings.Contains(baseURL, "manifest.json") {
		return strings.Replace(baseURL, "manifest.json", contentPath, 1)
	}
	
	// If no Real-Debrid key, just use base URL
	if g.RealDebridAPIKey == "" {
		return fmt.Sprintf("%s/%s", baseURL, contentPath)
	}
	
	// Detect addon type by URL and use appropriate config format
	lowerName := strings.ToLower(g.Name)
	lowerURL := strings.ToLower(baseURL)
	
	var configJSON []byte
	
	if strings.Contains(lowerName, "comet") || strings.Contains(lowerURL, "comet") {
		// Comet format
		config := map[string]interface{}{
			"indexers":      []string{"bitorrent", "thepiratebay", "yts", "eztv", "kickasstorrents", "torrentgalaxy"},
			"debridService": "realdebrid",
			"debridApiKey":  g.RealDebridAPIKey,
		}
		configJSON, _ = json.Marshal(config)
	} else if strings.Contains(lowerName, "torrentio") || strings.Contains(lowerURL, "torrentio") {
		// Torrentio format - config in URL path
		// Torrentio uses a different URL format: /providers=yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy,magnetdl,horriblesubs,nyaasi,tokyotosho,anidex|realdebrid=API_KEY/stream/...
		configPath := fmt.Sprintf("providers=yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy|realdebrid=%s", g.RealDebridAPIKey)
		return fmt.Sprintf("%s/%s/%s", g.BaseURL, configPath, contentPath)
	} else if strings.Contains(lowerName, "mediafusion") || strings.Contains(lowerURL, "mediafusion") {
		// MediaFusion format
		config := map[string]interface{}{
			"streaming_provider": map[string]string{
				"token":   g.RealDebridAPIKey,
				"service": "realdebrid",
			},
		}
		configJSON, _ = json.Marshal(config)
	} else {
		// Default: try MediaFusion format (most common)
		config := map[string]interface{}{
			"streaming_provider": map[string]string{
				"token":   g.RealDebridAPIKey,
				"service": "realdebrid",
			},
		}
		configJSON, _ = json.Marshal(config)
	}
	
	configBase64 := base64.StdEncoding.EncodeToString(configJSON)
	return fmt.Sprintf("%s/%s/%s", g.BaseURL, configBase64, contentPath)
}

func (g *GenericStremioProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	url := g.buildConfigURL("movie", imdbID, nil, nil)
	return g.fetchStreams(url, fmt.Sprintf("%s_movie_%s", g.Name, imdbID))
}

func (g *GenericStremioProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	url := g.buildConfigURL("series", imdbID, &season, &episode)
	return g.fetchStreams(url, fmt.Sprintf("%s_series_%s_%d_%d", g.Name, imdbID, season, episode))
}

func (g *GenericStremioProvider) fetchStreams(url, cacheKey string) ([]TorrentioStream, error) {
	// Check cache
	if cached, ok := g.Cache[cacheKey]; ok {
		if time.Since(cached.Timestamp) < 30*time.Minute {
			return g.convertToTorrentioStreams(cached.Data.Streams), nil
		}
	}
	
	// Rate limiting: max 2 concurrent requests to avoid overwhelming the addon
	g.rateLimiter <- struct{}{}        // Acquire
	defer func() { <-g.rateLimiter }() // Release
	
	// Retry logic for rate limiting and transient errors
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Exponential backoff: 1s, 2s, 4s
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("[RETRY] Attempt %d for %s (backing off %v)", attempt+1, g.Name, backoff)
			time.Sleep(backoff)
		}
		
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		
		resp, err := g.Client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		
		// Retry on 429 (rate limit) or 503 (service unavailable)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			lastErr = fmt.Errorf("rate limited/service unavailable: %d", resp.StatusCode)
			if attempt < maxRetries-1 {
				continue
			}
			return nil, lastErr
		}
		
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		
		// Check if response is an error from the addon
		var errorResponse struct {
			Err     string `json:"err"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			if errorResponse.Err != "" {
				lastErr = fmt.Errorf("addon error: %s", errorResponse.Err)
				if strings.Contains(strings.ToLower(errorResponse.Err), "too many") || 
				   strings.Contains(strings.ToLower(errorResponse.Err), "rate limit") {
					// Retry on rate limit errors
					if attempt < maxRetries-1 {
						continue
					}
				}
				return nil, lastErr
			}
			if errorResponse.Error != "" {
				return nil, fmt.Errorf("addon error: %s", errorResponse.Error)
			}
			if errorResponse.Message != "" {
				return nil, fmt.Errorf("addon error: %s", errorResponse.Message)
			}
		}
		
		var response GenericStreamResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		
		// Cache the response
		g.Cache[cacheKey] = &GenericStreamCachedResponse{
			Data:      &response,
			Timestamp: time.Now(),
		}
		
		return g.convertToTorrentioStreams(response.Streams), nil
	}
	
	// All retries failed
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch streams after %d attempts", maxRetries)
}

func (g *GenericStremioProvider) convertToTorrentioStreams(genericStreams []GenericStream) []TorrentioStream {
	streams := make([]TorrentioStream, len(genericStreams))
	
	for i, gs := range genericStreams {
		// Use filename from behaviorHints if available, otherwise fall back to title/name
		filename := gs.BehaviorHints.Filename
		if filename == "" {
			filename = gs.Title
		}
		if filename == "" {
			filename = gs.Name
		}
		
		// Use videoSize from behaviorHints if available
		size := gs.BehaviorHints.VideoSize
		
		// If no size from behaviorHints, try to parse from title first (Torrentio format)
		if size == 0 && gs.Title != "" {
			size = parseSizeFromDescription(gs.Title)
		}
		
		// Then try description
		if size == 0 && gs.Description != "" {
			size = parseSizeFromDescription(gs.Description)
		}
		
		// Check if stream is cached in debrid service
		// Look for explicit cache indicators in the name field
		// TorrentsDB uses "[RD download]", some addons use "[RD+]" or "⚡"
		nameLower := strings.ToLower(gs.Name)
		cached := false
		cacheIndicator := ""
		
		if strings.Contains(gs.Name, "[RD download]") {
			cached = true
			cacheIndicator = "[RD download]"
		} else if strings.Contains(gs.Name, "[RD+]") {
			cached = true
			cacheIndicator = "[RD+]"
		} else if strings.Contains(gs.Name, "⚡") {
			cached = true
			cacheIndicator = "⚡"
		} else if strings.Contains(nameLower, "⚡ cached") {
			cached = true
			cacheIndicator = "⚡ cached"
		} else if strings.Contains(nameLower, "instant available") {
			cached = true
			cacheIndicator = "instant available"
		}
		
		// Log the cached determination for debugging - this will help us see what indicators TorrentsDB is actually sending
		log.Printf("[CACHED-CHECK] Stream: %s | Cached: %v | Indicator Found: %s | Full Name: %s", 
			filename, cached, cacheIndicator, gs.Name)
		
		// Extract quality from name, title, or filename
		// TorrentsDB puts quality in the name field like "2160p", "1080p", etc.
		quality := extractQuality(gs.Name)
		if quality == "" {
			quality = extractQuality(gs.Title)
		}
		if quality == "" {
			quality = extractQuality(filename)
		}
		
		stream := TorrentioStream{
			Name:     gs.Name,
			Title:    filename, // Use the actual filename here
			InfoHash: gs.InfoHash,
			FileIdx:  gs.FileIdx,
			URL:      gs.URL,
			Source:   g.Name,
			Size:     size,
			Cached:   cached,
			Quality:  quality,
		}
		
		streams[i] = stream
	}
	
	return streams
}

// parseSizeFromDescription extracts file size from description text like "💾 1.5 GB" or "💾 500 MB"
func parseSizeFromDescription(desc string) int64 {
	// Look for size pattern in description
	// Common formats: "💾 1.5 GB", "💾 500 MB", "Size: 1.5GB"
	desc = strings.ToUpper(desc)
	
	// Find GB pattern
	if idx := strings.Index(desc, "GB"); idx > 0 {
		// Look backwards for the number
		numStr := extractNumberBefore(desc, idx)
		if num, err := strconv.ParseFloat(numStr, 64); err == nil {
			return int64(num * 1024 * 1024 * 1024)
		}
	}
	
	// Find MB pattern
	if idx := strings.Index(desc, "MB"); idx > 0 {
		numStr := extractNumberBefore(desc, idx)
		if num, err := strconv.ParseFloat(numStr, 64); err == nil {
			return int64(num * 1024 * 1024)
		}
	}
	
	return 0
}

// extractNumberBefore extracts a number from the string before the given index
func extractNumberBefore(s string, idx int) string {
	// Go backwards from idx to find the number
	end := idx
	for end > 0 && s[end-1] == ' ' {
		end--
	}
	
	start := end
	hasDecimal := false
	for start > 0 {
		c := s[start-1]
		if c >= '0' && c <= '9' {
			start--
		} else if c == '.' && !hasDecimal {
			hasDecimal = true
			start--
		} else if c == ' ' || c == ':' || c == 160 { // 160 is non-breaking space
			break
		} else {
			break
		}
	}
	
	return strings.TrimSpace(s[start:end])
}
