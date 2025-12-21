package services

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "sort"
    "strings"
    "time"

    "github.com/Zerr0-C00L/StreamArr/internal/database"
    "github.com/Zerr0-C00L/StreamArr/internal/models"
    isettings "github.com/Zerr0-C00L/StreamArr/internal/settings"
)

// IPTVVODImportSummary reports results of an import run
type IPTVVODImportSummary struct {
    SourcesChecked int `json:"sources_checked"`
    ItemsFound     int `json:"items_found"`
    MoviesImported int `json:"movies_imported"`
    SeriesImported int `json:"series_imported"`
    Skipped        int `json:"skipped"`
    Errors         int `json:"errors"`
}

// ImportIPTVVOD scans configured M3U/Xtream sources for VOD items and adds them to the Library
func ImportIPTVVOD(ctx context.Context, cfg *isettings.Settings, tmdb *TMDBClient, movieStore *database.MovieStore, seriesStore *database.SeriesStore) (*IPTVVODImportSummary, error) {
    client := &http.Client{Timeout: 30 * time.Second}
    summary := &IPTVVODImportSummary{}

    // Dedup map: key = kind:title:year
    seen := make(map[string]bool)
    
    // Count total sources
    totalSources := 0
    for _, xs := range cfg.XtreamSources {
        if xs.Enabled && strings.TrimSpace(xs.ServerURL) != "" {
            totalSources++
        }
    }
    for _, m3u := range cfg.M3USources {
        if m3u.Enabled && strings.TrimSpace(m3u.URL) != "" {
            totalSources++
        }
    }
    
    GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, 0, totalSources, "Starting IPTV VOD import...")
    currentSource := 0

    // Process Xtream sources using API
    for _, xs := range cfg.XtreamSources {
        if !xs.Enabled || strings.TrimSpace(xs.ServerURL) == "" || strings.TrimSpace(xs.Username) == "" || strings.TrimSpace(xs.Password) == "" {
            continue
        }
        currentSource++
        GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, currentSource, totalSources, 
            fmt.Sprintf("Scanning Xtream: %s", xs.Name))
        
        server := strings.TrimSuffix(xs.ServerURL, "/")
        
        // Import VOD movies from Xtream
        items, err := fetchXtreamVODMovies(ctx, client, server, xs.Username, xs.Password, xs.Name, xs.SelectedCategories)
        if err == nil {
            summary.ItemsFound += len(items)
            for i, it := range items {
                if i%50 == 0 {
                    GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, currentSource, totalSources, 
                        fmt.Sprintf("%s: Importing movies (%d/%d)", xs.Name, i, len(items)))
                }
                key := fmt.Sprintf("%s:%s:%d", it.Kind, normalizeTitle(it.Title), it.Year)
                if seen[key] {
                    summary.Skipped++
                    continue
                }
                seen[key] = true
                
                var importErr error
                if cfg.IPTVVODFastImport {
                    importErr = importMovieBasic(ctx, movieStore, it)
                } else {
                    importErr = importMovie(ctx, tmdb, movieStore, it)
                }
                if importErr != nil {
                    fmt.Printf("Error importing movie '%s' (%d): %v\n", it.Title, it.Year, importErr)
                    summary.Errors++
                } else {
                    summary.MoviesImported++
                }
            }
        } else {
            summary.Errors++
        }
        
        // Import series from Xtream
        seriesItems, err := fetchXtreamSeries(ctx, client, server, xs.Username, xs.Password, xs.Name, xs.SelectedCategories)
        if err == nil {
            summary.ItemsFound += len(seriesItems)
            for i, it := range seriesItems {
                if i%50 == 0 {
                    GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, currentSource, totalSources, 
                        fmt.Sprintf("%s: Importing series (%d/%d)", xs.Name, i, len(seriesItems)))
                }
                key := fmt.Sprintf("%s:%s:%d", it.Kind, normalizeTitle(it.Title), it.Year)
                if seen[key] {
                    summary.Skipped++
                    continue
                }
                seen[key] = true
                
                var importErr error
                if cfg.IPTVVODFastImport {
                    importErr = importSeriesBasic(ctx, seriesStore, it)
                } else {
                    importErr = importSeries(ctx, tmdb, seriesStore, it)
                }
                if importErr != nil {
                    fmt.Printf("Error importing series '%s' (%d): %v\n", it.Title, it.Year, importErr)
                    summary.Errors++
                } else {
                    summary.SeriesImported++
                }
            }
        } else {
            summary.Errors++
        }
        
        summary.SourcesChecked++
    }

    // Build list of M3U URLs to scan with selected categories
    type src struct { 
        url, name string
        selectedCategories []string
    }
    var m3uURLs []src
    for _, s := range cfg.M3USources {
        if s.Enabled && strings.TrimSpace(s.URL) != "" {
            m3uURLs = append(m3uURLs, src{url: s.URL, name: s.Name, selectedCategories: s.SelectedCategories})
        }
    }

    for _, s := range m3uURLs {
        currentSource++
        GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, currentSource, totalSources, 
            fmt.Sprintf("Scanning M3U: %s", s.name))
        
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
        if err != nil {
            summary.Errors++
            continue
        }
        req.Header.Set("User-Agent", "Mozilla/5.0")
        resp, err := client.Do(req)
        if err != nil {
            summary.Errors++
            continue
        }
        func() {
            defer resp.Body.Close()
            if resp.StatusCode != http.StatusOK {
                summary.Errors++
                return
            }
            // Parse M3U, collecting VOD entries only
            items := extractVODItems(resp.Body, s.name, s.selectedCategories)
            summary.ItemsFound += len(items)
            // Import each item
            for i, it := range items {
                if i%50 == 0 {
                    GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, currentSource, totalSources, 
                        fmt.Sprintf("%s: Importing (%d/%d)", s.name, i, len(items)))
                }
                key := fmt.Sprintf("%s:%s:%d", it.Kind, normalizeTitle(it.Title), it.Year)
                if seen[key] {
                    summary.Skipped++
                    continue
                }
                seen[key] = true

                switch it.Kind {
                case "movie":
                    var err error
                    if cfg.IPTVVODFastImport {
                        err = importMovieBasic(ctx, movieStore, it)
                    } else {
                        err = importMovie(ctx, tmdb, movieStore, it)
                    }
                    if err != nil {
                        summary.Errors++
                    } else {
                        summary.MoviesImported++
                    }
                case "series":
                    var err error
                    if cfg.IPTVVODFastImport {
                        err = importSeriesBasic(ctx, seriesStore, it)
                    } else {
                        err = importSeries(ctx, tmdb, seriesStore, it)
                    }
                    if err != nil {
                        summary.Errors++
                    } else {
                        summary.SeriesImported++
                    }
                default:
                    summary.Skipped++
                }
            }
        }()
        summary.SourcesChecked++
    }
    
    GlobalScheduler.UpdateProgress(ServiceIPTVVODSync, totalSources, totalSources, 
        fmt.Sprintf("Complete: %d movies, %d series imported", summary.MoviesImported, summary.SeriesImported))

    return summary, nil
}

type vodItem struct {
    Kind  string // "movie" or "series"
    Title string
    Year  int
    URL   string
    SourceName string
}

// extractVODItems scans M3U content for VOD entries and extracts title/year hints
func extractVODItems(r io.Reader, sourceName string, selectedCategories []string) []vodItem {
    items := make([]vodItem, 0)
    scanner := bufio.NewScanner(r)
    var currentTitle string
    var currentGroup string
    var currentGroupOriginal string // Keep original case for comparison
    var currentURL string

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if strings.HasPrefix(line, "#EXTINF:") {
            currentTitle = ""
            currentGroup = ""
            currentGroupOriginal = ""
            // tvg-name
            if idx := strings.Index(line, "tvg-name=\""); idx != -1 {
                end := strings.Index(line[idx+10:], "\"")
                if end != -1 {
                    currentTitle = line[idx+10 : idx+10+end]
                }
            }
            // fallback name after comma
            if currentTitle == "" {
                if c := strings.LastIndex(line, ","); c != -1 {
                    currentTitle = strings.TrimSpace(line[c+1:])
                }
            }
            // group-title (for kind hint)
            if idx := strings.Index(line, "group-title=\""); idx != -1 {
                end := strings.Index(line[idx+13:], "\"")
                if end != -1 {
                    currentGroupOriginal = line[idx+13 : idx+13+end]
                    currentGroup = strings.ToLower(currentGroupOriginal)
                }
            }
        } else if currentTitle != "" && !strings.HasPrefix(line, "#") && line != "" {
            // URL line – decide if VOD
            lowerURL := strings.ToLower(line)
            lowerTitle := strings.ToLower(currentTitle)
            currentURL = line
            
            // Check if it's VOD content (movies or series)
            isVOD := strings.Contains(lowerURL, "/movie/") || strings.Contains(lowerURL, "/series/") || strings.Contains(lowerURL, "/serije/") ||
                strings.HasSuffix(lowerURL, ".mp4") || strings.HasSuffix(lowerURL, ".mkv") || strings.HasSuffix(lowerURL, ".avi")
            isVOD = isVOD || strings.Contains(currentGroup, "vod") || strings.Contains(currentGroup, "movie") || 
                strings.Contains(currentGroup, "series") || strings.Contains(currentGroup, "film")
            
            // Check if category is selected (if categories are specified)
            categoryAllowed := true
            if len(selectedCategories) > 0 && currentGroupOriginal != "" {
                categoryAllowed = false
                for _, selCat := range selectedCategories {
                    if selCat == currentGroupOriginal {
                        categoryAllowed = true
                        break
                    }
                }
            }
            
            if isVOD && categoryAllowed {
                kind := "movie"
                var title string
                var year int
                
                // Detect series by multiple indicators
                isSeries := strings.Contains(lowerURL, "/series/") || strings.Contains(lowerURL, "/serije/") ||
                    strings.Contains(currentGroup, "series") || strings.Contains(currentGroup, "tv shows") ||
                    // Check for season/episode patterns in title: S01 E01, S01E01, Season 1, etc.
                    regexp.MustCompile(`(?i)s\d{1,2}\s*e\d{1,2}`).MatchString(lowerTitle) ||
                    regexp.MustCompile(`(?i)season\s*\d+`).MatchString(lowerTitle) ||
                    regexp.MustCompile(`(?i)episode\s*\d+`).MatchString(lowerTitle) ||
                    regexp.MustCompile(`(?i)epizoda\s*\d+`).MatchString(lowerTitle)
                
                if isSeries {
                    kind = "series"
                    // Extract series name without season/episode info
                    title = extractSeriesName(currentTitle)
                    title, year = splitTitleYear(title)
                } else {
                    title, year = splitTitleYear(currentTitle)
                }
                
                items = append(items, vodItem{Kind: kind, Title: title, Year: year, URL: currentURL, SourceName: sourceName})
            }
            currentTitle = ""
            currentGroup = ""
            currentURL = ""
        }
    }
    return items
}

var yearRe = regexp.MustCompile(`(?i)\b(19|20)\d{2}\b`)

func splitTitleYear(name string) (string, int) {
    // Strip common prefixes first (language codes, channel names, etc.)
    prefixes := []string{"EX - ", "EN - ", "US - ", "UK - ", "EXYU - ", "|EXYU| ", "HD - "}
    for _, prefix := range prefixes {
        if strings.HasPrefix(name, prefix) {
            name = strings.TrimPrefix(name, prefix)
            break
        }
    }
    
    year := 0
    // Prefer a year at end or in parentheses
    // e.g., "Dune (2021)" or "Dune 2021"
    if m := yearRe.FindAllString(name, -1); len(m) > 0 {
        last := m[len(m)-1]
        if len(last) == 4 {
            // parse simple int
            year = atoiSafe(last)
            // remove from title for cleaner TMDB search
            name = strings.ReplaceAll(name, last, "")
        }
    }
    title := strings.TrimSpace(stripQualityTags(name))
    return title, year
}

func atoiSafe(s string) int {
    n := 0
    for _, ch := range s {
        if ch >= '0' && ch <= '9' {
            n = n*10 + int(ch-'0')
        }
    }
    return n
}

// stripQualityTags removes common quality/extra tags from titles
func stripQualityTags(s string) string {
    lower := strings.ToLower(s)
    tags := []string{"1080p", "720p", "2160p", "480p", "hdr", "x264", "x265", "h264", "h265", "webrip", "web-dl", "dvdrip", "bluray", "remux", "aac", "dts", "atmos"}
    for _, t := range tags {
        lower = strings.ReplaceAll(lower, t, "")
    }
    // collapse whitespace back onto original case-preserving form where possible
    fields := strings.Fields(lower)
    return strings.Join(fields, " ")
}

func normalizeTitle(s string) string {
    s = strings.ToLower(strings.TrimSpace(s))
    s = strings.NewReplacer(":", " ", "-", " ", ".", " ", "_", " ", "(", " ", ")", " ").Replace(s)
    fields := strings.Fields(s)
    return strings.Join(fields, " ")
}

// extractSeriesName removes season/episode information from series titles
func extractSeriesName(title string) string {
    // Remove patterns like "S01 E01", "Season 1", "Episode 1", "Epizoda 1", etc.
    re := regexp.MustCompile(`(?i)\s*s\d{1,2}\s*e\d{1,2}.*$`)
    title = re.ReplaceAllString(title, "")
    re = regexp.MustCompile(`(?i)\s*season\s*\d+.*$`)
    title = re.ReplaceAllString(title, "")
    re = regexp.MustCompile(`(?i)\s*episode\s*\d+.*$`)
    title = re.ReplaceAllString(title, "")
    re = regexp.MustCompile(`(?i)\s*epizoda\s*\d+.*$`)
    title = re.ReplaceAllString(title, "")
    return strings.TrimSpace(title)
}

// --- Fast import (no TMDB) helpers ---

// synthID generates a stable negative ID for items lacking TMDB IDs
func synthID(kind, title string, year int) int {
    // 32-bit FNV-1a hash
    h := uint32(2166136261)
    key := fmt.Sprintf("%s|%s|%d", kind, normalizeTitle(title), year)
    for i := 0; i < len(key); i++ {
        h ^= uint32(key[i])
        h *= 16777619
    }
    // ensure negative, avoid 0
    val := int(h & 0x3fffffff) // 30 bits
    if val == 0 { val = 1 }
    return -val
}

func importMovieBasic(ctx context.Context, store *database.MovieStore, it vodItem) error {
    m := &models.Movie{
        TMDBID:        synthID("movie", it.Title, it.Year),
        Title:         it.Title,
        OriginalTitle: it.Title,
        Overview:      "",
        Monitored:     true,
        Available:     true,
        QualityProfile: "1080p",
        Metadata:      models.Metadata{},
    }
    if m.Metadata == nil { m.Metadata = models.Metadata{} }
    m.Metadata["source"] = "iptv_vod"
    m.Metadata["imported_at"] = time.Now().Format(time.RFC3339)
    m.Metadata["iptv_vod_sources"] = []map[string]interface{}{{"name": it.SourceName, "url": it.URL}}
    return store.Add(ctx, m)
}

func importSeriesBasic(ctx context.Context, store *database.SeriesStore, it vodItem) error {
    s := &models.Series{
        TMDBID:        synthID("series", it.Title, it.Year),
        Title:         it.Title,
        OriginalTitle: it.Title,
        Monitored:     true,
        QualityProfile: "1080p",
        Metadata:      models.Metadata{},
    }
    if s.Metadata == nil { s.Metadata = models.Metadata{} }
    s.Metadata["source"] = "iptv_vod"
    s.Metadata["imported_at"] = time.Now().Format(time.RFC3339)
    s.Metadata["iptv_vod_sources"] = []map[string]interface{}{{"name": it.SourceName, "url": it.URL}}
    return store.Add(ctx, s)
}

func importMovie(ctx context.Context, tmdb *TMDBClient, store *database.MovieStore, it vodItem) error {
    // Search TMDB
    movies, err := tmdb.SearchMovies(ctx, it.Title, 1)
    if err != nil || len(movies) == 0 {
        return fmt.Errorf("no TMDB match for movie: %s", it.Title)
    }
    // Choose best candidate by year/title proximity
    sort.SliceStable(movies, func(i, j int) bool {
        // Prefer exact year match, then higher vote average if available via metadata
        yi := yearFromMovie(movies[i])
        yj := yearFromMovie(movies[j])
        if it.Year != 0 && yi != yj {
            di := abs(it.Year - yi)
            dj := abs(it.Year - yj)
            if di != dj { return di < dj }
        }
        // fallback: lexical closeness by normalized title
        return levenshtein(normalizeTitle(movies[i].Title), normalizeTitle(it.Title)) <
            levenshtein(normalizeTitle(movies[j].Title), normalizeTitle(it.Title))
    })

    // Fetch full movie to populate metadata properly
    m, _, err2 := tmdb.GetMovieWithCollection(ctx, movies[0].TMDBID)
    if err2 != nil {
        // fallback to basic
        m = movies[0]
    }
    // Ensure monitored/available defaults
    if m.QualityProfile == "" { m.QualityProfile = "1080p" }
    m.Monitored = true
    m.Available = true
    if m.Metadata == nil { m.Metadata = models.Metadata{} }
    m.Metadata["source"] = "iptv_vod"
    m.Metadata["imported_at"] = time.Now().Format(time.RFC3339)

    // Merge IPTV VOD source into metadata (dedupe by URL)
    if m.Metadata == nil { m.Metadata = models.Metadata{} }
    sources := ensureSourcesArray(m.Metadata["iptv_vod_sources"]) 
    if !hasSourceURL(sources, it.URL) {
        sources = append(sources, map[string]interface{}{"name": it.SourceName, "url": it.URL})
        m.Metadata["iptv_vod_sources"] = sources
    }

    // Try upsert: if exists, update metadata; else add
    if existing, errGet := store.GetByTMDBID(ctx, m.TMDBID); errGet == nil && existing != nil {
        existing.Metadata = mergeMetadata(existing.Metadata, m.Metadata)
        existing.Monitored = true
        existing.Available = true
        if existing.QualityProfile == "" { existing.QualityProfile = m.QualityProfile }
        return store.Update(ctx, existing)
    }
    return store.Add(ctx, m)
}

func importSeries(ctx context.Context, tmdb *TMDBClient, store *database.SeriesStore, it vodItem) error {
    series, err := tmdb.SearchSeries(ctx, it.Title, 1)
    if err != nil || len(series) == 0 {
        return fmt.Errorf("no TMDB match for series: %s", it.Title)
    }
    sort.SliceStable(series, func(i, j int) bool {
        // Prefer closer year if available
        yi := yearFromSeries(series[i])
        yj := yearFromSeries(series[j])
        if it.Year != 0 && yi != yj {
            di := abs(it.Year - yi)
            dj := abs(it.Year - yj)
            if di != dj { return di < dj }
        }
        return levenshtein(normalizeTitle(series[i].Title), normalizeTitle(it.Title)) <
            levenshtein(normalizeTitle(series[j].Title), normalizeTitle(it.Title))
    })

    // Fetch full series to include external IDs
    s, err2 := tmdb.GetSeries(ctx, series[0].TMDBID)
    if err2 != nil {
        s = series[0]
    }
    if s.QualityProfile == "" { s.QualityProfile = "1080p" }
    s.Monitored = true
    if s.Metadata == nil { s.Metadata = models.Metadata{} }
    s.Metadata["source"] = "iptv_vod"
    s.Metadata["imported_at"] = time.Now().Format(time.RFC3339)

    if s.Metadata == nil { s.Metadata = models.Metadata{} }
    sources := ensureSourcesArray(s.Metadata["iptv_vod_sources"]) 
    if !hasSourceURL(sources, it.URL) {
        sources = append(sources, map[string]interface{}{"name": it.SourceName, "url": it.URL})
        s.Metadata["iptv_vod_sources"] = sources
    }
    if existing, errGet := store.GetByTMDBID(ctx, s.TMDBID); errGet == nil && existing != nil {
        existing.Metadata = mergeMetadata(existing.Metadata, s.Metadata)
        existing.Monitored = true
        if existing.QualityProfile == "" { existing.QualityProfile = s.QualityProfile }
        return store.Update(ctx, existing)
    }
    return store.Add(ctx, s)
}

func ensureSourcesArray(v interface{}) []map[string]interface{} {
    out := []map[string]interface{}{}
    if v == nil { return out }
    if arr, ok := v.([]interface{}); ok {
        for _, e := range arr {
            if m, ok2 := e.(map[string]interface{}); ok2 {
                out = append(out, m)
            }
        }
    } else if arr2, ok := v.([]map[string]interface{}); ok {
        return arr2
    }
    return out
}

func hasSourceURL(arr []map[string]interface{}, url string) bool {
    for _, m := range arr {
        if u, ok := m["url"].(string); ok && u == url {
            return true
        }
    }
    return false
}

func mergeMetadata(a, b models.Metadata) models.Metadata {
    if a == nil { a = models.Metadata{} }
    for k, v := range b {
        if k == "iptv_vod_sources" {
            existing := ensureSourcesArray(a[k])
            incoming := ensureSourcesArray(v)
            for _, src := range incoming {
                if u, ok := src["url"].(string); ok && !hasSourceURL(existing, u) {
                    existing = append(existing, src)
                }
            }
            a[k] = existing
        } else {
            a[k] = v
        }
    }
    return a
}

// CleanupIPTVVOD removes sources from metadata that belong to disabled/removed providers.
// If an item has no remaining IPTV sources and was imported via iptv_vod, it is deleted.
func CleanupIPTVVOD(ctx context.Context, cfg *isettings.Settings, movieStore *database.MovieStore, seriesStore *database.SeriesStore) error {
    active := map[string]bool{}
    for _, s := range cfg.M3USources { if s.Enabled { active[s.Name] = true } }
    for _, s := range cfg.XtreamSources { if s.Enabled { active[s.Name] = true } }

    db := movieStore.GetDB()
    // Movies
    rows, err := db.QueryContext(ctx, `SELECT id, metadata FROM library_movies`)
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var id int64
            var metadataJSON []byte
            if err := rows.Scan(&id, &metadataJSON); err != nil { continue }
            meta := models.Metadata{}
            _ = json.Unmarshal(metadataJSON, &meta)
            sources := ensureSourcesArray(meta["iptv_vod_sources"]) 
            filtered := make([]map[string]interface{}, 0, len(sources))
            for _, s := range sources {
                name, _ := s["name"].(string)
                if active[name] { filtered = append(filtered, s) }
            }
            changed := len(filtered) != len(sources)
            if changed {
                if len(filtered) == 0 {
                    delete(meta, "iptv_vod_sources")
                } else {
                    meta["iptv_vod_sources"] = filtered
                }
                m, _ := movieStore.Get(ctx, id)
                if m != nil {
                    m.Metadata = meta
                    // If orphaned and originally imported via iptv_vod, delete
                    if len(filtered) == 0 && meta["source"] == "iptv_vod" {
                        _ = movieStore.Delete(ctx, id)
                    } else {
                        _ = movieStore.Update(ctx, m)
                    }
                }
            }
        }
    }

    // Series
    rows2, err2 := seriesStore.GetDB().QueryContext(ctx, `SELECT id, metadata FROM library_series`)
    if err2 == nil {
        defer rows2.Close()
        for rows2.Next() {
            var id int64
            var metadataJSON []byte
            if err := rows2.Scan(&id, &metadataJSON); err != nil { continue }
            meta := models.Metadata{}
            _ = json.Unmarshal(metadataJSON, &meta)
            sources := ensureSourcesArray(meta["iptv_vod_sources"]) 
            filtered := make([]map[string]interface{}, 0, len(sources))
            for _, s := range sources {
                name, _ := s["name"].(string)
                if active[name] { filtered = append(filtered, s) }
            }
            changed := len(filtered) != len(sources)
            if changed {
                if len(filtered) == 0 {
                    delete(meta, "iptv_vod_sources")
                } else {
                    meta["iptv_vod_sources"] = filtered
                }
                s, _ := seriesStore.Get(ctx, id)
                if s != nil {
                    s.Metadata = meta
                    if len(filtered) == 0 && meta["source"] == "iptv_vod" {
                        _ = seriesStore.Delete(ctx, id)
                    } else {
                        _ = seriesStore.Update(ctx, s)
                    }
                }
            }
        }
    }

    return nil
}

// fetchXtreamVODMovies fetches movies from Xtream API
func fetchXtreamVODMovies(ctx context.Context, client *http.Client, server, username, password, sourceName string, selectedCategories []string) ([]vodItem, error) {
    items := make([]vodItem, 0)
    
    // First, fetch category mappings if we have selected categories
    categoryIDToName := make(map[string]string)
    if len(selectedCategories) > 0 {
        catURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_categories", server, username, password)
        catReq, err := http.NewRequestWithContext(ctx, "GET", catURL, nil)
        if err != nil {
            return nil, err
        }
        
        catResp, err := client.Do(catReq)
        if err != nil {
            return nil, err
        }
        defer catResp.Body.Close()
        
        if catResp.StatusCode == http.StatusOK {
            var categories []struct {
                CategoryID   string `json:"category_id"`
                CategoryName string `json:"category_name"`
            }
            
            if err := json.NewDecoder(catResp.Body).Decode(&categories); err == nil {
                for _, cat := range categories {
                    categoryIDToName[cat.CategoryID] = cat.CategoryName
                }
                fmt.Printf("Xtream VOD: Loaded %d category mappings\n", len(categoryIDToName))
            }
        }
    }
    
    // Build selected category name map for quick lookup
    selectedCategoryMap := make(map[string]bool)
    for _, cat := range selectedCategories {
        selectedCategoryMap[cat] = true
    }
    
    // Fetch VOD streams
    url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_vod_streams", server, username, password)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("xtream API returned status %d", resp.StatusCode)
    }
    
    var streams []struct {
        StreamID      interface{} `json:"stream_id"`
        Name          string      `json:"name"`
        Title         string      `json:"title"`
        ContainerExt  string      `json:"container_extension"`
        CategoryID    string      `json:"category_id"`
        Year          string      `json:"added"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&streams); err != nil {
        return nil, err
    }
    
    fmt.Printf("Xtream VOD: Fetched %d total movies from %s\n", len(streams), sourceName)
    if len(selectedCategories) > 0 {
        fmt.Printf("Xtream VOD: Selected categories: %v\n", selectedCategories)
    }
    
    matchedCount := 0
    for _, stream := range streams {
        // If categories are selected, filter by category name
        if len(selectedCategories) > 0 {
            categoryName := categoryIDToName[stream.CategoryID]
            if !selectedCategoryMap[categoryName] {
                continue
            }
            matchedCount++
        }
        
        title := stream.Name
        if title == "" {
            title = stream.Title
        }
        
        // Extract year from title using splitTitleYear
        cleanTitle, year := splitTitleYear(title)
        
        streamID := fmt.Sprintf("%v", stream.StreamID)
        streamURL := fmt.Sprintf("%s/movie/%s/%s/%s.%s", server, username, password, streamID, stream.ContainerExt)
        
        items = append(items, vodItem{
            Kind:       "movie",
            Title:      cleanTitle,
            Year:       year,
            URL:        streamURL,
            SourceName: sourceName,
        })
    }
    
    if len(selectedCategories) > 0 {
        fmt.Printf("Xtream VOD: Matched %d movies after category filtering\n", matchedCount)
    }
    
    return items, nil
}

// fetchXtreamSeries fetches series from Xtream API
func fetchXtreamSeries(ctx context.Context, client *http.Client, server, username, password, sourceName string, selectedCategories []string) ([]vodItem, error) {
    items := make([]vodItem, 0)
    
    // First, fetch category mappings if we have selected categories
    categoryIDToName := make(map[string]string)
    if len(selectedCategories) > 0 {
        catURL := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series_categories", server, username, password)
        catReq, err := http.NewRequestWithContext(ctx, "GET", catURL, nil)
        if err != nil {
            return nil, err
        }
        
        catResp, err := client.Do(catReq)
        if err != nil {
            return nil, err
        }
        defer catResp.Body.Close()
        
        if catResp.StatusCode == http.StatusOK {
            var categories []struct {
                CategoryID   string `json:"category_id"`
                CategoryName string `json:"category_name"`
            }
            
            if err := json.NewDecoder(catResp.Body).Decode(&categories); err == nil {
                for _, cat := range categories {
                    categoryIDToName[cat.CategoryID] = cat.CategoryName
                }
                fmt.Printf("Xtream Series: Loaded %d category mappings\n", len(categoryIDToName))
            }
        }
    }
    
    // Build selected category name map for quick lookup
    selectedCategoryMap := make(map[string]bool)
    for _, cat := range selectedCategories {
        selectedCategoryMap[cat] = true
    }
    
    // Fetch series
    url := fmt.Sprintf("%s/player_api.php?username=%s&password=%s&action=get_series", server, username, password)
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("xtream API returned status %d", resp.StatusCode)
    }
    
    var series []struct {
        SeriesID     interface{} `json:"series_id"`
        Name         string      `json:"name"`
        Title        string      `json:"title"`
        CategoryID   string      `json:"category_id"`
        ReleaseDate  string      `json:"releaseDate"`
        Year         string      `json:"year"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
        return nil, err
    }
    
    fmt.Printf("Xtream Series: Fetched %d total series from %s\n", len(series), sourceName)
    if len(selectedCategories) > 0 {
        fmt.Printf("Xtream Series: Selected categories: %v\n", selectedCategories)
    }
    
    matchedCount := 0
    for _, s := range series {
        // If categories are selected, filter by category name
        if len(selectedCategories) > 0 {
            categoryName := categoryIDToName[s.CategoryID]
            if !selectedCategoryMap[categoryName] {
                continue
            }
            matchedCount++
        }
        
        title := s.Name
        if title == "" {
            title = s.Title
        }
        
        // Extract year from title using splitTitleYear
        cleanTitle, year := splitTitleYear(title)
        
        seriesID := fmt.Sprintf("%v", s.SeriesID)
        // For series, we store a generic URL - actual episode URLs will be fetched when needed
        seriesURL := fmt.Sprintf("%s/series/%s/%s/%s.m3u8", server, username, password, seriesID)
        
        items = append(items, vodItem{
            Kind:       "series",
            Title:      cleanTitle,
            Year:       year,
            URL:        seriesURL,
            SourceName: sourceName,
        })
    }
    
    if len(selectedCategories) > 0 {
        fmt.Printf("Xtream Series: Matched %d series after category filtering\n", matchedCount)
    }
    
    return items, nil
}

func yearFromMovie(m *models.Movie) int {
    if m.ReleaseDate != nil {
        return m.ReleaseDate.Year()
    }
    // try metadata->release_date string
    if m.Metadata != nil {
        if s, ok := m.Metadata["release_date"].(string); ok && len(s) >= 4 {
            return atoiSafe(s[:4])
        }
    }
    return 0
}

func yearFromSeries(s *models.Series) int {
    if s.FirstAirDate != nil {
        return s.FirstAirDate.Year()
    }
    return 0
}

func abs(n int) int { if n < 0 { return -n }; return n }

// very small Levenshtein (optimized for short strings)
func levenshtein(a, b string) int {
    if a == b { return 0 }
    if len(a) == 0 { return len(b) }
    if len(b) == 0 { return len(a) }
    da := make([]int, len(b)+1)
    for j := 0; j <= len(b); j++ { da[j] = j }
    for i := 1; i <= len(a); i++ {
        prev := i-1
        da[0] = i
        for j := 1; j <= len(b); j++ {
            temp := da[j]
            cost := 0
            if a[i-1] != b[j-1] { cost = 1 }
            // min of deletion, insertion, substitution
            d := da[j] + 1
            if da[j-1] + 1 < d { d = da[j-1] + 1 }
            if prev + cost < d { d = prev + cost }
            da[j] = d
            prev = temp
        }
    }
    return da[len(b)]
}
