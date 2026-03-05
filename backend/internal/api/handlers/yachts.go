package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type YachtHandler struct {
	q *sqlcdb.Queries
}

func NewYachtHandler(q *sqlcdb.Queries) *YachtHandler {
	return &YachtHandler{q: q}
}

type yachtRequest struct {
	Name           string  `json:"name"`
	RegistrationNo *string `json:"registration_no"`
	YachtType      *string `json:"yacht_type"`
}

func (h *YachtHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	yachts, err := h.q.ListYachts(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list yachts")
		return
	}
	respondJSON(w, http.StatusOK, yachts)
}

func (h *YachtHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	yacht, err := h.q.GetYacht(r.Context(), sqlcdb.GetYachtParams{
		ID:      id,
		OwnerID: user.UserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "yacht not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get yacht")
		return
	}
	respondJSON(w, http.StatusOK, yacht)
}

func (h *YachtHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req yachtRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	yacht, err := h.q.CreateYacht(r.Context(), sqlcdb.CreateYachtParams{
		OwnerID:        user.UserID,
		Name:           req.Name,
		RegistrationNo: nullString(req.RegistrationNo),
		YachtType:      nullString(req.YachtType),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create yacht")
		return
	}
	respondJSON(w, http.StatusCreated, yacht)
}

func (h *YachtHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	var req yachtRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateYacht(r.Context(), sqlcdb.UpdateYachtParams{
		Name:           req.Name,
		RegistrationNo: nullString(req.RegistrationNo),
		YachtType:      nullString(req.YachtType),
		ID:             id,
		OwnerID:        user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update yacht")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *YachtHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	if err := h.q.DeleteYacht(r.Context(), sqlcdb.DeleteYachtParams{
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete yacht")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
