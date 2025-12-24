package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// SessionMiddleware validates JWT tokens from requests
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for login/public endpoints
		if path == "/api/v1/auth/login" ||
			path == "/api/v1/auth/setup" ||
			path == "/api/v1/auth/status" ||
			path == "/api/v1/auth/verify" ||
			path == "/api/v1/health" ||
			path == "/api/v1/version" ||
			path == "/api/v1/admin/restart" ||
			strings.HasPrefix(path, "/player_api.php") ||
			strings.HasPrefix(path, "/get.php") ||
			!strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized - No token provided", http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Unauthorized - Invalid token format", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := ValidateToken(tokenString)
		if err != nil {
			if err == ErrExpiredToken {
				http.Error(w, "Unauthorized - Token expired", http.StatusUnauthorized)
			} else {
				http.Error(w, "Unauthorized - Invalid token", http.StatusUnauthorized)
			}
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext retrieves user claims from request context
func GetUserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	return claims, ok
}

// RequireAdmin middleware ensures user is an admin
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetUserFromContext(r.Context())
		if !ok || !claims.IsAdmin {
			http.Error(w, "Forbidden - Admin access required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
