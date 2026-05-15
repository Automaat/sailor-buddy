package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type CruiseEnrollmentHandler struct {
	q sqlcdb.Querier
}

func NewCruiseEnrollmentHandler(q sqlcdb.Querier) *CruiseEnrollmentHandler {
	return &CruiseEnrollmentHandler{q: q}
}

func (h *CruiseEnrollmentHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	enrollments, err := h.q.ListCruiseEnrollments(r.Context(), sqlcdb.ListCruiseEnrollmentsParams{
		CruiseID: cruiseID,
		OrgID:    octx.OrgID,
	})
	if err != nil {
		slog.Error("list cruise enrollments", "cruise_id", cruiseID, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list enrollments")
		return
	}
	respondJSON(w, http.StatusOK, enrollments)
}

func (h *CruiseEnrollmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "enrollmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid enrollment id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.Status {
	case "accepted", "rejected", "waitlisted", "pending":
	default:
		respondError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if err := h.q.UpdateCruiseEnrollmentStatus(r.Context(), sqlcdb.UpdateCruiseEnrollmentStatusParams{
		Status: req.Status,
		ID:     id,
		OrgID:  octx.OrgID,
	}); err != nil {
		slog.Error("update cruise enrollment status", "enrollment_id", id, "org_id", octx.OrgID, "status", req.Status, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CruiseEnrollmentHandler) AssignToTrip(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "enrollmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid enrollment id")
		return
	}
	var req struct {
		TripID *int64 `json:"trip_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.q.AssignCruiseEnrollmentToTrip(r.Context(), sqlcdb.AssignCruiseEnrollmentToTripParams{
		TripID: nullInt64(req.TripID),
		ID:     id,
		OrgID:  octx.OrgID,
	}); err != nil {
		slog.Error("assign cruise enrollment to trip", "enrollment_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to assign enrollment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CruiseEnrollmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "enrollmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid enrollment id")
		return
	}
	if err := h.q.DeleteCruiseEnrollment(r.Context(), sqlcdb.DeleteCruiseEnrollmentParams{
		ID:    id,
		OrgID: octx.OrgID,
	}); err != nil {
		slog.Error("delete cruise enrollment", "enrollment_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete enrollment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
