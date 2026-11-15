package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type CruiseHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

func NewCruiseHandler(q sqlcdb.Querier, db *sql.DB) *CruiseHandler {
	return &CruiseHandler{q: q, runTx: sqlTxRunner(db)}
}

type cruiseIDParam struct {
	ID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type createCruiseInput struct {
	Body dto.CruiseBody
}

type updateCruiseInput struct {
	ID   int64 `path:"cruiseID" doc:"Cruise ID"`
	Body dto.CruiseBody
}

type cruiseOutput struct {
	Body dto.Cruise
}

type cruiseListOutput struct {
	Body dto.Page[dto.Cruise]
}

type cruiseEnrollmentsParam struct {
	CruiseID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type cruiseEnrollmentParam struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
}

type cruiseEnrollmentStatusInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.EnrollmentStatusBody
}

type cruiseEnrollmentTripInput struct {
	CruiseID     int64 `path:"cruiseID" doc:"Cruise ID"`
	EnrollmentID int64 `path:"enrollmentID" doc:"Enrollment ID"`
	Body         dto.AssignTripBody
}

type cruiseEnrollmentsOutput struct {
	Body []dto.CruiseEnrollmentDetail
}

// RegisterCruiseRoutes wires the club cruise operations onto the API. Reads are
// open to any member; mutations require an admin.
func RegisterCruiseRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	registerCruiseRoutes(api, NewCruiseHandler(q, db))
}

func registerCruiseRoutes(api huma.API, h *CruiseHandler) {
	tag := []string{"Cruises"}

	huma.Register(api, huma.Operation{
		OperationID: "list-cruises", Method: http.MethodGet, Path: "/cruises",
		Summary: "List cruises", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-cruise", Method: http.MethodGet, Path: "/cruises/{cruiseID}",
		Summary: "Get a cruise", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-cruise", Method: http.MethodPost, Path: "/cruises",
		Summary: "Create a cruise edition (admin)",
		Description: "A new cruise is an edition of a series: series_id and a non-blank edition label are " +
			"required (422 otherwise). Cruises without a series come only from standalone trips (POST /trips " +
			"without cruise_id); PUT keeps both fields optional.",
		Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise", Method: http.MethodPut, Path: "/cruises/{cruiseID}",
		Summary: "Update a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise", Method: http.MethodDelete, Path: "/cruises/{cruiseID}",
		Summary: "Delete a cruise (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "generate-cruise-enroll-token", Method: http.MethodPost, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Generate a cruise enrollment token (admin)", Tags: tag,
	}, h.generateEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-cruise-enroll-token", Method: http.MethodDelete, Path: "/cruises/{cruiseID}/enroll-token",
		Summary: "Clear a cruise enrollment token (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearEnrollToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-trips", Method: http.MethodGet, Path: "/cruises/{cruiseID}/trips",
		Summary: "List a cruise's child trips", Tags: tag,
	}, h.listChildTrips)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-voyages", Method: http.MethodGet, Path: "/cruises/{cruiseID}/voyages",
		Summary: "List a cruise's child voyages", Tags: tag,
	}, h.listChildVoyages)

	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-enrollments", Method: http.MethodGet,
		Path:    "/cruises/{cruiseID}/enrollments",
		Summary: "List a cruise's enrollments (admin)", Tags: tag,
	}, h.listEnrollments)
	huma.Register(api, huma.Operation{
		OperationID: "update-cruise-enrollment-status", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/status",
		Summary: "Update a cruise enrollment's status (admin)",
		Description: "Accepting an enrollment assigned to a leg needs a free place on it: 409 when the " +
			"leg's accepted crew already fills max_crew. 404 when the enrollment belongs to another cruise.",
		Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateEnrollmentStatus)
	huma.Register(api, huma.Operation{
		OperationID: "assign-cruise-enrollment-trip", Method: http.MethodPut,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}/trip",
		Summary: "Assign a cruise enrollment to a trip (admin)",
		Description: "Moving an accepted enrollment onto a leg needs a free place on it: 409 when the " +
			"leg's accepted crew already fills max_crew. 404 when the enrollment belongs to another cruise.",
		Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.assignEnrollmentToTrip)
	huma.Register(api, huma.Operation{
		OperationID: "delete-cruise-enrollment", Method: http.MethodDelete,
		Path:    "/cruises/{cruiseID}/enrollments/{enrollmentID}",
		Summary: "Delete a cruise enrollment (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteEnrollment)
}

func (h *CruiseHandler) list(ctx context.Context, in *pageParams) (*cruiseListOutput, error) {
	var key dateKey
	ok, err := decodeCursor(in.After, &key)
	if err != nil {
		return nil, err
	}
	cruises, err := h.q.ListCruises(ctx, sqlcdb.ListCruisesParams{
		AfterID: afterID(ok, key.ID), AfterEmbarkDate: afterNullText(key.Date),
		RowLimit: fetchLimit(in.Limit), RowOffset: in.Offset,
	})
	if err != nil {
		slog.Error("list cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	total, err := h.q.CountCruises(ctx)
	if err != nil {
		slog.Error("count cruises", "err", err)
		return nil, huma.Error500InternalServerError("failed to list cruises")
	}
	return &cruiseListOutput{Body: buildPage(cruises, total, in.Limit, in.Offset, dto.CruisesFromDB,
		func(r sqlcdb.Cruise) any { return dateKey{Date: nullTextKey(r.EmbarkDate), ID: r.ID} })}, nil
}

func (h *CruiseHandler) get(ctx context.Context, in *cruiseIDParam) (*cruiseOutput, error) {
	cruise, err := h.q.GetCruise(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("cruise not found")
		}
		slog.Error("get cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) create(ctx context.Context, in *createCruiseInput) (*cruiseOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := requireEdition(&in.Body); err != nil {
		return nil, err
	}
	seriesID, err := h.resolveSeries(ctx, in.Body.SeriesID)
	if err != nil {
		return nil, err
	}
	included, notIncluded, err := cruisePriceLists(&in.Body)
	if err != nil {
		return nil, err
	}
	chatURL, err := groupChatURL(in.Body.GroupChatUrl)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	if err := retainUploadImages(ctx, h.q, in.Body.ImageLogoUrl, in.Body.ImagePhotoUrl, in.Body.ImageRouteUrl); err != nil {
		return nil, err
	}
	cruise, err := h.q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		CreatedBy:              types.NullInt64{Int64: user.UserID, Valid: true},
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		Description:            nullString(in.Body.Description),
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		MaxCrew:                nullInt64(in.Body.MaxCrew),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Kind:                   in.Body.Kind,
		SeriesID:               seriesID,
		Edition:                nullEdition(in.Body.Edition),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		Included:               included,
		NotIncluded:            notIncluded,
		GroupChatUrl:           chatURL,
		TravelNotes:            nullTrimmed(in.Body.TravelNotes),
		MinAge:                 nullInt64(in.Body.MinAge),
	})
	if err != nil {
		if isForeignKeyViolation(err, cruiseSeriesFKey) {
			return nil, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("create cruise", "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create cruise")
	}
	return &cruiseOutput{Body: dto.CruiseFromDB(cruise)}, nil
}

func (h *CruiseHandler) update(ctx context.Context, in *updateCruiseInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	seriesID, err := h.resolveSeries(ctx, in.Body.SeriesID)
	if err != nil {
		return nil, err
	}
	included, notIncluded, err := cruisePriceLists(&in.Body)
	if err != nil {
		return nil, err
	}
	chatURL, err := groupChatURL(in.Body.GroupChatUrl)
	if err != nil {
		return nil, err
	}
	if err := retainUploadImages(ctx, h.q, in.Body.ImageLogoUrl, in.Body.ImagePhotoUrl, in.Body.ImageRouteUrl); err != nil {
		return nil, err
	}
	if err := h.q.UpdateCruise(ctx, sqlcdb.UpdateCruiseParams{
		Name:                   in.Body.Name,
		EmbarkDate:             nullString(in.Body.EmbarkDate),
		DisembarkDate:          nullString(in.Body.DisembarkDate),
		Countries:              nullString(in.Body.Countries),
		StartPort:              nullString(in.Body.StartPort),
		EndPort:                nullString(in.Body.EndPort),
		Description:            nullString(in.Body.Description),
		ImageLogoUrl:           nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl:          nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl:          nullString(in.Body.ImageRouteUrl),
		MaxCrew:                nullInt64(in.Body.MaxCrew),
		CostPerPerson:          nullFloat64(in.Body.CostPerPerson),
		Kind:                   in.Body.Kind,
		SeriesID:               seriesID,
		Edition:                nullEdition(in.Body.Edition),
		Currency:               currencyOrDefault(in.Body.Currency),
		CostPerPersonNonMember: nullFloat64(in.Body.CostPerPersonNonMember),
		YachtFund:              nullFloat64(in.Body.YachtFund),
		Deposit:                nullFloat64(in.Body.Deposit),
		Included:               included,
		NotIncluded:            notIncluded,
		GroupChatUrl:           chatURL,
		TravelNotes:            nullTrimmed(in.Body.TravelNotes),
		MinAge:                 nullInt64(in.Body.MinAge),
		ID:                     in.ID,
	}); err != nil {
		if isForeignKeyViolation(err, cruiseSeriesFKey) {
			return nil, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("update cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) delete(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteCruise(ctx, in.ID); err != nil {
		if isForeignKeyViolation(err, "trips_cruise_id_fkey") || isForeignKeyViolation(err, "voyages_cruise_id_fkey") {
			return nil, huma.Error409Conflict("cruise still has trips or voyages; delete them first")
		}
		slog.Error("delete cruise", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete cruise")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) generateEnrollToken(ctx context.Context, in *cruiseIDParam) (*tokenOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)
	if err := h.q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
	}); err != nil {
		slog.Error("set cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *CruiseHandler) clearEnrollToken(ctx context.Context, in *cruiseIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.ClearCruiseEnrollToken(ctx, in.ID); err != nil {
		slog.Error("clear cruise enroll token", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) listChildTrips(ctx context.Context, in *cruiseIDParam) (*cruiseTripsOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	trips, err := h.q.ListCruiseTrips(ctx, in.ID)
	if err != nil {
		slog.Error("list cruise child trips", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trips")
	}
	return &cruiseTripsOutput{Body: dto.TripsFromDB(trips)}, nil
}

func (h *CruiseHandler) listChildVoyages(ctx context.Context, in *cruiseIDParam) (*cruiseVoyagesOutput, error) {
	if err := h.verifyCruise(ctx, in.ID); err != nil {
		return nil, err
	}
	voyages, err := h.q.ListCruiseVoyages(ctx, in.ID)
	if err != nil {
		slog.Error("list cruise child voyages", "cruise_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &cruiseVoyagesOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

func (h *CruiseHandler) listEnrollments(ctx context.Context, in *cruiseEnrollmentsParam) (*cruiseEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	enrollments, err := h.q.ListCruiseEnrollments(ctx, in.CruiseID)
	if err != nil {
		slog.Error("list cruise enrollments", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &cruiseEnrollmentsOutput{Body: dto.CruiseEnrollmentsFromDB(enrollments)}, nil
}

// errEnrollmentChanged aborts a cruise enrollment change transaction when the
// enrollment moved to another leg while its legs were being locked.
var errEnrollmentChanged = errors.New("enrollment changed")

// errLegGone aborts a capacity-checked change whose leg was completed or
// deleted meanwhile.
var errLegGone = errors.New("leg gone")

// cruiseEnrollmentTxError maps a capacity-checked cruise enrollment change
// error to its HTTP response; ok is false for unexpected errors.
func cruiseEnrollmentTxError(err error) (resp error, ok bool) {
	if errors.Is(err, errEnrollmentNotFound) {
		return huma.Error404NotFound("enrollment not found"), true
	}
	if errors.Is(err, errLegGone) {
		return huma.Error409Conflict("trip is no longer open: it was completed or deleted"), true
	}
	if errors.Is(err, errEnrollmentChanged) {
		return huma.Error409Conflict("enrollment changed meanwhile: reload and try again"), true
	}
	if full, isFull := errors.AsType[*legFullError](err); isFull {
		return huma.Error409Conflict(full.Error()), true
	}
	if errors.Is(err, errPersonChanged) || isDeadlock(err) {
		return huma.Error409Conflict(errPersonChangedMessage), true
	}
	return nil, false
}

// getCruiseEnrollment reads one enrollment of the cruise, or errEnrollmentNotFound.
func getCruiseEnrollment(ctx context.Context, q sqlcdb.Querier, cruiseID, id int64) (sqlcdb.CruiseEnrollment, error) {
	e, err := q.GetCruiseEnrollmentOfCruise(ctx, sqlcdb.GetCruiseEnrollmentOfCruiseParams{ID: id, CruiseID: cruiseID})
	if errors.Is(err, sql.ErrNoRows) {
		return e, errEnrollmentNotFound
	}
	if err != nil {
		return e, &QueryError{Op: "GetCruiseEnrollmentOfCruise", Err: err}
	}
	return e, nil
}

// updateEnrollmentStatus sets a cruise enrollment's status. Accepting one
// assigned to a leg takes one of the leg's max_crew places, checked under
// the leg's lock like a trip enrollment accept (409 when full).
func (h *CruiseHandler) updateEnrollmentStatus(ctx context.Context, in *cruiseEnrollmentStatusInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	txErr := (enrollmentOperations{runTx: h.runTx}).changeCruiseStatus(ctx, in.CruiseID, in.EnrollmentID, in.Body.Status)
	if txErr != nil {
		if resp, ok := cruiseEnrollmentTxError(txErr); ok {
			return nil, resp
		}
		slog.Error("update cruise enrollment status", "enrollment_id", in.EnrollmentID, "err", txErr)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

// assignEnrollmentToTrip moves a cruise enrollment to a leg (or off one).
// Moving an accepted enrollment onto a new leg takes one of that leg's
// max_crew places, checked under the leg's lock (409 when full).
func (h *CruiseHandler) assignEnrollmentToTrip(ctx context.Context, in *cruiseEnrollmentTripInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	txErr := (enrollmentOperations{runTx: h.runTx}).assignCruiseEnrollment(ctx, in.CruiseID, in.EnrollmentID, in.Body.TripID)
	if txErr != nil {
		if isForeignKeyViolation(txErr, "cruise_enrollments_trip_id_fkey") {
			return nil, huma.Error409Conflict("trip is no longer open: it was completed or deleted")
		}
		if resp, ok := cruiseEnrollmentTxError(txErr); ok {
			return nil, resp
		}
		slog.Error("assign cruise enrollment to trip", "enrollment_id", in.EnrollmentID, "err", txErr)
		return nil, huma.Error500InternalServerError("failed to assign enrollment")
	}
	return &noContentOutput{}, nil
}

func (h *CruiseHandler) deleteEnrollment(ctx context.Context, in *cruiseEnrollmentParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	txErr := (enrollmentOperations{runTx: h.runTx}).deleteCruiseEnrollment(ctx, in.CruiseID, in.EnrollmentID)
	if txErr != nil {
		if resp, ok := cruiseEnrollmentTxError(txErr); ok {
			return nil, resp
		}
		slog.Error("delete cruise enrollment", "enrollment_id", in.EnrollmentID, "err", txErr)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}

// verifyCruise confirms the cruise exists.
func (h *CruiseHandler) verifyCruise(ctx context.Context, cruiseID int64) error {
	if _, err := h.q.GetCruise(ctx, cruiseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise", "cruise_id", cruiseID, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}

// cruiseSeriesFKey is the PostgreSQL name of the cruises.series_id foreign key.
const cruiseSeriesFKey = "cruises_series_id_fkey"

// resolveSeries checks that an optional series exists. A missing series is a
// client error (422), not a server failure.
func (h *CruiseHandler) resolveSeries(ctx context.Context, id *int64) (types.NullInt64, error) {
	if id == nil {
		return types.NullInt64{}, nil
	}
	if _, err := h.q.GetCruiseSeries(ctx, *id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.NullInt64{}, huma.Error422UnprocessableEntity("series not found")
		}
		slog.Error("verify series", "series_id", *id, "err", err)
		return types.NullInt64{}, huma.Error500InternalServerError("failed to verify series")
	}
	return types.NullInt64{Int64: *id, Valid: true}, nil
}

// requireEdition rejects a new cruise that is not an edition of a series: it
// needs series_id and a non-blank edition label.
func requireEdition(b *dto.CruiseBody) error {
	if b.SeriesID == nil {
		return huma.Error422UnprocessableEntity("series_id is required: create an edition from its series")
	}
	if !nullEdition(b.Edition).Valid {
		return huma.Error422UnprocessableEntity("edition is required")
	}
	return nil
}

// nullEdition stores a blank edition label as null.
func nullEdition(s *string) types.NullString {
	return nullTrimmed(s)
}

// nullTrimmed trims surrounding whitespace and stores a blank value as null.
func nullTrimmed(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return types.NullString{}
	}
	return types.NullString{String: v, Valid: true}
}
