package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type EnrollmentHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

func NewEnrollmentHandler(q sqlcdb.Querier, db *sql.DB) *EnrollmentHandler {
	return &EnrollmentHandler{q: q, runTx: sqlTxRunner(db)}
}

type tokenPathParam struct {
	Token string `path:"token" doc:"Enrollment share token"`
}

type enrollInput struct {
	Token string `path:"token" doc:"Enrollment share token"`
	Body  dto.EnrollBody
}

type enrollInfoOutput struct {
	Body dto.EnrollInfo
}

type enrollmentOutput struct {
	Body dto.Enrollment
}

type enrollLegsInput struct {
	Token string `path:"token" doc:"Cruise enrollment share token"`
	Body  dto.EnrollLegsBody
}

type enrollLegsOutput struct {
	Body dto.EnrollLegsResponse
}

type tripEnrollmentsOutput struct {
	Body []dto.TripEnrollmentDetail
}

type tokenOutput struct {
	Body struct {
		Token string `json:"token"`
	}
}

type tripEnrollmentStatusInput struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	ID     int64 `path:"id" doc:"Enrollment ID"`
	Body   dto.EnrollmentStatusBody
}

type tripEnrollmentParam struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	ID     int64 `path:"id" doc:"Enrollment ID"`
}

// RegisterEnrollmentRoutes wires the share-token enrollment flow and the
// admin-side trip enrollment management onto the API.
func RegisterEnrollmentRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	registerEnrollmentRoutes(api, NewEnrollmentHandler(q, db))
}

func registerEnrollmentRoutes(api huma.API, h *EnrollmentHandler) {
	tag := []string{"Enrollment"}

	huma.Register(api, huma.Operation{
		OperationID: "resolve-enroll-token", Method: http.MethodGet, Path: "/enroll/{token}",
		Summary: "Resolve an enrollment token to its trip or cruise", Tags: tag,
	}, h.getByToken)
	huma.Register(api, huma.Operation{
		OperationID: "enroll", Method: http.MethodPost, Path: "/enroll/{token}",
		Summary: "Self-enroll via a share token", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.enroll)
	huma.Register(api, huma.Operation{
		OperationID: "enroll-cruise-legs", Method: http.MethodPost, Path: "/enroll/{token}/legs",
		Summary: "Self-enroll in one or more legs (trips) of a cruise via its share token",
		Description: "Creates one pending trip enrollment per picked leg. Legs the caller is already " +
			"enrolled in are reported as already_enrolled. If any picked trip is not a planned trip " +
			"of the token's cruise, the request fails with 422 and nothing is written.",
		Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.enrollLegs)

	huma.Register(api, huma.Operation{
		OperationID: "generate-trip-enroll-token", Method: http.MethodPost, Path: "/trips/{tripID}/enroll-token",
		Summary: "Generate a trip enrollment share token (admin)", Tags: tag,
	}, h.generateToken)
	huma.Register(api, huma.Operation{
		OperationID: "clear-trip-enroll-token", Method: http.MethodDelete, Path: "/trips/{tripID}/enroll-token",
		Summary: "Clear a trip enrollment share token (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.clearToken)
	huma.Register(api, huma.Operation{
		OperationID: "list-trip-enrollments", Method: http.MethodGet, Path: "/trips/{tripID}/enrollments",
		Summary: "List a trip's enrollments", Tags: tag,
	}, h.listEnrollments)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip-enrollment-status", Method: http.MethodPut, Path: "/trips/{tripID}/enrollments/{id}/status",
		Summary: "Update a trip enrollment's status (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateStatus)
	huma.Register(api, huma.Operation{
		OperationID: "delete-trip-enrollment", Method: http.MethodDelete, Path: "/trips/{tripID}/enrollments/{id}",
		Summary: "Delete a trip enrollment (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteEnrollment)
}

// getByToken resolves a share token, which may belong to a trip or a cruise.
func (h *EnrollmentHandler) getByToken(ctx context.Context, in *tokenPathParam) (*enrollInfoOutput, error) {
	user := middleware.GetUser(ctx)
	tok := types.NullString{String: in.Token, Valid: true}

	trip, terr := h.q.GetTripByEnrollToken(ctx, tok)
	if terr == nil {
		counts, _ := h.q.CountTripEnrollments(ctx, trip.ID)
		info := dto.EnrollInfo{
			Kind:          "trip",
			Trip:          new(dto.EnrollTripFromRow(trip)),
			AcceptedCount: counts.Accepted,
			TotalCount:    counts.Total,
		}
		if accepted, err := h.q.CountAcceptedForTrip(ctx, trip.ID); err == nil {
			info.TripAccepted = []dto.TripAccepted{{TripID: trip.ID, AcceptedCount: accepted}}
		} else {
			slog.Error("count accepted for trip", "trip_id", trip.ID, "err", err)
		}
		if user != nil {
			if e, err := h.q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: trip.ID, UserID: user.UserID}); err == nil {
				info.Enrolled = true
				info.Enrollment = new(dto.TripEnrollmentToDTO(e))
			}
		}
		return &enrollInfoOutput{Body: info}, nil
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token", "err", terr)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	cruise, cerr := h.q.GetCruiseByEnrollToken(ctx, tok)
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid enrollment link")
		}
		slog.Error("get cruise by enroll token", "err", cerr)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	counts, _ := h.q.CountCruiseEnrollments(ctx, cruise.ID)
	childTrips, tlistErr := h.q.ListCruiseTrips(ctx, types.NullInt64{Int64: cruise.ID, Valid: true})
	if tlistErr != nil {
		slog.Error("list cruise trips", "cruise_id", cruise.ID, "err", tlistErr)
		childTrips = nil
	}
	tripAccepted, taErr := h.q.CountAcceptedByCruiseTrip(ctx, types.NullInt64{Int64: cruise.ID, Valid: true})
	if taErr != nil {
		slog.Error("count accepted per cruise trip", "cruise_id", cruise.ID, "err", taErr)
		tripAccepted = nil
	}
	info := dto.EnrollInfo{
		Kind:          "cruise",
		Cruise:        new(dto.EnrollCruiseFromRow(cruise, user != nil)),
		Trips:         dto.TripsFromDB(childTrips),
		AcceptedCount: counts.Accepted,
		TotalCount:    counts.Total,
		TripAccepted:  dto.TripAcceptedFromDB(tripAccepted),
	}
	if user != nil {
		if e, err := h.q.GetUserCruiseEnrollment(ctx, sqlcdb.GetUserCruiseEnrollmentParams{CruiseID: cruise.ID, UserID: user.UserID}); err == nil {
			info.Enrolled = true
			info.Enrollment = new(dto.CruiseEnrollmentToDTO(e))
		}
		legs, lerr := h.q.ListUserCruiseTripEnrollments(ctx, sqlcdb.ListUserCruiseTripEnrollmentsParams{
			CruiseID: types.NullInt64{Int64: cruise.ID, Valid: true},
			UserID:   user.UserID,
		})
		if lerr != nil {
			slog.Error("list user cruise leg enrollments", "cruise_id", cruise.ID, "user_id", user.UserID, "err", lerr)
			return nil, huma.Error500InternalServerError("failed to get leg enrollments")
		}
		if len(legs) > 0 {
			info.Enrolled = true
			info.LegEnrollments = dto.TripEnrollmentsToDTO(legs)
		}
	}
	return &enrollInfoOutput{Body: info}, nil
}

// enroll self-enrolls the caller into the trip or cruise the token resolves to.
func (h *EnrollmentHandler) enroll(ctx context.Context, in *enrollInput) (*enrollmentOutput, error) {
	user := middleware.GetUser(ctx)
	tok := types.NullString{String: in.Token, Valid: true}

	trip, terr := h.q.GetTripByEnrollToken(ctx, tok)
	if terr == nil {
		status, _ := h.q.GetTripStatus(ctx, trip.ID)
		if status != sqlcdb.TripStatusPlanned {
			return nil, huma.Error409Conflict("enrollment closed: trip is not planned")
		}
		e, err := h.q.CreateTripEnrollment(ctx, sqlcdb.CreateTripEnrollmentParams{
			TripID: trip.ID,
			UserID: user.UserID,
			Note:   nullString(in.Body.Note),
		})
		if err != nil {
			if isUniqueViolation(err) {
				return nil, huma.Error409Conflict("already enrolled")
			}
			if isForeignKeyViolation(err, tripEnrollmentTripFKey) {
				return nil, huma.Error409Conflict("enrollment closed: trip is not planned")
			}
			slog.Error("create trip enrollment", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
			return nil, huma.Error500InternalServerError("failed to enroll")
		}
		return &enrollmentOutput{Body: dto.TripEnrollmentToDTO(e)}, nil
	}
	if !errors.Is(terr, sql.ErrNoRows) {
		slog.Error("get trip by enroll token for enroll", "err", terr)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	cruise, cerr := h.q.GetCruiseByEnrollToken(ctx, tok)
	if cerr != nil {
		if errors.Is(cerr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid enrollment link")
		}
		slog.Error("get cruise by enroll token for enroll", "err", cerr)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}
	e, err := h.q.CreateCruiseEnrollment(ctx, sqlcdb.CreateCruiseEnrollmentParams{
		CruiseID: cruise.ID,
		UserID:   user.UserID,
		Note:     nullString(in.Body.Note),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("already enrolled")
		}
		slog.Error("create cruise enrollment", "cruise_id", cruise.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to enroll")
	}
	return &enrollmentOutput{Body: dto.CruiseEnrollmentToDTO(e)}, nil
}

// invalidLegsError aborts the leg enrollment transaction when a picked trip is
// not a planned trip of the cruise.
type invalidLegsError struct{ msg string }

func (e *invalidLegsError) Error() string { return e.msg }

// enrollLegs enrolls the caller in each picked leg (trip) of the token's
// cruise. All legs are validated before any write, and the writes share one
// transaction, so an invalid pick leaves the database untouched.
func (h *EnrollmentHandler) enrollLegs(ctx context.Context, in *enrollLegsInput) (*enrollLegsOutput, error) {
	user := middleware.GetUser(ctx)
	if user == nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	cruise, err := h.q.GetCruiseByEnrollToken(ctx, types.NullString{String: in.Token, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid enrollment link")
		}
		slog.Error("get cruise by enroll token for leg enroll", "err", err)
		return nil, huma.Error500InternalServerError("failed to get cruise")
	}

	if len(in.Body.TripIDs) == 0 {
		return nil, huma.Error422UnprocessableEntity("pick at least one leg")
	}

	// Inserts run in ascending trip order so concurrent requests from one
	// user take the unique-index locks in the same order and cannot deadlock.
	insertOrder := slices.Clone(in.Body.TripIDs)
	slices.Sort(insertOrder)
	byTrip := make(map[int64]dto.EnrollLegResult, len(insertOrder))
	txErr := h.runTx(ctx, func(q sqlcdb.Querier) error {
		trips, err := q.LockCruiseTripStatuses(ctx, types.NullInt64{Int64: cruise.ID, Valid: true})
		if err != nil {
			return &QueryError{Op: "LockCruiseTripStatuses", Err: err}
		}
		status := make(map[int64]sqlcdb.TripStatus, len(trips))
		for i := range trips {
			status[trips[i].ID] = trips[i].Status
		}
		for _, id := range in.Body.TripIDs {
			st, ok := status[id]
			if !ok {
				return &invalidLegsError{msg: fmt.Sprintf("trip %d is not a leg of this cruise", id)}
			}
			if st != sqlcdb.TripStatusPlanned {
				return &invalidLegsError{msg: fmt.Sprintf("trip %d is not open for enrollment", id)}
			}
		}

		for _, id := range insertOrder {
			e, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
				TripID: id,
				UserID: user.UserID,
				Note:   nullString(in.Body.Note),
			})
			result := "created"
			if errors.Is(err, sql.ErrNoRows) {
				result = "already_enrolled"
				e, err = q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: id, UserID: user.UserID})
			}
			if err != nil {
				return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
			}
			byTrip[id] = dto.EnrollLegResult{
				TripID:     id,
				Result:     result,
				Enrollment: dto.TripEnrollmentToDTO(e),
			}
		}
		return nil
	})
	if txErr != nil {
		if inv, ok := errors.AsType[*invalidLegsError](txErr); ok {
			return nil, huma.Error422UnprocessableEntity(inv.msg)
		}
		slog.Error("enroll cruise legs", "cruise_id", cruise.ID, "user_id", user.UserID, "err", txErr)
		return nil, huma.Error500InternalServerError("failed to enroll")
	}
	results := make([]dto.EnrollLegResult, 0, len(in.Body.TripIDs))
	for _, id := range in.Body.TripIDs {
		results = append(results, byTrip[id])
	}
	return &enrollLegsOutput{Body: dto.EnrollLegsResponse{Results: results}}, nil
}

func (h *EnrollmentHandler) generateToken(ctx context.Context, in *tripIDParam) (*tokenOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := h.q.GetTrip(ctx, in.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for token generation", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}
	token := hex.EncodeToString(b)

	if err := h.q.SetTripEnrollToken(ctx, sqlcdb.SetTripEnrollTokenParams{
		EnrollToken: types.NullString{String: token, Valid: true},
		ID:          in.ID,
	}); err != nil {
		slog.Error("set trip enroll token", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to set token")
	}
	out := &tokenOutput{}
	out.Body.Token = token
	return out, nil
}

func (h *EnrollmentHandler) clearToken(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.ClearTripEnrollToken(ctx, in.ID); err != nil {
		slog.Error("clear trip enroll token", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to clear token")
	}
	return &noContentOutput{}, nil
}

func (h *EnrollmentHandler) listEnrollments(ctx context.Context, in *tripIDParam) (*tripEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	enrollments, err := h.q.ListTripEnrollments(ctx, in.ID)
	if err != nil {
		slog.Error("list trip enrollments", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &tripEnrollmentsOutput{Body: dto.TripEnrollmentsFromDB(enrollments)}, nil
}

func (h *EnrollmentHandler) updateStatus(ctx context.Context, in *tripEnrollmentStatusInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.UpdateTripEnrollmentStatus(ctx, sqlcdb.UpdateTripEnrollmentStatusParams{
		Status: in.Body.Status,
		ID:     in.ID,
	}); err != nil {
		slog.Error("update trip enrollment status", "enrollment_id", in.ID, "status", in.Body.Status, "err", err)
		return nil, huma.Error500InternalServerError("failed to update status")
	}
	return &noContentOutput{}, nil
}

func (h *EnrollmentHandler) deleteEnrollment(ctx context.Context, in *tripEnrollmentParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteTripEnrollment(ctx, in.ID); err != nil {
		slog.Error("delete trip enrollment", "enrollment_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete enrollment")
	}
	return &noContentOutput{}, nil
}

// tripEnrollmentTripFKey is the PostgreSQL name of the trip_enrollments.trip_id
// foreign key. Its violation means the trip was completed or deleted between
// the status check and the insert.
const tripEnrollmentTripFKey = "trip_enrollments_trip_id_fkey"

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}
