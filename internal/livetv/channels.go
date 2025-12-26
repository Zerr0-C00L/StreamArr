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
	Name               string   `json:"name"`
	URL                string   `json:"url"`
	EPGURL             string   `json:"epg_url,omitempty"`
	Enabled            bool     `json:"enabled"`
	SelectedCategories []string `json:"selected_categories,omitempty"`
}

// XtreamSource represents an Xtream Codes compatible IPTV provider
type XtreamSource struct {
	Name      string `json:"name"`
	ServerURL string `json:"server_url"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Enabled   bool   `json:"enabled"`
}

// Third-party IPTV integration removed

type ChannelManager struct {
	channels           map[string]*Channel
	mu                 sync.RWMutex
	sources            []ChannelSource
	m3uSources         []M3USource
	xtreamSources      []XtreamSource
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

// Third-party IPTV configuration setter removed

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

	// Third-party IPTV loading removed

	// Load from custom M3U sources (user-configured)
	for _, source := range cm.m3uSources {
		if !source.Enabled {
			continue
		}
		fmt.Printf("[DEBUG] Loading %s with selected categories: %v (count: %d)\n", source.Name, source.SelectedCategories, len(source.SelectedCategories))
		channels, err := cm.loadFromM3UURLWithCategories(source.URL, source.Name, source.SelectedCategories)
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
		if len(cm.m3uSources) == 0 && len(cm.xtreamSources) == 0 {
			fmt.Println("Add Custom M3U/Xtream Sources in Settings → Live TV")
		}
		cm.channels = make(map[string]*Channel)
		return nil
	}

	// Smart duplicate merging - normalize channel names and keep best quality
	// Only merge duplicates WITHIN THE SAME CATEGORY (not across categories)
	cm.channels = make(map[string]*Channel)
	channelsByNormalizedName := make(map[string]*Channel)

	for _, ch := range allChannels {
		// Include category in the deduplication key so same-named channels in different categories are kept
		normalizedKey := ch.Category + "|" + normalizeChannelName(ch.Name)

		existing, exists := channelsByNormalizedName[normalizedKey]
		if !exists {
			// First occurrence - add it
			channelsByNormalizedName[normalizedKey] = ch
			cm.channels[ch.ID] = ch
		} else {
			// Duplicate found within same category - keep the one with better data (logo, stream URL)
			if shouldReplaceChannel(existing, ch) {
				// Remove old channel
				delete(cm.channels, existing.ID)
				// Add new channel
				channelsByNormalizedName[normalizedKey] = ch
				cm.channels[ch.ID] = ch
			}
		}
	}

	// Debug: Count channels per category
	categoryCount := make(map[string]int)
	for _, ch := range cm.channels {
		categoryCount[ch.Category]++
	}
	fmt.Printf("Live TV: Loaded %d unique channels (merged from %d total)\n", len(cm.channels), len(allChannels))
	fmt.Printf("[DEBUG] Channel count per category: %v\n", categoryCount)
	
	// Debug: Find which sources have uncategorized channels
	uncategorizedBySource := make(map[string]int)
	for _, ch := range cm.channels {
		if ch.Category == "Uncategorized" {
			uncategorizedBySource[ch.Source]++
		}
	}
	if len(uncategorizedBySource) > 0 {
		fmt.Printf("[DEBUG] Uncategorized channels by source: %v\n", uncategorizedBySource)
	}
	
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

// categoryMapping maps various category names to normalized English categories
var categoryMapping = map[string]string{
	// Action
	"action":        "Action",
	"action sports": "Action",
	"adventure":     "Action",

	// Animation & Anime
	"animation":              "Animation",
	"animated":               "Animation",
	"anime":                  "Animation",
	"anime & gaming":         "Animation",
	"anime & geek":           "Animation",
	"animazione e bambini":   "Animation",
	"anime & gaming, kids & family": "Animation",

	// Comedy
	"comedy":              "Comedy",
	"comedia":             "Comedy",
	"comédia":             "Comedy",
	"comédie":             "Comedy",
	"humor":               "Comedy",
	"komedi":              "Comedy",
	"komedie":             "Comedy",
	"sitcom":              "Comedy",
	"sitcoms":             "Comedy",
	"sitcoms + comedy":    "Comedy",
	"dark comedy":         "Comedy",
	"comedy drama":        "Comedy",
	"comedy live":         "Comedy",
	"comedy, hit tv":      "Comedy",
	"comedy live, entertainment live": "Comedy",

	// Crime & Mystery
	"crime":                "Crime & Mystery",
	"crime drama":          "Crime & Mystery",
	"crime files":          "Crime & Mystery",
	"true crime":           "Crime & Mystery",
	"policiacas":           "Crime & Mystery",
	"crime & mystère":      "Crime & Mystery",
	"crimen y misterio":    "Crime & Mystery",
	"investigación":        "Crime & Mystery",
	"investigação":         "Crime & Mystery",
	"mystery":              "Crime & Mystery",
	"serie crime":          "Crime & Mystery",
	"séries policières":    "Crime & Mystery",
	"true crime, drama tv": "Crime & Mystery",
	"true crime, en español": "Crime & Mystery",

	// Documentary
	"documentary":          "Documentary",
	"documentaries":        "Documentary",
	"documentaire":         "Documentary",
	"documentales":         "Documentary",
	"documentari":          "Documentary",
	"dokumentarer":         "Documentary",
	"dokumentärer":         "Documentary",
	"dokus + wissen":       "Documentary",
	"documentary + science": "Documentary",
	"biography":            "Documentary",

	// Drama
	"drama":           "Drama",
	"drama tv":        "Drama",
	"bingeable drama": "Drama",
	"tv dramas":       "Drama",
	"novelas":         "Drama",
	"séries":          "Drama",
	"serie":           "Drama",
	"serie classiche": "Drama",
	"serien-marathon": "Drama",
	"hit tv":          "Drama",
	"hit tv, drama tv": "Drama",
	"hit tv, reality": "Drama",

	// Entertainment
	"entertainment":      "Entertainment",
	"divertissement":     "Entertainment",
	"entretenimiento":    "Entertainment",
	"intrattenimento":    "Entertainment",
	"pop culture":        "Entertainment",
	"entertainment live": "Entertainment",
	"talkshow":           "Entertainment",
	"daytime & talk shows": "Entertainment",
	"daytime tv":         "Entertainment",
	"emissions cultes":   "Entertainment",
	"auction":            "Entertainment",

	// Food & Lifestyle
	"food":                "Food & Lifestyle",
	"food & home":         "Food & Lifestyle",
	"cooking":             "Food & Lifestyle",
	"lifestyle":           "Food & Lifestyle",
	"home improvement":    "Food & Lifestyle",
	"living":              "Food & Lifestyle",
	"good eats":           "Food & Lifestyle",
	"estilo de vida":      "Food & Lifestyle",
	"home + food":         "Food & Lifestyle",
	"house/garden":        "Food & Lifestyle",
	"mad & livsstil":      "Food & Lifestyle",
	"mat & livsstil":      "Food & Lifestyle",
	"food & home, lifestyle": "Food & Lifestyle",
	"food and fitness live": "Food & Lifestyle",
	"lifestyle, en español": "Food & Lifestyle",

	// Horror & Paranormal
	"horror":              "Horror & Paranormal",
	"horror & paranormal": "Horror & Paranormal",
	"paranormal":          "Horror & Paranormal",
	"chills & thrills":    "Horror & Paranormal",
	"terror":              "Horror & Paranormal",
	"house of horror":     "Horror & Paranormal",
	"horror e paranormale": "Horror & Paranormal",
	"zona paranormal":     "Horror & Paranormal",
	"overnaturlig":        "Horror & Paranormal",
	"övernaturligt":       "Horror & Paranormal",
	"mistérios e sobrenatural": "Horror & Paranormal",
	"chills & thrills, sci-fi & action": "Horror & Paranormal",

	// Kids & Family
	"kids":            "Kids & Family",
	"kids & family":   "Kids & Family",
	"infantil":        "Kids & Family",
	"barn":            "Kids & Family",
	"for barn":        "Kids & Family",
	"for børn":        "Kids & Family",
	"children-music":  "Kids & Family",
	"nickelodeon":     "Kids & Family",
	"kids en français": "Kids & Family",
	"kids & family, anime & gaming": "Kids & Family",
	"kids & family, en español": "Kids & Family",
	"teen":            "Kids & Family",

	// Movies
	"cine":             "Movies",
	"cinéma":           "Movies",
	"películas":        "Movies",
	"westerns":         "Movies",
	"western":          "Movies",
	"explosão de cinema": "Movies",
	"romance":          "Movies",
	"à binge-watch":    "Movies",

	// Music
	"music":        "Music",
	"música":       "Music",
	"musica":       "Music",
	"musik":        "Music",
	"musikk":       "Music",
	"musique":      "Music",
	"music videos": "Music",
	"mtv":          "Music",
	"music live":   "Music",
	"music talk":   "Music",
	"music, en español": "Music",
	"music live, entertainment live": "Music",
	"det bedste fra mtv": "Music",
	"det beste fra mtv": "Music",
	"det bästa från mtv": "Music",
	"mtv en pluto tv": "Music",

	// News
	"news":           "News",
	"noticias":       "News",
	"notícias":       "News",
	"nachrichten":    "News",
	"nyheter":        "News",
	"local news":     "News",
	"national news":  "News",
	"global news":    "News",
	"business news":  "News",
	"news + opinion": "News",
	"news e mondo":   "News",
	"news, en español": "News",
	"news, hit tv":   "News",
	"news flash live": "News",
	"news flash live, entertainment live": "News",
	"news flash live, finance and business": "News",
	"news flash live, stirr cities": "News",
	"news flash live, sports live": "News",
	"weather":        "News",
	"finance and business": "News",
	"bus./financial": "News",

	// Reality
	"reality":              "Reality",
	"reality show":         "Reality",
	"tv réalité":           "Reality",
	"realityserier":        "Reality",
	"competition reality":  "Reality",
	"competencia":          "Reality",
	"real life adventure":  "Reality",
	"dansk reality & underholdning": "Reality",
	"norsk reality og underholdning": "Reality",
	"svensk reality och underhållning": "Reality",
	"paradise hotel":       "Reality",
	"reality, en español":  "Reality",
	"reality, food & home": "Reality",
	"history & science, reality": "Reality",

	// Sci-Fi & Fantasy
	"sci-fi":              "Sci-Fi & Fantasy",
	"sci-fi & fantasy":    "Sci-Fi & Fantasy",
	"sci-fi & action":     "Sci-Fi & Fantasy",
	"sci-fi & supernatural": "Sci-Fi & Fantasy",
	"science fiction":     "Sci-Fi & Fantasy",
	"star trek":           "Sci-Fi & Fantasy",
	"ciencia ficción":     "Sci-Fi & Fantasy",

	// Sports
	"sports":          "Sports",
	"sport":           "Sports",
	"deportes":        "Sports",
	"esportes":        "Sports",
	"live sports":     "Sports",
	"sports live":     "Sports",
	"sports on now":   "Sports",
	"baseball":        "Sports",
	"basketball":      "Sports",
	"football":        "Sports",
	"soccer":          "Sports",
	"golf":            "Sports",
	"boxing":          "Sports",
	"billiards":       "Sports",
	"fishing":         "Sports",
	"hunting":         "Sports",
	"olympics":        "Sports",
	"bullfighting":    "Sports",
	"sports & auto":   "Sports",
	"auto":            "Sports",
	"motori e sport":  "Sports",
	"deportes y gaming": "Sports",
	"sports, en español": "Sports",
	"sports live, entertainment live": "Sports",

	// Classic TV
	"classic tv":         "Classic TV",
	"classic tv comedy":  "Classic TV",
	"retro":              "Classic TV",
	"retrô":              "Classic TV",
	"klassiske tv-serier": "Classic TV",
	"gamla godingar":     "Classic TV",
	"gamle godbiter":     "Classic TV",
	"classic tv, hit tv": "Classic TV",

	// Game Shows
	"game shows":          "Game Shows",
	"game show":           "Game Shows",
	"daytime + game shows": "Game Shows",
	"games & competition": "Game Shows",
	"hit tv, game shows":  "Game Shows",

	// Nature & Science
	"nature":           "Nature & Science",
	"nature & travel":  "Nature & Science",
	"animals":          "Nature & Science",
	"animals + nature": "Nature & Science",
	"science & nature": "Nature & Science",
	"history & science": "Nature & Science",
	"history + science": "Nature & Science",
	"environment":      "Nature & Science",
	"natureza":         "Nature & Science",
	"history":          "Nature & Science",

	// Spanish Content
	"en español":         "En Español",
	"español":            "En Español",
	"hit tv, en español": "En Español",

	// Holiday/Seasonal
	"season's greetings":   "Holiday",
	"merry christmas!":     "Holiday",
	"epische weihnachten":  "Holiday",
	"natale":               "Holiday",
	"jul på pluto tv":      "Holiday",
	"¡feliz navidad!":      "Holiday",
	"¡fiestas con todo!":   "Holiday",
	"boas festas coca-cola": "Holiday",

	// Faith
	"faith": "Faith & Family",

	// International/Regional
	"100% français":     "International",
	"international":     "International",
	"tv brasileira":     "International",

	// Gaming
	"gaming":   "Gaming",
	"computers": "Gaming",

	// Art & Culture
	"art": "Arts & Culture",

	// Shopping
	"shopping":      "Shopping",
	"shopping live": "Shopping",

	// Platform-specific (map to relevant category)
	"new on pluto tv":       "Entertainment",
	"nuevo en pluto tv":     "Entertainment",
	"nouveau sur pluto tv":  "Entertainment",
	"nyt på pluto tv":       "Entertainment",
	"nytt på pluto tv":      "Entertainment",
	"how to use pluto tv":   "Entertainment",
	"paramount+ apresenta":  "Entertainment",
	"paramount+ presenta":   "Entertainment",
	"curiosidad":            "Entertainment",
	"curiosidades":          "Entertainment",
	"south park":            "Comedy",
	"black entertainment":   "Entertainment",
	"sinnliche fantasien":   "Entertainment",
	"law":                   "Crime & Mystery",
	"health":                "Food & Lifestyle",
	"special":               "Entertainment",
	"other":                 "Entertainment",
}

// NormalizeCategory maps a category to a normalized English category
func NormalizeCategory(category string) string {
	if category == "" {
		return "Uncategorized"
	}
	
	// Try exact match (case-insensitive)
	lowerCat := strings.ToLower(strings.TrimSpace(category))
	if normalized, ok := categoryMapping[lowerCat]; ok {
		return normalized
	}
	
	// Try partial match for compound categories
	for key, value := range categoryMapping {
		if strings.Contains(lowerCat, key) {
			return value
		}
	}
	
	// Return original if no mapping found
	return category
}

// SmartCategorizeChannel uses AI-like keyword matching to categorize a channel by its name
// This is used for channels that have no category from the M3U source
func SmartCategorizeChannel(channelName string) string {
	name := strings.ToLower(channelName)
	
	// News channels - check first as they're common
	newsKeywords := []string{"news", "cnn", "msnbc", "bbc", "cnbc", "bloomberg", "c-span", "cspan",
		"sky news", "al jazeera", "abc news", "cbs news", "nbc news", "newsmax", "oan", "fox news",
		"headline", "euronews", "n1", "reuters", "ap news", "world news", "breaking", "inter 24/7",
		"france 24", "dateline", "actualidad", "info", "cnñ", "xpress"}
	for _, kw := range newsKeywords {
		if strings.Contains(name, kw) {
			return "News"
		}
	}
	
	// Weather
	if strings.Contains(name, "weather") || strings.Contains(name, "accuweather") || strings.Contains(name, "météo") {
		return "News"
	}
	
	// Sports channels
	sportsKeywords := []string{"sport", "espn", "nfl", "nba", "mlb", "nhl", "golf", "tennis",
		"bein", "dazn", "soccer", "football", "baseball", "basketball", "hockey", "cricket",
		"wwe", "ufc", "boxing", "racing", "f1", "formula", "nascar", "motogp", "olympic",
		"athletic", "arena", "supersport", "eurosport", "pga", "fifa", "mlb", "nascar",
		"red bull", "x games", "outdoor", "hunting", "fishing", "stadium", "poker", "racer",
		"triton poker", "fight club", "dfb", "lucha", "bassmaster", "extreme jobs", "deportes"}
	for _, kw := range sportsKeywords {
		if strings.Contains(name, kw) {
			return "Sports"
		}
	}
	
	// Kids & Family channels
	kidsKeywords := []string{"disney", "nick", "nickelodeon", "cartoon", "boomerang", "pbs kids",
		"baby", "junior", "kids", "children", "sesame", "sprout", "universal kids", "kidz",
		"toon", "animaniacs", "lego", "pokemon", "dora", "spongebob", "paw patrol", "wiggles",
		"barney", "my little pony", "garfield", "shaun the sheep", "arthur", "nastya",
		"pink panther", "barbie", "hasbro", "mattel", "dragons", "that girl", "polly pocket",
		"mrbeast", "pocket.watch", "ryan and friends", "addams family", "teens"}
	for _, kw := range kidsKeywords {
		if strings.Contains(name, kw) {
			return "Kids & Family"
		}
	}
	
	// Music channels
	musicKeywords := []string{"mtv", "vh1", "vevo", "music", "hit", "radio", "fm ", "concert",
		"hip hop", "rock", "jazz", "country", "cmt", "bet ", "soul", "r&b", "pop", "classic rock",
		"80s", "90s", "70s", "fuse", "revolt", "trace", "xite", "dance moms", "dance"}
	for _, kw := range musicKeywords {
		if strings.Contains(name, kw) {
			return "Music"
		}
	}
	
	// Movies channels
	movieKeywords := []string{"movie", "cinema", "film", "hbo", "cinemax", "showtime", "starz",
		"epix", "mgm", "tcm", "amc", "ifc", "sundance", "hallmark", "lifetime movie",
		"fx movie", "sony movie", "thriller", "action movie", "western", "cine", "trailers",
		"allociné", "allocine", "runtime", "pelimex", "hollywood", "wonderful life", "cindie",
		"box office", "acorn"}
	for _, kw := range movieKeywords {
		if strings.Contains(name, kw) {
			return "Movies"
		}
	}
	
	// Documentary/Nature/Science
	docKeywords := []string{"discovery", "national geographic", "nat geo", "history", "science",
		"animal planet", "smithsonian", "pbs", "nature", "planet earth", "wild", "ocean",
		"space", "cosmos", "universe", "world", "geo", "documentary", "vice", "curiosity",
		"mayday", "air disaster", "catastrophe", "bondi vet", "timber kings", "life down under",
		"historia", "echos du monde", "cosmic", "frontiers", "ax men", "modern marvels",
		"expedientes", "evidence of evil", "ctv gets real", "big stories"}
	for _, kw := range docKeywords {
		if strings.Contains(name, kw) {
			return "Nature & Science"
		}
	}
	
	// Crime & Mystery
	crimeKeywords := []string{"crime", "mystery", "detective", "investigation", "true crime",
		"law & order", "csi", "ncis", "forensic", "court", "justice", "fbi", "cia", "police",
		"midsomer", "first 48", "mi-5", "relic hunter", "outlaw", "lawless", "murdoch",
		"blacklist", "caso cerrado", "love after lockup", "chaos on cam", "mysteries",
		"shades of black", "sobrenaturales"}
	for _, kw := range crimeKeywords {
		if strings.Contains(name, kw) {
			return "Crime & Mystery"
		}
	}
	
	// Comedy
	comedyKeywords := []string{"comedy", "funny", "laugh", "sitcom", "stand up", "comic",
		"snl", "saturday night", "conan", "late night", "daily show", "colbert", "graham norton",
		"green acres", "weeds", "nurse jackie", "comédie", "wendy williams", "les débatteurs",
		"les filles", "ça c'est drôle", "kim's convenience", "bizaar"}
	for _, kw := range comedyKeywords {
		if strings.Contains(name, kw) {
			return "Comedy"
		}
	}
	
	// Food & Lifestyle
	foodKeywords := []string{"food", "cook", "kitchen", "chef", "recipe", "hgtv", "home",
		"garden", "diy", "lifestyle", "travel", "tlc", "bravo", "e!", "magnolia",
		"renovation", "design", "property", "house", "taste", "bon appetit", "viajes",
		"sabores", "voyages", "saveurs", "voyage", "mueble", "come dine", "hotel inspector",
		"inside outside", "epicurieux", "platos", "bodas"}
	for _, kw := range foodKeywords {
		if strings.Contains(name, kw) {
			return "Food & Lifestyle"
		}
	}
	
	// Reality TV
	realityKeywords := []string{"reality", "real housewives", "survivor", "big brother",
		"bachelor", "bachelorette", "love island", "jersey shore", "kardashian", "pawn",
		"storage", "auction", "swap", "makeover", "idol", "voice", "talent", "x factor",
		"shark tank", "little women", "vidas extremas", "dude perfect", "the doctors",
		"geraldo", "bold and the beautiful", "pasión", "passion", "duck dynasty",
		"dragons' den", "gata salvaje", "amor", "piel salvaje", "we tv", "osbournes",
		"preston & brianna", "tvone"}
	for _, kw := range realityKeywords {
		if strings.Contains(name, kw) {
			return "Reality"
		}
	}
	
	// Horror & Paranormal
	horrorKeywords := []string{"horror", "scary", "fear", "terror", "paranormal", "ghost",
		"haunted", "supernatural", "zombie", "vampire", "monster", "scream", "chiller", "monstruos",
		"hantise", "haunting"}
	for _, kw := range horrorKeywords {
		if strings.Contains(name, kw) {
			return "Horror & Paranormal"
		}
	}
	
	// Sci-Fi & Fantasy
	scifiKeywords := []string{"sci-fi", "scifi", "science fiction", "star trek", "star wars",
		"fantasy", "syfy", "doctor who", "alien", "galaxy", "futuristic", "outer limits",
		"the outpost", "cirque du soleil"}
	for _, kw := range scifiKeywords {
		if strings.Contains(name, kw) {
			return "Sci-Fi & Fantasy"
		}
	}
	
	// Animation/Anime
	animeKeywords := []string{"anime", "animation", "animated", "toonami", "crunchyroll",
		"funimation", "manga", "cartoon network", "adult swim"}
	for _, kw := range animeKeywords {
		if strings.Contains(name, kw) {
			return "Animation"
		}
	}
	
	// Game Shows
	gameShowKeywords := []string{"game show", "gameshow", "wheel of fortune", "jeopardy",
		"price is right", "deal or no deal", "family feud", "who wants", "quiz", "trivia",
		"pointless", "game & fish", "game-on"}
	for _, kw := range gameShowKeywords {
		if strings.Contains(name, kw) {
			return "Game Shows"
		}
	}
	
	// Classic TV
	classicKeywords := []string{"classic", "retro", "vintage", "golden", "nostalgia", "old school",
		"tv land", "antenna", "me tv", "metv", "decades", "buzzr", "legends", "bonanza",
		"alerte à malibu"}
	for _, kw := range classicKeywords {
		if strings.Contains(name, kw) {
			return "Classic TV"
		}
	}
	
	// Spanish/Latino/French/International content
	internationalKeywords := []string{"español", "espanol", "spanish", "latino", "latina", "telemundo",
		"univision", "azteca", "televisa", "galavision", "unimas", "novela", "mexic", 
		"teleonce", "asesinatos", "soleil", "éxitos", "séries", "favoris", "tv5monde",
		"noovo", "canela", "zee mundo", "amasian", "hong kong", "wedotv", "atres",
		"aventura", "plus", "365blk", "emoción", "comercio", "merli", "xataka", "latinx",
		"revry", "itv", "black effect", "stars & stories", "wedo"}
	for _, kw := range internationalKeywords {
		if strings.Contains(name, kw) {
			return "International"
		}
	}
	
	// Holiday/Christmas
	holidayKeywords := []string{"christmas", "holiday", "xmas", "santa", "halloween", "easter",
		"thanksgiving", "valentine", "new year"}
	for _, kw := range holidayKeywords {
		if strings.Contains(name, kw) {
			return "Holiday"
		}
	}
	
	// Faith & Family
	faithKeywords := []string{"faith", "church", "gospel", "christian", "religious", "god",
		"jesus", "prayer", "worship", "trinity", "daystar", "tbn", "catholic", "bible",
		"inspirational", "uplift"}
	for _, kw := range faithKeywords {
		if strings.Contains(name, kw) {
			return "Faith & Family"
		}
	}
	
	// Shopping
	shoppingKeywords := []string{"shop", "qvc", "hsn", "shopping", "jewelry", "buy", "deal"}
	for _, kw := range shoppingKeywords {
		if strings.Contains(name, kw) {
			return "Shopping"
		}
	}
	
	// Drama (general) - specific shows
	dramaKeywords := []string{"drama", "soap", "series", "primetime", "rookie blue", "the outpost"}
	for _, kw := range dramaKeywords {
		if strings.Contains(name, kw) {
			return "Drama"
		}
	}
	
	// Entertainment (broad catch-all for networks)
	entertainmentKeywords := []string{"abc", "nbc", "cbs", "fox", "cw", "tbs", "tnt", "usa",
		"freeform", "paramount", "ion", "bounce", "grit", "comet", "charge",
		"pluto", "tubi", "roku", "plex", "stirr", "xumo", "live", "channel",
		"network", "broadcast", "stream", "sony one", "stars central", "cut ", "wineman",
		"9 story", "america", "fast", "play", "combatv", "rio", "mgg", "billies"}
	for _, kw := range entertainmentKeywords {
		if strings.Contains(name, kw) {
			return "Entertainment"
		}
	}
	
	// Still uncategorized
	return "Uncategorized"
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

// ExtractEPGURLFromM3U extracts the url-tvg attribute from M3U content header
func ExtractEPGURLFromM3U(content string) string {
	// Look for url-tvg in the first line (#EXTM3U)
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	
	firstLine := lines[0]
	if !strings.HasPrefix(firstLine, "#EXTM3U") {
		return ""
	}
	
	// Extract url-tvg="..."
	if idx := strings.Index(firstLine, "url-tvg=\""); idx != -1 {
		start := idx + 9 // len("url-tvg=\"")
		end := strings.Index(firstLine[start:], "\"")
		if end != -1 {
			return firstLine[start : start+end]
		}
	}
	
	return ""
}

// FetchAndExtractEPGURL fetches an M3U URL and extracts the EPG URL from it
func FetchAndExtractEPGURL(url string) string {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	
	// Only read the first 1KB to get the header
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	if n == 0 {
		return ""
	}
	
	return ExtractEPGURLFromM3U(string(buf[:n]))
}

// loadFromM3UURL loads channels from a remote M3U URL
func (cm *ChannelManager) loadFromM3UURL(url string, sourceName string) ([]*Channel, error) {
	return cm.loadFromM3UURLWithCategories(url, sourceName, nil)
}

// loadFromM3UURLWithCategories loads channels from a remote M3U URL with category filtering
func (cm *ChannelManager) loadFromM3UURLWithCategories(url string, sourceName string, selectedCategories []string) ([]*Channel, error) {
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
	
	return cm.parseM3UWithCategories(string(body), sourceName, selectedCategories)
}

// Third-party IPTV loader removed

// parseM3U parses M3U content and returns channels
func (cm *ChannelManager) parseM3U(content string, sourceName string) ([]*Channel, error) {
	return cm.parseM3UWithCategories(content, sourceName, nil)
}

// parseM3UWithCategories parses M3U content and returns channels, filtering by selected categories
func (cm *ChannelManager) parseM3UWithCategories(content string, sourceName string, selectedCategories []string) ([]*Channel, error) {
	channels := make([]*Channel, 0)
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	fmt.Printf("[DEBUG] parseM3UWithCategories: sourceName=%s, selectedCategories=%v, count=%d\n", sourceName, selectedCategories, len(selectedCategories))
	
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
			
			// Extract group-title as category
			if idx := strings.Index(line, "group-title=\""); idx != -1 {
				end := strings.Index(line[idx+13:], "\"")
				if end != -1 {
					currentChannel.Category = line[idx+13 : idx+13+end]
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
					// Use category from M3U group-title, fallback to "Uncategorized" if not set
					if currentChannel.Category == "" {
						currentChannel.Category = "Uncategorized"
					}
					
					// Store original category for filtering, then normalize for display
					originalCategory := currentChannel.Category
					currentChannel.Category = NormalizeCategory(currentChannel.Category)
					
					// If still uncategorized, try smart categorization based on channel name
					if currentChannel.Category == "Uncategorized" {
						smartCategory := SmartCategorizeChannel(currentChannel.Name)
						if smartCategory != "Uncategorized" {
							currentChannel.Category = smartCategory
						}
					}
					
					// Filter by selected categories if specified (match against original OR normalized)
					if len(selectedCategories) > 0 {
						categoryMatches := false
						for _, selectedCat := range selectedCategories {
							if strings.EqualFold(originalCategory, selectedCat) || strings.EqualFold(currentChannel.Category, selectedCat) {
								categoryMatches = true
								break
							}
						}
						if categoryMatches {
							channels = append(channels, currentChannel)
						}
					} else {
						// No filter - include all channels
						channels = append(channels, currentChannel)
					}
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
			// Use group-title as category
			currentChannel.Category = currentGroupTitle

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
					// Use category from M3U group-title, fallback to "Uncategorized" if not set
					if currentChannel.Category == "" {
						currentChannel.Category = "Uncategorized"
					}
					// Normalize category
					currentChannel.Category = NormalizeCategory(currentChannel.Category)
					
					// If still uncategorized, try smart categorization based on channel name
					if currentChannel.Category == "Uncategorized" {
						smartCategory := SmartCategorizeChannel(currentChannel.Name)
						if smartCategory != "Uncategorized" {
							currentChannel.Category = smartCategory
						}
					}
					
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
