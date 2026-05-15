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
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgCruiseHandler struct {
	q sqlcdb.Querier
}

func NewOrgCruiseHandler(q sqlcdb.Querier) *OrgCruiseHandler {
	return &OrgCruiseHandler{q: q}
}

type orgCruiseRequest struct {
	Name          string   `json:"name"`
	EmbarkDate    *string  `json:"embark_date"`
	DisembarkDate *string  `json:"disembark_date"`
	Countries     *string  `json:"countries"`
	StartPort     *string  `json:"start_port"`
	EndPort       *string  `json:"end_port"`
	Description   *string  `json:"description"`
	ImageLogoUrl  *string  `json:"image_logo_url"`
	ImagePhotoUrl *string  `json:"image_photo_url"`
	ImageRouteUrl *string  `json:"image_route_url"`
	MaxCrew       *int64   `json:"max_crew"`
	CostPerPerson *float64 `json:"cost_per_person"`
}

func (h *OrgCruiseHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	cruises, err := h.q.ListCruises(r.Context(), octx.OrgID)
	if err != nil {
		slog.Error("list org cruises", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list cruises")
		return
	}
	respondJSON(w, http.StatusOK, cruises)
}

func (h *OrgCruiseHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	cruise, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: id, OrgID: octx.OrgID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		slog.Error("get org cruise", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}
	respondJSON(w, http.StatusOK, cruise)
}

func (h *OrgCruiseHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	var req orgCruiseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	cruise, err := h.q.CreateCruise(r.Context(), sqlcdb.CreateCruiseParams{
		OrgID:         octx.OrgID,
		Name:          req.Name,
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		Description:   nullString(req.Description),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		MaxCrew:       nullInt64(req.MaxCrew),
		CostPerPerson: nullFloat64(req.CostPerPerson),
	})
	if err != nil {
		slog.Error("create org cruise", "org_id", octx.OrgID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create cruise")
		return
	}
	respondJSON(w, http.StatusCreated, cruise)
}

func (h *OrgCruiseHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	var req orgCruiseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateCruise(r.Context(), sqlcdb.UpdateCruiseParams{
		Name:          req.Name,
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		Description:   nullString(req.Description),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		MaxCrew:       nullInt64(req.MaxCrew),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ID:            id,
		OrgID:         octx.OrgID,
	}); err != nil {
		slog.Error("update org cruise", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update cruise")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgCruiseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if err := h.q.DeleteCruise(r.Context(), sqlcdb.DeleteCruiseParams{ID: id, OrgID: octx.OrgID}); err != nil {
		slog.Error("delete org cruise", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete cruise")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgCruiseHandler) GenerateEnrollToken(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: id, OrgID: octx.OrgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		slog.Error("verify cruise for token generation", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
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
		ID:          id,
		OrgID:       octx.OrgID,
	}); err != nil {
		slog.Error("set cruise enroll token", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to set token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *OrgCruiseHandler) ClearEnrollToken(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if err := h.q.ClearCruiseEnrollToken(r.Context(), sqlcdb.ClearCruiseEnrollTokenParams{ID: id, OrgID: octx.OrgID}); err != nil {
		slog.Error("clear cruise enroll token", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to clear token")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgCruiseHandler) ListChildTrips(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: id, OrgID: octx.OrgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		slog.Error("verify cruise for child trips", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
		return
	}
	trips, err := h.q.ListCruiseTrips(r.Context(), sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		slog.Error("list cruise child trips", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list trips")
		return
	}
	respondJSON(w, http.StatusOK, trips)
}

func (h *OrgCruiseHandler) ListChildVoyages(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: id, OrgID: octx.OrgID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		slog.Error("verify cruise for child voyages", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
		return
	}
	voyages, err := h.q.ListCruiseVoyages(r.Context(), sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		slog.Error("list cruise child voyages", "cruise_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list voyages")
		return
	}
	respondJSON(w, http.StatusOK, voyages)
}
