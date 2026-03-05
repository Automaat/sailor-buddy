package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type CruiseHandler struct {
	q *sqlcdb.Queries
}

func NewCruiseHandler(q *sqlcdb.Queries) *CruiseHandler {
	return &CruiseHandler{q: q}
}

type cruiseRequest struct {
	Name          string   `json:"name"`
	Year          *int64   `json:"year"`
	EmbarkDate    *string  `json:"embark_date"`
	DisembarkDate *string  `json:"disembark_date"`
	Countries     *string  `json:"countries"`
	StartPort     *string  `json:"start_port"`
	EndPort       *string  `json:"end_port"`
	HoursTotal    *float64 `json:"hours_total"`
	HoursSail     *float64 `json:"hours_sail"`
	HoursEngine   *float64 `json:"hours_engine"`
	HoursOver6bf  *float64 `json:"hours_over_6bf"`
	Miles         *float64 `json:"miles"`
	Days          *int64   `json:"days"`
	CaptainName   *string  `json:"captain_name"`
	YachtID       *int64   `json:"yacht_id"`
	TidalWaters   *int64   `json:"tidal_waters"`
	CostTotal     *float64 `json:"cost_total"`
	CostPerPerson *float64 `json:"cost_per_person"`
	ImageLogoUrl  *string  `json:"image_logo_url"`
	ImagePhotoUrl *string  `json:"image_photo_url"`
	ImageRouteUrl *string  `json:"image_route_url"`
	Description   *string  `json:"description"`
}

func (h *CruiseHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruises, err := h.q.ListCruises(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list cruises")
		return
	}
	respondJSON(w, http.StatusOK, cruises)
}

func (h *CruiseHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	cruise, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{
		ID:      id,
		OwnerID: user.UserID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}
	respondJSON(w, http.StatusOK, cruise)
}

func (h *CruiseHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req cruiseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	cruise, err := h.q.CreateCruise(r.Context(), sqlcdb.CreateCruiseParams{
		OwnerID:       user.UserID,
		Name:          req.Name,
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		HoursTotal:    nullFloat64(req.HoursTotal),
		HoursSail:     nullFloat64(req.HoursSail),
		HoursEngine:   nullFloat64(req.HoursEngine),
		HoursOver6bf:  nullFloat64(req.HoursOver6bf),
		Miles:         nullFloat64(req.Miles),
		Days:          nullInt64(req.Days),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		TidalWaters:   nullInt64(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create cruise")
		return
	}
	respondJSON(w, http.StatusCreated, cruise)
}

func (h *CruiseHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	var req cruiseRequest
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
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		HoursTotal:    nullFloat64(req.HoursTotal),
		HoursSail:     nullFloat64(req.HoursSail),
		HoursEngine:   nullFloat64(req.HoursEngine),
		HoursOver6bf:  nullFloat64(req.HoursOver6bf),
		Miles:         nullFloat64(req.Miles),
		Days:          nullInt64(req.Days),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		TidalWaters:   nullInt64(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		ID:            id,
		OwnerID:       user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update cruise")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *CruiseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if err := h.q.DeleteCruise(r.Context(), sqlcdb.DeleteCruiseParams{
		ID:      id,
		OwnerID: user.UserID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete cruise")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func nullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}
