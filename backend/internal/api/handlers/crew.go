package handlers

import (
	"database/sql"
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
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "crew member not found")
			return
		}
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
		respondError(w, http.StatusInternalServerError, "failed to delete crew member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CrewHandler) AssignCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{
		ID:      cruiseID,
		OwnerID: user.UserID,
	}); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
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
	assignment, err := h.q.CreateCrewAssignment(r.Context(), sqlcdb.CreateCrewAssignmentParams{
		CruiseID:     cruiseID,
		CrewMemberID: req.CrewMemberID,
		Role:         req.Role,
		PatentNumber: nullString(req.PatentNumber),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to assign crew member")
		return
	}
	respondJSON(w, http.StatusCreated, assignment)
}

func (h *CrewHandler) ListCruiseCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	assignments, err := h.q.ListCruiseCrewAssignments(r.Context(), sqlcdb.ListCruiseCrewAssignmentsParams{
		CruiseID: cruiseID,
		OwnerID:  user.UserID,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list cruise crew")
		return
	}
	respondJSON(w, http.StatusOK, assignments)
}

func (h *CrewHandler) RemoveCruiseCrew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	assignmentID, err := strconv.ParseInt(chi.URLParam(r, "assignmentID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assignment id")
		return
	}
	if err := h.q.DeleteCrewAssignment(r.Context(), sqlcdb.DeleteCrewAssignmentParams{
		ID:      assignmentID,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove crew assignment")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
