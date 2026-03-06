package handlers

import (
	"net/http"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"id":    user.UserID,
		"email": user.Email,
	})
}
