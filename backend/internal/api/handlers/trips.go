package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type TripHandler struct {
	q  sqlcdb.Querier
	db *sql.DB
}

func NewTripHandler(q sqlcdb.Querier, db *sql.DB) *TripHandler {
	return &TripHandler{q: q, db: db}
}

type tripRequest struct {
	Name          string   `json:"name"`
	EmbarkDate    *string  `json:"embark_date"`
	DisembarkDate *string  `json:"disembark_date"`
	Countries     *string  `json:"countries"`
	StartPort     *string  `json:"start_port"`
	EndPort       *string  `json:"end_port"`
	CaptainName   *string  `json:"captain_name"`
	YachtID       *int64   `json:"yacht_id"`
	CostTotal     *float64 `json:"cost_total"`
	CostPerPerson *float64 `json:"cost_per_person"`
	MaxCrew       *int64   `json:"max_crew"`
	ImageLogoUrl  *string  `json:"image_logo_url"`
	ImagePhotoUrl *string  `json:"image_photo_url"`
	ImageRouteUrl *string  `json:"image_route_url"`
	Description   *string  `json:"description"`
	CruiseID      *int64   `json:"cruise_id"`
}

type completeTripRequest struct {
	Year         *int64   `json:"year"`
	HoursTotal   *float64 `json:"hours_total"`
	HoursSail    *float64 `json:"hours_sail"`
	HoursEngine  *float64 `json:"hours_engine"`
	HoursOver6bf *float64 `json:"hours_over_6bf"`
	Miles        *float64 `json:"miles"`
	Days         *int64   `json:"days"`
	TidalWaters  *int64   `json:"tidal_waters"`
}

func (h *TripHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	trips, err := h.q.ListTrips(r.Context(), user.UserID)
	if err != nil {
		slog.Error("list trips", "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list trips")
		return
	}
	respondJSON(w, http.StatusOK, trips)
}

func (h *TripHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	trip, err := h.q.GetTrip(r.Context(), sqlcdb.GetTripParams{ID: id, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found")
			return
		}
		slog.Error("get trip", "trip_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get trip")
		return
	}
	respondJSON(w, http.StatusOK, trip)
}

func (h *TripHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	trip, err := h.q.CreateTrip(r.Context(), sqlcdb.CreateTripParams{
		OwnerID:       user.UserID,
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
		slog.Error("create trip", "user_id", user.UserID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create trip")
		return
	}
	respondJSON(w, http.StatusCreated, trip)
}

func (h *TripHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
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
	if err := h.q.UpdateTrip(r.Context(), sqlcdb.UpdateTripParams{
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
		ID:            id,
		OwnerID:       user.UserID,
	}); err != nil {
		slog.Error("update trip", "trip_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update trip")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *TripHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	if err := h.q.DeleteTrip(r.Context(), sqlcdb.DeleteTripParams{ID: id, OwnerID: user.UserID}); err != nil {
		slog.Error("delete trip", "trip_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete trip")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *TripHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid trip id")
		return
	}
	trip, err := h.q.CancelTrip(r.Context(), sqlcdb.CancelTripParams{ID: id, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found or invalid transition")
			return
		}
		slog.Error("cancel trip", "trip_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to cancel trip")
		return
	}
	respondJSON(w, http.StatusOK, trip)
}

func (h *TripHandler) Complete(w http.ResponseWriter, r *http.Request) {
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

	voyage, err := completeTripTx(r, h.db, types.NullInt64{}, id, user.UserID, req)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "trip not found or not in planned state")
			return
		}
		slog.Error("complete trip", "trip_id", id, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to complete trip")
		return
	}
	respondJSON(w, http.StatusCreated, voyage)
}

// completeTripTx wraps the trip → voyage transition. If orgID.Valid, scopes by org_id;
// otherwise scopes by owner_id with org_id IS NULL.
func completeTripTx(r *http.Request, db *sql.DB, orgID types.NullInt64, tripID, userID int64, req completeTripRequest) (sqlcdb.Voyage, error) {
	ctx := r.Context()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "BeginTx", Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	qtx := sqlcdb.New(tx)

	var trip sqlcdb.Trip
	if orgID.Valid {
		trip, err = qtx.GetOrgTrip(ctx, sqlcdb.GetOrgTripParams{ID: tripID, OrgID: orgID})
	} else {
		trip, err = qtx.GetTrip(ctx, sqlcdb.GetTripParams{ID: tripID, OwnerID: userID})
	}
	if err != nil {
		return sqlcdb.Voyage{}, err
	}
	if trip.Status != sqlcdb.TripStatusPlanned {
		return sqlcdb.Voyage{}, sql.ErrNoRows
	}

	voyage, err := createVoyageFromTrip(ctx, qtx, orgID, trip, req)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "CreateVoyageFromTrip", Err: err}
	}

	if err := qtx.RepointCrewAssignmentsToVoyage(ctx, sqlcdb.RepointCrewAssignmentsToVoyageParams{
		VoyageID: types.NullInt64{Int64: voyage.ID, Valid: true},
		TripID:   types.NullInt64{Int64: trip.ID, Valid: true},
	}); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "RepointCrewAssignmentsToVoyage", Err: err}
	}

	if err := qtx.DeleteTripEnrollmentsForTrip(ctx, trip.ID); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "DeleteTripEnrollmentsForTrip", Err: err}
	}

	if orgID.Valid {
		if err := qtx.DeleteOrgTrip(ctx, sqlcdb.DeleteOrgTripParams{ID: trip.ID, OrgID: orgID}); err != nil {
			return sqlcdb.Voyage{}, &QueryError{Op: "DeleteOrgTrip", Err: err}
		}
	} else {
		if err := qtx.DeleteTrip(ctx, sqlcdb.DeleteTripParams{ID: trip.ID, OwnerID: userID}); err != nil {
			return sqlcdb.Voyage{}, &QueryError{Op: "DeleteTrip", Err: err}
		}
	}

	if err := tx.Commit(); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "Commit", Err: err}
	}
	return voyage, nil
}

// createVoyageFromTrip inserts the voyage row for a completed trip, choosing the
// org-scoped or owner-scoped query based on orgID.Valid. The voyage year falls
// back to the trip embark date when the request omits it.
func createVoyageFromTrip(ctx context.Context, qtx *sqlcdb.Queries, orgID types.NullInt64, trip sqlcdb.Trip, req completeTripRequest) (sqlcdb.Voyage, error) {
	year := req.Year
	if year == nil && trip.EmbarkDate.Valid {
		if t, perr := time.Parse(time.DateOnly, trip.EmbarkDate.String); perr == nil {
			y := int64(t.Year())
			year = &y
		}
	}

	if orgID.Valid {
		return qtx.CreateOrgVoyage(ctx, sqlcdb.CreateOrgVoyageParams{
			OwnerID:       trip.OwnerID,
			OrgID:         orgID,
			CruiseID:      trip.CruiseID,
			Name:          trip.Name,
			Year:          nullInt64(year),
			EmbarkDate:    trip.EmbarkDate,
			DisembarkDate: trip.DisembarkDate,
			Countries:     trip.Countries,
			StartPort:     trip.StartPort,
			EndPort:       trip.EndPort,
			CaptainName:   trip.CaptainName,
			YachtID:       trip.YachtID,
			HoursTotal:    valOrZeroFloat(req.HoursTotal),
			HoursSail:     valOrZeroFloat(req.HoursSail),
			HoursEngine:   valOrZeroFloat(req.HoursEngine),
			HoursOver6bf:  valOrZeroFloat(req.HoursOver6bf),
			Miles:         valOrZeroFloat(req.Miles),
			Days:          valOrZeroInt(req.Days),
			TidalWaters:   valOrZeroInt(req.TidalWaters),
			CostTotal:     trip.CostTotal,
			CostPerPerson: trip.CostPerPerson,
			ImageLogoUrl:  trip.ImageLogoUrl,
			ImagePhotoUrl: trip.ImagePhotoUrl,
			ImageRouteUrl: trip.ImageRouteUrl,
			Description:   trip.Description,
		})
	}
	return qtx.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		OwnerID:       trip.OwnerID,
		Name:          trip.Name,
		Year:          nullInt64(year),
		EmbarkDate:    trip.EmbarkDate,
		DisembarkDate: trip.DisembarkDate,
		Countries:     trip.Countries,
		StartPort:     trip.StartPort,
		EndPort:       trip.EndPort,
		CaptainName:   trip.CaptainName,
		YachtID:       trip.YachtID,
		HoursTotal:    valOrZeroFloat(req.HoursTotal),
		HoursSail:     valOrZeroFloat(req.HoursSail),
		HoursEngine:   valOrZeroFloat(req.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(req.HoursOver6bf),
		Miles:         valOrZeroFloat(req.Miles),
		Days:          valOrZeroInt(req.Days),
		TidalWaters:   valOrZeroInt(req.TidalWaters),
		CostTotal:     trip.CostTotal,
		CostPerPerson: trip.CostPerPerson,
		ImageLogoUrl:  trip.ImageLogoUrl,
		ImagePhotoUrl: trip.ImagePhotoUrl,
		ImageRouteUrl: trip.ImageRouteUrl,
		Description:   trip.Description,
	})
}
