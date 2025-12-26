package livetv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DaddyLiveSource implements Channel source for DaddyLive
type DaddyLiveSource struct {
	baseURL string
	client  *http.Client
}

func NewDaddyLiveSource() *DaddyLiveSource {
	return &DaddyLiveSource{
		baseURL: "https://cdn.daddylivehd.sx",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *DaddyLiveSource) Name() string {
	return "DaddyLive"
}

func (d *DaddyLiveSource) GetChannels() ([]*Channel, error) {
	// DaddyLive uses a specific channels list JSON endpoint
	url := fmt.Sprintf("%s/channels.json", d.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://dlhd.so/")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse DaddyLive JSON format
	var daddyChannels map[string]struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Logo     string `json:"logo"`
		Stream   string `json:"stream"`
	}

	if err := json.Unmarshal(body, &daddyChannels); err != nil {
		return nil, fmt.Errorf("failed to parse channels: %w", err)
	}

	channels := make([]*Channel, 0, len(daddyChannels))
	for id, ch := range daddyChannels {
		streamURL := fmt.Sprintf("%s/stream/%s.m3u8", d.baseURL, ch.Stream)
		
		channels = append(channels, &Channel{
			ID:       id,
			Name:     ch.Name,
			Logo:     ch.Logo,
			Category: ch.Category,
			StreamURL: streamURL,
			Active:   true,
			Source:   "DaddyLive",
		})
	}

	return channels, nil
}

// DrewLiveSource implements Channel source for DrewLive
type DrewLiveSource struct {
	baseURL string
	client  *http.Client
}

func NewDrewLiveSource() *DrewLiveSource {
	return &DrewLiveSource{
		baseURL: "https://vipstreams.in",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (d *DrewLiveSource) Name() string {
	return "DrewLive"
}

func (d *DrewLiveSource) GetChannels() ([]*Channel, error) {
	// DrewLive channels are scraped from the website
	url := fmt.Sprintf("%s/channels", d.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML to extract channels
	channels := d.parseChannelsFromHTML(string(body))
	return channels, nil
}

func (d *DrewLiveSource) parseChannelsFromHTML(html string) []*Channel {
	channels := []*Channel{}
	
	// Regex patterns to extract channel data
	channelPattern := regexp.MustCompile(`data-channel-id="([^"]+)"[^>]*>`)
	namePattern := regexp.MustCompile(`<h3[^>]*>([^<]+)</h3>`)
	logoPattern := regexp.MustCompile(`<img[^>]*src="([^"]+)"`)

	matches := channelPattern.FindAllStringSubmatch(html, -1)
	names := namePattern.FindAllStringSubmatch(html, -1)
	logos := logoPattern.FindAllStringSubmatch(html, -1)

	for i, match := range matches {
		if i >= len(names) {
			break
		}

		channelID := match[1]
		channelName := strings.TrimSpace(names[i][1])
		logo := ""
		if i < len(logos) {
			logo = logos[i][1]
		}

		channels = append(channels, &Channel{
			ID:       channelID,
			Name:     channelName,
			Logo:     logo,
			Category: "General",
			StreamURL: fmt.Sprintf("%s/stream/%s", d.baseURL, channelID),
			Active:   true,
			Source:   "DrewLive",
		})
	}

	return channels
}

// TheTVAppSource implements Channel source for TheTVApp
type TheTVAppSource struct {
	baseURL string
	client  *http.Client
}

func NewTheTVAppSource() *TheTVAppSource {
	return &TheTVAppSource{
		baseURL: "https://thetvapp.to",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *TheTVAppSource) Name() string {
	return "TheTVApp"
}

func (t *TheTVAppSource) GetChannels() ([]*Channel, error) {
	// TheTVApp has multiple categories
	categories := []string{"usa", "uk", "sports", "entertainment", "news"}
	allChannels := []*Channel{}

	for _, category := range categories {
		url := fmt.Sprintf("%s/api/channels/%s", t.baseURL, category)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Accept", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var channels []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Logo     string `json:"logo"`
			StreamURL string `json:"stream_url"`
		}

		if err := json.Unmarshal(body, &channels); err != nil {
			continue
		}

		for _, ch := range channels {
			allChannels = append(allChannels, &Channel{
				ID:       ch.ID,
				Name:     ch.Name,
				Logo:     ch.Logo,
				Category: strings.Title(category),
				StreamURL: ch.StreamURL,
				Active:   true,
				Source:   "TheTVApp",
			})
		}
	}

	return allChannels, nil
}

// MoveOnJoySource implements Channel source for MoveOnJoy
type MoveOnJoySource struct {
	baseURL string
	client  *http.Client
}

func NewMoveOnJoySource() *MoveOnJoySource {
	return &MoveOnJoySource{
		baseURL: "https://moveonjoy.com",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (m *MoveOnJoySource) Name() string {
	return "MoveOnJoy"
}

func (m *MoveOnJoySource) GetChannels() ([]*Channel, error) {
	// MoveOnJoy channels list
	url := fmt.Sprintf("%s/api/live-channels", m.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Channels []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Logo     string `json:"logo"`
			Category string `json:"category"`
			Stream   string `json:"stream"`
		} `json:"channels"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse channels: %w", err)
	}

	channels := make([]*Channel, 0, len(response.Channels))
	for _, ch := range response.Channels {
		channels = append(channels, &Channel{
			ID:       fmt.Sprintf("moveonjoy-%d", ch.ID),
			Name:     ch.Name,
			Logo:     ch.Logo,
			Category: ch.Category,
			StreamURL: ch.Stream,
			Active:   true,
			Source:   "MoveOnJoy",
		})
	}

	return channels, nil
}

// StreamedSuSource implements Channel source for Streamed.su
type StreamedSuSource struct {
	baseURL string
	client  *http.Client
}

func NewStreamedSuSource() *StreamedSuSource {
	return &StreamedSuSource{
		baseURL: "https://streamed.su",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *StreamedSuSource) Name() string {
	return "Streamed.su"
}

func (s *StreamedSuSource) GetChannels() ([]*Channel, error) {
	// Streamed.su channels - primarily sports
	url := fmt.Sprintf("%s/api/streams", s.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var streams []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Sport string `json:"sport"`
		URL   string `json:"url"`
		Logo  string `json:"logo"`
	}

	if err := json.Unmarshal(body, &streams); err != nil {
		return nil, fmt.Errorf("failed to parse streams: %w", err)
	}

	channels := make([]*Channel, 0, len(streams))
	for _, stream := range streams {
		channels = append(channels, &Channel{
			ID:       stream.ID,
			Name:     stream.Title,
			Logo:     stream.Logo,
			Category: stream.Sport,
			StreamURL: stream.URL,
			Active:   true,
			Source:   "Streamed.su",
		})
	}

	return channels, nil
}
