package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type EnrollmentHandler struct {
	q sqlcdb.Querier
}

func NewEnrollmentHandler(q sqlcdb.Querier) *EnrollmentHandler {
	return &EnrollmentHandler{q: q}
}

func (h *EnrollmentHandler) GetCruiseByToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	cruise, err := h.q.GetCruiseByEnrollToken(r.Context(), sql.NullString{String: token, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "invalid enrollment link")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}

	user := middleware.GetUser(r.Context())
	enrollment, enrollmentErr := h.q.GetUserEnrollment(r.Context(), sqlcdb.GetUserEnrollmentParams{
		CruiseID: cruise.ID,
		UserID:   user.UserID,
	})

	counts, err := h.q.CountCruiseEnrollments(r.Context(), cruise.ID)
	if err != nil {
		log.Printf("failed to count enrollments for cruise %d: %v", cruise.ID, err)
	}

	type response struct {
		Cruise     any   `json:"cruise"`
		Enrolled   bool  `json:"enrolled"`
		Enrollment any   `json:"enrollment,omitempty"`
		Accepted   int64 `json:"accepted_count"`
		Total      int64 `json:"total_count"`
	}

	resp := response{
		Cruise:   cruise,
		Accepted: counts.Accepted,
		Total:    counts.Total,
	}
	if enrollmentErr == nil {
		resp.Enrolled = true
		resp.Enrollment = enrollment
	}

	respondJSON(w, http.StatusOK, resp)
}

type enrollRequest struct {
	Note *string `json:"note"`
}

func (h *EnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	cruise, err := h.q.GetCruiseByEnrollToken(r.Context(), sql.NullString{String: token, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "invalid enrollment link")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}

	user := middleware.GetUser(r.Context())

	var req enrollRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	enrollment, err := h.q.CreateCruiseEnrollment(r.Context(), sqlcdb.CreateCruiseEnrollmentParams{
		CruiseID: cruise.ID,
		UserID:   user.UserID,
		Note:     nullString(req.Note),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, http.StatusConflict, "already enrolled")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to enroll")
		return
	}
	respondJSON(w, http.StatusCreated, enrollment)
}

func (h *EnrollmentHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}

	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: cruiseID, OwnerID: user.UserID}); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := hex.EncodeToString(b)

	if err := h.q.SetCruiseEnrollToken(r.Context(), sqlcdb.SetCruiseEnrollTokenParams{
		EnrollToken: sql.NullString{String: token, Valid: true},
		ID:          cruiseID,
		OwnerID:     user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to set token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *EnrollmentHandler) ClearToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if err := h.q.ClearCruiseEnrollToken(r.Context(), sqlcdb.ClearCruiseEnrollTokenParams{
		ID:      cruiseID,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear token")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *EnrollmentHandler) ListEnrollments(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	enrollments, err := h.q.ListCruiseEnrollments(r.Context(), sqlcdb.ListCruiseEnrollmentsParams{
		CruiseID: cruiseID,
		OwnerID:  user.UserID,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list enrollments")
		return
	}
	respondJSON(w, http.StatusOK, enrollments)
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *EnrollmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid enrollment id")
		return
	}
	var req updateStatusRequest
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
	if err := h.q.UpdateEnrollmentStatus(r.Context(), sqlcdb.UpdateEnrollmentStatusParams{
		Status:  req.Status,
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *EnrollmentHandler) DeleteEnrollment(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid enrollment id")
		return
	}
	if err := h.q.DeleteCruiseEnrollment(r.Context(), sqlcdb.DeleteCruiseEnrollmentParams{
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete enrollment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
