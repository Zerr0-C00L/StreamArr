package providers

import (
	"fmt"
	"strings"

	"github.com/Zerr0-C00L/StreamArr/internal/scrapers"
	"github.com/Zerr0-C00L/StreamArr/internal/services"
)

// AutoEmbedAdapter adapts AutoEmbed scraper to StreamProvider interface
type AutoEmbedAdapter struct {
	client     *scrapers.AutoEmbedClient
	tmdbClient *services.TMDBClient
}

// NewAutoEmbedAdapter creates a new AutoEmbed adapter
func NewAutoEmbedAdapter(tmdbClient *services.TMDBClient) *AutoEmbedAdapter {
	return &AutoEmbedAdapter{
		client:     scrapers.NewAutoEmbedClient(),
		tmdbClient: tmdbClient,
	}
}

// GetMovieStreams implements StreamProvider interface for movies
func (a *AutoEmbedAdapter) GetMovieStreams(imdbID string) ([]TorrentioStream, error) {
	// Convert IMDB to TMDB
	tmdbID, err := a.tmdbClient.IMDBToTMDB(imdbID, "movie")
	if err != nil {
		return nil, fmt.Errorf("failed to convert IMDB to TMDB: %w", err)
	}

	// AutoEmbed returns single stream per region, try multiple regions
	regions := []string{"US", "EU", "UK"}
	streams := make([]TorrentioStream, 0)
	
	for _, region := range regions {
		source, err := a.client.GetMovieStream(fmt.Sprintf("%d", tmdbID), region)
		if err != nil {
			continue // Try next region
		}
		
		if source != nil {
			quality := inferQuality(source.Server, source.Region)
			streams = append(streams, TorrentioStream{
				Name:    fmt.Sprintf("AutoEmbed - %s (%s)", source.Server, source.Region),
				Title:   fmt.Sprintf("AutoEmbed %s Stream", source.Server),
				URL:     source.Link,
				Quality: quality,
				Cached:  true,
				Source:  "autoembed",
			})
		}
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found for IMDB %s", imdbID)
	}

	return streams, nil
}

// GetSeriesStreams implements StreamProvider interface for series
func (a *AutoEmbedAdapter) GetSeriesStreams(imdbID string, season, episode int) ([]TorrentioStream, error) {
	// Convert IMDB to TMDB
	tmdbID, err := a.tmdbClient.IMDBToTMDB(imdbID, "tv")
	if err != nil {
		return nil, fmt.Errorf("failed to convert IMDB to TMDB: %w", err)
	}

	// AutoEmbed returns single stream per region, try multiple regions
	regions := []string{"US", "EU", "UK"}
	streams := make([]TorrentioStream, 0)
	
	for _, region := range regions {
		source, err := a.client.GetSeriesStream(fmt.Sprintf("%d", tmdbID), season, episode, region)
		if err != nil {
			continue // Try next region
		}
		
		if source != nil {
			quality := inferQuality(source.Server, source.Region)
			streams = append(streams, TorrentioStream{
				Name:    fmt.Sprintf("AutoEmbed - %s S%02dE%02d (%s)", source.Server, season, episode, source.Region),
				Title:   fmt.Sprintf("AutoEmbed %s Stream", source.Server),
				URL:     source.Link,
				Quality: quality,
				Cached:  true,
				Source:  "autoembed",
			})
		}
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found for IMDB %s S%02dE%02d", imdbID, season, episode)
	}

	return streams, nil
}

// inferQuality infers quality from server name and region
func inferQuality(serverName, region string) string {
	lower := strings.ToLower(serverName)
	
	// Some servers indicate quality in name
	if strings.Contains(lower, "4k") || strings.Contains(lower, "2160") {
		return "2160p"
	} else if strings.Contains(lower, "hd") || strings.Contains(lower, "1080") {
		return "1080p"
	} else if strings.Contains(lower, "720") {
		return "720p"
	}
	
	// Premium regions typically have better quality
	if region == "EU" || region == "US" {
		return "1080p"
	}
	
	// Default
	return "720p"
}
