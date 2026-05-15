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

type VoyageHandler struct {
	q sqlcdb.Querier
}

func NewVoyageHandler(q sqlcdb.Querier) *VoyageHandler {
	return &VoyageHandler{q: q}
}

type voyageRequest struct {
	Name          string   `json:"name"`
	Year          *int64   `json:"year"`
	EmbarkDate    *string  `json:"embark_date"`
	DisembarkDate *string  `json:"disembark_date"`
	Countries     *string  `json:"countries"`
	StartPort     *string  `json:"start_port"`
	EndPort       *string  `json:"end_port"`
	CaptainName   *string  `json:"captain_name"`
	YachtID       *int64   `json:"yacht_id"`
	HoursTotal    *float64 `json:"hours_total"`
	HoursSail     *float64 `json:"hours_sail"`
	HoursEngine   *float64 `json:"hours_engine"`
	HoursOver6bf  *float64 `json:"hours_over_6bf"`
	Miles         *float64 `json:"miles"`
	Days          *int64   `json:"days"`
	TidalWaters   *int64   `json:"tidal_waters"`
	CostTotal     *float64 `json:"cost_total"`
	CostPerPerson *float64 `json:"cost_per_person"`
	ImageLogoUrl  *string  `json:"image_logo_url"`
	ImagePhotoUrl *string  `json:"image_photo_url"`
	ImageRouteUrl *string  `json:"image_route_url"`
	Description   *string  `json:"description"`
	CruiseID      *int64   `json:"cruise_id"`
}

func (h *VoyageHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyages, err := h.q.ListVoyages(r.Context(), user.UserID)
	if err != nil {
		slog.Error("list voyages", "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list voyages")
		return
	}
	respondJSON(w, http.StatusOK, voyages)
}

func (h *VoyageHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	voyage, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: id, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("get voyage", "voyage_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get voyage")
		return
	}
	respondJSON(w, http.StatusOK, voyage)
}

func (h *VoyageHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req voyageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	voyage, err := h.q.CreateVoyage(r.Context(), sqlcdb.CreateVoyageParams{
		OwnerID:       user.UserID,
		Name:          req.Name,
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		HoursTotal:    valOrZeroFloat(req.HoursTotal),
		HoursSail:     valOrZeroFloat(req.HoursSail),
		HoursEngine:   valOrZeroFloat(req.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(req.HoursOver6bf),
		Miles:         valOrZeroFloat(req.Miles),
		Days:          valOrZeroInt(req.Days),
		TidalWaters:   valOrZeroInt(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
	})
	if err != nil {
		slog.Error("create voyage", "user_id", user.UserID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create voyage")
		return
	}
	respondJSON(w, http.StatusCreated, voyage)
}

func (h *VoyageHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	var req voyageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateVoyage(r.Context(), sqlcdb.UpdateVoyageParams{
		Name:          req.Name,
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		HoursTotal:    valOrZeroFloat(req.HoursTotal),
		HoursSail:     valOrZeroFloat(req.HoursSail),
		HoursEngine:   valOrZeroFloat(req.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(req.HoursOver6bf),
		Miles:         valOrZeroFloat(req.Miles),
		Days:          valOrZeroInt(req.Days),
		TidalWaters:   valOrZeroInt(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		ID:            id,
		OwnerID:       user.UserID,
	}); err != nil {
		slog.Error("update voyage", "voyage_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update voyage")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *VoyageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	if err := h.q.DeleteVoyage(r.Context(), sqlcdb.DeleteVoyageParams{ID: id, OwnerID: user.UserID}); err != nil {
		slog.Error("delete voyage", "voyage_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete voyage")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
