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
	captainID, captainName, err := captainAssignment(ctx, h.q, types.NullInt64{}, in.Body.CaptainUserID, in.Body.CaptainName)
	if err != nil {
		return nil, err
	}
	maxCrew, err := defaultMaxCrew(ctx, h.q, in.Body.YachtID, in.Body.MaxCrew)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	trip, err := h.q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CreatedBy:              types.NullInt64{Int64: user.UserID, Valid: true},
		CruiseID:               nullInt64(in.Body.CruiseID),
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		CaptainName:            captainName,
		CaptainUserID:          captainID,
		YachtID:                nullInt64(in.Body.YachtID),
		CostTotal:              nullFloat64(in.Body.CostTotal),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		MaxCrew:                maxCrew,
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		Description:            nullString(in.Body.Description),
		Status:                 sqlcdb.TripStatusPlanned,
	})
	if err != nil {
		slog.Error("create trip", "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

// defaultMaxCrew returns the trip's crew limit: the explicit max_crew when
// given, otherwise the yacht's berth count when the yacht records one. A
// max_crew of 0 means "no limit" in the trip forms, so it also falls back.
func defaultMaxCrew(ctx context.Context, q sqlcdb.Querier, yachtID, maxCrew *int64) (types.NullInt64, error) {
	if (maxCrew != nil && *maxCrew != 0) || yachtID == nil {
		return nullInt64(maxCrew), nil
	}
	yacht, err := q.GetYacht(ctx, *yachtID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.NullInt64{}, huma.Error422UnprocessableEntity("yacht not found")
		}
		slog.Error("get yacht for trip crew limit", "yacht_id", *yachtID, "err", err)
		return types.NullInt64{}, huma.Error500InternalServerError("failed to create trip")
	}
	return yacht.Berths, nil
}

func (h *TripHandler) update(ctx context.Context, in *updateTripInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	current, err := h.q.GetTrip(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for update", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update trip")
	}
	captainID, captainName, err := captainAssignment(ctx, h.q, current.CaptainUserID, in.Body.CaptainUserID, in.Body.CaptainName)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateTrip(ctx, sqlcdb.UpdateTripParams{
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		CaptainName:            captainName,
		CaptainUserID:          captainID,
		YachtID:                nullInt64(in.Body.YachtID),
		CostTotal:              nullFloat64(in.Body.CostTotal),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		MaxCrew:                nullInt64(in.Body.MaxCrew),
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		Description:            nullString(in.Body.Description),
		CruiseID:               nullInt64(in.Body.CruiseID),
		ID:                     in.ID,
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
		if errors.Is(err, errTripCruiseChanged) {
			return nil, huma.Error409Conflict("trip moved to another cruise; retry")
		}
		slog.Error("complete trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to complete trip")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

// errTripCruiseChanged reports that the trip moved to another cruise between
// the cruise lock and the trip lock on every completion attempt.
var errTripCruiseChanged = errors.New("trip cruise changed during completion")

// completeTripAttempts bounds the retries when the trip's cruise changes
// between the cruise lock and the trip lock.
const completeTripAttempts = 3

// completeTripTx wraps the trip → voyage transition: it creates the voyage,
// copies ports, repoints crew assignments, drops enrollments and deletes the
// trip — all in one transaction.
func completeTripTx(ctx context.Context, db *sql.DB, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	return completeTripTxWith(ctx, db, nil, tripID, req)
}

// completeTripTxWith runs completeTripTx with an optional querier wrapper,
// which tests use to pause the transaction between statements.
func completeTripTxWith(ctx context.Context, db *sql.DB, wrap func(sqlcdb.Querier) sqlcdb.Querier, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	for range completeTripAttempts {
		voyage, err := completeTripOnce(ctx, db, wrap, tripID, req)
		if !errors.Is(err, errTripCruiseChanged) {
			return voyage, err
		}
	}
	return sqlcdb.Voyage{}, errTripCruiseChanged
}

func completeTripOnce(ctx context.Context, db *sql.DB, wrap func(sqlcdb.Querier) sqlcdb.Querier, tripID int64, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return sqlcdb.Voyage{}, &QueryError{Op: "BeginTx", Err: err}
	}
	defer func() { _ = tx.Rollback() }()

	var qtx sqlcdb.Querier = sqlcdb.New(tx)
	if wrap != nil {
		qtx = wrap(qtx)
	}

	// Lock the cruise, then the trip, before any write. DELETE FROM cruises
	// takes the same order (cruise, then trip via ON DELETE SET NULL), and a
	// concurrent leg enrollment share-locks the trip; any other order
	// deadlocks with one of them.
	cruiseID, err := qtx.LockTripCruise(ctx, tripID)
	locked := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return sqlcdb.Voyage{}, &QueryError{Op: "LockTripCruise", Err: err}
	}
	trip, err := qtx.GetTripForUpdate(ctx, tripID)
	if err != nil {
		return sqlcdb.Voyage{}, err
	}
	// A trip without a cruise needs no cruise lock. One that moved to a
	// cruise other than the locked one must retry, since locking the new
	// cruise now would invert the order.
	if trip.CruiseID.Valid && (!locked || trip.CruiseID.Int64 != cruiseID) {
		return sqlcdb.Voyage{}, errTripCruiseChanged
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
func createVoyageFromTrip(ctx context.Context, qtx sqlcdb.Querier, trip sqlcdb.Trip, req dto.CompleteTripBody) (sqlcdb.Voyage, error) {
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
		CaptainUserID: trip.CaptainUserID,
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
