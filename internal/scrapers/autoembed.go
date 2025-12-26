package scrapers

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// AutoEmbedClient handles AutoEmbed.cc scraping operations
type AutoEmbedClient struct {
	baseURL string
	client  *http.Client
}

// AutoEmbedStream represents a stream from AutoEmbed
type AutoEmbedStream struct {
	Server string `json:"server"`
	Region string `json:"region"`
	Link   string `json:"link"`
}

// NewAutoEmbedClient creates a new AutoEmbed scraper client
func NewAutoEmbedClient() *AutoEmbedClient {
	return &AutoEmbedClient{
		baseURL: "https://player.autoembed.cc/embed/",
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// fetchHTML makes HTTP request and returns HTML content
func (a *AutoEmbedClient) fetchHTML(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", a.baseURL)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// ExtractStreams extracts streams from AutoEmbed for a given media
func (a *AutoEmbedClient) ExtractStreams(mediaType, tmdbID string, season, episode *int, region string) (*AutoEmbedStream, error) {
	// Build URL
	var embedURL string
	if mediaType == "movie" {
		embedURL = fmt.Sprintf("%smovie/%s?server=1", a.baseURL, tmdbID)
	} else if mediaType == "series" && season != nil && episode != nil {
		embedURL = fmt.Sprintf("%stv/%s/%d/%d?server=1", a.baseURL, tmdbID, *season, *episode)
	} else {
		return nil, fmt.Errorf("invalid parameters for media type")
	}

	// Fetch HTML content
	html, err := a.fetchHTML(embedURL)
	if err != nil {
		return nil, err
	}

	// Determine allowed regions
	allowedRegions := []string{}
	if region == "en" || region == "gb" || region == "us" {
		allowedRegions = []string{"gb", "us"}
	} else {
		allowedRegions = []string{region}
	}

	// Extract data-server values
	serverRegex := regexp.MustCompile(`data-server="([^"]+)"`)
	serverMatches := serverRegex.FindAllStringSubmatch(html, -1)

	// Extract flag src values
	flagRegex := regexp.MustCompile(`<img src="([^"]+)"`)
	flagMatches := flagRegex.FindAllStringSubmatch(html, -1)

	if len(serverMatches) == 0 || len(flagMatches) == 0 {
		return nil, fmt.Errorf("no servers or flags found in HTML")
	}

	// Process each server and filter by region
	for i, serverMatch := range serverMatches {
		if i >= len(flagMatches) {
			break
		}

		encodedServer := serverMatch[1]
		flagURL := flagMatches[i][1]

		// Extract region from flag URL (e.g., /flagsapi.com/GB/)
		flagRegion := ""
		flagIdx := strings.Index(flagURL, "/flagsapi.com/")
		if flagIdx != -1 && flagIdx+14 < len(flagURL) {
			flagRegion = strings.ToLower(flagURL[flagIdx+14 : flagIdx+16])
		}

		// Check if region is allowed
		regionAllowed := false
		for _, allowed := range allowedRegions {
			if flagRegion == allowed {
				regionAllowed = true
				break
			}
		}
		if !regionAllowed {
			continue
		}

		// Decode server URL (base64)
		decodedURL, err := a.decodeBase64URL(encodedServer)
		if err != nil {
			continue
		}

		// Fetch server response
		serverHTML, err := a.fetchHTML(decodedURL)
		if err != nil {
			continue
		}

		// Extract file link using regex (supports both formats)
		fileRegex := regexp.MustCompile(`file:\s*"([^"]+)"|"file":\s*"([^"]+)"`)
		fileMatch := fileRegex.FindStringSubmatch(serverHTML)

		if len(fileMatch) > 1 {
			fileURL := ""
			if fileMatch[1] != "" {
				fileURL = fileMatch[1]
			} else if fileMatch[2] != "" {
				fileURL = fileMatch[2]
			}

			if fileURL != "" {
				return &AutoEmbedStream{
					Server: fmt.Sprintf("autoembed-%d", i+1),
					Region: flagRegion,
					Link:   fileURL,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no valid stream found for region %s", region)
}

// decodeBase64URL decodes base64 encoded URL
func (a *AutoEmbedClient) decodeBase64URL(encoded string) (string, error) {
	decoded, err := base64Decode(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// base64Decode helper function
func base64Decode(s string) ([]byte, error) {
	// Try standard base64 first
	decoded, err := base64URLDecode(s)
	if err == nil {
		return decoded, nil
	}
	// Fallback to URL-safe base64
	return base64URLDecode(s)
}

func base64URLDecode(s string) ([]byte, error) {
	// Handle both standard and URL-safe base64
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	
	return base64DecodeStandard(s)
}

func base64DecodeStandard(s string) ([]byte, error) {
	decoded := make([]byte, len(s)*3/4)
	n, err := decodeBase64(decoded, []byte(s))
	if err != nil {
		return nil, err
	}
	return decoded[:n], nil
}

func decodeBase64(dst, src []byte) (int, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	
	n := 0
	for i := 0; i < len(src); i += 4 {
		if i+3 >= len(src) {
			break
		}
		
		// Decode 4 base64 chars to 3 bytes
		b1 := indexOf(alphabet, src[i])
		b2 := indexOf(alphabet, src[i+1])
		b3 := indexOf(alphabet, src[i+2])
		b4 := indexOf(alphabet, src[i+3])
		
		if b1 < 0 || b2 < 0 {
			return 0, fmt.Errorf("invalid base64 character")
		}
		
		dst[n] = byte((b1 << 2) | (b2 >> 4))
		n++
		
		if b3 >= 0 && src[i+2] != '=' {
			dst[n] = byte((b2 << 4) | (b3 >> 2))
			n++
		}
		
		if b4 >= 0 && src[i+3] != '=' {
			dst[n] = byte((b3 << 6) | b4)
			n++
		}
	}
	
	return n, nil
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// GetMovieStream is a convenience method for movies
func (a *AutoEmbedClient) GetMovieStream(tmdbID, region string) (*AutoEmbedStream, error) {
	return a.ExtractStreams("movie", tmdbID, nil, nil, region)
}

// GetSeriesStream is a convenience method for TV series
func (a *AutoEmbedClient) GetSeriesStream(tmdbID string, season, episode int, region string) (*AutoEmbedStream, error) {
	return a.ExtractStreams("series", tmdbID, &season, &episode, region)
}
