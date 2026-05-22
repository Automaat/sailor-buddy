package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
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

// RegisterDashboardRoutes wires the club sailing summary onto the API.
func RegisterDashboardRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewDashboardHandler(q)
	huma.Register(api, huma.Operation{
		OperationID: "get-dashboard", Method: http.MethodGet, Path: "/dashboard",
		Summary: "Club sailing summary", Tags: []string{"Dashboard"},
	}, h.get)
}

func (h *DashboardHandler) get(ctx context.Context, _ *struct{}) (*dashboardOutput, error) {
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
	return &dashboardOutput{Body: dto.Dashboard{
		VoyageCount:      stats.VoyageCount,
		TotalHours:       stats.TotalHours,
		TotalMiles:       stats.TotalMiles,
		TotalDays:        stats.TotalDays,
		TotalHoursSail:   stats.TotalHoursSail,
		TotalHoursEngine: stats.TotalHoursEngine,
		ByYear:           dto.VoyagesByYearFromDB(byYear),
	}}, nil
}
