package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
		Description: "Creates a trip enrollment for the caller (201, result created). It starts pending, " +
			"or waitlisted when accepted plus pending sign-ups already hold all of the leg's max_crew " +
			"places; a waitlisted result carries its waitlist_position. Signing up " +
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
		Summary: "Withdraw the caller's pending or waitlisted leg sign-up",
		Description: "Deletes the caller's sign-up while it is pending or on the waiting list and unpaid. " +
			"404 when the caller has not signed up; 409 once an admin has accepted or rejected it or " +
			"recorded a payment on it (deposit or paid).",
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
			"(embark date, then id) and sign-up time. 404 when the cruise does not exist.",
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

// clubZone is the club's time zone: a sign-up deadline is a calendar date
// there, open until the end of that day.
var clubZone = mustLoadLocation("Europe/Warsaw")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// clubToday is the club's calendar date (YYYY-MM-DD) at now.
func clubToday(now time.Time) string {
	return now.In(clubZone).Format(time.DateOnly)
}

// deadlinePassed reports whether a leg's sign-up deadline (YYYY-MM-DD,
// inclusive) is before today. A leg without a deadline never passes it.
func deadlinePassed(deadline types.NullString, today string) bool {
	return deadline.Valid && deadline.String != "" && today > deadline.String
}

// today is the club's current calendar date; tests pin h.now.
func (h *EnrollmentHandler) today() string {
	if h.now != nil {
		return clubToday(h.now())
	}
	return clubToday(time.Now())
}

// checkSignUpOpen is the single rule for whether a locked leg takes new
// sign-ups on the club date today. A full leg still takes sign-ups:
// initialSignUpStatus puts them on the waiting list.
func checkSignUpOpen(trip sqlcdb.LockTripForSignUpRow, today string) error {
	if trip.Status != sqlcdb.TripStatusPlanned {
		return &signUpClosedError{msg: "sign-up closed: the leg is not planned"}
	}
	if !trip.Open {
		return &signUpClosedError{msg: "sign-up closed: sign-ups for this leg are switched off"}
	}
	if deadlinePassed(trip.SignUpDeadline, today) {
		return &signUpClosedError{msg: "sign-up closed: the sign-up deadline has passed"}
	}
	return nil
}

// legOpen reports the same rule as checkSignUpOpen for an unlocked trip read.
func legOpen(t *sqlcdb.Trip, today string) bool {
	return t.Status == sqlcdb.TripStatusPlanned && t.EnrollToken.Valid && !deadlinePassed(t.SignUpDeadline, today)
}

// signUpOf returns the user's sign-up for a leg: their trip enrollment or,
// failing that, a cruise enrollment assigned to the leg, made before sign-ups
// moved to legs. A cruise enrollment carries cruise_id in the result. found
// is false when the user has neither.
func signUpOf(ctx context.Context, q sqlcdb.Querier, tripID, userID int64) (e dto.Enrollment, found bool, err error) {
	te, err := q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: tripID, UserID: userID})
	if err == nil {
		return dto.TripEnrollmentToDTO(te), true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return dto.Enrollment{}, false, &QueryError{Op: "GetUserTripEnrollment", Err: err}
	}
	ce, err := q.GetUserCruiseEnrollmentForTrip(ctx, sqlcdb.GetUserCruiseEnrollmentForTripParams{
		TripID: types.NullInt64{Int64: tripID, Valid: true},
		UserID: userID,
	})
	if err == nil {
		return dto.CruiseEnrollmentToDTO(ce), true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return dto.Enrollment{}, false, &QueryError{Op: "GetUserCruiseEnrollmentForTrip", Err: err}
	}
	return dto.Enrollment{}, false, nil
}

// errCruiseLevelSignUp is the 409 for changing a cruise-level enrollment
// through the leg sign-up operations; admins manage those.
func errCruiseLevelSignUp() error {
	return huma.Error409Conflict("cruise-level sign-up: ask an admin to change it")
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
	out := dto.LegSignUp{TripID: trip.ID, Open: legOpen(&trip, h.today()), AcceptedCount: accepted}
	e, found, err := signUpOf(ctx, h.q, trip.ID, user.UserID)
	if err != nil {
		slog.Error("get user leg sign-up", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get sign-up")
	}
	if found {
		out.Enrollment = &e
		if err := setWaitlistPosition(ctx, h.q, &e, &out.WaitlistPosition); err != nil {
			slog.Error("get waitlist position for sign-up", "trip_id", trip.ID, "user_id", user.UserID, "err", err)
			return nil, huma.Error500InternalServerError("failed to get sign-up")
		}
	}
	return &legSignUpOutput{Body: out}, nil
}

// signUpForTrip creates the caller's sign-up for a leg, or returns the
// existing one. The trip row is locked first, so the open check, the
// waitlist count and the insert see the same leg state, and concurrent
// sign-ups for the leg count one another; the lock order matches trip
// completion (trip, then its enrollments). The sign-up starts pending, or
// waitlisted when the leg's places are taken (initialSignUpStatus). An
// existing sign-up, including a cruise enrollment assigned to the leg, is
// returned even when the leg has since closed. When a concurrent request
// from the same user wins the insert, the conditional insert returns no row
// and the winner's enrollment is returned as already_enrolled.
func (h *EnrollmentHandler) signUpForTrip(ctx context.Context, in *tripIDParam) (*signUpOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	key := sqlcdb.GetUserTripEnrollmentParams{TripID: in.ID, UserID: user.UserID}
	today := h.today()
	var out dto.EnrollLegResult
	txErr := h.runTx(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, in.ID)
		if err != nil {
			return err
		}
		existing, found, err := signUpOf(ctx, q, trip.ID, user.UserID)
		if err != nil {
			return err
		}
		if found {
			out = dto.EnrollLegResult{TripID: trip.ID, Result: "already_enrolled", Enrollment: existing}
			return setWaitlistPosition(ctx, q, &out.Enrollment, &out.WaitlistPosition)
		}
		if err := checkSignUpOpen(trip, today); err != nil {
			return err
		}
		status, err := initialSignUpStatus(ctx, q, trip.ID, trip.MaxCrew)
		if err != nil {
			return err
		}
		result := "created"
		e, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
			TripID: trip.ID,
			UserID: user.UserID,
			Status: status,
		})
		if errors.Is(err, sql.ErrNoRows) {
			result = "already_enrolled"
			e, err = q.GetUserTripEnrollment(ctx, key)
		}
		if err != nil {
			return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
		}
		out = dto.EnrollLegResult{TripID: trip.ID, Result: result, Enrollment: dto.TripEnrollmentToDTO(e)}
		return setWaitlistPosition(ctx, q, &out.Enrollment, &out.WaitlistPosition)
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
			return nil, h.noTripSignUp(ctx, in.TripID, user.UserID)
		}
		slog.Error("update trip sign-up note", "trip_id", in.TripID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update note")
	}
	return &enrollmentBodyOutput{Body: dto.TripEnrollmentToDTO(e)}, nil
}

// withdrawTripSignUp deletes the caller's pending or waitlisted, unpaid
// sign-up. When nothing was deleted, a follow-up read tells a missing
// sign-up (404) from one an admin has accepted or rejected, one with a
// recorded payment or a cruise-level enrollment (409).
func (h *EnrollmentHandler) withdrawTripSignUp(ctx context.Context, in *tripIDParam) (*noContentOutput, error) {
	user, err := signedIn(ctx)
	if err != nil {
		return nil, err
	}
	_, err = h.q.WithdrawUserTripEnrollment(ctx, sqlcdb.WithdrawUserTripEnrollmentParams{
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
	e, found, gerr := signUpOf(ctx, h.q, in.ID, user.UserID)
	switch {
	case gerr != nil:
		slog.Error("get trip sign-up for withdraw", "trip_id", in.ID, "user_id", user.UserID, "err", gerr)
		return nil, huma.Error500InternalServerError("failed to withdraw sign-up")
	case !found:
		return nil, huma.Error404NotFound("not signed up for this leg")
	case e.CruiseID != nil:
		return nil, errCruiseLevelSignUp()
	case (e.Status == enrollmentPending || e.Status == enrollmentWaitlisted) && e.PaymentStatus != "unpaid":
		return nil, huma.Error409Conflict("a payment is recorded on this sign-up: ask an admin to withdraw it")
	}
	return nil, huma.Error409Conflict("only a pending or waitlisted sign-up can be withdrawn")
}

// noTripSignUp explains why the caller has no trip enrollment to change:
// none at all (404), or only a cruise enrollment assigned to the leg (409).
func (h *EnrollmentHandler) noTripSignUp(ctx context.Context, tripID, userID int64) error {
	e, found, err := signUpOf(ctx, h.q, tripID, userID)
	switch {
	case err != nil:
		slog.Error("get trip sign-up", "trip_id", tripID, "user_id", userID, "err", err)
		return huma.Error500InternalServerError("failed to get sign-up")
	case found && e.CruiseID != nil:
		return errCruiseLevelSignUp()
	}
	return huma.Error404NotFound("not signed up for this leg")
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
	byTrip := make(map[int64]dto.Enrollment, len(mine)+1)
	ce, err := h.q.GetUserCruiseEnrollment(ctx, sqlcdb.GetUserCruiseEnrollmentParams{CruiseID: in.CruiseID, UserID: user.UserID})
	switch {
	case err == nil && ce.TripID.Valid:
		byTrip[ce.TripID.Int64] = dto.CruiseEnrollmentToDTO(ce)
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		slog.Error("get user cruise enrollment for sign-ups", "cruise_id", in.CruiseID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list sign-ups")
	}
	for i := range mine {
		byTrip[mine[i].TripID] = dto.TripEnrollmentToDTO(mine[i])
	}
	today := h.today()
	out := make([]dto.LegSignUp, len(trips))
	for i := range trips {
		t := &trips[i]
		out[i] = dto.LegSignUp{TripID: t.ID, Open: legOpen(t, today), AcceptedCount: accepted[t.ID]}
		if e, ok := byTrip[t.ID]; ok {
			out[i].Enrollment = &e
			if err := setWaitlistPosition(ctx, h.q, &e, &out[i].WaitlistPosition); err != nil {
				slog.Error("get waitlist position for sign-ups", "trip_id", t.ID, "user_id", user.UserID, "err", err)
				return nil, huma.Error500InternalServerError("failed to list sign-ups")
			}
		}
	}
	return &legSignUpsOutput{Body: out}, nil
}

func (h *EnrollmentHandler) listCruiseTripEnrollments(ctx context.Context, in *cruiseIDPathParam) (*tripEnrollmentsOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.verifyCruise(ctx, in.CruiseID); err != nil {
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
