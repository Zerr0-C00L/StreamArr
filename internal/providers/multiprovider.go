package providers

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
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

// ReleaseFilters contains settings for filtering out unwanted releases
type ReleaseFilters struct {
	Enabled           bool
	ExcludedQualities string // e.g., "REMUX|HDR|DV|CAM|TS"
	ExcludedGroups    string // e.g., "TVHUB|FILM"
	ExcludedLanguages string // e.g., "RUSSIAN|RUS|HINDI"
	ExcludedCustom    string // custom patterns
}

// StreamSortOptions contains settings for stream sorting and selection
type StreamSortOptions struct {
	SortOrder  string // e.g., "quality,size,seeders" - comma-separated sort fields
	SortPrefer string // "best" (highest quality/size), "smallest" (lowest size), "balanced"
}

type StreamProvider interface {
	GetMovieStreams(imdbID string) ([]TorrentioStream, error)
	GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error)
}

type MultiProvider struct {
	Providers     []StreamProvider
	ProviderNames []string
}

func NewMultiProvider(rdAPIKey string, addons []StremioAddon, tmdbClient *services.TMDBClient) *MultiProvider {
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
	
	// Add default Torrentio addon if Real-Debrid is configured
	if len(mp.Providers) == 0 && rdAPIKey != "" {
		log.Println("⚠️  No Stremio addons configured - using Torrentio with Real-Debrid")
		
		// Torrentio is a popular public Stremio addon that works with Real-Debrid
		torrentioProvider := NewGenericStremioProvider("Torrentio", "https://torrentio.stremio.space/manifest.json", rdAPIKey)
		mp.Providers = append(mp.Providers, torrentioProvider)
		mp.ProviderNames = append(mp.ProviderNames, "Torrentio (Real-Debrid)")
		log.Println("✓ Torrentio addon loaded with Real-Debrid")
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

func (mp *MultiProvider) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	var lastErr error
	var allStreams []TorrentioStream
	
	for i, provider := range mp.Providers {
		providerName := mp.ProviderNames[i]
		
		streams, err := provider.GetMovieStreams(imdbID)
		if err != nil {
			log.Printf("Provider %s failed for movie %s: %v", providerName, imdbID, err)
			lastErr = err
			continue
		}
		
		if len(streams) > 0 {
			log.Printf("Provider %s returned %d streams for movie %s", providerName, len(streams), imdbID)
			allStreams = append(allStreams, streams...)
		}
	}
	
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

func (mp *MultiProvider) GetBestStream(imdbID string, season, episode *int, maxQuality int, filters *ReleaseFilters, sortOpts *StreamSortOptions) (*TorrentioStream, error) {
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
	
	// Build exclusion regex pattern from filters
	var excludePattern *regexp.Regexp
	if filters != nil && filters.Enabled {
		patterns := make([]string, 0)
		if filters.ExcludedQualities != "" {
			patterns = append(patterns, filters.ExcludedQualities)
		}
		if filters.ExcludedGroups != "" {
			patterns = append(patterns, filters.ExcludedGroups)
		}
		if filters.ExcludedLanguages != "" {
			patterns = append(patterns, filters.ExcludedLanguages)
		}
		if filters.ExcludedCustom != "" {
			patterns = append(patterns, filters.ExcludedCustom)
		}
		
		if len(patterns) > 0 {
			// Use lookahead/lookbehind-like pattern that matches terms separated by
			// dots, spaces, dashes, underscores, or at string boundaries
			// This catches things like .DV. or .HDR. or -REMUX- in filenames
			combinedPattern := `(?i)(?:^|[\s.\-_\[\]()])(`  + strings.Join(patterns, "|") + `)(?:$|[\s.\-_\[\]()])`
			excludePattern, _ = regexp.Compile(combinedPattern)
		}
	}
	
	// Filter by max quality, cached status, and release filters
	filteredStreams := make([]TorrentioStream, 0)
	for _, s := range streams {
		// Apply release filters
		if excludePattern != nil {
			// Check Name, Title, and URL fields for filter matches
			// URL decode the URL to catch encoded patterns like %5B47BT%5D -> [47BT]
			decodedURL, _ := url.QueryUnescape(s.URL)
			checkStr := s.Name + " " + s.Title + " " + decodedURL
			if excludePattern.MatchString(checkStr) {
				log.Printf("Filtered out stream (release filter): %s", truncateString(s.Name, 80))
				continue
			}
		}
		
		if s.Cached {
			quality := parseQualityInt(s.Quality)
			if quality <= maxQuality {
				filteredStreams = append(filteredStreams, s)
			}
		}
	}
	
	if len(filteredStreams) == 0 {
		// No cached streams after filtering, try uncached streams with filters
		for _, s := range streams {
			if excludePattern != nil {
				decodedURL, _ := url.QueryUnescape(s.URL)
				checkStr := s.Name + " " + s.Title + " " + decodedURL
				if excludePattern.MatchString(checkStr) {
					continue
				}
			}
			filteredStreams = append(filteredStreams, s)
		}
		
		if len(filteredStreams) == 0 {
			return nil, fmt.Errorf("no streams available after filtering")
		}
		return &filteredStreams[0], nil
	}
	
	// Sort streams based on sort options
	sortOrder := "quality,size,seeders" // default
	sortPrefer := "best"                 // default: highest quality, then largest size
	
	if sortOpts != nil {
		if sortOpts.SortOrder != "" {
			sortOrder = sortOpts.SortOrder
		}
		if sortOpts.SortPrefer != "" {
			sortPrefer = sortOpts.SortPrefer
		}
	}
	
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
