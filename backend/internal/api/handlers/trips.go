package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
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
	q     sqlcdb.Querier
	db    *sql.DB
	runTx txRunner
}

func NewTripHandler(q sqlcdb.Querier, db *sql.DB) *TripHandler {
	return &TripHandler{q: q, db: db, runTx: sqlTxRunner(db)}
}

// --- huma operation input/output types ---

type tripIDParam struct {
	ID int64 `path:"tripID" doc:"Trip ID"`
}

type createTripInput struct {
	Body dto.CreateTripBody
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
	registerTripRoutes(api, NewTripHandler(q, db))
}

func registerTripRoutes(api huma.API, h *TripHandler) {
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
		Summary: "Create a trip (admin)",
		Description: "Without cruise_id the trip is a standalone etap: one transaction creates a wrapper " +
			"cruise without a series (name, dates, ports, countries, description, crew limit and prices " +
			"copied from the trip, kind sea) and the trip in it.",
		Tags: tag, DefaultStatus: http.StatusCreated,
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
	var key dateKey
	ok, err := decodeCursor(in.After, &key)
	if err != nil {
		return nil, err
	}
	hidden := canSeeHidden(ctx)
	trips, err := h.q.ListTrips(ctx, sqlcdb.ListTripsParams{
		IncludeHidden: hidden,
		AfterID:       afterID(ok, key.ID), AfterEmbarkDate: afterNullText(key.Date),
		RowLimit: fetchLimit(in.Limit), RowOffset: in.Offset,
	})
	if err != nil {
		slog.Error("list trips", "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	total, err := h.q.CountTrips(ctx, hidden)
	if err != nil {
		slog.Error("count trips", "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &tripListOutput{Body: buildPage(trips, total, in.Limit, in.Offset, dto.TripsFromDB,
		func(r sqlcdb.Trip) any { return dateKey{Date: nullTextKey(r.EmbarkDate), ID: r.ID} })}, nil
}

func (h *TripHandler) get(ctx context.Context, in *tripIDParam) (*tripOutput, error) {
	trip, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.ID, IncludeHidden: canSeeHidden(ctx)})
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
	b := &in.Body
	if b.CruiseID != nil {
		if err := requireParentCruise(ctx, h.q, *b.CruiseID); err != nil {
			return nil, err
		}
	}
	captainID, captainName, err := captainAssignment(ctx, h.q, b.CaptainPersonID, b.CaptainName)
	if err != nil {
		return nil, err
	}
	if err := checkSignUpDeadline(&b.TripFields); err != nil {
		return nil, err
	}
	maxCrew, err := defaultMaxCrew(ctx, h.q, b.YachtID, b.MaxCrew)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	if err := retainUploadImages(ctx, h.q, b.ImageLogoUrl, b.ImagePhotoUrl, b.ImageRouteUrl); err != nil {
		return nil, err
	}
	createdBy := types.NullInt64{Int64: user.UserID, Valid: true}
	params := sqlcdb.CreateTripParams{
		CreatedBy:              createdBy,
		Name:                   b.Name,
		EmbarkDate:             nullString(b.EmbarkDate),
		DisembarkDate:          nullString(b.DisembarkDate),
		Countries:              nullString(b.Countries),
		StartPort:              nullString(b.StartPort),
		EndPort:                nullString(b.EndPort),
		CaptainName:            captainName,
		CaptainPersonID:        captainID,
		YachtID:                nullInt64(b.YachtID),
		CostTotal:              nullFloat64(b.CostTotal),
		CostPerPerson:          nullFloat64(b.CostPerPerson),
		Currency:               currencyOrDefault(b.Currency),
		CostPerPersonNonMember: nullFloat64(b.CostPerPersonNonMember),
		YachtFund:              nullFloat64(b.YachtFund),
		Deposit:                nullFloat64(b.Deposit),
		MaxCrew:                maxCrew,
		ImageLogoUrl:           nullString(b.ImageLogoUrl),
		ImagePhotoUrl:          nullString(b.ImagePhotoUrl),
		ImageRouteUrl:          nullString(b.ImageRouteUrl),
		Description:            nullString(b.Description),
		Status:                 sqlcdb.TripStatusPlanned,
		SignUpDeadline:         nullString(b.SignUpDeadline),
	}
	var trip sqlcdb.Trip
	if b.CruiseID != nil {
		params.CruiseID = *b.CruiseID
		trip, err = h.q.CreateTrip(ctx, params)
	} else {
		trip, err = createStandaloneTrip(ctx, h.runTx, params)
	}
	if err != nil {
		if isForeignKeyViolation(err, "trips_captain_person_id_fkey") {
			return nil, captainPersonNotFound()
		}
		if isForeignKeyViolation(err, "trips_yacht_id_fkey") {
			return nil, yachtNotFound()
		}
		slog.Error("create trip", "name", b.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create trip")
	}
	return &tripOutput{Body: dto.TripFromDB(trip)}, nil
}

// createStandaloneTrip creates a trip without a chosen cruise: in one
// transaction it creates a wrapper cruise without a series from the trip's
// fields, then the trip in it. A failed trip insert rolls the cruise back.
func createStandaloneTrip(ctx context.Context, runTx txRunner, trip sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
	var created sqlcdb.Trip
	err := runTx(ctx, func(q sqlcdb.Querier) error {
		cruise, err := q.CreateCruise(ctx, wrapperCruiseParams(trip))
		if err != nil {
			return &QueryError{Op: "CreateCruise", Err: err}
		}
		trip.CruiseID = cruise.ID
		created, err = q.CreateTrip(ctx, trip)
		if err != nil {
			return &QueryError{Op: "CreateTrip", Err: err}
		}
		return nil
	})
	return created, err
}

// cruiseKindSea is the kind of a standalone trip's wrapper cruise.
const cruiseKindSea = "sea"

// wrapperCruiseParams is the cruise a standalone trip gets: no series, kind
// sea, and the trip's name, dates, ports, countries, description, crew limit
// and prices.
func wrapperCruiseParams(t sqlcdb.CreateTripParams) sqlcdb.CreateCruiseParams {
	return sqlcdb.CreateCruiseParams{
		CreatedBy:              t.CreatedBy,
		Name:                   t.Name,
		EmbarkDate:             t.EmbarkDate,
		DisembarkDate:          t.DisembarkDate,
		Countries:              t.Countries,
		StartPort:              t.StartPort,
		EndPort:                t.EndPort,
		Description:            t.Description,
		MaxCrew:                t.MaxCrew,
		CostPerPerson:          t.CostPerPerson,
		Kind:                   cruiseKindSea,
		Currency:               t.Currency,
		CostPerPersonNonMember: t.CostPerPersonNonMember,
		YachtFund:              t.YachtFund,
		Deposit:                t.Deposit,
		Included:               json.RawMessage("[]"),
		NotIncluded:            json.RawMessage("[]"),
	}
}

// checkSignUpDeadline rejects a sign-up deadline after the leg's embark
// date: sign-ups must close before the leg sails. huma already enforces
// the YYYY-MM-DD format; a leg without a parseable embark date takes any
// deadline.
func checkSignUpDeadline(b *dto.TripFields) error {
	if b.SignUpDeadline == nil || b.EmbarkDate == nil {
		return nil
	}
	deadline, err := time.Parse(time.DateOnly, *b.SignUpDeadline)
	if err != nil {
		return huma.Error422UnprocessableEntity("sign_up_deadline must be a YYYY-MM-DD date")
	}
	embark, embarkErr := time.Parse(time.DateOnly, *b.EmbarkDate)
	if embarkErr == nil && deadline.After(embark) {
		return huma.Error422UnprocessableEntity("sign_up_deadline must not be after embark_date")
	}
	return nil
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
			return types.NullInt64{}, yachtNotFound()
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
	if err := requireParentCruise(ctx, h.q, in.Body.CruiseID); err != nil {
		return nil, err
	}
	if err := checkSignUpDeadline(&in.Body.TripFields); err != nil {
		return nil, err
	}
	current, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.ID, IncludeHidden: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for update", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update trip")
	}
	if in.Body.CruiseID != current.CruiseID {
		linked, err := h.q.CountTripLinkedEnrollments(ctx, in.ID)
		if err != nil {
			slog.Error("count trip enrollments for cruise move", "trip_id", in.ID, "err", err)
			return nil, huma.Error500InternalServerError("failed to update trip")
		}
		if linked > 0 {
			return nil, huma.Error409Conflict("trip has enrollments; remove them before moving it to another cruise")
		}
	}
	captainID, captainName, err := captainAssignment(ctx, h.q, in.Body.CaptainPersonID, in.Body.CaptainName)
	if err != nil {
		return nil, err
	}
	if err := retainUploadImages(ctx, h.q, in.Body.ImageLogoUrl, in.Body.ImagePhotoUrl, in.Body.ImageRouteUrl); err != nil {
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
		CaptainPersonID:        captainID,
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
		SignUpDeadline:         nullString(in.Body.SignUpDeadline),
		CruiseID:               in.Body.CruiseID,
		ID:                     in.ID,
	}); err != nil {
		if isForeignKeyViolation(err, "trips_captain_person_id_fkey") {
			return nil, captainPersonNotFound()
		}
		if isForeignKeyViolation(err, "trips_yacht_id_fkey") {
			return nil, yachtNotFound()
		}
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
		if errors.Is(err, errTripCaptainChanged) {
			return nil, huma.Error409Conflict("trip captain changed; retry")
		}
		if errors.Is(err, errTripYachtChanged) {
			return nil, huma.Error409Conflict("trip yacht changed; retry")
		}
		slog.Error("complete trip", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to complete trip")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}
