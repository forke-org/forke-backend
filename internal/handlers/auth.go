// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/forke/forke-backend/internal/config"
	"github.com/forke/forke-backend/internal/database"
	"github.com/forke/forke-backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAuthHandler(db *database.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret123"`
}

type RegisterRequest struct {
	Name     string `json:"name" example:"Ayushman"`
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"ayushman"`
	Password string `json:"password" example:"secret123"`
	Role     string `json:"role" example:"developer"` // developer | owner
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

// Login godoc
// @Summary User Login
// @Description Authenticate developer or owner with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "User credentials"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request","message":"invalid json payload"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var userID, name, username, email, passwordHash, role string
	err := h.db.Pool.QueryRow(ctx, `
		SELECT id, name, username, email, password_hash, role
		FROM users
		WHERE email = $1 AND is_banned = false
	`, req.Email).Scan(&userID, &name, &username, &email, &passwordHash, &role)

	if err != nil {
		http.Error(w, `{"error":"unauthorized","message":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"unauthorized","message":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	// Generate JWT
	token, err := h.generateJWT(userID, email, role)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	// Set Cross-domain cookie
	h.setAuthCookie(w, token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       userID,
			"name":     name,
			"username": username,
			"email":    email,
			"role":     role,
		},
	})
}

// Register godoc
// @Summary User Registration
// @Description Register a new developer or owner account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration data"
// @Success 201 {object} map[string]interface{}
// @Failure 409 {object} map[string]string
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request","message":"invalid json payload"}`, http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = "developer"
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to hash password"}`, http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	var userID string
	err = h.db.Pool.QueryRow(ctx, `
		INSERT INTO users (name, username, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5::user_role)
		RETURNING id
	`, req.Name, req.Username, req.Email, string(hashedPassword), req.Role).Scan(&userID)

	if err != nil {
		http.Error(w, `{"error":"conflict","message":"user with this email or username already exists"}`, http.StatusConflict)
		return
	}

	token, err := h.generateJWT(userID, req.Email, req.Role)
	if err != nil {
		http.Error(w, `{"error":"server_error","message":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	h.setAuthCookie(w, token)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       userID,
			"name":     req.Name,
			"username": req.Username,
			"email":    req.Email,
			"role":     req.Role,
		},
	})
}

// Me godoc
// @Summary Get Current User
// @Description Get current authenticated user profile
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Router /auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.UserContextKey).(*middleware.JWTClaims)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	var id, name, username, email, role string
	var level, xp int
	err := h.db.Pool.QueryRow(ctx, `
		SELECT id, name, COALESCE(username, ''), email, role, level, xp
		FROM users
		WHERE id = $1
	`, claims.UserID).Scan(&id, &name, &username, &email, &role, &level, &xp)

	if err != nil {
		http.Error(w, `{"error":"not_found","message":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       id,
		"name":     name,
		"username": username,
		"email":    email,
		"role":     role,
		"level":    level,
		"xp":       xp,
	})
}

// Logout godoc
// @Summary Logout User
// @Description Invalidate session cookie
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]string
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "forke_token",
		Value:    "",
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "logged out successfully"})
}

func (h *AuthHandler) generateJWT(userID, email, role string) (string, error) {
	claims := middleware.JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "forke-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func (h *AuthHandler) setAuthCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     "forke_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(7 * 24 * time.Hour),
		HttpOnly: true,
		Secure:   h.cfg.AppEnv == "production",
		SameSite: http.SameSiteLaxMode,
	}
	if h.cfg.AppEnv == "production" {
		cookie.Domain = h.cfg.CookieDomain
	}
	http.SetCookie(w, cookie)
}
