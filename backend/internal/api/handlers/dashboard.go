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

type DashboardHandler struct {
	q sqlcdb.Querier
}

func NewDashboardHandler(q sqlcdb.Querier) *DashboardHandler {
	return &DashboardHandler{q: q}
}

type dashboardOutput struct {
	Body dto.Dashboard
}

// RegisterDashboardRoutes wires the caller's sea-time summary onto the API.
func RegisterDashboardRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewDashboardHandler(q)
	huma.Register(api, huma.Operation{
		OperationID: "get-dashboard", Method: http.MethodGet, Path: "/dashboard",
		Summary: "Caller's sea time, plus club totals for admins",
		Description: "Personal totals count voyages where the caller is the assigned captain " +
			"or on the crew list through a crew record linked to their account.",
		Tags: []string{"Dashboard"},
	}, h.get)
}

func (h *DashboardHandler) get(ctx context.Context, _ *struct{}) (*dashboardOutput, error) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	rows, err := h.q.ListUserSeaTimeVoyages(ctx, u.UserID)
	if err != nil {
		slog.Error("dashboard personal sea time", "err", err)
		return nil, huma.Error500InternalServerError("failed to get personal sea time")
	}
	out := &dashboardOutput{Body: dto.Dashboard{Personal: dto.PersonalSeaTimeFromDB(rows)}}
	if u.Role != "admin" {
		return out, nil
	}
	stats, err := h.q.GetDashboardStats(ctx)
	if err != nil {
		slog.Error("dashboard stats", "err", err)
		return nil, huma.Error500InternalServerError("failed to get dashboard stats")
	}
	byYear, err := h.q.GetVoyagesByYear(ctx)
	if err != nil {
		slog.Error("dashboard yearly breakdown", "err", err)
		return nil, huma.Error500InternalServerError("failed to get yearly breakdown")
	}
	out.Body.Club = dto.ClubSeaTimeFromDB(stats, byYear)
	return out, nil
}
