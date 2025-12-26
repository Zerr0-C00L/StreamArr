package debrid

import "context"

// DebridService defines the interface for debrid service providers
// Supports Real-Debrid, AllDebrid, Premiumize, Debrid-Link
type DebridService interface {
	// CheckCache checks which torrent hashes are cached on the debrid service
	// Returns a map of hash -> isCached for instant availability lookup
	CheckCache(ctx context.Context, hashes []string) (map[string]bool, error)

	// GetStreamURL returns the direct streaming URL for a cached torrent hash
	// This URL can be used for instant playback without downloading
	GetStreamURL(ctx context.Context, hash string, fileIndex int) (string, error)

	// GetAvailableFiles returns list of files available in a cached torrent
	// Used to select which file to stream when torrent has multiple files
	GetAvailableFiles(ctx context.Context, hash string) ([]TorrentFile, error)

	// GetServiceName returns the name of the debrid service (e.g., "Real-Debrid")
	GetServiceName() string

	// IsAuthenticated checks if the service has valid authentication
	IsAuthenticated(ctx context.Context) bool
}

// TorrentFile represents a file within a cached torrent
type TorrentFile struct {
	Index    int     // File index for selection
	Path     string  // File path within torrent
	Size     int64   // File size in bytes
	Selected bool    // Whether this file is selected for download/stream
	MimeType string  // MIME type (video/mp4, etc.)
}

// CacheStatus represents the cache status of a torrent
type CacheStatus struct {
	Hash      string
	IsCached  bool
	Files     []TorrentFile
	InstantID string // Service-specific instant availability ID
}
