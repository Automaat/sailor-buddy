package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type AuthHandler struct {
	q         *sqlcdb.Queries
	jwtSecret string
}

func NewAuthHandler(q *sqlcdb.Queries, jwtSecret string) *AuthHandler {
	return &AuthHandler{q: q, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		respondError(w, http.StatusBadRequest, "email, name, and password are required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	user, err := h.q.CreateUser(r.Context(), sqlcdb.CreateUserParams{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
	})
	if err != nil {
		respondError(w, http.StatusConflict, "email already registered")
		return
	}
	access, err := auth.GenerateAccessToken(h.jwtSecret, user.ID, user.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}
	raw, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}
	if err := h.q.CreateRefreshToken(r.Context(), sqlcdb.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store refresh token")
		return
	}
	respondJSON(w, http.StatusCreated, tokenResponse{
		AccessToken:  access,
		RefreshToken: raw,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	access, err := auth.GenerateAccessToken(h.jwtSecret, user.ID, user.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}
	raw, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}
	if err := h.q.CreateRefreshToken(r.Context(), sqlcdb.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store refresh token")
		return
	}
	respondJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		RefreshToken: raw,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		respondError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	tokenHash := auth.HashToken(req.RefreshToken)
	stored, err := h.q.GetRefreshToken(r.Context(), tokenHash)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	if err := h.q.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to revoke old token")
		return
	}
	user, err := h.q.GetUserByID(r.Context(), stored.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}
	access, err := auth.GenerateAccessToken(h.jwtSecret, user.ID, user.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate access token")
		return
	}
	raw, newHash, err := auth.GenerateRefreshToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}
	if err := h.q.CreateRefreshToken(r.Context(), sqlcdb.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store refresh token")
		return
	}
	respondJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		RefreshToken: raw,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		respondError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}
	tokenHash := auth.HashToken(req.RefreshToken)
	if err := h.q.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}
