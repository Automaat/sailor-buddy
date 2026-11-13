package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

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

type myCruisesOutput struct {
	Body dto.MyCruises
}

type updateMeInput struct {
	Body dto.UpdateProfileBody
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
		Summary: "Save the current user's profile and finish onboarding", Tags: []string{"Auth"},
	}, h.updateMe)
	huma.Register(api, huma.Operation{
		OperationID: "list-my-cruises", Method: http.MethodGet, Path: "/auth/me/cruises",
		Summary: "Cruises the current user took part in", Tags: []string{"Auth"},
	}, h.myCruises)
}

func (h *AuthHandler) myCruises(ctx context.Context, _ *struct{}) (*myCruisesOutput, error) {
	claims := middleware.GetUser(ctx)
	if claims == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	ids, err := h.q.ListUserCruiseIDs(ctx, claims.UserID)
	if err != nil {
		slog.Error("list user cruises", "user_id", claims.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list your cruises")
	}
	if ids == nil {
		ids = []int64{}
	}
	return &myCruisesOutput{Body: dto.MyCruises{CruiseIDs: ids}}, nil
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
	params, err := profileParams(claims.UserID, in.Body)
	if err != nil {
		return nil, err
	}
	if !params.PatentNumber.Valid {
		current, err := h.q.GetUserByID(ctx, claims.UserID)
		if err != nil {
			slog.Error("get current user", "user_id", claims.UserID, "err", err)
			return nil, huma.Error500InternalServerError("failed to load profile")
		}
		if current.IsCaptain {
			return nil, huma.Error422UnprocessableEntity("captains must keep a patent number")
		}
	}
	user, err := h.q.UpdateUserProfile(ctx, params)
	if err != nil {
		slog.Error("update user profile", "user_id", claims.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update profile")
	}
	return &meOutput{Body: dto.MeFromUser(user)}, nil
}

// profileParams trims the form and enforces the cross-field rules: names
// must not be blank, a patent type needs a number, and a number without a
// type is dropped.
func profileParams(userID int64, b dto.UpdateProfileBody) (sqlcdb.UpdateUserProfileParams, error) {
	first := strings.TrimSpace(b.FirstName)
	last := strings.TrimSpace(b.LastName)
	if first == "" || last == "" {
		return sqlcdb.UpdateUserProfileParams{}, huma.Error422UnprocessableEntity("first and last name are required")
	}
	phone := trimmedPtr(b.Phone)
	patentType := b.PatentType
	number := trimmedPtr(b.PatentNumber)
	if patentType == nil {
		number = nil
	} else if number == nil {
		return sqlcdb.UpdateUserProfileParams{}, huma.Error422UnprocessableEntity("patent number is required for the selected patent")
	}
	return sqlcdb.UpdateUserProfileParams{
		ID:           userID,
		FirstName:    first,
		LastName:     last,
		Phone:        nullString(phone),
		PatentType:   nullString(patentType),
		PatentNumber: nullString(number),
	}, nil
}

func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
