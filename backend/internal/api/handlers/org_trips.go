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
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type OrgTripHandler struct {
	q  sqlcdb.Querier
	db *sql.DB
}

func NewOrgTripHandler(q sqlcdb.Querier, db *sql.DB) *OrgTripHandler {
	return &OrgTripHandler{q: q, db: db}
}

func (h *OrgTripHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	trips, err := h.q.ListOrgTrips(r.Context(), types.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
		slog.Error("list org trips", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list trips")
		return
	}
	respondJSON(w, http.StatusOK, trips)
}

func (h *OrgTripHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	trip, err := h.q.GetOrgTrip(r.Context(), sqlcdb.GetOrgTripParams{ID: id, OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true}})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found")
			return
		}
		slog.Error("get org trip", "trip_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get trip")
		return
	}
	respondJSON(w, http.StatusOK, trip)
}

func (h *OrgTripHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	var req tripRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	trip, err := h.q.CreateOrgTrip(r.Context(), sqlcdb.CreateOrgTripParams{
		OwnerID:       user.UserID,
		OrgID:         types.NullInt64{Int64: octx.OrgID, Valid: true},
		CruiseID:      nullInt64(req.CruiseID),
		Name:          req.Name,
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		MaxCrew:       nullInt64(req.MaxCrew),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		Status:        sqlcdb.TripStatusPlanned,
	})
	if err != nil {
		slog.Error("create org trip", "org_id", octx.OrgID, "user_id", user.UserID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create trip")
		return
	}
	respondJSON(w, http.StatusCreated, trip)
}

func (h *OrgTripHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	var req tripRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateOrgTrip(r.Context(), sqlcdb.UpdateOrgTripParams{
		Name:          req.Name,
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		MaxCrew:       nullInt64(req.MaxCrew),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		CruiseID:      nullInt64(req.CruiseID),
		ID:            id,
		OrgID:         types.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("update org trip", "trip_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update trip")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgTripHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	if err := h.q.DeleteOrgTrip(r.Context(), sqlcdb.DeleteOrgTripParams{ID: id, OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true}}); err != nil {
		slog.Error("delete org trip", "trip_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete trip")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgTripHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	trip, err := h.q.CancelOrgTrip(r.Context(), sqlcdb.CancelOrgTripParams{ID: id, OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true}})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found or invalid transition")
			return
		}
		slog.Error("cancel org trip", "trip_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel trip")
		return
	}
	respondJSON(w, http.StatusOK, trip)
}

func (h *OrgTripHandler) Complete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	var req completeTripRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	voyage, err := completeTripTx(r, h.db, types.NullInt64{Int64: octx.OrgID, Valid: true}, id, user.UserID, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found or not in planned state")
			return
		}
		slog.Error("complete org trip", "trip_id", id, "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to complete trip")
		return
	}
	respondJSON(w, http.StatusCreated, voyage)
}
