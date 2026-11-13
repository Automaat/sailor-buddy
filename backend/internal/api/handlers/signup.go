package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type cruiseIDPathParam struct {
	CruiseID int64 `path:"cruiseID" doc:"Cruise ID"`
}

type legSignUpOutput struct {
	Body dto.LegSignUp
}

type legSignUpsOutput struct {
	Body []dto.LegSignUp
}

type signUpOutput struct {
	Status int
	Body   dto.EnrollLegResult
}

type signUpNoteInput struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	Body   dto.SignUpNoteBody
}

type enrollmentBodyOutput struct {
	Body dto.Enrollment
}

// registerSignUpRoutes wires the signed-in caller's own one-click sign-up
// for a leg (trip) of a cruise. A sign-up is a trip_enrollments row; unlike
// the share-token flow in enrollment.go, these operations need no token:
//
//	GET    /trips/{tripID}/sign-up               caller's sign-up state for one leg
//	POST   /trips/{tripID}/sign-up               sign up (idempotent)
//	PUT    /trips/{tripID}/sign-up/note          set the note on the caller's sign-up
//	DELETE /trips/{tripID}/sign-up               withdraw a pending sign-up
//	GET    /cruises/{cruiseID}/leg-sign-ups      caller's sign-up state for every leg
//	GET    /cruises/{cruiseID}/trip-enrollments  every leg sign-up of a cruise (admin)
func registerSignUpRoutes(api huma.API, h *EnrollmentHandler) {
	tag := []string{"Sign-up"}

	huma.Register(api, huma.Operation{
		OperationID: "get-trip-sign-up", Method: http.MethodGet, Path: "/trips/{tripID}/sign-up",
		Summary: "Get the caller's sign-up state for a leg",
		Description: "Returns whether the leg takes new sign-ups, its accepted crew count and the " +
			"caller's own sign-up, if any. 404 when the trip does not exist.",
		Tags: tag,
	}, h.getTripSignUp)
	huma.Register(api, huma.Operation{
		OperationID: "sign-up-for-trip", Method: http.MethodPost, Path: "/trips/{tripID}/sign-up",
		Summary: "Sign the caller up for a leg with one click",
		Description: "Creates a pending trip enrollment for the caller (201, result created). Signing up " +
			"again returns the existing enrollment (200, result already_enrolled). Open only while the " +
			"leg is planned and its sign-ups are switched on (enroll token set): otherwise 422 and " +
			"nothing is written. 404 when the trip does not exist; completed legs are voyages, not " +
			"trips, so they also return 404. Any signed-in user may sign up; the profile need not be complete.",
		Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.signUpForTrip)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip-sign-up-note", Method: http.MethodPut, Path: "/trips/{tripID}/sign-up/note",
		Summary: "Set the note on the caller's leg sign-up",
		Description: "Replaces the note to the organizer on the caller's existing sign-up. An empty or " +
			"omitted note clears it. 404 when the caller has not signed up for the leg.",
		Tags: tag,
	}, h.updateTripSignUpNote)
	huma.Register(api, huma.Operation{
		OperationID: "withdraw-trip-sign-up", Method: http.MethodDelete, Path: "/trips/{tripID}/sign-up",
		Summary: "Withdraw the caller's pending leg sign-up",
		Description: "Deletes the caller's sign-up while it is pending. 404 when the caller has not " +
			"signed up; 409 once an admin has reviewed it (accepted, rejected or waitlisted).",
		Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.withdrawTripSignUp)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-leg-sign-ups", Method: http.MethodGet, Path: "/cruises/{cruiseID}/leg-sign-ups",
		Summary: "Get the caller's sign-up state for every leg of a cruise",
		Description: "One entry per trip of the cruise, in the cruise trip list order. 404 when the " +
			"cruise does not exist.",
		Tags: tag,
	}, h.listCruiseLegSignUps)
	huma.Register(api, huma.Operation{
		OperationID: "list-cruise-trip-enrollments", Method: http.MethodGet, Path: "/cruises/{cruiseID}/trip-enrollments",
		Summary: "List every leg sign-up of a cruise (admin)",
		Description: "Trip enrollments of all the cruise's legs with the enrolled user, ordered by leg " +
			"(embark date, then id) and sign-up time.",
		Tags: tag,
	}, h.listCruiseTripEnrollments)
}

// signedIn returns the caller or a 401. The auth middleware already rejects
// requests without a valid token; this guards handlers used without it.
func signedIn(ctx context.Context) (*auth.Claims, error) {
	user := middleware.GetUser(ctx)
	if user == nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}
	return user, nil
}

// signUpClosedError aborts a sign-up transaction when the leg is not open.
type signUpClosedError struct{ msg string }

func (e *signUpClosedError) Error() string { return e.msg }

// checkSignUpOpen is the single rule for whether a locked leg takes new
// sign-ups. Later sign-up rules (deadlines, capacity) belong here.
func checkSignUpOpen(trip sqlcdb.LockTripForSignUpRow) error {
	if trip.Status != sqlcdb.TripStatusPlanned {
		return &signUpClosedError{msg: "sign-up closed: the leg is not planned"}
	}
	if !trip.Open {
		return &signUpClosedError{msg: "sign-up closed: sign-ups for this leg are switched off"}
	}
	return nil
}

// legOpen reports the same rule as checkSignUpOpen for an unlocked trip read.
func legOpen(status sqlcdb.TripStatus, enrollToken types.NullString) bool {
	return status == sqlcdb.TripStatusPlanned && enrollToken.Valid
}

func (h *EnrollmentHandler) getTripSignUp(ctx context.Context, in *tripIDParam) (*legSignUpOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	trip, err := h.q.GetTrip(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for sign-up", "trip_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}
	accepted, err := h.q.CountAcceptedForTrip(ctx, trip.ID)
	if err != nil {
		slog.Error("count accepted for trip sign-up", "trip_id", trip.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to count accepted crew")
	}
	out := dto.LegSignUp{TripID: trip.ID, Open: legOpen(trip.Status, trip.EnrollToken), AcceptedCount: accepted}
	e, err := h.q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: trip.ID, UserID: user.UserID})
	switch {
	case err == nil:
		out.Enrollment = new(dto.TripEnrollmentToDTO(e))
	case !errors.Is(err, sql.ErrNoRows):
		slog.Error("get user trip enrollment", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get sign-up")
	}
	return &legSignUpOutput{Body: out}, nil
}

// signUpForTrip creates the caller's pending sign-up for a leg, or returns
// the existing one. The trip row is share-locked first, so the open check
// and the insert see the same leg state, in the same lock order as trip
// completion (trip, then its enrollments). An existing sign-up is returned
// even when the leg has since closed. When a concurrent request from the
// same user wins the insert, the conditional insert returns no row and the
// winner's enrollment is returned as already_enrolled.
func (h *EnrollmentHandler) signUpForTrip(ctx context.Context, in *tripIDParam) (*signUpOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	key := sqlcdb.GetUserTripEnrollmentParams{TripID: in.ID, UserID: user.UserID}
	var out dto.EnrollLegResult
	txErr := h.runTx(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, in.ID)
		if err != nil {
			return err
		}
		e, err := q.GetUserTripEnrollment(ctx, key)
		if err == nil {
			out = dto.EnrollLegResult{TripID: trip.ID, Result: "already_enrolled", Enrollment: dto.TripEnrollmentToDTO(e)}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return &QueryError{Op: "GetUserTripEnrollment", Err: err}
		}
		if err := checkSignUpOpen(trip); err != nil {
			return err
		}
		result := "created"
		e, err = q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
			TripID: trip.ID,
			UserID: user.UserID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			result = "already_enrolled"
			e, err = q.GetUserTripEnrollment(ctx, key)
		}
		if err != nil {
			return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
		}
		out = dto.EnrollLegResult{TripID: trip.ID, Result: result, Enrollment: dto.TripEnrollmentToDTO(e)}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		if closed, ok := errors.AsType[*signUpClosedError](txErr); ok {
			return nil, huma.Error422UnprocessableEntity(closed.msg)
		}
		slog.Error("sign up for trip", "trip_id", in.ID, "user_id", user.UserID, "err", txErr)
		return nil, huma.Error500InternalServerError("failed to sign up")
	}
	status := http.StatusCreated
	if out.Result == "already_enrolled" {
		status = http.StatusOK
	}
	return &signUpOutput{Status: status, Body: out}, nil
}

func (h *EnrollmentHandler) updateTripSignUpNote(ctx context.Context, in *signUpNoteInput) (*enrollmentBodyOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	var note types.NullString
	if in.Body.Note != nil {
		if trimmed := strings.TrimSpace(*in.Body.Note); trimmed != "" {
			note = types.NullString{String: trimmed, Valid: true}
		}
	}
	e, err := h.q.UpdateUserTripEnrollmentNote(ctx, sqlcdb.UpdateUserTripEnrollmentNoteParams{
		TripID: in.TripID,
		UserID: user.UserID,
		Note:   note,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not signed up for this leg")
		}
		slog.Error("update trip sign-up note", "trip_id", in.TripID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update note")
	}
	return &enrollmentBodyOutput{Body: dto.TripEnrollmentToDTO(e)}, nil
}

// withdrawTripSignUp deletes the caller's pending sign-up. When nothing was
// deleted, a follow-up read tells a missing sign-up (404) from one an admin
// has already reviewed (409).
func (h *EnrollmentHandler) withdrawTripSignUp(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	key := sqlcdb.GetUserTripEnrollmentParams{TripID: in.ID, UserID: user.UserID}
	_, err = h.q.DeletePendingUserTripEnrollment(ctx, sqlcdb.DeletePendingUserTripEnrollmentParams{
		TripID: in.ID,
		UserID: user.UserID,
	})
	if err == nil {
		return &noContentOutput{}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("withdraw trip sign-up", "trip_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to withdraw sign-up")
	}
	if _, gerr := h.q.GetUserTripEnrollment(ctx, key); gerr != nil {
		if errors.Is(gerr, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("not signed up for this leg")
		}
		slog.Error("get trip sign-up for withdraw", "trip_id", in.ID, "user_id", user.UserID, "err", gerr)
		return nil, huma.Error500InternalServerError("failed to withdraw sign-up")
	}
	return nil, huma.Error409Conflict("only a pending sign-up can be withdrawn")
}

func (h *EnrollmentHandler) listCruiseLegSignUps(ctx context.Context, in *cruiseIDPathParam) (*legSignUpsOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.CruiseID); err != nil {
		return nil, err
	}
	trips, err := h.q.ListCruiseTrips(ctx, in.CruiseID)
	if err != nil {
		slog.Error("list cruise trips for sign-ups", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list legs")
	}
	counts, err := h.q.CountAcceptedByCruiseTrip(ctx, in.CruiseID)
	if err != nil {
		slog.Error("count accepted per cruise trip for sign-ups", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to count accepted crew")
	}
	mine, err := h.q.ListUserCruiseTripEnrollments(ctx, sqlcdb.ListUserCruiseTripEnrollmentsParams{
		CruiseID: in.CruiseID,
		UserID:   user.UserID,
	})
	if err != nil {
		slog.Error("list user cruise sign-ups", "cruise_id", in.CruiseID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list sign-ups")
	}
	accepted := make(map[int64]int64, len(counts))
	for _, c := range counts {
		accepted[c.TripID] = c.Accepted
	}
	byTrip := make(map[int64]sqlcdb.TripEnrollment, len(mine))
	for i := range mine {
		byTrip[mine[i].TripID] = mine[i]
	}
	out := make([]dto.LegSignUp, len(trips))
	for i := range trips {
		t := &trips[i]
		out[i] = dto.LegSignUp{TripID: t.ID, Open: legOpen(t.Status, t.EnrollToken), AcceptedCount: accepted[t.ID]}
		if e, ok := byTrip[t.ID]; ok {
			out[i].Enrollment = new(dto.TripEnrollmentToDTO(e))
		}
	}
	return &legSignUpsOutput{Body: out}, nil
}

func (h *EnrollmentHandler) listCruiseTripEnrollments(ctx context.Context, in *cruiseIDPathParam) (*tripEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := h.q.ListCruiseTripEnrollments(ctx, in.CruiseID)
	if err != nil {
		slog.Error("list cruise trip enrollments", "cruise_id", in.CruiseID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list enrollments")
	}
	return &tripEnrollmentsOutput{Body: dto.CruiseTripEnrollmentsFromDB(rows)}, nil
}

// verifyCruise confirms the cruise exists.
func (h *EnrollmentHandler) verifyCruise(ctx context.Context, cruiseID int64) error {
	if _, err := h.q.GetCruise(ctx, cruiseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("cruise not found")
		}
		slog.Error("verify cruise for sign-ups", "cruise_id", cruiseID, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}
