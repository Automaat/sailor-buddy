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

// RegisterTripRoutes wires the club trip operations onto the huma API. Reads
// are open to any member; mutations require an admin.
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
		Summary: "Create a trip (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip", Method: http.MethodPut, Path: "/trips/{tripID}",
		Summary: "Update a trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-trip", Method: http.MethodDelete, Path: "/trips/{tripID}",
		Summary: "Delete a trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "complete-trip", Method: http.MethodPost, Path: "/trips/{tripID}/complete",
		Summary: "Complete a trip into a voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.complete)
	huma.Register(api, huma.Operation{
		OperationID: "cancel-trip", Method: http.MethodPost, Path: "/trips/{tripID}/cancel",
		Summary: "Cancel a trip (admin)", Tags: tag,
	}, h.cancel)
}

func (h *TripHandler) list(ctx context.Context, in *pageParams) (*tripListOutput, error) {
	trips, err := h.q.ListTrips(ctx, sqlcdb.ListTripsParams{Limit: in.Limit, Offset: in.Offset})
	if err != nil {
		slog.Error("list trips", "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	total, err := h.q.CountTrips(ctx)
	if err != nil {
		slog.Error("count trips", "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &tripListOutput{Body: dto.NewPage(dto.TripsFromDB(trips), total, in.Limit, in.Offset)}, nil
}

func (h *TripHandler) get(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	trip, err := h.q.GetTrip(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) create(ctx context.Context, in *createTripInput) (*tripOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	trip, err := h.q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CreatedBy:     types.NullInt64{Int64: user.UserID, Valid: true},
		CruiseID:      nullInt64(in.Body.CruiseID),
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
		slog.Error("create trip", "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) update(ctx context.Context, in *updateTripInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
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
		CruiseID:      nullInt64(in.Body.CruiseID),
		ID:            in.ID,
	}); err != nil {
		slog.Error("update trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update trip")
	}
	return &noContentOutput{}, nil
}

func (h *TripHandler) delete(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteTrip(ctx, in.ID); err != nil {
		slog.Error("delete trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete trip")
	}
	return &noContentOutput{}, nil
}

func (h *TripHandler) cancel(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	trip, err := h.q.CancelTrip(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or invalid transition")
		}
		slog.Error("cancel trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to cancel trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

func (h *TripHandler) complete(ctx context.Context, in *completeTripInput) (*voyageOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	voyage, err := completeTripTx(ctx, h.db, in.ID, in.Body)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found or not in planned state")
		}
		slog.Error("complete trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to complete trip")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

// completeTripTx wraps the trip → voyage transition: it creates the voyage,
// copies ports, repoints crew assignments, drops enrollments and deletes the
// trip — all in one transaction.
func completeTripTx(ctx context.Context, db *sql.DB, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "BeginTx", Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	qtx := sqlcdb.New(tx)

	trip, err := qtx.GetTrip(ctx, tripID)
	if err != nil {
		return sqlcdb.Voyage{}, err
	}
	if trip.Status != sqlcdb.TripStatusPlanned {
		return sqlcdb.Voyage{}, sql.ErrNoRows
	}

	voyage, err := createVoyageFromTrip(ctx, qtx, trip, req)
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

	if err := qtx.DeleteTrip(ctx, trip.ID); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "DeleteTrip", Err: err}
	}

	if err := tx.Commit(); err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "Commit", Err: err}
	}
	return voyage, nil
}

// createVoyageFromTrip inserts the voyage row for a completed trip. The voyage
// year falls back to the trip embark date when the request omits it.
func createVoyageFromTrip(ctx context.Context, qtx *sqlcdb.Queries, trip sqlcdb.Trip, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	year := req.Year
	if year == nil && trip.EmbarkDate.Valid {
		if t, perr := time.Parse(time.DateOnly, trip.EmbarkDate.String); perr == nil {
			y := int64(t.Year())
			year = &y
		}
	}

	return qtx.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		CreatedBy:     trip.CreatedBy,
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
