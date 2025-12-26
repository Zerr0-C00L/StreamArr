package epg

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/livetv"
)

// Manager handles Electronic Program Guide data
type Manager struct {
	programs map[string][]livetv.EPGProgram // channel_id -> programs
	mu       sync.RWMutex
	sources  []EPGSource
	lastUpdate time.Time
}

type EPGSource interface {
	GetEPG(channelID string) ([]livetv.EPGProgram, error)
	Name() string
}

func NewEPGManager() *Manager {
	manager := &Manager{
		programs: make(map[string][]livetv.EPGProgram),
		sources:  []EPGSource{},
	}

	// Register EPG sources
	manager.sources = append(manager.sources,
		NewXMLTVSource(),
		NewPlutoTVEPGSource(),
	)

	return manager
}

// NewEPGManagerWithSettings creates an EPG manager with country-specific sources
// NewEPGManagerWithSettings kept for backward compatibility; now ignores countries
func NewEPGManagerWithSettings(_ []string) *Manager {
	return NewEPGManager()
}

// GetEPG returns EPG data for a specific channel
func (e *Manager) GetEPG(channelID string, date time.Time) []livetv.EPGProgram {
	e.mu.RLock()
	defer e.mu.RUnlock()

	programs, ok := e.programs[channelID]
	if !ok {
		return []livetv.EPGProgram{}
	}

	// Filter by date
	filtered := []livetv.EPGProgram{}
	start := date.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	for _, p := range programs {
		if p.StartTime.After(start) && p.StartTime.Before(end) {
			filtered = append(filtered, p)
		}
	}

	// Sort by start time
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.Before(filtered[j].StartTime)
	})

	return filtered
}

// GetCurrentProgram returns the currently airing program for a channel
func (e *Manager) GetCurrentProgram(channelID string) *livetv.EPGProgram {
	e.mu.RLock()
	defer e.mu.RUnlock()

	programs, ok := e.programs[channelID]
	if !ok {
		return nil
	}

	now := time.Now()
	for _, p := range programs {
		if now.After(p.StartTime) && now.Before(p.EndTime) {
			return &p
		}
	}

	return nil
}

// HasEPG checks if a channel has EPG data
func (e *Manager) HasEPG(channelID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.programs[channelID]
	return ok
}

// GetChannelCount returns number of channels with EPG data
func (e *Manager) GetChannelCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.programs)
}

// UpdateEPG refreshes EPG data from all sources
func (e *Manager) UpdateEPG(channels []livetv.Channel) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing programs
	e.programs = make(map[string][]livetv.EPGProgram)

	// Get the XMLTV source and bulk load all EPG data
	for _, source := range e.sources {
		if xmltvSource, ok := source.(*XMLTVSource); ok {
			xmltvSource.BulkLoadEPG(e.programs)
		}
	}

	fmt.Printf("EPG: Loaded programs for %d channels\n", len(e.programs))
	e.lastUpdate = time.Now()
	return nil
}

// Update triggers a refresh of EPG data
func (e *Manager) Update() {
	e.UpdateEPG(nil)
}

// Clear removes all cached EPG data
func (e *Manager) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.programs = make(map[string][]livetv.EPGProgram)
	e.lastUpdate = time.Time{}
	fmt.Println("EPG: Cache cleared")
}

// GenerateXMLTV generates XMLTV format EPG
func (e *Manager) GenerateXMLTV(channels []livetv.Channel) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tv := XMLTV{
		GeneratorName: "StreamArr Pro",
		GeneratorURL:  "https://github.com/streamarr/streamarr",
	}

	// Add channels
	for _, ch := range channels {
		tv.Channels = append(tv.Channels, XMLTVChannel{
			ID: ch.ID,
			DisplayNames: []DisplayName{
				{Value: ch.Name},
			},
			Icon: Icon{Src: ch.Logo},
		})
	}

	// Add programs
	for channelID, programs := range e.programs {
		for _, prog := range programs {
			tv.Programs = append(tv.Programs, XMLTVProgram{
				Start:   prog.StartTime.Format("20060102150405 -0700"),
				Stop:    prog.EndTime.Format("20060102150405 -0700"),
				Channel: channelID,
				Title:   []Title{{Value: prog.Title}},
				Desc:    []Desc{{Value: prog.Description}},
				Category: []Category{{Value: prog.Category}},
			})
		}
	}

	output, err := xml.MarshalIndent(tv, "", "  ")
	if err != nil {
		return "", err
	}

	return xml.Header + string(output), nil
}

// XMLTV format structures
type XMLTV struct {
	XMLName       xml.Name        `xml:"tv"`
	GeneratorName string          `xml:"generator-info-name,attr"`
	GeneratorURL  string          `xml:"generator-info-url,attr"`
	Channels      []XMLTVChannel  `xml:"channel"`
	Programs      []XMLTVProgram  `xml:"programme"`
}

type XMLTVChannel struct {
	ID           string        `xml:"id,attr"`
	DisplayNames []DisplayName `xml:"display-name"`
	Icon         Icon          `xml:"icon"`
}

type DisplayName struct {
	Value string `xml:",chardata"`
}

type Icon struct {
	Src string `xml:"src,attr"`
}

type XMLTVProgram struct {
	Start    string     `xml:"start,attr"`
	Stop     string     `xml:"stop,attr"`
	Channel  string     `xml:"channel,attr"`
	Title    []Title    `xml:"title"`
	Desc     []Desc     `xml:"desc"`
	Category []Category `xml:"category"`
}

type Title struct {
	Value string `xml:",chardata"`
}

type Desc struct {
	Value string `xml:",chardata"`
}

type Category struct {
	Value string `xml:",chardata"`
}

// XMLTVSource reads from XMLTV files/URLs
type XMLTVSource struct {
	urls   []string
	client *http.Client
}

func NewXMLTVSource() *XMLTVSource {
	return &XMLTVSource{
		urls: []string{
			// Serbian Forum EPG - PRIMARY SOURCE for Balkan channels (1,261 channels, 30,426 programs)
			"http://epg.serbianforum.org/losmij/epg.xml.gz",
			// Additional EPG sources
			"https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/epg.xml",
			"https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/Pluto-TV/us.xml",
			"http://epg.streamstv.me/epg/guide-all.xml.gz",
		},
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// NewXMLTVSourceWithCountries creates an XMLTV source with dynamic country EPG
// NewXMLTVSourceWithCountries removed (country-specific XMLTV source no longer used)

// AddCustomURL adds a custom EPG URL to the XMLTV source
func (x *XMLTVSource) AddCustomURL(url string) {
	// Check if URL already exists
	for _, existingURL := range x.urls {
		if existingURL == url {
			return
		}
	}
	x.urls = append(x.urls, url)
}

// AddCustomEPGURLs adds custom EPG URLs from M3U sources to the manager
func (e *Manager) AddCustomEPGURLs(urls []string) {
	for _, source := range e.sources {
		if xmltvSource, ok := source.(*XMLTVSource); ok {
			for _, url := range urls {
				if url != "" {
					xmltvSource.AddCustomURL(url)
					fmt.Printf("EPG: Added custom EPG URL: %s\n", url)
				}
			}
			return
		}
	}
}

func (x *XMLTVSource) Name() string {
	return "XMLTV"
}

// BulkLoadEPG loads all EPG data from all URLs into the programs map
func (x *XMLTVSource) BulkLoadEPG(programs map[string][]livetv.EPGProgram) {
	for _, url := range x.urls {
		fmt.Printf("EPG: Loading from %s\n", url)
		resp, err := x.client.Get(url)
		if err != nil {
			fmt.Printf("EPG: Error fetching %s: %v\n", url, err)
			continue
		}

		var reader io.Reader = resp.Body
		
		// Check if the URL ends with .gz and decompress if needed
		if strings.HasSuffix(url, ".gz") {
			gzReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				fmt.Printf("EPG: Error decompressing %s: %v\n", url, err)
				resp.Body.Close()
				continue
			}
			reader = gzReader
			defer gzReader.Close()
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(reader)
		if err != nil {
			fmt.Printf("EPG: Error reading %s: %v\n", url, err)
			continue
		}

		var tv XMLTV
		if err := xml.Unmarshal(body, &tv); err != nil {
			fmt.Printf("EPG: Error parsing %s: %v\n", url, err)
			continue
		}

		channelCount := 0
		programCount := 0
		for _, prog := range tv.Programs {
			start, _ := time.Parse("20060102150405 -0700", prog.Start)
			end, _ := time.Parse("20060102150405 -0700", prog.Stop)

			title := ""
			if len(prog.Title) > 0 {
				title = prog.Title[0].Value
			}

			desc := ""
			if len(prog.Desc) > 0 {
				desc = prog.Desc[0].Value
			}

			category := ""
			if len(prog.Category) > 0 {
				category = prog.Category[0].Value
			}

			program := livetv.EPGProgram{
				Title:       title,
				Description: desc,
				StartTime:   start,
				EndTime:     end,
				Category:    category,
			}

			if programs[prog.Channel] == nil {
				channelCount++
			}
			programs[prog.Channel] = append(programs[prog.Channel], program)
			programCount++
		}
		fmt.Printf("EPG: Loaded %d programs for %d channels from %s\n", programCount, channelCount, url)
	}
}

func (x *XMLTVSource) GetEPG(channelID string) ([]livetv.EPGProgram, error) {
	// Parse XMLTV files and extract EPG for specific channel
	for _, url := range x.urls {
		resp, err := x.client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var tv XMLTV
		if err := xml.Unmarshal(body, &tv); err != nil {
			continue
		}

		programs := []livetv.EPGProgram{}
		for _, prog := range tv.Programs {
			if prog.Channel != channelID {
				continue
			}

			start, _ := time.Parse("20060102150405 -0700", prog.Start)
			end, _ := time.Parse("20060102150405 -0700", prog.Stop)

			title := ""
			if len(prog.Title) > 0 {
				title = prog.Title[0].Value
			}

			desc := ""
			if len(prog.Desc) > 0 {
				desc = prog.Desc[0].Value
			}

			category := ""
			if len(prog.Category) > 0 {
				category = prog.Category[0].Value
			}

			programs = append(programs, livetv.EPGProgram{
				Title:       title,
				Description: desc,
				StartTime:   start,
				EndTime:     end,
				Category:    category,
			})
		}

		return programs, nil
	}

	return nil, fmt.Errorf("no EPG data found for channel %s", channelID)
}

// PlutoTVEPGSource gets EPG directly from PlutoTV API
type PlutoTVEPGSource struct {
	baseURL string
	client  *http.Client
}

func NewPlutoTVEPGSource() *PlutoTVEPGSource {
	return &PlutoTVEPGSource{
		baseURL: "https://api.pluto.tv",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *PlutoTVEPGSource) Name() string {
	return "PlutoTV EPG"
}

func (p *PlutoTVEPGSource) GetEPG(channelID string) ([]livetv.EPGProgram, error) {
	// PlutoTV channel ID format: plutotv-{slug}
	if len(channelID) < 9 || channelID[:8] != "plutotv-" {
		return nil, fmt.Errorf("not a PlutoTV channel")
	}

	slug := channelID[8:]
	
	// Get EPG timeline
	start := time.Now().Add(-6 * time.Hour)
	stop := time.Now().Add(18 * time.Hour)
	
	url := fmt.Sprintf("%s/v2/channels/%s/timelines?start=%s&stop=%s",
		p.baseURL, slug, start.Format(time.RFC3339), stop.Format(time.RFC3339))

	resp, err := p.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var timeline []struct {
		Title       string    `json:"title"`
		Episode     struct {
			Description string `json:"description"`
		} `json:"episode"`
		Start       time.Time `json:"start"`
		Stop        time.Time `json:"stop"`
		Category    string    `json:"category"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&timeline); err != nil {
		return nil, err
	}

	programs := make([]livetv.EPGProgram, 0, len(timeline))
	for _, item := range timeline {
		programs = append(programs, livetv.EPGProgram{
			Title:       item.Title,
			Description: item.Episode.Description,
			StartTime:   item.Start,
			EndTime:     item.Stop,
			Category:    item.Category,
		})
	}

	return programs, nil
}

// NormalizeChannelID attempts to match various channel ID formats
// Tries multiple strategies to find EPG data for a channel
func (e *Manager) NormalizeChannelID(channelID, channelName string) []string {
	candidates := []string{channelID}
	
	// Add the channel name as-is
	if channelName != "" && channelName != channelID {
		candidates = append(candidates, channelName)
	}
	
	// Remove country codes like (SR), (BA), (HR), @SD, @HD
	normalized := strings.TrimSpace(channelName)
	normalized = strings.NewReplacer(
		" (SR)", "",
		" (BA)", "",
		" (HR)", "",
		" (RS)", "",
		" (BH)", "",
		" (CG)", "",
		" (SN)", "",
		"@SD", "",
		"@HD", "",
		"@FHD", "",
		" HD", "",
		" SD", "",
	).Replace(normalized)
	normalized = strings.TrimSpace(normalized)
	
	if normalized != channelName && normalized != channelID {
		candidates = append(candidates, normalized)
	}
	
	// Try with common variations
	lower := strings.ToLower(normalized)
	candidates = append(candidates, 
		strings.ToUpper(normalized),
		strings.Title(lower),
		lower,
	)
	
	// Remove dots and special chars
	simplified := strings.Map(func(r rune) rune {
		if r == '.' || r == '_' || r == '-' {
			return ' '
		}
		return r
	}, normalized)
	simplified = strings.TrimSpace(simplified)
	if simplified != normalized {
		candidates = append(candidates, simplified)
	}
	
	return candidates
}

// GetEPGWithFallback tries to get EPG using multiple matching strategies
func (e *Manager) GetEPGWithFallback(channelID, channelName string, date time.Time) []livetv.EPGProgram {
	candidates := e.NormalizeChannelID(channelID, channelName)
	
	// Try each candidate
	for _, candidate := range candidates {
		programs := e.GetEPG(candidate, date)
		if len(programs) > 0 {
			return programs
		}
	}
	
	return []livetv.EPGProgram{}
}

// GetCurrentProgramWithFallback tries to get current program using multiple matching strategies
func (e *Manager) GetCurrentProgramWithFallback(channelID, channelName string) *livetv.EPGProgram {
	candidates := e.NormalizeChannelID(channelID, channelName)
	
	// Try each candidate
	for _, candidate := range candidates {
		program := e.GetCurrentProgram(candidate)
		if program != nil {
			return program
		}
	}
	
	return nil
}
