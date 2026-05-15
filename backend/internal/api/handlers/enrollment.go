package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type EnrollmentHandler struct {
	q sqlcdb.Querier
}

func NewEnrollmentHandler(q sqlcdb.Querier) *EnrollmentHandler {
	return &EnrollmentHandler{q: q}
}

// GetByToken handles /enroll/{token}. The token may belong to either a trip
// (standalone trip enrollment) or a cruise (org event-level enrollment).
// Response shape: {kind: "trip" | "cruise", ...}
func (h *EnrollmentHandler) GetByToken(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	user := middleware.GetUser(r.Context())

	// Try trip first.
	trip, terr := h.q.GetTripByEnrollToken(r.Context(), types.NullString{String: token, Valid: true})
	if terr == nil {
		counts, _ := h.q.CountTripEnrollments(r.Context(), trip.ID)
		enrollment, enrollErr := h.q.GetUserTripEnrollment(r.Context(), sqlcdb.GetUserTripEnrollmentParams{
			TripID: trip.ID,
			UserID: user.UserID,
		})
		resp := map[string]any{
			"kind":           "trip",
			"trip":           trip,
			"accepted_count": counts.Accepted,
			"total_count":    counts.Total,
		}
		if enrollErr == nil {
			resp["enrolled"] = true
			resp["enrollment"] = enrollment
		} else {
			resp["enrolled"] = false
		}
		respondJSON(w, http.StatusOK, resp)
		return
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token", "err", terr)
		respondError(w, http.StatusInternalServerError, "failed to get trip")
		return
	}

	// Fall through to cruise.
	cruise, cerr := h.q.GetCruiseByEnrollToken(r.Context(), types.NullString{String: token, Valid: true})
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "invalid enrollment link")
			return
		}
		slog.Error("get cruise by enroll token", "err", cerr)
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}
	counts, _ := h.q.CountCruiseEnrollments(r.Context(), cruise.ID)
	enrollment, enrollErr := h.q.GetUserCruiseEnrollment(r.Context(), sqlcdb.GetUserCruiseEnrollmentParams{
		CruiseID: cruise.ID,
		UserID:   user.UserID,
	})
	trips, tlistErr := h.q.ListCruiseTrips(r.Context(), types.NullInt64{Int64: cruise.ID, Valid: true})
	if tlistErr != nil {
		slog.Error("list cruise trips", "cruise_id", cruise.ID, "err", tlistErr)
		trips = nil
	}
	resp := map[string]any{
		"kind":           "cruise",
		"cruise":         cruise,
		"trips":          trips,
		"accepted_count": counts.Accepted,
		"total_count":    counts.Total,
	}
	if enrollErr == nil {
		resp["enrolled"] = true
		resp["enrollment"] = enrollment
	} else {
		resp["enrolled"] = false
	}
	respondJSON(w, http.StatusOK, resp)
}

type enrollRequest struct {
	Note *string `json:"note"`
}

// Enroll handles POST /enroll/{token}. Routes to trip or cruise enrollment by token.
func (h *EnrollmentHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	user := middleware.GetUser(r.Context())

	var req enrollRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	trip, terr := h.q.GetTripByEnrollToken(r.Context(), types.NullString{String: token, Valid: true})
	if terr == nil {
		status, _ := h.q.GetTripStatus(r.Context(), trip.ID)
		if status != sqlcdb.TripStatusPlanned {
			respondError(w, http.StatusConflict, "enrollment closed: trip is not planned")
			return
		}
		enrollment, err := h.q.CreateTripEnrollment(r.Context(), sqlcdb.CreateTripEnrollmentParams{
			TripID: trip.ID,
			UserID: user.UserID,
			Note:   nullString(req.Note),
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				respondError(w, http.StatusConflict, "already enrolled")
				return
			}
			slog.Error("create trip enrollment", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
			respondError(w, http.StatusInternalServerError, "failed to enroll")
			return
		}
		respondJSON(w, http.StatusCreated, enrollment)
		return
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token for enroll", "err", terr)
		respondError(w, http.StatusInternalServerError, "failed to get trip")
		return
	}

	cruise, cerr := h.q.GetCruiseByEnrollToken(r.Context(), types.NullString{String: token, Valid: true})
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "invalid enrollment link")
			return
		}
		slog.Error("get cruise by enroll token for enroll", "err", cerr)
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
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
		slog.Error("create cruise enrollment", "cruise_id", cruise.ID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to enroll")
		return
	}
	respondJSON(w, http.StatusCreated, enrollment)
}

// --- per-trip enrollment management (admin side) ---

func (h *EnrollmentHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	tripID, err := strconv.ParseInt(chi.URLParam(r, "tripID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}

	if _, err := h.q.GetTrip(r.Context(), sqlcdb.GetTripParams{ID: tripID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found")
			return
		}
		slog.Error("get trip for token generation", "trip_id", tripID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get trip")
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := hex.EncodeToString(b)

	if err := h.q.SetTripEnrollToken(r.Context(), sqlcdb.SetTripEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          tripID,
		OwnerID:     user.UserID,
	}); err != nil {
		slog.Error("set trip enroll token", "trip_id", tripID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to set token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *EnrollmentHandler) ClearToken(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	tripID, err := strconv.ParseInt(chi.URLParam(r, "tripID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	if err := h.q.ClearTripEnrollToken(r.Context(), sqlcdb.ClearTripEnrollTokenParams{
		ID:      tripID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("clear trip enroll token", "trip_id", tripID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to clear token")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *EnrollmentHandler) ListEnrollments(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	tripID, err := strconv.ParseInt(chi.URLParam(r, "tripID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	enrollments, err := h.q.ListTripEnrollments(r.Context(), sqlcdb.ListTripEnrollmentsParams{
		TripID:  tripID,
		OwnerID: user.UserID,
	})
	if err != nil {
		slog.Error("list trip enrollments", "trip_id", tripID, "user_id", user.UserID, "err", err)
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
	if err := h.q.UpdateTripEnrollmentStatus(r.Context(), sqlcdb.UpdateTripEnrollmentStatusParams{
		Status:  req.Status,
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("update trip enrollment status", "enrollment_id", id, "user_id", user.UserID, "status", req.Status, "err", err)
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
	if err := h.q.DeleteTripEnrollment(r.Context(), sqlcdb.DeleteTripEnrollmentParams{
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("delete trip enrollment", "enrollment_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete enrollment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
