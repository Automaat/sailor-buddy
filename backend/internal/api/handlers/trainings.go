package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type TrainingHandler struct {
	q sqlcdb.Querier
}

func NewTrainingHandler(q sqlcdb.Querier) *TrainingHandler {
	return &TrainingHandler{q: q}
}

type trainingRequest struct {
	Date      *string  `json:"date"`
	Name      string   `json:"name"`
	Organizer *string  `json:"organizer"`
	Cost      *float64 `json:"cost"`
	Url       *string  `json:"url"`
}

func (h *TrainingHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	trainings, err := h.q.ListTrainings(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list trainings")
		return
	}
	respondJSON(w, http.StatusOK, trainings)
}

func (h *TrainingHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid training id")
		return
	}
	training, err := h.q.GetTraining(r.Context(), sqlcdb.GetTrainingParams{
		ID:     id,
		UserID: user.UserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "training not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get training")
		return
	}
	respondJSON(w, http.StatusOK, training)
}

func (h *TrainingHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req trainingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	training, err := h.q.CreateTraining(r.Context(), sqlcdb.CreateTrainingParams{
		UserID:    user.UserID,
		Date:      nullString(req.Date),
		Name:      req.Name,
		Organizer: nullString(req.Organizer),
		Cost:      nullFloat64(req.Cost),
		Url:       nullString(req.Url),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create training")
		return
	}
	respondJSON(w, http.StatusCreated, training)
}

func (h *TrainingHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid training id")
		return
	}
	var req trainingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateTraining(r.Context(), sqlcdb.UpdateTrainingParams{
		Date:      nullString(req.Date),
		Name:      req.Name,
		Organizer: nullString(req.Organizer),
		Cost:      nullFloat64(req.Cost),
		Url:       nullString(req.Url),
		ID:        id,
		UserID:    user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update training")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *TrainingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid training id")
		return
	}
	if err := h.q.DeleteTraining(r.Context(), sqlcdb.DeleteTrainingParams{
		ID:     id,
		UserID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete training")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
