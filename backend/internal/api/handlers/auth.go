package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type AuthHandler struct {
	q sqlcdb.Querier
}

func NewAuthHandler(q sqlcdb.Querier) *AuthHandler {
	return &AuthHandler{q: q}
}

type meOutput struct {
	Body dto.Me
}

type updateMeInput struct {
	Body dto.UpdatePatentBody
}

// RegisterAuthRoutes wires the current-user endpoints onto the API.
func RegisterAuthRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewAuthHandler(q)
	huma.Register(api, huma.Operation{
		OperationID: "get-me", Method: http.MethodGet, Path: "/auth/me",
		Summary: "Current authenticated user", Tags: []string{"Auth"},
	}, h.me)
	huma.Register(api, huma.Operation{
		OperationID: "update-me", Method: http.MethodPut, Path: "/auth/me",
		Summary: "Update the current user's sailing patent", Tags: []string{"Auth"},
	}, h.updateMe)
}

func (h *AuthHandler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	claims := middleware.GetUser(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	user, err := h.q.GetUserByID(ctx, claims.UserID)
	if err != nil {
		slog.Error("get current user", "user_id", claims.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to load profile")
	}
	return &meOutput{Body: dto.MeFromUser(user)}, nil
}

func (h *AuthHandler) updateMe(ctx context.Context, in *updateMeInput) (*meOutput, error) {
	claims := middleware.GetUser(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	if err := h.q.UpdateUserPatent(ctx, sqlcdb.UpdateUserPatentParams{
		PatentType:   nullString(in.Body.PatentType),
		PatentNumber: nullString(in.Body.PatentNumber),
		ID:           claims.UserID,
	}); err != nil {
		slog.Error("update user patent", "user_id", claims.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update patent")
	}
	user, err := h.q.GetUserByID(ctx, claims.UserID)
	if err != nil {
		slog.Error("get current user", "user_id", claims.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to load profile")
	}
	return &meOutput{Body: dto.MeFromUser(user)}, nil
}
