package livetv

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Channel struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Logo        string   `json:"logo"`
	StreamURL   string   `json:"stream_url"`
	Category    string   `json:"category"`
	Language    string   `json:"language"`
	Country     string   `json:"country"`
	IsLive      bool     `json:"is_live"`
	Active      bool     `json:"active"`
	Source      string   `json:"source"`
	EPG         []EPGProgram `json:"epg,omitempty"`
}

type EPGProgram struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Category    string    `json:"category"`
}

// M3USource represents a custom M3U playlist source
type M3USource struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type ChannelManager struct {
	channels       map[string]*Channel
	mu             sync.RWMutex
	sources        []ChannelSource
	m3uSources     []M3USource
	httpClient     *http.Client
	enablePlutoTV  bool
}

type ChannelSource interface {
	GetChannels() ([]*Channel, error)
	Name() string
}

func NewChannelManager() *ChannelManager {
	cm := &ChannelManager{
		channels:      make(map[string]*Channel),
		sources:       make([]ChannelSource, 0),
		m3uSources:    make([]M3USource, 0),
		enablePlutoTV: true, // Enabled by default
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	
	// Note: Removed broken third-party sources
	// Users can add their own M3U sources in Settings
	
	return cm
}

// SetM3USources sets the custom M3U sources
func (cm *ChannelManager) SetM3USources(sources []M3USource) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.m3uSources = sources
}

// SetPlutoTVEnabled enables/disables built-in Pluto TV
func (cm *ChannelManager) SetPlutoTVEnabled(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.enablePlutoTV = enabled
}

func (cm *ChannelManager) LoadChannels() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	allChannels := make([]*Channel, 0)
	
	// First, try to load from local M3U file (contains DADDY LIVE, MoveOnJoy, TheTVApp)
	localChannels, err := cm.loadFromLocalM3U("./channels/m3u_formatted.dat")
	if err != nil {
		fmt.Printf("Warning: Could not load local M3U file: %v\n", err)
	} else {
		allChannels = append(allChannels, localChannels...)
		fmt.Printf("Loaded %d channels from local M3U file\n", len(localChannels))
	}
	
	// Load Pluto TV from GitHub (built-in source) - if enabled
	if cm.enablePlutoTV {
		plutoChannels, err := cm.loadFromM3UURL(
			"https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/Pluto-TV/us.m3u8",
			"Pluto TV",
		)
		if err != nil {
			fmt.Printf("Warning: Could not load Pluto TV: %v\n", err)
		} else {
			allChannels = append(allChannels, plutoChannels...)
			fmt.Printf("Loaded %d channels from Pluto TV\n", len(plutoChannels))
		}
	}
	
	// Load from custom M3U sources (user-configured)
	for _, source := range cm.m3uSources {
		if !source.Enabled {
			continue
		}
		channels, err := cm.loadFromM3UURL(source.URL, source.Name)
		if err != nil {
			fmt.Printf("Error loading channels from %s: %v\n", source.Name, err)
			continue
		}
		allChannels = append(allChannels, channels...)
		fmt.Printf("Loaded %d channels from %s\n", len(channels), source.Name)
	}
	
	// Smart duplicate merging - normalize channel names and keep best quality
	cm.channels = make(map[string]*Channel)
	channelsByNormalizedName := make(map[string]*Channel)
	
	for _, ch := range allChannels {
		normalizedName := normalizeChannelName(ch.Name)
		
		existing, exists := channelsByNormalizedName[normalizedName]
		if !exists {
			// First occurrence - add it
			channelsByNormalizedName[normalizedName] = ch
			cm.channels[ch.ID] = ch
		} else {
			// Duplicate found - keep the one with better data (logo, stream URL)
			if shouldReplaceChannel(existing, ch) {
				// Remove old channel
				delete(cm.channels, existing.ID)
				// Add new channel
				channelsByNormalizedName[normalizedName] = ch
				cm.channels[ch.ID] = ch
			}
		}
	}
	
	fmt.Printf("Live TV: Loaded %d unique channels (merged from %d total)\n", len(cm.channels), len(allChannels))
	return nil
}

// normalizeChannelName normalizes a channel name for duplicate detection
func normalizeChannelName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	// Remove common suffixes/prefixes
	n = strings.TrimSuffix(n, " hd")
	n = strings.TrimSuffix(n, " sd")
	n = strings.TrimSuffix(n, " east")
	n = strings.TrimSuffix(n, " west")
	n = strings.TrimPrefix(n, "us: ")
	n = strings.TrimPrefix(n, "uk: ")
	// Remove extra spaces
	n = strings.Join(strings.Fields(n), " ")
	return n
}

// shouldReplaceChannel determines if new channel should replace existing
func shouldReplaceChannel(existing, new *Channel) bool {
	// Prefer channels with logos
	if existing.Logo == "" && new.Logo != "" {
		return true
	}
	// Prefer non-Pluto TV sources (they have EPG from provider group-title)
	if strings.Contains(existing.Source, "Pluto") && !strings.Contains(new.Source, "Pluto") {
		return true
	}
	return false
}

// loadFromLocalM3U loads channels from a local M3U file
func (cm *ChannelManager) loadFromLocalM3U(filePath string) ([]*Channel, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return cm.parseM3U(string(file), "Local")
}

// loadFromM3UURL loads channels from a remote M3U URL
func (cm *ChannelManager) loadFromM3UURL(url string, sourceName string) ([]*Channel, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	
	resp, err := cm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	return cm.parseM3U(string(body), sourceName)
}

// parseM3U parses M3U content and returns channels
func (cm *ChannelManager) parseM3U(content string, sourceName string) ([]*Channel, error) {
	channels := make([]*Channel, 0)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	var currentChannel *Channel
	channelID := 0
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "#EXTINF:") {
			// Parse channel info
			currentChannel = &Channel{
				IsLive: true,
				Active: true,
				Source: sourceName,
			}
			
			// Extract tvg-name or channel name
			if idx := strings.Index(line, "tvg-name=\""); idx != -1 {
				end := strings.Index(line[idx+10:], "\"")
				if end != -1 {
					currentChannel.Name = line[idx+10 : idx+10+end]
				}
			}
			
			// Extract tvg-logo
			if idx := strings.Index(line, "tvg-logo=\""); idx != -1 {
				end := strings.Index(line[idx+10:], "\"")
				if end != -1 {
					currentChannel.Logo = line[idx+10 : idx+10+end]
				}
			}
			
			// We ignore the provider's group-title and use smart category mapping instead
			// The category will be set based on channel name after parsing
			
			// Extract tvg-id
			if idx := strings.Index(line, "tvg-id=\""); idx != -1 {
				end := strings.Index(line[idx+8:], "\"")
				if end != -1 {
					currentChannel.ID = line[idx+8 : idx+8+end]
				}
			}
			
			// Fallback: get name from end of line after last comma
			if currentChannel.Name == "" {
				if commaIdx := strings.LastIndex(line, ","); commaIdx != -1 {
					currentChannel.Name = strings.TrimSpace(line[commaIdx+1:])
				}
			}
			
			// Generate ID if not present
			if currentChannel.ID == "" {
				channelID++
				currentChannel.ID = fmt.Sprintf("%s_%d", sourceName, channelID)
			}
			
		} else if !strings.HasPrefix(line, "#") && line != "" && currentChannel != nil {
			// This is the stream URL
			currentChannel.StreamURL = line
			if currentChannel.Name != "" {
				// Set category based on channel name (smart mapping)
				currentChannel.Category = mapChannelToCategory(currentChannel.Name)
				channels = append(channels, currentChannel)
			}
			currentChannel = nil
		}
	}
	
	return channels, nil
}

// mapChannelToCategory determines the category based on channel name
// Categories: Sports, News, Movies, Entertainment, Kids, Music, Documentary, Lifestyle, General
func mapChannelToCategory(channelName string) string {
	name := strings.ToLower(channelName)
	
	// Sports channels
	sportsKeywords := []string{"sport", "espn", "fox sports", "nfl", "nba", "mlb", "nhl", "golf", "tennis",
		"bein", "sky sports", "bt sport", "dazn", "acc network", "big ten", "sec network", "pac-12",
		"nbcsn", "cbs sports", "soccer", "football", "baseball", "basketball", "hockey", "cricket",
		"wwe", "ufc", "boxing", "racing", "f1", "formula", "nascar", "motogp", "olympic", "athletic"}
	for _, kw := range sportsKeywords {
		if strings.Contains(name, kw) {
			return "Sports"
		}
	}
	
	// News channels
	newsKeywords := []string{"news", "cnn", "fox news", "msnbc", "bbc news", "cnbc", "bloomberg",
		"c-span", "cspan", "sky news", "al jazeera", "abc news", "cbs news", "nbc news",
		"newsmax", "oan", "weather", "headline"}
	for _, kw := range newsKeywords {
		if strings.Contains(name, kw) {
			return "News"
		}
	}
	
	// Movie channels
	movieKeywords := []string{"movie", "hbo", "cinemax", "showtime", "starz", "epix", "mgm",
		"tcm", "amc", "ifc", "sundance", "fx movie", "sony movie", "lifetime movie", "hallmark movie"}
	for _, kw := range movieKeywords {
		if strings.Contains(name, kw) {
			return "Movies"
		}
	}
	
	// Kids channels
	kidsKeywords := []string{"disney", "nick", "cartoon", "boomerang", "pbs kids", "baby",
		"junior", "kids", "teen", "sprout", "universal kids", "discovery family"}
	for _, kw := range kidsKeywords {
		if strings.Contains(name, kw) {
			return "Kids"
		}
	}
	
	// Music channels
	musicKeywords := []string{"mtv", "vh1", "bet", "cmt", "music", "vevo", "fuse", "revolt",
		"bet jams", "bet soul", "bet gospel", "axs tv"}
	for _, kw := range musicKeywords {
		if strings.Contains(name, kw) {
			return "Music"
		}
	}
	
	// Documentary channels
	docKeywords := []string{"discovery", "national geographic", "nat geo", "history", "science",
		"animal planet", "smithsonian", "pbs", "a&e", "ae", "investigation", "crime",
		"american heroes", "military", "nature", "planet earth", "vice"}
	for _, kw := range docKeywords {
		if strings.Contains(name, kw) {
			return "Documentary"
		}
	}
	
	// Lifestyle channels
	lifestyleKeywords := []string{"food", "cooking", "hgtv", "tlc", "bravo", "e!", "oxygen",
		"lifetime", "we tv", "own", "hallmark", "travel", "diy", "magnolia", "bet her",
		"style", "fashion", "home", "garden"}
	for _, kw := range lifestyleKeywords {
		if strings.Contains(name, kw) {
			return "Lifestyle"
		}
	}
	
	// Entertainment (catch-all for broadcast and entertainment)
	entertainmentKeywords := []string{"abc", "nbc", "cbs", "fox", "cw", "tbs", "tnt", "usa",
		"fx", "freeform", "syfy", "comedy", "paramount", "pop", "tv land", "comet",
		"ion", "bounce", "court", "reelz", "grit"}
	for _, kw := range entertainmentKeywords {
		if strings.Contains(name, kw) {
			return "Entertainment"
		}
	}
	
	return "General"
}

func (cm *ChannelManager) GetAllChannels() []*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	channels := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	return channels
}

func (cm *ChannelManager) GetChannel(id string) (*Channel, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	ch, ok := cm.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", id)
	}
	return ch, nil
}

func (cm *ChannelManager) GetChannelsByCategory(category string) []*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	channels := make([]*Channel, 0)
	for _, ch := range cm.channels {
		if strings.EqualFold(ch.Category, category) {
			channels = append(channels, ch)
		}
	}
	return channels
}

func (cm *ChannelManager) SearchChannels(query string) []*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	query = strings.ToLower(query)
	channels := make([]*Channel, 0)
	
	for _, ch := range cm.channels {
		if strings.Contains(strings.ToLower(ch.Name), query) ||
		   strings.Contains(strings.ToLower(ch.Category), query) {
			channels = append(channels, ch)
		}
	}
	return channels
}

// GetChannelCount returns the number of loaded channels
func (cm *ChannelManager) GetChannelCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.channels)
}

// GetCategories returns all unique channel categories
func (cm *ChannelManager) GetCategories() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	categoryMap := make(map[string]bool)
	for _, ch := range cm.channels {
		if ch.Category != "" {
			categoryMap[ch.Category] = true
		}
	}
	
	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}
	return categories
}
