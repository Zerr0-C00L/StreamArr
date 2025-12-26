package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest represents login credentials
type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse contains the JWT token
type LoginResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Login handles user authentication
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password required")
		return
	}

	// Query user from database using the users table schema
	var userID int
	var hashedPassword string
	var role string

	err := h.userStore.DB().QueryRow(`
		SELECT user_id, password_hash, role 
		FROM users 
		WHERE username = $1
	`, req.Username).Scan(&userID, &hashedPassword, &role)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		log.Printf("Error querying user: %v", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// Check if user is admin
	isAdmin := (role == "admin")

	// Generate JWT token
	token, err := auth.GenerateToken(userID, req.Username, isAdmin, req.RememberMe)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Calculate expiration
	expiration := time.Now().Add(24 * time.Hour)
	if req.RememberMe {
		expiration = time.Now().Add(30 * 24 * time.Hour)
	}

	respondJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		Username:  req.Username,
		IsAdmin:   isAdmin,
		ExpiresAt: expiration,
	})
}

// VerifyToken validates the current token
func (h *Handler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		respondError(w, http.StatusUnauthorized, "no token provided")
		return
	}

	// Remove "Bearer " prefix
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	claims, err := auth.ValidateToken(tokenString)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":    true,
		"user_id":  claims.UserID,
		"username": claims.Username,
		"is_admin": claims.IsAdmin,
	})
}

// Logout invalidates the current session (client-side only for JWT)
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// With JWT, logout is client-side (delete token)
	// We just acknowledge the request
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

// AuthStatus checks if setup is required (no users exist)
func (h *Handler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	var count int
	err := h.userStore.DB().QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		// If table doesn't exist, setup is required
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"setup_required": true,
			"user_count":     0,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"setup_required": count == 0,
		"user_count":     count,
	})
}

// CreateFirstUser creates the initial admin user if no users exist
func (h *Handler) CreateFirstUser(w http.ResponseWriter, r *http.Request) {
	// Check if any users exist
	var count int
	err := h.userStore.DB().QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check users")
		return
	}

	if count > 0 {
		respondError(w, http.StatusBadRequest, "users already exist")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password required")
		return
	}

	// Default email if not provided
	if req.Email == "" {
		req.Email = req.Username + "@streamarr.local"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Create user with admin role
	_, err = h.userStore.DB().Exec(`
		INSERT INTO users (username, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, 'admin', $4)
	`, req.Username, req.Email, string(hashedPassword), time.Now())

	if err != nil {
		log.Printf("Error creating user: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "admin user created successfully",
	})
}

// UpdateProfile handles user profile updates (username, email, profile_picture)
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	log.Printf("UpdateProfile: request received")

	// Get user from context (set by auth middleware)
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		log.Printf("UpdateProfile: unauthorized - no claims in context")
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	log.Printf("UpdateProfile: user %d", claims.UserID)

	var req struct {
		Username       *string `json:"username"`
		Email          *string `json:"email"`
		ProfilePicture *string `json:"profile_picture"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("UpdateProfile: decode error: %v", err)
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	log.Printf("UpdateProfile: username=%v, email=%v, has_picture=%v",
		req.Username != nil, req.Email != nil, req.ProfilePicture != nil)

	updates := make(map[string]interface{})
	if req.Username != nil && *req.Username != "" {
		updates["username"] = *req.Username
	}
	if req.Email != nil && *req.Email != "" {
		updates["email"] = *req.Email
	}
	// Allow setting profile_picture to empty string (to remove it)
	if req.ProfilePicture != nil {
		updates["profile_picture"] = *req.ProfilePicture
		log.Printf("UpdateProfile: picture length=%d", len(*req.ProfilePicture))
	}

	if len(updates) == 0 {
		log.Printf("UpdateProfile: no fields to update")
		respondError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	userID := claims.UserID
	if err := h.userStore.UpdateUser(userID, updates); err != nil {
		log.Printf("UpdateProfile: update error: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	// Get updated user data
	user, err := h.userStore.GetUserByID(userID)
	if err != nil {
		log.Printf("UpdateProfile: get user error: %v", err)
		respondError(w, http.StatusInternalServerError, "failed to fetch updated profile")
		return
	}

	log.Printf("UpdateProfile: success for user %s", user.Username)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"message":         "profile updated successfully",
		"username":        user.Username,
		"profile_picture": user.ProfilePicture,
	})
}

// ChangePassword handles password changes for authenticated users
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		respondError(w, http.StatusBadRequest, "current and new password required")
		return
	}

	// Get user by ID to verify current password
	user, err := h.userStore.GetUserByID(claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "user not found")
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		respondError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Update password in database
	updates := map[string]interface{}{
		"password": string(hashedPassword),
	}

	if err := h.userStore.UpdateUser(claims.UserID, updates); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "password changed successfully",
	})
}

// GetCurrentUser returns the current user's profile information
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	claims, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.userStore.GetUserByID(claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":              user.ID,
		"username":        user.Username,
		"email":           user.Email,
		"role":            user.Role,
		"profile_picture": user.ProfilePicture,
	})
}
