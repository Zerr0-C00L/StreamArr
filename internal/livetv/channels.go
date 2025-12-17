package livetv

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
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
	EPGURL  string `json:"epg_url,omitempty"`
	Enabled bool   `json:"enabled"`
}

// XtreamSource represents an Xtream Codes compatible IPTV provider
type XtreamSource struct {
	Name      string `json:"name"`
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Enabled   bool   `json:"enabled"`
}

// IPTVOrgConfig holds settings for iptv-org.github.io integration
type IPTVOrgConfig struct {
	Enabled    bool     `json:"enabled"`
	Countries  []string `json:"countries"`   // Country codes: us, uk, ca, etc.
	Languages  []string `json:"languages"`   // Language codes: eng, spa, fra, etc.
	Categories []string `json:"categories"`  // Categories: news, sports, movies, etc.
}

type ChannelManager struct {
	channels           map[string]*Channel
	mu                 sync.RWMutex
	sources            []ChannelSource
	m3uSources         []M3USource
	xtreamSources      []XtreamSource
	iptvOrgConfig      IPTVOrgConfig
	httpClient         *http.Client
	validateStreams    bool
	validationTimeout  time.Duration
	validationCache    map[string]validationCacheEntry
	cacheMutex         sync.RWMutex
	includeLiveTV      bool
	iptvImportMode     string // "live_only", "vod_only", "both"
}

type validationCacheEntry struct {
	isValid   bool
	timestamp time.Time
}

type ChannelSource interface {
	GetChannels() ([]*Channel, error)
	Name() string
}

func NewChannelManager() *ChannelManager {
	cm := &ChannelManager{
		channels:          make(map[string]*Channel),
		sources:           make([]ChannelSource, 0),
		m3uSources:        make([]M3USource, 0),
		xtreamSources:     make([]XtreamSource, 0),
		validateStreams:   false, // Disabled by default (can be enabled in settings)
		validationTimeout: 10 * time.Second, // Increased from 3s to reduce false positives
		validationCache:   make(map[string]validationCacheEntry),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		includeLiveTV:     false, // Default false after factory reset
		iptvImportMode:    "live_only",
	}
	// Note: Removed broken third-party sources
	// Users can add their own M3U sources in Settings
	return cm
}
// SetIncludeLiveTV sets the includeLiveTV flag from settings
func (cm *ChannelManager) SetIncludeLiveTV(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.includeLiveTV = enabled
}

// SetIPTVImportMode sets how IPTV content is handled: live_only, vod_only, both
func (cm *ChannelManager) SetIPTVImportMode(mode string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	switch mode {
	case "live_only", "vod_only", "both":
		cm.iptvImportMode = mode
	default:
		cm.iptvImportMode = "live_only"
	}
}

// SetM3USources sets the custom M3U sources
func (cm *ChannelManager) SetM3USources(sources []M3USource) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.m3uSources = sources
}

// SetXtreamSources sets the custom Xtream sources
func (cm *ChannelManager) SetXtreamSources(sources []XtreamSource) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.xtreamSources = sources
}

// SetIPTVOrgConfig sets the IPTV-Org configuration
func (cm *ChannelManager) SetIPTVOrgConfig(config IPTVOrgConfig) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.iptvOrgConfig = config
}

// SetStreamValidation enables/disables stream URL validation
func (cm *ChannelManager) SetStreamValidation(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.validateStreams = enabled
	
	// Clear validation cache when disabling validation
	if !enabled {
		cm.cacheMutex.Lock()
		cm.validationCache = make(map[string]validationCacheEntry)
		cm.cacheMutex.Unlock()
	}
}

// validateStreamURL checks if a stream URL is accessible (with 24-hour caching)
func (cm *ChannelManager) validateStreamURL(url string) bool {
	if !cm.validateStreams {
		return true // Skip validation if disabled
	}
	
	// Check cache first (24-hour validity)
	cm.cacheMutex.RLock()
	if entry, exists := cm.validationCache[url]; exists {
		if time.Since(entry.timestamp) < 24*time.Hour {
			cm.cacheMutex.RUnlock()
			return entry.isValid
		}
	}
	cm.cacheMutex.RUnlock()
	
	// Not in cache or expired, validate now
	client := &http.Client{
		Timeout: cm.validationTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects, just check if URL responds
		},
	}
	
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	
	// Add headers that some streams require
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	
	resp, err := client.Do(req)
	if err != nil {
		// Cache the result (failed validation)
		cm.cacheMutex.Lock()
		cm.validationCache[url] = validationCacheEntry{
			isValid:   false,
			timestamp: time.Now(),
		}
		cm.cacheMutex.Unlock()
		return false
	}
	defer resp.Body.Close()
	
	// Accept any 2xx or 3xx status code as valid
	isValid := resp.StatusCode >= 200 && resp.StatusCode < 400
	
	// Cache the result
	cm.cacheMutex.Lock()
	cm.validationCache[url] = validationCacheEntry{
		isValid:   isValid,
		timestamp: time.Now(),
	}
	cm.cacheMutex.Unlock()
	
	return isValid
}

// validateChannelsConcurrent validates multiple channels concurrently
func (cm *ChannelManager) validateChannelsConcurrent(channels []*Channel, concurrency int) []*Channel {
	if !cm.validateStreams || len(channels) == 0 {
		return channels
	}
	
	type result struct {
		channel *Channel
		valid   bool
	}
	
	resultsChan := make(chan result, len(channels))
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	
	// Validate channels concurrently
	for _, ch := range channels {
		wg.Add(1)
		go func(channel *Channel) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore
			
			valid := cm.validateStreamURL(channel.StreamURL)
			resultsChan <- result{channel: channel, valid: valid}
		}(ch)
	}
	
	// Close results channel when all validations complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect valid channels
	validChannels := make([]*Channel, 0, len(channels))
	for res := range resultsChan {
		if res.valid {
			validChannels = append(validChannels, res.channel)
		}
	}
	
	return validChannels
}

func (cm *ChannelManager) LoadChannels() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Only load channels if Live TV is enabled in settings
	// This flag should be set from settings.IncludeLiveTV
	if !cm.isLiveTVEnabled() {
		cm.channels = make(map[string]*Channel)
		fmt.Println("Live TV: Disabled, no channels loaded")
		return nil
	}

	// If IPTV import mode is VOD-only, do not load Live TV channels
	if strings.EqualFold(cm.iptvImportMode, "vod_only") {
		cm.channels = make(map[string]*Channel)
		fmt.Println("Live TV: VOD-only mode; no live channels loaded")
		return nil
	}

	allChannels := make([]*Channel, 0)

	// Load from IPTV-Org if enabled
	if cm.iptvOrgConfig.Enabled {
		fmt.Println("Live TV: Loading channels from IPTV-Org...")
		iptvChannels, err := cm.loadFromIPTVOrg()
		if err != nil {
			fmt.Printf("Warning: Error loading IPTV-Org channels: %v\n", err)
		} else {
			allChannels = append(allChannels, iptvChannels...)
			fmt.Printf("Loaded %d channels from IPTV-Org\n", len(iptvChannels))
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

	// Load from custom Xtream sources (user-configured)
	for _, source := range cm.xtreamSources {
		if !source.Enabled {
			continue
		}
		channels, err := cm.loadFromXtreamSource(source)
		if err != nil {
			fmt.Printf("Error loading channels from Xtream %s: %v\n", source.Name, err)
			continue
		}
		allChannels = append(allChannels, channels...)
		fmt.Printf("Loaded %d channels from Xtream %s\n", len(channels), source.Name)
	}

	// Check if we have any channels at all
	if len(allChannels) == 0 {
		fmt.Println("Live TV: No channels loaded")
		if !cm.iptvOrgConfig.Enabled && len(cm.m3uSources) == 0 && len(cm.xtreamSources) == 0 {
			fmt.Println("Enable IPTV-Org or add Custom M3U/Xtream Sources in Settings → Live TV")
		}
		cm.channels = make(map[string]*Channel)
		return nil
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

// isLiveTVEnabled returns true if Live TV is enabled in settings
func (cm *ChannelManager) isLiveTVEnabled() bool {
	return cm.includeLiveTV
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

// loadFromLocalM3U loads channels from a local M3U file with provider extraction
func (cm *ChannelManager) loadFromLocalM3U(filePath string) ([]*Channel, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return cm.parseM3UWithProviders(string(file))
}

// loadFromXtreamSource loads channels from an Xtream Codes API compatible provider
func (cm *ChannelManager) loadFromXtreamSource(source XtreamSource) ([]*Channel, error) {
	// Build the M3U URL from Xtream credentials
	// Xtream API provides M3U playlist via: http://server:port/get.php?username=xxx&password=xxx&type=m3u_plus&output=ts
	serverURL := strings.TrimSuffix(source.ServerURL, "/")
	m3uURL := fmt.Sprintf("%s/get.php?username=%s&password=%s&type=m3u_plus&output=ts", 
		serverURL, source.Username, source.Password)
	
	return cm.loadFromM3UURL(m3uURL, fmt.Sprintf("Xtream: %s", source.Name))
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

// loadFromIPTVOrg loads channels from iptv-org.github.io based on config
func (cm *ChannelManager) loadFromIPTVOrg() ([]*Channel, error) {
	allChannels := make([]*Channel, 0)
	
	// Base URL for iptv-org playlists
	baseURL := "https://iptv-org.github.io/iptv"
	
	// If specific categories are selected, load by category
	if len(cm.iptvOrgConfig.Categories) > 0 {
		for _, category := range cm.iptvOrgConfig.Categories {
			url := fmt.Sprintf("%s/categories/%s.m3u", baseURL, category)
			channels, err := cm.loadFromM3UURL(url, fmt.Sprintf("IPTV-org (%s)", category))
			if err != nil {
				fmt.Printf("Warning: Could not load IPTV-org category %s: %v\n", category, err)
				continue
			}
			allChannels = append(allChannels, channels...)
		}
	}
	
	// If specific countries are selected, load by country
	if len(cm.iptvOrgConfig.Countries) > 0 {
		for _, country := range cm.iptvOrgConfig.Countries {
			url := fmt.Sprintf("%s/countries/%s.m3u", baseURL, country)
			channels, err := cm.loadFromM3UURL(url, fmt.Sprintf("IPTV-org (%s)", strings.ToUpper(country)))
			if err != nil {
				fmt.Printf("Warning: Could not load IPTV-org country %s: %v\n", country, err)
				continue
			}
			allChannels = append(allChannels, channels...)
		}
	}
	
	// If specific languages are selected, load by language
	if len(cm.iptvOrgConfig.Languages) > 0 {
		for _, language := range cm.iptvOrgConfig.Languages {
			url := fmt.Sprintf("%s/languages/%s.m3u", baseURL, language)
			channels, err := cm.loadFromM3UURL(url, fmt.Sprintf("IPTV-org (%s)", language))
			if err != nil {
				fmt.Printf("Warning: Could not load IPTV-org language %s: %v\n", language, err)
				continue
			}
			allChannels = append(allChannels, channels...)
		}
	}
	
	// If no specific filters, load the main index (all channels)
	if len(cm.iptvOrgConfig.Categories) == 0 && len(cm.iptvOrgConfig.Countries) == 0 && len(cm.iptvOrgConfig.Languages) == 0 {
		url := fmt.Sprintf("%s/index.m3u", baseURL)
		channels, err := cm.loadFromM3UURL(url, "IPTV-org (All)")
		if err != nil {
			return nil, fmt.Errorf("could not load IPTV-org index: %w", err)
		}
		allChannels = append(allChannels, channels...)
	}
	
	return allChannels, nil
}

// parseM3U parses M3U content and returns channels
func (cm *ChannelManager) parseM3U(content string, sourceName string) ([]*Channel, error) {
	channels := make([]*Channel, 0)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	var currentChannel *Channel
	var currentIsVOD bool
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

			// Detect VOD via group-title when available
			currentIsVOD = false
			if idx := strings.Index(line, "group-title="); idx != -1 {
				start := idx + len("group-title=")
				if start < len(line) && line[start] == '"' {
					end := strings.Index(line[start+1:], "\"")
					if end != -1 {
						group := strings.ToLower(line[start+1 : start+1+end])
						if strings.Contains(group, "vod") || strings.Contains(group, "movie") || strings.Contains(group, "series") || strings.Contains(group, "film") {
							currentIsVOD = true
						}
					}
				}
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
			// Detect VOD via stream URL patterns
			isVODURL := strings.Contains(strings.ToLower(line), "/movie/") || strings.Contains(strings.ToLower(line), "/series/") || strings.HasSuffix(strings.ToLower(line), ".mp4") || strings.HasSuffix(strings.ToLower(line), ".mkv")
			// In Live TV, we never include VOD entries (they belong to Library when supported)
			shouldInclude := !currentIsVOD && !isVODURL
			if shouldInclude {
				if currentChannel.Name != "" {
					// Set category based on channel name (smart mapping)
					currentChannel.Category = mapChannelToCategory(currentChannel.Name)
					channels = append(channels, currentChannel)
				}
			}
			currentChannel = nil
		}
	}
	
	// Validate channels concurrently if validation is enabled
	if cm.validateStreams {
		totalParsed := len(channels)
		channels = cm.validateChannelsConcurrent(channels, 100) // 100 concurrent validations
		totalFiltered := totalParsed - len(channels)
		if totalFiltered > 0 {
			fmt.Printf("Filtered %d broken channels from %s (%d valid out of %d total)\n", 
				totalFiltered, sourceName, len(channels), totalParsed)
		}
	}
	
	return channels, nil
}

// parseM3UWithProviders parses M3U content and extracts provider names from group-title
// This is used for local M3U files that contain multiple providers
func (cm *ChannelManager) parseM3UWithProviders(content string) ([]*Channel, error) {
	channels := make([]*Channel, 0)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	var currentChannel *Channel
	var currentIsVOD bool
	var currentGroupTitle string
	channelID := 0
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if strings.HasPrefix(line, "#EXTINF:") {
			// Parse channel info
			currentChannel = &Channel{
				IsLive: true,
				Active: true,
			}
			
			// Extract group-title to determine provider
			currentGroupTitle = ""
			if idx := strings.Index(line, "group-title=\""); idx != -1 {
				end := strings.Index(line[idx+13:], "\"")
				if end != -1 {
					currentGroupTitle = line[idx+13 : idx+13+end]
				}
			}
			
			// Map group-title to provider name
			currentChannel.Source = extractProviderName(currentGroupTitle)

			// Detect VOD via group-title
			currentIsVOD = false
			lowerGroup := strings.ToLower(currentGroupTitle)
			if strings.Contains(lowerGroup, "vod") || strings.Contains(lowerGroup, "movie") || strings.Contains(lowerGroup, "series") || strings.Contains(lowerGroup, "film") {
				currentIsVOD = true
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
				currentChannel.ID = fmt.Sprintf("%s_%d", currentChannel.Source, channelID)
			}
			
		} else if !strings.HasPrefix(line, "#") && line != "" && currentChannel != nil {
			// This is the stream URL
			currentChannel.StreamURL = line
			// Detect VOD via stream URL patterns
			isVODURL := strings.Contains(strings.ToLower(line), "/movie/") || strings.Contains(strings.ToLower(line), "/series/") || strings.HasSuffix(strings.ToLower(line), ".mp4") || strings.HasSuffix(strings.ToLower(line), ".mkv")
			if !currentIsVOD && !isVODURL {
				if currentChannel.Name != "" {
					// Set category based on channel name (smart mapping)
					currentChannel.Category = mapChannelToCategory(currentChannel.Name)
					channels = append(channels, currentChannel)
				}
			}
			currentChannel = nil
		}
	}
	
	// Validate channels concurrently if validation is enabled
	if cm.validateStreams {
		totalParsed := len(channels)
		channels = cm.validateChannelsConcurrent(channels, 100) // 100 concurrent validations
		totalFiltered := totalParsed - len(channels)
		if totalFiltered > 0 {
			fmt.Printf("Filtered %d broken channels from local M3U (%d valid out of %d total)\n", 
				totalFiltered, len(channels), totalParsed)
		}
	}
	
	return channels, nil
}

// extractProviderName extracts the provider name from group-title
func extractProviderName(groupTitle string) string {
	groupTitle = strings.TrimSpace(groupTitle)
	if groupTitle == "" {
		return "Other"
	}
	
	// Check for pattern like "Category (Provider)"
	if idx := strings.Index(groupTitle, "("); idx != -1 {
		endIdx := strings.Index(groupTitle, ")")
		if endIdx != -1 && endIdx > idx {
			provider := strings.TrimSpace(groupTitle[idx+1 : endIdx])
			return provider
		}
	}
	
	return groupTitle
}

// mapChannelToCategory determines the category based on channel name
// Categories: 24/7, Sports, News, Movies, Entertainment, Kids, Music, Documentary, Lifestyle, Latino, Reality, Religious, Shopping, General
func mapChannelToCategory(channelName string) string {
	name := strings.ToLower(channelName)
	
	// Balkan channels - check for country prefixes first (HR: BA: RS: SI: ME: MK: AL: XK: EX-YU:)
	// Handle both "AL:" and "AL: " formats
	balkanPrefixes := []string{
		"hr:", "hr ", "ba:", "ba ", "rs:", "rs ", "si:", "si ", 
		"me:", "me ", "mk:", "mk ", "al:", "al ", "xk:", "xk ", 
		"ex-yu:", "ex-yu ", "ex yu:", "ex yu ", "srb:", "srb ", "cro:", "cro ", "slo:", "slo ",
		"bih:", "bih ", "mne:", "mne ", "mkd:", "mkd ",
	}
	for _, prefix := range balkanPrefixes {
		if strings.HasPrefix(name, prefix) {
			return "Balkan"
		}
	}
	
	// Also check for Balkan keywords anywhere in name
	balkanKeywords := []string{
		"croatia", "serbia", "bosnia", "slovenia", "montenegro", "macedonia", "albania", "kosovo",
		"hrt", "rtv slo", "rts ", "rtrs", "bht", "nova tv hr", "pink rs", "pink bh", "n1 hr", "n1 rs", "n1 ba",
		"hayat", "face tv", "rtcg", "arena sport ba", "arena sport hr", "arena sport rs",
	}
	for _, kw := range balkanKeywords {
		if strings.Contains(name, kw) {
			return "Balkan"
		}
	}
	
	// 24/7 channels - check first for specific pattern
	if strings.Contains(name, "24/7") || strings.Contains(name, "24-7") || strings.Contains(name, "247") {
		return "24/7"
	}
	
	// Latino/Spanish channels - check first to catch Spanish variants
	latinoKeywords := []string{"latino", "latina", "español", "espanol", "spanish", "telemundo", 
		"univision", "azteca", "galavision", "unimas", "estrella", "telefe", "caracol",
		"mexiquense", "cine latino", "cine mexicano", "novela", "íconos latinos", "iconos latinos",
		"en español", "en espanol", "latv", "sony cine", "cine sony", "pluto tv cine",
		"comedia", "acción", "accion", "clásicos", "clasicos", "peliculas", "películas"}
	for _, kw := range latinoKeywords {
		if strings.Contains(name, kw) {
			return "Latino"
		}
	}
	
	// Sports channels (including Balkan variants)
	sportsKeywords := []string{"sport", "espn", "fox sports", "nfl", "nba", "mlb", "nhl", "golf", "tennis",
		"bein", "sky sports", "bt sport", "dazn", "acc network", "big ten", "sec network", "pac-12",
		"nbcsn", "cbs sports", "soccer", "football", "baseball", "basketball", "hockey", "cricket",
		"wwe", "ufc", "boxing", "racing", "f1", "formula", "nascar", "motogp", "olympic", "athletic",
		"arena sport", "supersport", "sport klub", "eurosport", "arena"}
	for _, kw := range sportsKeywords {
		if strings.Contains(name, kw) {
			return "Sports"
		}
	}
	
	// News channels (including Balkan variants)
	newsKeywords := []string{"news", "cnn", "fox news", "msnbc", "bbc news", "cnbc", "bloomberg",
		"c-span", "cspan", "sky news", "al jazeera", "abc news", "cbs news", "nbc news",
		"newsmax", "oan", "weather", "headline", "euronews", "n1"}
	for _, kw := range newsKeywords {
		if strings.Contains(name, kw) {
			return "News"
		}
	}
	
	// Movie channels (including Balkan variants)
	movieKeywords := []string{"movie", "hbo", "cinemax", "showtime", "starz", "epix", "mgm",
		"tcm", "amc", "ifc", "sundance", "fx movie", "sony movie", "lifetime movie", "hallmark movie",
		"cinestar", "film", "kino", "cinema"}
	for _, kw := range movieKeywords {
		if strings.Contains(name, kw) {
			return "Movies"
		}
	}
	
	// Kids channels (including Balkan variants)
	kidsKeywords := []string{"disney", "nick", "cartoon", "boomerang", "pbs kids", "baby",
		"junior", "kids", "teen", "sprout", "universal kids", "discovery family", "bravo! kids", "bravo kids"}
	for _, kw := range kidsKeywords {
		if strings.Contains(name, kw) {
			return "Kids"
		}
	}
	
	// Music channels (including Balkan variants)
	musicKeywords := []string{"mtv", "vh1", "bet", "cmt", "music", "vevo", "fuse", "revolt",
		"bet jams", "bet soul", "bet gospel", "axs tv", "radio", "muzik", "hit fm", "dj",
		"klape", "tambure", "folk"}
	for _, kw := range musicKeywords {
		if strings.Contains(name, kw) {
			return "Music"
		}
	}
	
	// Documentary channels (including Balkan variants)
	docKeywords := []string{"discovery", "national geographic", "nat geo", "history", "science",
		"animal planet", "smithsonian", "pbs", "a&e", "ae", "investigation", "crime",
		"american heroes", "military", "nature", "planet earth", "vice", "dokumentar",
		"edutv", "edu"}
	for _, kw := range docKeywords {
		if strings.Contains(name, kw) {
			return "Documentary"
		}
	}
	
	// Lifestyle channels (including Balkan variants)
	lifestyleKeywords := []string{"food", "cooking", "hgtv", "tlc", "bravo!", "bravo tv", "e!", "oxygen",
		"lifetime", "we tv", "own", "hallmark", "travel", "diy", "magnolia", "bet her",
		"style", "fashion", "home", "garden", "health", "wellness"}
	for _, kw := range lifestyleKeywords {
		if strings.Contains(name, kw) {
			return "Lifestyle"
		}
	}
	
	// Reality TV channels
	realityKeywords := []string{"reality", "real housewives", "survivor", "big brother", 
		"bachelor", "bachelorette", "love island", "jersey shore", "kardashian"}
	for _, kw := range realityKeywords {
		if strings.Contains(name, kw) {
			return "Reality"
		}
	}
	
	// Religious/Faith channels
	religiousKeywords := []string{"church", "faith", "gospel", "religious", "christian", 
		"catholic", "god", "jesus", "prayer", "worship", "trinity", "daystar", "tbn"}
	for _, kw := range religiousKeywords {
		if strings.Contains(name, kw) {
			return "Religious"
		}
	}
	
	// Shopping/QVC channels
	shoppingKeywords := []string{"shop", "qvc", "hsn", "shopping", "jewelry"}
	for _, kw := range shoppingKeywords {
		if strings.Contains(name, kw) {
			return "Shopping"
		}
	}
	
	// Entertainment (catch-all for broadcast and entertainment including Balkan variants)
	entertainmentKeywords := []string{"abc", "nbc", "cbs", "fox", "cw", "tbs", "tnt", "usa",
		"fx", "freeform", "syfy", "comedy", "paramount", "pop", "tv land", "comet",
		"ion", "bounce", "court", "reelz", "grit", "pink", "nova", "happy", "grand",
		"extra", "trend", "city", "kohavision"}
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
	
	// Sort channels by ID for stable ordering
	// This ensures consistent indexing across requests
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ID < channels[j].ID
	})
	
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
