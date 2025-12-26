package providers

import (
	"fmt"
	"strings"

	"github.com/Zerr0-C00L/StreamArr/internal/scrapers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
)

// VidSrcAdapter adapts VidSrc scraper to StreamProvider interface
type VidSrcAdapter struct {
	client     *scrapers.VidSrcClient
	tmdbClient *services.TMDBClient
}

// NewVidSrcAdapter creates a new VidSrc adapter
func NewVidSrcAdapter(tmdbClient *services.TMDBClient) *VidSrcAdapter {
	return &VidSrcAdapter{
		client:     scrapers.NewVidSrcClient(),
		tmdbClient: tmdbClient,
	}
}

// GetMovieStreams implements StreamProvider interface for movies
func (v *VidSrcAdapter) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	// Convert IMDB ID to TMDB ID
	tmdbID, err := v.tmdbClient.IMDBToTMDB(imdbID, "movie")
	if err != nil {
		return nil, fmt.Errorf("failed to convert IMDB to TMDB: %w", err)
	}

	// Get streams from VidSrc
	sources, err := v.client.GetMovieStreams(fmt.Sprintf("%d", tmdbID))
	if err != nil {
		return nil, err
	}

	// Convert to TorrentioStream format
	streams := make([]TorrentioStream, 0, len(sources))
	for _, source := range sources {
		quality := extractQuality(source.Server)
		streams = append(streams, TorrentioStream{
			Name:    fmt.Sprintf("VidSrc - %s", source.Server),
			Title:   fmt.Sprintf("VidSrc %s Stream", source.Type),
			URL:     source.Link,
			Quality: quality,
			Cached:  true, // Direct streaming is always "cached"
			Source:  "vidsrc",
		})
	}

	return streams, nil
}

// GetSeriesStreams implements StreamProvider interface for series
func (v *VidSrcAdapter) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	// Convert IMDB ID to TMDB ID
	tmdbID, err := v.tmdbClient.IMDBToTMDB(imdbID, "tv")
	if err != nil {
		return nil, fmt.Errorf("failed to convert IMDB to TMDB: %w", err)
	}

	// Get streams from VidSrc
	sources, err := v.client.GetSeriesStreams(fmt.Sprintf("%d", tmdbID), season, episode)
	if err != nil {
		return nil, err
	}

	// Convert to TorrentioStream format
	streams := make([]TorrentioStream, 0, len(sources))
	for _, source := range sources {
		quality := extractQuality(source.Server)
		streams = append(streams, TorrentioStream{
			Name:    fmt.Sprintf("VidSrc - %s S%02dE%02d", source.Server, season, episode),
			Title:   fmt.Sprintf("VidSrc %s Stream", source.Type),
			URL:     source.Link,
			Quality: quality,
			Cached:  true,
			Source:  "vidsrc",
		})
	}

	return streams, nil
}

// extractQuality attempts to extract quality from server name
func extractQuality(serverName string) string {
	lower := strings.ToLower(serverName)
	
	if strings.Contains(lower, "4k") || strings.Contains(lower, "2160") {
		return "2160p"
	} else if strings.Contains(lower, "1080") {
		return "1080p"
	} else if strings.Contains(lower, "720") {
		return "720p"
	} else if strings.Contains(lower, "480") {
		return "480p"
	}
	
	// Default to 1080p for direct streaming sources
	return "1080p"
}
