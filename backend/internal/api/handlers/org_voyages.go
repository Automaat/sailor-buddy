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

type OrgVoyageHandler struct {
	q sqlcdb.Querier
}

func NewOrgVoyageHandler(q sqlcdb.Querier) *OrgVoyageHandler {
	return &OrgVoyageHandler{q: q}
}

func (h *OrgVoyageHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	voyages, err := h.q.ListOrgVoyages(r.Context(), sql.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
		slog.Error("list org voyages", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list voyages")
		return
	}
	respondJSON(w, http.StatusOK, voyages)
}

func (h *OrgVoyageHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	voyage, err := h.q.GetOrgVoyage(r.Context(), sqlcdb.GetOrgVoyageParams{ID: id, OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true}})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("get org voyage", "voyage_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get voyage")
		return
	}
	respondJSON(w, http.StatusOK, voyage)
}

func (h *OrgVoyageHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
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
	voyage, err := h.q.CreateOrgVoyage(r.Context(), sqlcdb.CreateOrgVoyageParams{
		OwnerID:       user.UserID,
		OrgID:         sql.NullInt64{Int64: octx.OrgID, Valid: true},
		CruiseID:      nullInt64(req.CruiseID),
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
		slog.Error("create org voyage", "org_id", octx.OrgID, "user_id", user.UserID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create voyage")
		return
	}
	respondJSON(w, http.StatusCreated, voyage)
}

func (h *OrgVoyageHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
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
	if err := h.q.UpdateOrgVoyage(r.Context(), sqlcdb.UpdateOrgVoyageParams{
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
		CruiseID:      nullInt64(req.CruiseID),
		ID:            id,
		OrgID:         sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("update org voyage", "voyage_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update voyage")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgVoyageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}
	if err := h.q.DeleteOrgVoyage(r.Context(), sqlcdb.DeleteOrgVoyageParams{ID: id, OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true}}); err != nil {
		slog.Error("delete org voyage", "voyage_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete voyage")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}
