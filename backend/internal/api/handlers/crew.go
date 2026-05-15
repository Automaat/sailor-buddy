package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type CrewHandler struct {
	q sqlcdb.Querier
}

func NewCrewHandler(q sqlcdb.Querier) *CrewHandler {
	return &CrewHandler{q: q}
}

type crewMemberRequest struct {
	FullName     string  `json:"full_name"`
	Email        *string `json:"email"`
	PatentNumber *string `json:"patent_number"`
}

type crewAssignmentRequest struct {
	CrewMemberID int64   `json:"crew_member_id"`
	Role         string  `json:"role"`
	PatentNumber *string `json:"patent_number"`
}

func (h *CrewHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	members, err := h.q.ListCrewMembers(r.Context(), user.UserID)
	if err != nil {
		slog.Error("list crew members", "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list crew members")
		return
	}
	respondJSON(w, http.StatusOK, members)
}

func (h *CrewHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	member, err := h.q.GetCrewMember(r.Context(), sqlcdb.GetCrewMemberParams{
		ID:      id,
		OwnerID: user.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "crew member not found")
			return
		}
		slog.Error("get crew member", "crew_member_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get crew member")
		return
	}
	respondJSON(w, http.StatusOK, member)
}

func (h *CrewHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req crewMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FullName == "" {
		respondError(w, http.StatusBadRequest, "full_name is required")
		return
	}
	member, err := h.q.CreateCrewMember(r.Context(), sqlcdb.CreateCrewMemberParams{
		OwnerID:      user.UserID,
		FullName:     req.FullName,
		Email:        nullString(req.Email),
		PatentNumber: nullString(req.PatentNumber),
	})
	if err != nil {
		slog.Error("create crew member", "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create crew member")
		return
	}
	respondJSON(w, http.StatusCreated, member)
}

func (h *CrewHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	var req crewMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FullName == "" {
		respondError(w, http.StatusBadRequest, "full_name is required")
		return
	}
	if err := h.q.UpdateCrewMember(r.Context(), sqlcdb.UpdateCrewMemberParams{
		FullName:     req.FullName,
		Email:        nullString(req.Email),
		PatentNumber: nullString(req.PatentNumber),
		ID:           id,
		OwnerID:      user.UserID,
	}); err != nil {
		slog.Error("update crew member", "crew_member_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update crew member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CrewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	if err := h.q.DeleteCrewMember(r.Context(), sqlcdb.DeleteCrewMemberParams{
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("delete crew member", "crew_member_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete crew member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CrewHandler) AssignTripCrew(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("verify trip for crew assignment", "trip_id", tripID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify trip")
		return
	}
	var req crewAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == "" {
		respondError(w, http.StatusBadRequest, "role is required")
		return
	}
	assignment, err := h.q.CreateTripCrewAssignment(r.Context(), sqlcdb.CreateTripCrewAssignmentParams{
		TripID:       sql.NullInt64{Int64: tripID, Valid: true},
		CrewMemberID: req.CrewMemberID,
		Role:         req.Role,
		PatentNumber: nullString(req.PatentNumber),
	})
	if err != nil {
		slog.Error("assign trip crew", "trip_id", tripID, "crew_member_id", req.CrewMemberID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to assign crew member")
		return
	}
	respondJSON(w, http.StatusCreated, assignment)
}

func (h *CrewHandler) ListTripCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	tripID, err := strconv.ParseInt(chi.URLParam(r, "tripID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	assignments, err := h.q.ListTripCrewAssignments(r.Context(), sqlcdb.ListTripCrewAssignmentsParams{
		TripID:  sql.NullInt64{Int64: tripID, Valid: true},
		OwnerID: user.UserID,
	})
	if err != nil {
		slog.Error("list trip crew", "trip_id", tripID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list trip crew")
		return
	}
	respondJSON(w, http.StatusOK, assignments)
}

func (h *CrewHandler) RemoveTripCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	assignmentID, err := strconv.ParseInt(chi.URLParam(r, "assignmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assignment id")
		return
	}
	if err := h.q.DeleteTripCrewAssignment(r.Context(), sqlcdb.DeleteTripCrewAssignmentParams{
		ID:      assignmentID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("remove trip crew assignment", "assignment_id", assignmentID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to remove crew assignment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CrewHandler) AssignVoyageCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	if _, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("verify voyage for crew assignment", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify voyage")
		return
	}
	var req crewAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == "" {
		respondError(w, http.StatusBadRequest, "role is required")
		return
	}
	assignment, err := h.q.CreateVoyageCrewAssignment(r.Context(), sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID:     sql.NullInt64{Int64: voyageID, Valid: true},
		CrewMemberID: req.CrewMemberID,
		Role:         req.Role,
		PatentNumber: nullString(req.PatentNumber),
	})
	if err != nil {
		slog.Error("assign voyage crew", "voyage_id", voyageID, "crew_member_id", req.CrewMemberID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to assign crew member")
		return
	}
	respondJSON(w, http.StatusCreated, assignment)
}

func (h *CrewHandler) ListVoyageCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	assignments, err := h.q.ListVoyageCrewAssignments(r.Context(), sqlcdb.ListVoyageCrewAssignmentsParams{
		VoyageID: sql.NullInt64{Int64: voyageID, Valid: true},
		OwnerID:  user.UserID,
	})
	if err != nil {
		slog.Error("list voyage crew", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list voyage crew")
		return
	}
	respondJSON(w, http.StatusOK, assignments)
}

func (h *CrewHandler) RemoveVoyageCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	assignmentID, err := strconv.ParseInt(chi.URLParam(r, "assignmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assignment id")
		return
	}
	if err := h.q.DeleteVoyageCrewAssignment(r.Context(), sqlcdb.DeleteVoyageCrewAssignmentParams{
		ID:      assignmentID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("remove voyage crew assignment", "assignment_id", assignmentID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to remove crew assignment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
