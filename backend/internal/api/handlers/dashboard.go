package handlers

import (
	"log/slog"
	"net/http"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type DashboardHandler struct {
	q sqlcdb.Querier
}

func NewDashboardHandler(q sqlcdb.Querier) *DashboardHandler {
	return &DashboardHandler{q: q}
}

type dashboardResponse struct {
	VoyageCount      int64                        `json:"voyage_count"`
	TotalHours       float64                      `json:"total_hours"`
	TotalMiles       float64                      `json:"total_miles"`
	TotalDays        int64                        `json:"total_days"`
	TotalHoursSail   float64                      `json:"total_hours_sail"`
	TotalHoursEngine float64                      `json:"total_hours_engine"`
	ByYear           []sqlcdb.GetVoyagesByYearRow `json:"by_year"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	stats, err := h.q.GetDashboardStats(r.Context(), user.UserID)
	if err != nil {
		slog.Error("dashboard stats", "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}
	byYear, err := h.q.GetVoyagesByYear(r.Context(), user.UserID)
	if err != nil {
		slog.Error("dashboard yearly breakdown", "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get yearly breakdown")
		return
	}
	if byYear == nil {
		byYear = []sqlcdb.GetVoyagesByYearRow{}
	}
	respondJSON(w, http.StatusOK, dashboardResponse{
		VoyageCount:      stats.VoyageCount,
		TotalHours:       stats.TotalHours,
		TotalMiles:       stats.TotalMiles,
		TotalDays:        stats.TotalDays,
		TotalHoursSail:   stats.TotalHoursSail,
		TotalHoursEngine: stats.TotalHoursEngine,
		ByYear:           byYear,
	})
}
