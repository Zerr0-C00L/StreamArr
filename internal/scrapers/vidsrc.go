package scrapers

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VidSrcClient handles VidSrc.rip scraping operations
type VidSrcClient struct {
	baseURL string
	client  *http.Client
}

// VidSrcStream represents a stream source from VidSrc
type VidSrcStream struct {
	Server string `json:"server"`
	Type   string `json:"type"` // mp4 or m3u8
	Link   string `json:"link"`
}

// VidSrcResponse represents the API response
type VidSrcResponse struct {
	Sources []struct {
		File string `json:"file"`
	} `json:"sources"`
}

// NewVidSrcClient creates a new VidSrc scraper client
func NewVidSrcClient() *VidSrcClient {
	return &VidSrcClient{
		baseURL: "https://vidsrc.rip",
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// fetchURL makes HTTP request and returns response body
func (v *VidSrcClient) fetchURL(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// xorEncryptDecrypt performs XOR encryption/decryption
func (v *VidSrcClient) xorEncryptDecrypt(key, data []byte) []byte {
	result := make([]byte, len(data))
	keyLen := len(key)

	for i, b := range data {
		result[i] = b ^ key[i%keyLen]
	}

	return result
}

// fetchKeyFromImage fetches the encryption key from VidSrc's skip button image
func (v *VidSrcClient) fetchKeyFromImage() ([]byte, error) {
	return v.fetchURL(v.baseURL + "/images/skip-button.png")
}

// generateVRF generates the VRF parameter for API authentication
func (v *VidSrcClient) generateVRF(sourceIdentifier, tmdbID string) (string, error) {
	key, err := v.fetchKeyFromImage()
	if err != nil {
		return "", fmt.Errorf("failed to fetch key: %w", err)
	}

	input := fmt.Sprintf("/api/source/%s/%s", sourceIdentifier, tmdbID)
	decodedInput, err := url.QueryUnescape(input)
	if err != nil {
		return "", err
	}

	xorResult := v.xorEncryptDecrypt(key, []byte(decodedInput))
	vrf := url.QueryEscape(base64.StdEncoding.EncodeToString(xorResult))

	return vrf, nil
}

// buildAPIURL constructs the VidSrc API URL with VRF
func (v *VidSrcClient) buildAPIURL(sourceIdentifier, tmdbID string, season, episode *int) (string, error) {
	vrf, err := v.generateVRF(sourceIdentifier, tmdbID)
	if err != nil {
		return "", err
	}

	apiURL := fmt.Sprintf("%s/api/source/%s/%s?vrf=%s", v.baseURL, sourceIdentifier, tmdbID, vrf)

	if season != nil && episode != nil {
		apiURL += fmt.Sprintf("&s=%d&e=%d", *season, *episode)
	}

	return apiURL, nil
}

// GetStreams fetches available streams from VidSrc.rip
func (v *VidSrcClient) GetStreams(tmdbID string, mediaType string, season, episode *int) ([]VidSrcStream, error) {
	sources := []string{"flixhq", "vidsrcuk", "vidsrcicu"}
	var streams []VidSrcStream

	for _, source := range sources {
		apiURL, err := v.buildAPIURL(source, tmdbID, season, episode)
		if err != nil {
			continue
		}

		body, err := v.fetchURL(apiURL)
		if err != nil {
			continue
		}

		// Parse JSON response manually (simple approach)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, `"sources"`) {
			continue
		}

		// Extract file URL from sources array
		startIdx := strings.Index(bodyStr, `"file":"`)
		if startIdx == -1 {
			continue
		}
		startIdx += 8 // length of `"file":"`

		endIdx := strings.Index(bodyStr[startIdx:], `"`)
		if endIdx == -1 {
			continue
		}

		fileURL := bodyStr[startIdx : startIdx+endIdx]
		if fileURL == "" {
			continue
		}

		// Determine stream type
		streamType := "m3u8"
		if strings.Contains(fileURL, ".mp4") {
			streamType = "mp4"
		}

		streams = append(streams, VidSrcStream{
			Server: source,
			Type:   streamType,
			Link:   fileURL,
		})
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found for TMDB ID %s", tmdbID)
	}

	return streams, nil
}

// GetMovieStreams is a convenience method for movies
func (v *VidSrcClient) GetMovieStreams(tmdbID string) ([]VidSrcStream, error) {
	return v.GetStreams(tmdbID, "movie", nil, nil)
}

// GetSeriesStreams is a convenience method for TV series
func (v *VidSrcClient) GetSeriesStreams(tmdbID string, season, episode int) ([]VidSrcStream, error) {
	return v.GetStreams(tmdbID, "series", &season, &episode)
}
