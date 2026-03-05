package handlers

import (
	"net/http"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type DashboardHandler struct {
	q *sqlcdb.Queries
}

func NewDashboardHandler(q *sqlcdb.Queries) *DashboardHandler {
	return &DashboardHandler{q: q}
}

type dashboardResponse struct {
	Stats  sqlcdb.GetDashboardStatsRow  `json:"stats"`
	ByYear []sqlcdb.GetCruisesByYearRow `json:"by_year"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	stats, err := h.q.GetDashboardStats(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}
	byYear, err := h.q.GetCruisesByYear(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get yearly breakdown")
		return
	}
	respondJSON(w, http.StatusOK, dashboardResponse{
		Stats:  stats,
		ByYear: byYear,
	})
}
