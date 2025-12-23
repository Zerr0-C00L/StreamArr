package debrid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	realDebridBaseURL = "https://api.real-debrid.com/rest/1.0"
)

// RealDebrid implements DebridService for Real-Debrid
type RealDebrid struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewRealDebrid creates a new Real-Debrid service instance
func NewRealDebrid(apiKey string, logger *slog.Logger) *RealDebrid {
	return &RealDebrid{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// CheckCache checks which hashes are cached on Real-Debrid
func (rd *RealDebrid) CheckCache(ctx context.Context, hashes []string) (map[string]bool, error) {
	if len(hashes) == 0 {
		return make(map[string]bool), nil
	}

	// Real-Debrid instant availability endpoint
	// POST /torrents/instantAvailability/{hash1}/{hash2}/...
	url := fmt.Sprintf("%s/torrents/instantAvailability/%s",
		realDebridBaseURL,
		strings.Join(hashes, "/"))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+rd.apiKey)

	resp, err := rd.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("real-debrid API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	// Format: { "hash1": { "rd": [{ "files": [...] }] }, "hash2": {} }
	var availability map[string]map[string][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&availability); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Build result map
	cached := make(map[string]bool, len(hashes))
	for _, hash := range hashes {
		hashLower := strings.ToLower(hash)
		if rdData, exists := availability[hashLower]; exists && len(rdData["rd"]) > 0 {
			cached[hash] = true
		} else {
			cached[hash] = false
		}
	}

	rd.logger.Info("Checked Real-Debrid cache",
		"total", len(hashes),
		"cached", countCached(cached))

	return cached, nil
}

// GetStreamURL returns the direct streaming URL for a cached hash
func (rd *RealDebrid) GetStreamURL(ctx context.Context, hash string, fileIndex int) (string, error) {
	// Step 1: Add magnet to Real-Debrid
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)

	addURL := fmt.Sprintf("%s/torrents/addMagnet", realDebridBaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", addURL, strings.NewReader(fmt.Sprintf("magnet=%s", magnetURL)))
	if err != nil {
		return "", fmt.Errorf("create add magnet request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+rd.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := rd.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("add magnet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("add magnet failed (status %d): %s", resp.StatusCode, string(body))
	}

	var addResult struct {
		ID  string `json:"id"`
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&addResult); err != nil {
		return "", fmt.Errorf("decode add magnet response: %w", err)
	}

	// Step 2: Select files (all files)
	selectURL := fmt.Sprintf("%s/torrents/selectFiles/%s", realDebridBaseURL, addResult.ID)
	selectReq, err := http.NewRequestWithContext(ctx, "POST", selectURL, strings.NewReader("files=all"))
	if err != nil {
		return "", fmt.Errorf("create select files request: %w", err)
	}

	selectReq.Header.Set("Authorization", "Bearer "+rd.apiKey)
	selectReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	selectResp, err := rd.httpClient.Do(selectReq)
	if err != nil {
		return "", fmt.Errorf("select files: %w", err)
	}
	defer selectResp.Body.Close()

	// Step 3: Get torrent info to find download link
	infoURL := fmt.Sprintf("%s/torrents/info/%s", realDebridBaseURL, addResult.ID)
	infoReq, err := http.NewRequestWithContext(ctx, "GET", infoURL, nil)
	if err != nil {
		return "", fmt.Errorf("create info request: %w", err)
	}

	infoReq.Header.Set("Authorization", "Bearer "+rd.apiKey)

	infoResp, err := rd.httpClient.Do(infoReq)
	if err != nil {
		return "", fmt.Errorf("get torrent info: %w", err)
	}
	defer infoResp.Body.Close()

	var torrentInfo struct {
		Links []string `json:"links"`
	}
	if err := json.NewDecoder(infoResp.Body).Decode(&torrentInfo); err != nil {
		return "", fmt.Errorf("decode torrent info: %w", err)
	}

	if len(torrentInfo.Links) == 0 {
		return "", fmt.Errorf("no download links available")
	}

	// Step 4: Unrestrict the link to get direct download URL
	unrestrictURL := fmt.Sprintf("%s/unrestrict/link", realDebridBaseURL)
	unrestrictReq, err := http.NewRequestWithContext(ctx, "POST", unrestrictURL,
		strings.NewReader(fmt.Sprintf("link=%s", torrentInfo.Links[0])))
	if err != nil {
		return "", fmt.Errorf("create unrestrict request: %w", err)
	}

	unrestrictReq.Header.Set("Authorization", "Bearer "+rd.apiKey)
	unrestrictReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	unrestrictResp, err := rd.httpClient.Do(unrestrictReq)
	if err != nil {
		return "", fmt.Errorf("unrestrict link: %w", err)
	}
	defer unrestrictResp.Body.Close()

	var unrestrictResult struct {
		Download string `json:"download"`
	}
	if err := json.NewDecoder(unrestrictResp.Body).Decode(&unrestrictResult); err != nil {
		return "", fmt.Errorf("decode unrestrict response: %w", err)
	}

	return unrestrictResult.Download, nil
}

// GetAvailableFiles returns list of files in a cached torrent
func (rd *RealDebrid) GetAvailableFiles(ctx context.Context, hash string) ([]TorrentFile, error) {
	// For instant availability, we need to check the cache status first
	cached, err := rd.CheckCache(ctx, []string{hash})
	if err != nil {
		return nil, fmt.Errorf("check cache: %w", err)
	}

	if !cached[hash] {
		return nil, fmt.Errorf("torrent not cached")
	}

	// Note: Full implementation would parse the instant availability response
	// to get detailed file information. For now, return basic info.
	return []TorrentFile{}, nil
}

// GetServiceName returns the service name
func (rd *RealDebrid) GetServiceName() string {
	return "Real-Debrid"
}

// IsAuthenticated checks if API key is valid
func (rd *RealDebrid) IsAuthenticated(ctx context.Context) bool {
	url := fmt.Sprintf("%s/user", realDebridBaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+rd.apiKey)

	resp, err := rd.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Helper function to count cached hashes
func countCached(cached map[string]bool) int {
	count := 0
	for _, isCached := range cached {
		if isCached {
			count++
		}
	}
	return count
}
