package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
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

// completeTripRequest aliases the DTO body so the trip → voyage transaction
// helper is shared by the owner-scoped and org-scoped trip completions.
type completeTripRequest = dto.CompleteTripBody

// --- huma operation input/output types ---

type tripIDParam struct {
	ID int64 `path:"tripID" doc:"Trip ID"`
}

type createTripInput struct {
	Body dto.TripBody
}

type updateTripInput struct {
	ID   int64 `path:"tripID" doc:"Trip ID"`
	Body dto.TripBody
}

type completeTripInput struct {
	ID   int64 `path:"tripID" doc:"Trip ID"`
	Body dto.CompleteTripBody
}

type tripOutput struct {
	Body dto.Trip
}

type tripListOutput struct {
	Body dto.Page[dto.Trip]
}

// cruiseTripsOutput is the unpaginated array body for a cruise's child trips.
type cruiseTripsOutput struct {
	Body []dto.Trip
}

type voyageOutput struct {
	Body dto.Voyage
}

type noContentOutput struct{}

// RegisterTripRoutes wires the owner-scoped trip operations onto the huma API.
func RegisterTripRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	h := NewTripHandler(q, db)
	tag := []string{"Trips"}

	huma.Register(api, huma.Operation{
		OperationID: "list-trips", Method: http.MethodGet, Path: "/trips",
		Summary: "List trips", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-trip", Method: http.MethodGet, Path: "/trips/{tripID}",
		Summary: "Get a trip", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-trip", Method: http.MethodPost, Path: "/trips",
		Summary: "Create a trip", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip", Method: http.MethodPut, Path: "/trips/{tripID}",
		Summary: "Update a trip", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-trip", Method: http.MethodDelete, Path: "/trips/{tripID}",
		Summary: "Delete a trip", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "complete-trip", Method: http.MethodPost, Path: "/trips/{tripID}/complete",
		Summary: "Complete a trip into a voyage", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.complete)
	huma.Register(api, huma.Operation{
		OperationID: "cancel-trip", Method: http.MethodPost, Path: "/trips/{tripID}/cancel",
		Summary: "Cancel a trip", Tags: tag,
	}, h.cancel)
}

func (h *TripHandler) list(ctx context.Context, in *pageParams) (*tripListOutput, error) {
	user := middleware.GetUser(ctx)
	trips, err := h.q.ListTrips(ctx, sqlcdb.ListTripsParams{
		OwnerID: user.UserID,
		Limit:   in.Limit,
		Offset:  in.Offset,
	})
	if err != nil {
		slog.Error("list trips", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	total, err := h.q.CountTrips(ctx, user.UserID)
	if err != nil {
		slog.Error("count trips", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &tripListOutput{Body: dto.NewPage(dto.TripsFromDB(trips), total, in.Limit, in.Offset)}, nil
}

func (h *TripHandler) get(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	user := middleware.GetUser(ctx)
	trip, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.ID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) create(ctx context.Context, in *createTripInput) (*tripOutput, error) {
	user := middleware.GetUser(ctx)
	trip, err := h.q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		OwnerID:       user.UserID,
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		Status:        sqlcdb.TripStatusPlanned,
	})
	if err != nil {
		slog.Error("create trip", "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) update(ctx context.Context, in *updateTripInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateTrip(ctx, sqlcdb.UpdateTripParams{
		Name:          in.Body.Name,
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		MaxCrew:       nullInt64(in.Body.MaxCrew),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		ID:            in.ID,
		OwnerID:       user.UserID,
	}); err != nil {
		slog.Error("update trip", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update trip")
	}
	return &noContentOutput{}, nil
}

func (h *TripHandler) delete(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteTrip(ctx, sqlcdb.DeleteTripParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("delete trip", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete trip")
	}
	return &noContentOutput{}, nil
}

func (h *TripHandler) cancel(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	user := middleware.GetUser(ctx)
	trip, err := h.q.CancelTrip(ctx, sqlcdb.CancelTripParams{ID: in.ID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or invalid transition")
		}
		slog.Error("cancel trip", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to cancel trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) complete(ctx context.Context, in *completeTripInput) (*voyageOutput, error) {
	user := middleware.GetUser(ctx)
	voyage, err := completeTripTx(ctx, h.db, types.NullInt64{}, in.ID, user.UserID, in.Body)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or not in planned state")
		}
		slog.Error("complete trip", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to complete trip")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

// completeTripTx wraps the trip → voyage transition. If orgID.Valid, scopes by org_id;
// otherwise scopes by owner_id with org_id IS NULL.
func completeTripTx(ctx context.Context, db *sql.DB, orgID types.NullInt64, tripID, userID int64, req completeTripRequest) (sqlcdb.Voyage, error) {
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

	for i, port := range req.Ports {
		position := int64(i)
		if port.Position != nil {
			position = *port.Position
		}
		if _, err := qtx.CreateVoyagePort(ctx, sqlcdb.CreateVoyagePortParams{
			VoyageID:  voyage.ID,
			Name:      port.Name,
			Latitude:  port.Latitude,
			Longitude: port.Longitude,
			Position:  position,
		}); err != nil {
			return sqlcdb.Voyage{}, &QueryError{Op: "CreateVoyagePort", Err: err}
		}
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
