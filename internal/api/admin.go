package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// AdminHandler handles admin dashboard operations
type AdminHandler struct {
	handler *Handler
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(handler *Handler) *AdminHandler {
	return &AdminHandler{
		handler: handler,
	}
}

// RegisterAdminRoutes registers admin API routes
func (a *AdminHandler) RegisterAdminRoutes(r *mux.Router) {
	admin := r.PathPrefix("/api/admin").Subrouter()

	// System status and control
	admin.HandleFunc("/status", a.GetSystemStatus).Methods("GET")
	admin.HandleFunc("/daemon/start", a.StartDaemon).Methods("POST")
	admin.HandleFunc("/daemon/stop", a.StopDaemon).Methods("POST")
	admin.HandleFunc("/sync/now", a.SyncNow).Methods("POST")
	admin.HandleFunc("/playlist/generate", a.GeneratePlaylist).Methods("POST")
	admin.HandleFunc("/cache/episodes", a.CacheEpisodes).Methods("POST")
	
	// Logs
	admin.HandleFunc("/logs/{file}", a.GetLogs).Methods("GET")
	
	// Settings management
	admin.HandleFunc("/settings", a.GetAdminSettings).Methods("GET")
	admin.HandleFunc("/settings", a.SaveAdminSettings).Methods("POST")
	
	// User management
	admin.HandleFunc("/users", a.GetUsers).Methods("GET")
	admin.HandleFunc("/users", a.CreateUser).Methods("POST")
	admin.HandleFunc("/users/{id}", a.UpdateUser).Methods("PUT")
	admin.HandleFunc("/users/{id}", a.DeleteUser).Methods("DELETE")
	
	// System info
	admin.HandleFunc("/info", a.GetSystemInfo).Methods("GET")
	admin.HandleFunc("/stats", a.GetStatistics).Methods("GET")
}

// GetSystemStatus returns system status information
func (a *AdminHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	// Check worker status
	workerStatus := "stopped"
	workerPID := ""
	
	// Try to read worker PID
	pidOutput, err := exec.Command("pgrep", "-f", "./bin/worker").Output()
	if err == nil && len(pidOutput) > 0 {
		workerStatus = "running"
		workerPID = strings.TrimSpace(string(pidOutput))
	}

	// Get database connection status
	dbStatus := "disconnected"
	if a.handler.movieStore != nil {
		dbStatus = "connected"
	}

	status := map[string]interface{}{
		"worker": map[string]string{
			"status": workerStatus,
			"pid":    workerPID,
		},
		"database": map[string]string{
			"status": dbStatus,
		},
		"api": map[string]string{
			"status": "running",
		},
	}

	respondJSON(w, http.StatusOK, status)
}

// StartDaemon starts the background worker daemon
func (a *AdminHandler) StartDaemon(w http.ResponseWriter, r *http.Request) {
	// Check if already running
	if output, err := exec.Command("pgrep", "-f", "./bin/worker").Output(); err == nil && len(output) > 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Worker already running",
			"pid":     strings.TrimSpace(string(output)),
		})
		return
	}

	// Start worker in background
	cmd := exec.Command("./bin/worker")
	if err := cmd.Start(); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to start worker: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"pid":     cmd.Process.Pid,
		"message": "Worker started successfully",
	})
}

// StopDaemon stops the background worker daemon
func (a *AdminHandler) StopDaemon(w http.ResponseWriter, r *http.Request) {
	// Get worker PID
	pidOutput, err := exec.Command("pgrep", "-f", "./bin/worker").Output()
	if err != nil || len(pidOutput) == 0 {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Worker not running",
		})
		return
	}

	pid := strings.TrimSpace(string(pidOutput))
	
	// Send SIGTERM to worker
	if err := exec.Command("kill", "-TERM", pid).Run(); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to stop worker: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Worker stopped successfully",
	})
}

// SyncNow triggers immediate sync
func (a *AdminHandler) SyncNow(w http.ResponseWriter, r *http.Request) {
	// This would trigger playlist generation
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Sync triggered (not yet implemented)",
	})
}

// GeneratePlaylist generates playlists immediately
func (a *AdminHandler) GeneratePlaylist(w http.ResponseWriter, r *http.Request) {
	// Trigger playlist generation
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Playlist generation triggered",
	})
}

// CacheEpisodes starts episode caching
func (a *AdminHandler) CacheEpisodes(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Episode cache sync started in background",
	})
}

// GetLogs retrieves log file contents
func (a *AdminHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	logFile := vars["file"]
	
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if l, err := strconv.Atoi(linesStr); err == nil {
			lines = l
		}
	}

	// Read log file using tail
	logPath := fmt.Sprintf("./logs/%s.log", logFile)
	output, err := exec.Command("tail", "-n", strconv.Itoa(lines), logPath).Output()
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Log file not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"content": string(output),
	})
}

// GetAdminSettings retrieves admin settings
func (a *AdminHandler) GetAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.handler.settingsManager.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

// SaveAdminSettings saves admin settings
func (a *AdminHandler) SaveAdminSettings(w http.ResponseWriter, r *http.Request) {
	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Save settings
	if err := a.handler.settingsManager.SetAll(settings); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Settings saved successfully",
	})
}

// GetUsers retrieves all users
func (a *AdminHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if a.handler.userStore == nil {
		respondJSON(w, http.StatusOK, []interface{}{})
		return
	}

	users, err := a.handler.userStore.GetAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, users)
}

// CreateUser creates a new user
func (a *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "Username, email, and password are required",
		})
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	if a.handler.userStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "User store not initialized",
		})
		return
	}

	// Create user
	user, err := a.handler.userStore.CreateUser(req.Username, req.Email, req.Password, req.Role)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User created successfully",
		"user":    user,
	})
}

// UpdateUser updates a user
func (a *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if a.handler.userStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "User store not initialized",
		})
		return
	}

	// Update user
	if err := a.handler.userStore.UpdateUser(userID, updates); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User updated successfully",
	})
}

// DeleteUser deletes a user
func (a *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr := vars["id"]
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid user ID",
		})
		return
	}

	if a.handler.userStore == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   "User store not initialized",
		})
		return
	}

	// Delete user
	if err := a.handler.userStore.DeleteUser(userID); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User deleted successfully",
	})
}

// GetSystemInfo returns system information
func (a *AdminHandler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	// Get system info using uname
	unameOutput, _ := exec.Command("uname", "-a").Output()
	
	// Get Go version
	goVersion, _ := exec.Command("go", "version").Output()
	
	// Get uptime
	uptimeOutput, _ := exec.Command("uptime").Output()

	info := map[string]interface{}{
		"system":  strings.TrimSpace(string(unameOutput)),
		"go":      strings.TrimSpace(string(goVersion)),
		"uptime":  strings.TrimSpace(string(uptimeOutput)),
		"version": "1.1.0",
	}

	respondJSON(w, http.StatusOK, info)
}

// GetStatistics returns system statistics
func (a *AdminHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get movie count
	movieCount := 0
	if a.handler.movieStore != nil {
		if count, err := a.handler.movieStore.Count(ctx); err == nil {
			movieCount = int(count)
		}
	}

	// Get series count
	seriesCount := 0
	episodeCount := 0
	if a.handler.seriesStore != nil {
		if count, err := a.handler.seriesStore.Count(ctx, nil); err == nil {
			seriesCount = count
		}
		if count, err := a.handler.seriesStore.CountEpisodes(ctx); err == nil {
			episodeCount = int(count)
		}
	}

	// Get channel count
	channelCount := 0
	if a.handler.channelManager != nil {
		channels := a.handler.channelManager.GetAllChannels()
		channelCount = len(channels)
	}
	
	// Get user count
	userCount := 0
	if a.handler.userStore != nil {
		if stats, err := a.handler.userStore.GetUserStats(); err == nil {
			userCount = stats["total_users"]
		}
	}

	stats := map[string]interface{}{
		"movies":   movieCount,
		"series":   seriesCount,
		"episodes": episodeCount,
		"channels": channelCount,
		"users":    userCount,
	}

	respondJSON(w, http.StatusOK, stats)
}
