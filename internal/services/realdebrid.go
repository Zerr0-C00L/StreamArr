package services

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

const (
	rdBaseURL = "https://api.real-debrid.com/rest/1.0"
)

type RealDebridClient struct {
	apiKey     string
	httpClient *http.Client
}

type rdTorrentInfo struct {
	ID          string   `json:"id"`
	Filename    string   `json:"filename"`
	Hash        string   `json:"hash"`
	Bytes       int64    `json:"bytes"`
	Host        string   `json:"host"`
	Status      string   `json:"status"`
	Added       string   `json:"added"`
	Links       []string `json:"links"`
}

type rdInstantAvailability struct {
	Hash string                 `json:"-"`
	Data map[string]interface{} `json:"-"`
}

type rdUnrestrictLink struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Filesize int64  `json:"filesize"`
	Link     string `json:"link"`
	Host     string `json:"host"`
	Download string `json:"download"`
}

func NewRealDebridClient(apiKey string) *RealDebridClient {
	return &RealDebridClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckInstantAvailability checks if torrents are instantly available
func (c *RealDebridClient) CheckInstantAvailability(ctx context.Context, infoHashes []string) (map[string]bool, error) {
	if len(infoHashes) == 0 {
		return make(map[string]bool), nil
	}

	// Real-Debrid allows checking up to 100 hashes at once
	const batchSize = 100
	availability := make(map[string]bool)

	for i := 0; i < len(infoHashes); i += batchSize {
		end := i + batchSize
		if end > len(infoHashes) {
			end = len(infoHashes)
		}
		batch := infoHashes[i:end]

		endpoint := fmt.Sprintf("%s/torrents/instantAvailability/%s", rdBaseURL, strings.Join(batch, "/"))
		
		data, err := c.makeRequest(ctx, "GET", endpoint, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check availability: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal availability: %w", err)
		}

		// Parse availability response
		for _, hash := range batch {
			hashData, exists := result[strings.ToLower(hash)]
			if exists {
				// If the hash exists in response and has data, it's available
				if hashMap, ok := hashData.(map[string]interface{}); ok && len(hashMap) > 0 {
					availability[hash] = true
				} else {
					availability[hash] = false
				}
			} else {
				availability[hash] = false
			}
		}
	}

	return availability, nil
}

// AddMagnet adds a magnet link to Real-Debrid
func (c *RealDebridClient) AddMagnet(ctx context.Context, magnetLink string) (string, error) {
	endpoint := fmt.Sprintf("%s/torrents/addMagnet", rdBaseURL)
	
	params := url.Values{}
	params.Set("magnet", magnetLink)

	data, err := c.makeRequest(ctx, "POST", endpoint, params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to add magnet: %w", err)
	}

	var result struct {
		ID  string `json:"id"`
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal add magnet response: %w", err)
	}

	return result.ID, nil
}

// SelectFiles selects all files from a torrent
func (c *RealDebridClient) SelectFiles(ctx context.Context, torrentID string, fileIDs []int) error {
	endpoint := fmt.Sprintf("%s/torrents/selectFiles/%s", rdBaseURL, torrentID)
	
	// Convert file IDs to comma-separated string
	fileIDStrs := make([]string, len(fileIDs))
	for i, id := range fileIDs {
		fileIDStrs[i] = fmt.Sprintf("%d", id)
	}
	
	params := url.Values{}
	params.Set("files", strings.Join(fileIDStrs, ","))

	_, err := c.makeRequest(ctx, "POST", endpoint, params, nil)
	if err != nil {
		return fmt.Errorf("failed to select files: %w", err)
	}

	return nil
}

// GetTorrentInfo retrieves information about a torrent
func (c *RealDebridClient) GetTorrentInfo(ctx context.Context, torrentID string) (*rdTorrentInfo, error) {
	endpoint := fmt.Sprintf("%s/torrents/info/%s", rdBaseURL, torrentID)
	
	data, err := c.makeRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent info: %w", err)
	}

	var info rdTorrentInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal torrent info: %w", err)
	}

	return &info, nil
}

// UnrestrictLink converts a Real-Debrid link to a direct download link
func (c *RealDebridClient) UnrestrictLink(ctx context.Context, link string) (*rdUnrestrictLink, error) {
	endpoint := fmt.Sprintf("%s/unrestrict/link", rdBaseURL)
	
	params := url.Values{}
	params.Set("link", link)

	data, err := c.makeRequest(ctx, "POST", endpoint, params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unrestrict link: %w", err)
	}

	var result rdUnrestrictLink
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal unrestrict response: %w", err)
	}

	return &result, nil
}

// GetStreamURL gets a direct streaming URL for a torrent
func (c *RealDebridClient) GetStreamURL(ctx context.Context, infoHash string) (string, error) {

	
	// Build magnet link
	magnetLink := fmt.Sprintf("magnet:?xt=urn:btih:%s", infoHash)

	// Add magnet to Real-Debrid
	torrentID, err := c.AddMagnet(ctx, magnetLink)
	if err != nil {
		return "", fmt.Errorf("failed to add magnet: %w", err)
	}
	fmt.Printf("[RD-DEBUG] Added magnet, torrent ID: %s\n", torrentID)

	// Wait for torrent to be ready (cached torrents are instant)
	time.Sleep(2 * time.Second)

	// Get torrent info to find the largest file
	info, err := c.GetTorrentInfo(ctx, torrentID)
	if err != nil {
		// Clean up the torrent if we can't get info
		_ = c.DeleteTorrent(ctx, torrentID)
		return "", fmt.Errorf("failed to get torrent info: %w", err)
	}
	fmt.Printf("[RD-DEBUG] Torrent status: %s, Links: %d, Bytes: %d\n", info.Status, len(info.Links), info.Bytes)

	// Check if torrent is ready - should be "downloaded" for cached torrents
	if info.Status != "downloaded" && info.Status != "waiting_files_selection" {
		_ = c.DeleteTorrent(ctx, torrentID)
		return "", fmt.Errorf("torrent not cached (status: %s)", info.Status)
	}

	// Select all files
	if info.Status == "waiting_files_selection" {
		fmt.Printf("[RD-DEBUG] Selecting files for torrent %s\n", torrentID)
		if err := c.SelectFiles(ctx, torrentID, []int{1}); err != nil {
			_ = c.DeleteTorrent(ctx, torrentID)
			return "", fmt.Errorf("failed to select files: %w", err)
		}
		time.Sleep(1 * time.Second)
		
		// Refresh info
		info, err = c.GetTorrentInfo(ctx, torrentID)
		if err != nil {
			_ = c.DeleteTorrent(ctx, torrentID)
			return "", fmt.Errorf("failed to refresh torrent info: %w", err)
		}
		fmt.Printf("[RD-DEBUG] After file selection - Status: %s, Links: %d\n", info.Status, len(info.Links))
		
		// If still not downloaded after selection, it's not cached - delete it
		if info.Status != "downloaded" {
			fmt.Printf("[RD-DEBUG] Torrent not instantly cached (status: %s), deleting...\n", info.Status)
			_ = c.DeleteTorrent(ctx, torrentID)
			return "", fmt.Errorf("torrent not cached on RD (status: %s)", info.Status)
		}
	}

	// Get the first link
	if len(info.Links) == 0 {
		_ = c.DeleteTorrent(ctx, torrentID)
		return "", fmt.Errorf("no download links available (status: %s)", info.Status)
	}

	fmt.Printf("[RD-DEBUG] Unrestricting link: %s\n", info.Links[0])
	// Unrestrict the link to get direct download URL
	unrestricted, err := c.UnrestrictLink(ctx, info.Links[0])
	if err != nil {
		_ = c.DeleteTorrent(ctx, torrentID)
		return "", fmt.Errorf("failed to unrestrict link: %w", err)
	}

	// Return the direct download URL for streaming
	// Use the 'download' field which is the actual streaming URL
	if unrestricted.Download != "" {
		fmt.Printf("[RD-DEBUG] Got download URL: %s\n", unrestricted.Download)
		return unrestricted.Download, nil
	}
	
	fmt.Printf("[RD-DEBUG] Got link URL: %s\n", unrestricted.Link)
	return unrestricted.Link, nil
}

// DeleteTorrent removes a torrent from Real-Debrid
func (c *RealDebridClient) DeleteTorrent(ctx context.Context, torrentID string) error {
	endpoint := fmt.Sprintf("%s/torrents/delete/%s", rdBaseURL, torrentID)
	
	_, err := c.makeRequest(ctx, "DELETE", endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete torrent: %w", err)
	}

	return nil
}

// makeRequest performs an HTTP request to Real-Debrid API with retry logic for rate limiting
func (c *RealDebridClient) makeRequest(ctx context.Context, method, endpoint string, params url.Values, body io.Reader) ([]byte, error) {
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Exponential backoff for retries: 2s, 4s, 8s
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
		
		reqURL := endpoint
		if method == "GET" && params != nil {
			reqURL = fmt.Sprintf("%s?%s", endpoint, params.Encode())
		}

		var reqBody io.Reader
		if method == "POST" && params != nil {
			reqBody = strings.NewReader(params.Encode())
		} else if body != nil {
			reqBody = body
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
		if method == "POST" && params != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Retry on 429 (Too Many Requests)
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited by Real-Debrid")
			if attempt < maxRetries-1 {
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Real-Debrid API returned status %d: %s", resp.StatusCode, string(data))
		}

		return data, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("max retries exceeded")
}

// TestConnection tests the Real-Debrid API connection
func (c *RealDebridClient) TestConnection(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/user", rdBaseURL)
	
	_, err := c.makeRequest(ctx, "GET", endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Real-Debrid: %w", err)
	}

	return nil
}
