package providers

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/Zerr0-C00L/StreamArr/internal/services"
)

// TorrentioStream represents a stream from Stremio addons (kept for compatibility)
type TorrentioStream struct {
	Name          string `json:"name"`
	Title         string `json:"title"`
	InfoHash      string `json:"infoHash"`
	FileIdx       int    `json:"fileIdx,omitempty"`
	URL           string `json:"url"`
	Quality       string `json:"quality,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Seeders       int    `json:"seeders,omitempty"`
	Cached        bool   `json:"cached,omitempty"`
	Source        string `json:"source,omitempty"`
	BehaviorHints struct {
		Filename   string `json:"filename,omitempty"`
		BingeGroup string `json:"bingeGroup,omitempty"`
		VideoSize  int64  `json:"videoSize,omitempty"`
	} `json:"behaviorHints,omitempty"`
}

// StremioAddon represents a Stremio addon configuration
type StremioAddon struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type StreamProvider interface {
	GetMovieStreams(imdbID string) ([]TorrentioStream, error)
	GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error)
}

type MultiProvider struct {
	Providers        []StreamProvider
	ProviderNames    []string
	dmmDirectProvider *DMMDirectProvider // Direct DMM queries
	// Settings getters for dynamic configuration
	getSortOrder     func() string
	getSortPrefer    func() string
}

// Removed Zilean-related code: provider and config
// Removed Comet-related code: provider and config

func NewMultiProvider(rdAPIKey string, addons []StremioAddon, tmdbClient *services.TMDBClient) *MultiProvider {
	return NewMultiProviderWithConfig(rdAPIKey, addons, tmdbClient)
}

// Removed NewMultiProviderWithZilean (deprecated)

func NewMultiProviderWithConfig(rdAPIKey string, addons []StremioAddon, tmdbClient *services.TMDBClient) *MultiProvider {
	mp := &MultiProvider{
		Providers:     make([]StreamProvider, 0),
		ProviderNames: make([]string, 0),
	}
	
	// Add each enabled addon as a generic Stremio provider
	for _, addon := range addons {
		if !addon.Enabled {
			continue
		}
		
		provider := NewGenericStremioProvider(addon.Name, addon.URL, rdAPIKey)
		mp.Providers = append(mp.Providers, provider)
		mp.ProviderNames = append(mp.ProviderNames, addon.Name)
		log.Printf("Loaded Stremio addon: %s (%s)", addon.Name, addon.URL)
	}

	// Add fallback free providers if no addons and no Real-Debrid
	if len(mp.Providers) == 0 {
		log.Println("⚠️  No Stremio addons configured and no Real-Debrid - using fallback free providers")
		
		// Add AutoEmbed provider
		if tmdbClient != nil {
			autoEmbedProvider := NewAutoEmbedAdapter(tmdbClient)
			mp.Providers = append(mp.Providers, autoEmbedProvider)
			mp.ProviderNames = append(mp.ProviderNames, "AutoEmbed")
			log.Println("✓ AutoEmbed fallback provider loaded")
		}
		
		// Add VidSrc provider
		if tmdbClient != nil {
			vidSrcProvider := NewVidSrcAdapter(tmdbClient)
			mp.Providers = append(mp.Providers, vidSrcProvider)
			mp.ProviderNames = append(mp.ProviderNames, "VidSrc")
			log.Println("✓ VidSrc fallback provider loaded")
		}
	}
	
	return mp
}

// SetSortSettings configures the sorting behavior dynamically
func (mp *MultiProvider) SetSortSettings(getSortOrder, getSortPrefer func() string) {
	mp.getSortOrder = getSortOrder
	mp.getSortPrefer = getSortPrefer
}

// StreamRequest represents a request for streams, following Stremio SDK pattern
type StreamRequest struct {
	Type        string // "movie" or "series"
	ID          string // IMDb ID (tt prefix) or TMDB ID (tmdb prefix)
	Season      int    // Only for series
	Episode     int    // Only for series
	ReleaseYear int    // Movie/Series release year for filtering (0 = no filtering)
	Title       string // Media title for reference
}

// StreamResponse represents the response from stream handlers
type StreamResponse struct {
	Streams []TorrentioStream `json:"streams"`
}

// GetStreams is the main method that follows Stremio SDK pattern
// It handles both movies and series with proper type validation
func (mp *MultiProvider) GetStreams(req StreamRequest) ([]TorrentioStream, error) {
	// Validate request
	if req.Type != "movie" && req.Type != "series" {
		return nil, fmt.Errorf("invalid type: %s, must be 'movie' or 'series'", req.Type)
	}
	
	if req.ID == "" {
		return nil, fmt.Errorf("ID is required")
	}
	
	// Route to appropriate handler
	if req.Type == "movie" {
		return mp.GetMovieStreamsWithYear(req.ID, req.ReleaseYear)
	} else if req.Type == "series" {
		if req.Season < 0 || req.Episode < 0 {
			return nil, fmt.Errorf("season and episode must be >= 0 for series")
		}
		return mp.GetSeriesStreams(req.ID, req.Season, req.Episode)
	}
	
	return nil, fmt.Errorf("unknown type: %s", req.Type)
}

// GetMovieStreamsWithYear fetches movie streams and filters by release year
func (mp *MultiProvider) GetMovieStreamsWithYear(imdbID string, releaseYear int) ([]TorrentioStream, error) {
	streams, err := mp.GetMovieStreams(imdbID)
	if err != nil {
		return nil, err
	}
	
	log.Printf("[YEAR-FILTER] Retrieved %d streams for %s before year filtering (release year: %d)", len(streams), imdbID, releaseYear)
	
	// If no release year provided, return all streams
	if releaseYear <= 0 {
		log.Printf("[YEAR-FILTER] No release year specified, returning all %d streams", len(streams))
		return streams, nil
	}
	
	// Filter streams by year - only keep streams matching the release year
	filtered := []TorrentioStream{}
	for _, stream := range streams {
		streamYear := extractYearFromStream(stream)
		
		// Keep stream if:
		// - No year found in stream (could be valid)
		// - Year matches release year
		if streamYear == 0 || streamYear == releaseYear {
			filtered = append(filtered, stream)
			if streamYear == 0 {
				log.Printf("[YEAR-FILTER] Keeping stream with no year detected: %s", stream.Title)
			} else {
				log.Printf("[YEAR-FILTER] Keeping stream with matching year %d: %s", streamYear, stream.Title)
			}
		} else {
			log.Printf("[YEAR-FILTER] ❌ Filtering out stream with mismatched year: %d != %d - %s", streamYear, releaseYear, stream.Title)
		}
	}
	
	log.Printf("[YEAR-FILTER] Filtered %d -> %d streams (removed %d)", len(streams), len(filtered), len(streams)-len(filtered))
	
	return filtered, nil
}

func (mp *MultiProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	var lastErr error
	var allStreams []TorrentioStream
	
	log.Printf("[PROVIDER] Fetching movie streams for IMDB ID: %s", imdbID)
	
	for i, provider := range mp.Providers {
		providerName := mp.ProviderNames[i]
		
		streams, err := provider.GetMovieStreams(imdbID)
		if err != nil {
			log.Printf("[PROVIDER] %s failed for movie %s: %v", providerName, imdbID, err)
			lastErr = err
			continue
		}
		
		log.Printf("[PROVIDER] %s returned %d streams for movie %s", providerName, len(streams), imdbID)
		
		// Filter out any episode streams that shouldn't be included in movie results
		filteredStreams := filterMovieStreams(streams)
		
		if len(filteredStreams) < len(streams) {
			log.Printf("[PROVIDER] %s: Filtered out %d episode-like streams (kept %d movie streams)", 
				providerName, len(streams)-len(filteredStreams), len(filteredStreams))
		}
		
		if len(filteredStreams) > 0 {
			log.Printf("[PROVIDER] %s added %d valid movie streams", providerName, len(filteredStreams))
			allStreams = append(allStreams, filteredStreams...)
		}
	}
	
	log.Printf("[PROVIDER] Total streams collected: %d", len(allStreams))
	
	if len(allStreams) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	
	return allStreams, nil
}

func (mp *MultiProvider) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	var lastErr error
	var allStreams []TorrentioStream
	
	for i, provider := range mp.Providers {
		providerName := mp.ProviderNames[i]
		
		streams, err := provider.GetSeriesStreams(imdbID, season, episode)
		if err != nil {
			log.Printf("Provider %s failed for series %s S%02dE%02d: %v", providerName, imdbID, season, episode, err)
			lastErr = err
			continue
		}
		
		if len(streams) > 0 {
			log.Printf("Provider %s returned %d streams for series %s S%02dE%02d", providerName, len(streams), imdbID, season, episode)
			allStreams = append(allStreams, streams...)
		}
	}
	
	if len(allStreams) == 0 && lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	
	return allStreams, nil
}

func (mp *MultiProvider) GetBestStream(imdbID string, season, episode *int, maxQuality int) (*TorrentioStream, error) {
	var streams []TorrentioStream
	var err error
	
	if season != nil && episode != nil {
		streams, err = mp.GetSeriesStreams(imdbID, *season, *episode)
	} else {
		streams, err = mp.GetMovieStreams(imdbID)
	}
	
	if err != nil {
		return nil, err
	}
	
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found")
	}
	
	// Prioritize cached streams, then accept uncached
	filteredStreams := make([]TorrentioStream, 0)
	for _, s := range streams {
		if s.Cached {
			filteredStreams = append(filteredStreams, s)
		}
	}
	
	if len(filteredStreams) == 0 {
		// No cached streams, accept all uncached streams
		filteredStreams = streams
	}
	
	if len(filteredStreams) == 0 {
		return nil, fmt.Errorf("no streams available after filtering")
	}
	
	// Sort streams - get settings from user preferences or use defaults
	sortOrder := "quality,size,seeders"
	sortPrefer := "best"
	if mp.getSortOrder != nil {
		sortOrder = mp.getSortOrder()
	}
	if mp.getSortPrefer != nil {
		sortPrefer = mp.getSortPrefer()
	}
	
	log.Printf("[SORT] Using sort order: %s, preference: %s", sortOrder, sortPrefer)
	
	// Parse sort fields
	sortFields := strings.Split(sortOrder, ",")
	
	// Sort streams using configurable sorting
	sortedStreams := make([]TorrentioStream, len(filteredStreams))
	copy(sortedStreams, filteredStreams)
	
	// Sort function based on preference
	for i := 0; i < len(sortedStreams)-1; i++ {
		for j := i + 1; j < len(sortedStreams); j++ {
			shouldSwap := false
			
			for _, field := range sortFields {
				field = strings.TrimSpace(field)
				cmp := compareStreams(sortedStreams[i], sortedStreams[j], field, sortPrefer)
				if cmp < 0 {
					shouldSwap = true
					break
				} else if cmp > 0 {
					break
				}
				// cmp == 0, continue to next field
			}
			
			if shouldSwap {
				sortedStreams[i], sortedStreams[j] = sortedStreams[j], sortedStreams[i]
			}
		}
	}
	
	if len(sortedStreams) > 0 {
		selected := sortedStreams[0]
		log.Printf("Selected stream: %s (Quality: %s, Size: %d MB, Seeders: %d)", 
			truncateString(selected.Name, 60), selected.Quality, selected.Size/(1024*1024), selected.Seeders)
		return &selected, nil
	}
	
	return nil, fmt.Errorf("no streams available")
}

// compareStreams compares two streams by a specific field
// Returns: 1 if a > b, -1 if a < b, 0 if equal
func compareStreams(a, b TorrentioStream, field string, prefer string) int {
	switch field {
	case "quality":
		aQuality := parseQualityInt(a.Quality)
		bQuality := parseQualityInt(b.Quality)
		if prefer == "smallest" || prefer == "lowest" {
			// For smallest preference, lower quality is better
			if aQuality < bQuality {
				return 1
			} else if aQuality > bQuality {
				return -1
			}
		} else {
			// Default: higher quality is better
			if aQuality > bQuality {
				return 1
			} else if aQuality < bQuality {
				return -1
			}
		}
	case "size":
		if prefer == "smallest" || prefer == "lowest" {
			// Smaller size is better
			if a.Size < b.Size && a.Size > 0 {
				return 1
			} else if a.Size > b.Size && b.Size > 0 {
				return -1
			}
		} else {
			// Default: larger size is better (usually better quality)
			if a.Size > b.Size {
				return 1
			} else if a.Size < b.Size {
				return -1
			}
		}
	case "seeders":
		if prefer == "smallest" || prefer == "lowest" {
			// Fewer seeders (unusual preference)
			if a.Seeders < b.Seeders {
				return 1
			} else if a.Seeders > b.Seeders {
				return -1
			}
		} else {
			// Default: more seeders is better
			if a.Seeders > b.Seeders {
				return 1
			} else if a.Seeders < b.Seeders {
				return -1
			}
		}
	}
	return 0
}

// truncateString truncates a string to max length
func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// parseQualityInt converts a quality string to an integer value for comparison
func parseQualityInt(quality string) int {
	q := strings.ToUpper(quality)
	
	// Check for resolution-based quality indicators
	if strings.Contains(q, "4K") || strings.Contains(q, "2160") {
		return 2160
	}
	if strings.Contains(q, "1080") {
		return 1080
	}
	if strings.Contains(q, "720") {
		return 720
	}
	if strings.Contains(q, "480") {
		return 480
	}
	if strings.Contains(q, "SD") || strings.Contains(q, "360") {
		return 360
	}
	
	// Default to 720p if quality is unknown
	return 720
}

// filterMovieStreams removes streams that appear to be from TV series episodes
// This prevents episode results (S01E01, S02E05, etc.) from appearing in movie stream results
func filterMovieStreams(streams []TorrentioStream) []TorrentioStream {
	filtered := []TorrentioStream{}
	
	for _, stream := range streams {
		// Check if this stream looks like a TV episode
		// Common patterns: "S01E01", "1x01", "Season 1 Episode 1", etc.
		if isEpisodeStream(stream) {
			log.Printf("[EPISODE-FILTER] ❌ Filtering out episode-like stream: %s (from source: %s)", stream.Title, stream.Source)
			continue
		}
		filtered = append(filtered, stream)
	}
	
	if len(filtered) < len(streams) {
		log.Printf("[EPISODE-FILTER] Filtered %d -> %d streams (removed %d episode-like streams)", 
			len(streams), len(filtered), len(streams)-len(filtered))
	}
	
	return filtered
}

// isEpisodeStream checks if a stream appears to be from a TV series episode
func isEpisodeStream(stream TorrentioStream) bool {
	// Combine all text fields to search
	fullText := strings.ToLower(fmt.Sprintf("%s %s %s", stream.Name, stream.Title, stream.Source))
	
	// Check for season/episode patterns
	episodePatterns := []string{
		"s\\d+e\\d+",     // S01E01
		"\\d+x\\d+",      // 1x01
		"season.*episode", // Season 1 Episode 1
		"ep\\d+",         // EP01
		": s\\d+:",       // : S01:
	}
	
	for _, pattern := range episodePatterns {
		if matched, _ := regexp.MatchString(pattern, fullText); matched {
			return true
		}
	}
	
	return false
}

// extractYearFromStream extracts the year from stream title/filename
func extractYearFromStream(stream TorrentioStream) int {
	// Search in title and name for 4-digit year
	fullText := fmt.Sprintf("%s %s", stream.Title, stream.Name)
	
	// Look for years between 1900 and 2100
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := yearPattern.FindAllString(fullText, -1)
	
	if len(matches) > 0 {
		// Return the first year found
		if year, err := strconv.Atoi(matches[0]); err == nil {
			log.Printf("[YEAR-EXTRACT] Found year %d in stream: %s", year, stream.Title)
			return year
		}
	}
	
	log.Printf("[YEAR-EXTRACT] No year found in stream: %s", stream.Title)
	return 0
}
