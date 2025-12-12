package services

import (
	"sync"
	"time"
)

// ServiceStatus represents the current state of a background service
type ServiceStatus struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Running     bool      `json:"running"`
	Interval    string    `json:"interval"`
	LastRun     time.Time `json:"last_run"`
	NextRun     time.Time `json:"next_run"`
	LastError   string    `json:"last_error,omitempty"`
	RunCount    int64     `json:"run_count"`
	// Progress tracking
	Progress        int    `json:"progress"`         // 0-100 percentage
	ProgressMessage string `json:"progress_message"` // Current status message
	ItemsProcessed  int    `json:"items_processed"`
	ItemsTotal      int    `json:"items_total"`
}

// ServiceScheduler manages background services and their status
type ServiceScheduler struct {
	services map[string]*ServiceStatus
	mu       sync.RWMutex
}

// NewServiceScheduler creates a new service scheduler
func NewServiceScheduler() *ServiceScheduler {
	return &ServiceScheduler{
		services: make(map[string]*ServiceStatus),
	}
}

// Register adds a new service to track
func (s *ServiceScheduler) Register(name, description string, interval time.Duration, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.services[name] = &ServiceStatus{
		Name:        name,
		Description: description,
		Enabled:     enabled,
		Running:     false,
		Interval:    formatDuration(interval),
		LastRun:     time.Time{},
		NextRun:     time.Now().Add(interval),
		RunCount:    0,
	}
}

// MarkRunning marks a service as currently running
func (s *ServiceScheduler) MarkRunning(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if svc, exists := s.services[name]; exists {
		svc.Running = true
	}
}

// MarkComplete marks a service run as complete
func (s *ServiceScheduler) MarkComplete(name string, err error, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if svc, exists := s.services[name]; exists {
		svc.Running = false
		svc.LastRun = time.Now()
		svc.NextRun = time.Now().Add(interval)
		svc.RunCount++
		svc.Progress = 0
		svc.ProgressMessage = ""
		svc.ItemsProcessed = 0
		svc.ItemsTotal = 0
		if err != nil {
			svc.LastError = err.Error()
		} else {
			svc.LastError = ""
		}
	}
}

// UpdateProgress updates the progress of a running service
func (s *ServiceScheduler) UpdateProgress(name string, processed, total int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if svc, exists := s.services[name]; exists {
		svc.ItemsProcessed = processed
		svc.ItemsTotal = total
		svc.ProgressMessage = message
		if total > 0 {
			svc.Progress = (processed * 100) / total
		}
	}
}

// GetStatus returns the status of a specific service
func (s *ServiceScheduler) GetStatus(name string) *ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if svc, exists := s.services[name]; exists {
		return svc
	}
	return nil
}

// GetAllStatus returns the status of all services
func (s *ServiceScheduler) GetAllStatus() []*ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]*ServiceStatus, 0, len(s.services))
	for _, svc := range s.services {
		statuses = append(statuses, svc)
	}
	return statuses
}

// SetEnabled enables or disables a service
func (s *ServiceScheduler) SetEnabled(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if svc, exists := s.services[name]; exists {
		svc.Enabled = enabled
	}
}

// formatDuration converts a duration to a human-readable string
func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return string(rune(days+'0')) + " days"
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return string(rune(hours/10+'0')) + string(rune(hours%10+'0')) + " hours"
	}
	if d >= time.Minute {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return string(rune(mins/10+'0')) + string(rune(mins%10+'0')) + " minutes"
	}
	return d.String()
}

// Global scheduler instance
var GlobalScheduler = NewServiceScheduler()

// Service name constants
const (
	ServicePlaylist       = "playlist_generation"
	ServiceCacheCleanup   = "cache_cleanup"
	ServiceEPGUpdate      = "epg_update"
	ServiceChannelRefresh = "channel_refresh"
	ServiceMDBListSync    = "mdblist_sync"
	ServiceStreamSearch   = "stream_search"
	ServiceCollectionSync = "collection_sync"
	ServiceEpisodeScan    = "episode_scan"
)

// InitializeDefaultServices sets up the default service definitions
func InitializeDefaultServices() {
	GlobalScheduler.Register(ServicePlaylist, "Regenerates M3U8 playlist with all library content", 12*time.Hour, true)
	GlobalScheduler.Register(ServiceCacheCleanup, "Removes expired cache entries and old data", 1*time.Hour, true)
	GlobalScheduler.Register(ServiceEPGUpdate, "Updates Electronic Program Guide data for Live TV", 6*time.Hour, true)
	GlobalScheduler.Register(ServiceChannelRefresh, "Refreshes Live TV channel list from M3U sources", 1*time.Hour, true)
	GlobalScheduler.Register(ServiceMDBListSync, "Syncs library with configured MDBList watchlists", 6*time.Hour, true)
	GlobalScheduler.Register(ServiceStreamSearch, "Searches for streams for monitored content", 30*time.Minute, true)
	GlobalScheduler.Register(ServiceCollectionSync, "Syncs incomplete movie collections", 24*time.Hour, true)
	GlobalScheduler.Register(ServiceEpisodeScan, "Fetches episode metadata from TMDB for all series", 24*time.Hour, true)
}
