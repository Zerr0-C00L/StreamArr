package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
)

// AuthMiddleware checks for API key or basic auth
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check and public endpoints
		if r.URL.Path == "/api/v1/health" ||
			strings.HasPrefix(r.URL.Path, "/player_api.php") ||
			strings.HasPrefix(r.URL.Path, "/get.php") {
			next.ServeHTTP(w, r)
			return
		}

		// Get configured credentials
		apiKey := os.Getenv("STREAMARR_API_KEY")
		username := os.Getenv("STREAMARR_USERNAME")
		password := os.Getenv("STREAMARR_PASSWORD")

		// If no auth is configured, allow through (backward compatibility) but log warning
		if apiKey == "" && username == "" {
			log.Println("⚠️  WARNING: No authentication configured! Set STREAMARR_API_KEY or STREAMARR_USERNAME/PASSWORD")
			next.ServeHTTP(w, r)
			return
		}

		// Auth is configured - now check if valid credentials were provided
		authProvided := false

		// Check API key in header
		if apiKey != "" {
			providedKey := r.Header.Get("X-API-Key")
			if providedKey != "" {
				if subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
				authProvided = true // Wrong key provided
			}
		}

		// Check for basic auth
		if username != "" && password != "" {
			user, pass, ok := r.BasicAuth()
			if ok {
				if subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1 &&
					subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
				authProvided = true // Wrong credentials provided
			}
		}

		// Auth is required but not provided or incorrect
		w.Header().Set("WWW-Authenticate", `Basic realm="StreamArr Pro"`)
		if authProvided {
			log.Printf("🚫 Authentication failed for %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// IPWhitelistMiddleware restricts access to whitelisted IPs
func IPWhitelistMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		whitelist := os.Getenv("STREAMARR_IP_WHITELIST")
		if whitelist == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Get client IP (handle X-Forwarded-For from reverse proxy)
		clientIP := r.Header.Get("X-Forwarded-For")
		if clientIP == "" {
			clientIP = r.Header.Get("X-Real-IP")
		}
		if clientIP == "" {
			clientIP = strings.Split(r.RemoteAddr, ":")[0]
		}

		// Check if IP is in whitelist
		allowedIPs := strings.Split(whitelist, ",")
		for _, allowedIP := range allowedIPs {
			if strings.TrimSpace(allowedIP) == strings.TrimSpace(clientIP) {
				next.ServeHTTP(w, r)
				return
			}
		}

		log.Printf("🚫 Blocked request from unauthorized IP: %s", clientIP)
		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}
