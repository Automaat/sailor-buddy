package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

type meOutput struct {
	Body dto.Me
}

// RegisterAuthRoutes wires the current-user endpoint onto the API.
func RegisterAuthRoutes(api huma.API) {
	h := NewAuthHandler()
	huma.Register(api, huma.Operation{
		OperationID: "get-me", Method: http.MethodGet, Path: "/auth/me",
		Summary: "Current authenticated user", Tags: []string{"Auth"},
	}, h.me)
}

func (h *AuthHandler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	user := middleware.GetUser(ctx)
	if user == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	return &meOutput{Body: dto.Me{
		ID:        user.UserID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarURL: user.AvatarUrl,
	}}, nil
}
