package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// enrollmentOperations owns transaction boundaries; callers map errors to HTTP.
type enrollmentOperations struct {
	runTx txRunner
	today func() string
}

// cruiseEnrollmentTxAttempts bounds how often runCruiseEnrollmentTx starts
// a change again after the enrollment moved to another leg under it.
const cruiseEnrollmentTxAttempts = 3

// runCruiseEnrollmentTx runs fn in a transaction and runs it again, in a new
// transaction, while it fails with errEnrollmentChanged or as a deadlock
// victim, as when a person merge holds the person and waits for the leg.
// Rolling back first drops the leg and row locks, so the retry takes them in
// order again.
func (o enrollmentOperations) runCruiseEnrollmentTx(ctx context.Context, fn func(sqlcdb.Querier) error) error {
	var err error
	for range cruiseEnrollmentTxAttempts {
		err = o.runTx(ctx, fn)
		if !errors.Is(err, errEnrollmentChanged) && !isDeadlock(err) {
			return err
		}
	}
	return err
}

// lockCruiseEnrollment reads one enrollment of the cruise, locks its leg and
// the extra legs in ascending id order, then re-reads the enrollment under
// its row lock, in the same trip-then-enrollment order as trip completion and
// the trip enrollment accept. Holding the legs serializes the change with
// every other sign-up change on them, so accepts count places one at a time
// and crew entries added by acceptance follow the committed sign-ups. It
// returns the locked enrollment and legs by id. errEnrollmentChanged means the
// enrollment moved to another leg, or its leg was deleted, before the row
// lock: run the transaction again. errLegGone means an extra leg does not
// exist.
func lockCruiseEnrollment(ctx context.Context, q sqlcdb.Querier, cruiseID, id int64, extra ...int64) (sqlcdb.CruiseEnrollment, map[int64]sqlcdb.LockTripForSignUpRow, error) {
	before, err := getCruiseEnrollment(ctx, q, cruiseID, id)
	if err != nil {
		return before, nil, err
	}
	legs := slices.Clone(extra)
	if before.TripID.Valid {
		legs = append(legs, before.TripID.Int64)
	}
	slices.Sort(legs)
	legs = slices.Compact(legs)
	locked, gone, err := lockLegs(ctx, q, legs)
	if err != nil {
		return before, nil, err
	}
	if gone != 0 && slices.Contains(extra, gone) {
		return before, nil, errLegGone
	}
	if gone != 0 {
		return before, nil, errEnrollmentChanged
	}
	now, err := q.LockCruiseEnrollmentOfCruise(ctx, sqlcdb.LockCruiseEnrollmentOfCruiseParams{ID: id, CruiseID: cruiseID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return now, nil, errEnrollmentNotFound
		}
		return now, nil, &QueryError{Op: "LockCruiseEnrollmentOfCruise", Err: err}
	}
	if now.TripID != before.TripID {
		return now, nil, errEnrollmentChanged
	}
	return now, locked, nil
}

// lockLegs locks the given trips in order. gone is the first one that does
// not exist, or 0; the legs after it are left unlocked.
func lockLegs(ctx context.Context, q sqlcdb.Querier, ids []int64) (locked map[int64]sqlcdb.LockTripForSignUpRow, gone int64, err error) {
	locked = make(map[int64]sqlcdb.LockTripForSignUpRow, len(ids))
	for _, id := range ids {
		trip, err := q.LockTripForSignUp(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return locked, id, nil
		}
		if err != nil {
			return nil, 0, &QueryError{Op: "LockTripForSignUp", Err: err}
		}
		locked[id] = trip
	}
	return locked, 0, nil
}

func (o enrollmentOperations) changeTripStatus(ctx context.Context, tripID, enrollmentID int64, status string) error {
	return retryDeadlock(o.runTx)(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, tripID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errEnrollmentNotFound
			}
			return &QueryError{Op: "LockTripForSignUp", Err: err}
		}
		e, err := q.LockTripEnrollmentOfTrip(ctx, sqlcdb.LockTripEnrollmentOfTripParams{ID: enrollmentID, TripID: trip.ID})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errEnrollmentNotFound
			}
			return &QueryError{Op: "LockTripEnrollmentOfTrip", Err: err}
		}
		if status == enrollmentAccepted && e.Status != enrollmentAccepted {
			if err := checkPlaceFree(ctx, q, trip.ID, e.CrewMemberID, trip.MaxCrew); err != nil {
				return err
			}
		}
		if err := q.UpdateTripEnrollmentStatus(ctx, sqlcdb.UpdateTripEnrollmentStatusParams{
			Status: status,
			ID:     e.ID,
		}); err != nil {
			return &QueryError{Op: "UpdateTripEnrollmentStatus", Err: err}
		}
		if status == enrollmentAccepted {
			return addAcceptedPersonCrew(ctx, q, trip.ID, e.CrewMemberID)
		}
		return removeAcceptedPersonCrew(ctx, q, trip.ID, e.CrewMemberID)
	})
}

func (o enrollmentOperations) deleteTripEnrollment(ctx context.Context, tripID, enrollmentID int64) error {
	return o.runTx(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, tripID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return &QueryError{Op: "LockTripForSignUp", Err: err}
		}
		e, err := q.LockTripEnrollmentOfTrip(ctx, sqlcdb.LockTripEnrollmentOfTripParams{ID: enrollmentID, TripID: trip.ID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return &QueryError{Op: "LockTripEnrollmentOfTrip", Err: err}
		}
		if err := q.DeleteTripEnrollment(ctx, e.ID); err != nil {
			return &QueryError{Op: "DeleteTripEnrollment", Err: err}
		}
		return removeAcceptedPersonCrew(ctx, q, trip.ID, e.CrewMemberID)
	})
}

func (o enrollmentOperations) changeCruiseStatus(ctx context.Context, cruiseID, enrollmentID int64, status string) error {
	return o.runCruiseEnrollmentTx(ctx, func(q sqlcdb.Querier) error {
		e, legs, err := lockCruiseEnrollment(ctx, q, cruiseID, enrollmentID)
		if err != nil {
			return err
		}
		if status == enrollmentAccepted && e.Status != enrollmentAccepted && e.TripID.Valid {
			leg := legs[e.TripID.Int64]
			if err := checkUserPlaceFree(ctx, q, leg.ID, e.UserID, leg.MaxCrew); err != nil {
				return err
			}
		}
		if err := q.UpdateCruiseEnrollmentStatus(ctx, sqlcdb.UpdateCruiseEnrollmentStatusParams{
			Status: status,
			ID:     e.ID,
		}); err != nil {
			return &QueryError{Op: "UpdateCruiseEnrollmentStatus", Err: err}
		}
		after := e
		after.Status = status
		return syncCruiseEnrollmentCrew(ctx, q, e, after)
	})
}

func (o enrollmentOperations) assignCruiseEnrollment(ctx context.Context, cruiseID, enrollmentID int64, tripID *int64) error {
	return o.runCruiseEnrollmentTx(ctx, func(q sqlcdb.Querier) error {
		target := tripID
		var extra []int64
		if target != nil {
			extra = append(extra, *target)
		}
		e, legs, err := lockCruiseEnrollment(ctx, q, cruiseID, enrollmentID, extra...)
		if err != nil {
			return err
		}
		moving := target != nil && (!e.TripID.Valid || e.TripID.Int64 != *target)
		if moving && e.Status == enrollmentAccepted {
			leg := legs[*target]
			if err := checkUserPlaceFree(ctx, q, leg.ID, e.UserID, leg.MaxCrew); err != nil {
				return err
			}
		}
		if err := q.AssignCruiseEnrollmentToTrip(ctx, sqlcdb.AssignCruiseEnrollmentToTripParams{
			TripID: nullInt64(target),
			ID:     e.ID,
		}); err != nil {
			return err
		}
		after, err := getCruiseEnrollment(ctx, q, cruiseID, enrollmentID)
		if err != nil {
			return err
		}
		return syncCruiseEnrollmentCrew(ctx, q, e, after)
	})
}

func (o enrollmentOperations) deleteCruiseEnrollment(ctx context.Context, cruiseID, enrollmentID int64) error {
	return o.runCruiseEnrollmentTx(ctx, func(q sqlcdb.Querier) error {
		e, _, err := lockCruiseEnrollment(ctx, q, cruiseID, enrollmentID)
		if errors.Is(err, errEnrollmentNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := q.DeleteCruiseEnrollment(ctx, e.ID); err != nil {
			return &QueryError{Op: "DeleteCruiseEnrollment", Err: err}
		}
		if !e.TripID.Valid {
			return nil
		}
		return removeAcceptedCrew(ctx, q, e.TripID.Int64, e.UserID)
	})
}

// checkUserPlaceFree is checkPlaceFree for the person of userID's account,
// for a cruise enrollment.
func checkUserPlaceFree(ctx context.Context, q sqlcdb.Querier, tripID, userID int64, maxCrew types.NullInt64) error {
	if _, ok := crewLimit(maxCrew); !ok {
		return nil
	}
	personID, err := lockUserPerson(ctx, q, userID)
	if err != nil {
		return err
	}
	return checkPlaceFree(ctx, q, tripID, personID, maxCrew)
}

// signUp creates or returns the user's sign-up for a leg: one of the
// account's person. A hidden leg is missing (sql.ErrNoRows) unless
// seeHidden; checkSignUpOpen then refuses it. The person is locked after
// the leg, as in acceptance; a person merge that holds the person and waits
// for the leg deadlocks with it, and the losing side runs again.
func (o enrollmentOperations) signUp(ctx context.Context, tripID, userID int64, today string, seeHidden bool) (dto.EnrollLegResult, error) {
	key := sqlcdb.GetUserTripEnrollmentParams{TripID: tripID, UserID: userID}
	var out dto.EnrollLegResult
	err := retryDeadlock(o.runTx)(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, tripID)
		if err != nil {
			return err
		}
		if !trip.Published && !seeHidden {
			return sql.ErrNoRows
		}
		existing, found, err := signUpOf(ctx, q, trip.ID, userID)
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
		personID, err := lockUserPerson(ctx, q, userID)
		if err != nil {
			return err
		}
		result := "created"
		e, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
			TripID:       trip.ID,
			CrewMemberID: personID,
			Status:       status,
		})
		if errors.Is(err, sql.ErrNoRows) {
			result = "already_enrolled"
			e, err = q.GetUserTripEnrollment(ctx, key)
		}
		if err != nil {
			return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
		}
		out = dto.EnrollLegResult{TripID: trip.ID, Result: result, Enrollment: dto.UserTripEnrollmentToDTO(e, userID)}
		return setWaitlistPosition(ctx, q, &out.Enrollment, &out.WaitlistPosition)
	})
	return out, err
}

// errAlreadySignedUp aborts an admin sign-up when the person already has a
// sign-up for the leg.
var errAlreadySignedUp = errors.New("person already signed up for the leg")

// errSignUpPersonNotFound aborts an admin sign-up for a person that does not
// exist, or was deleted or merged away before the insert.
var errSignUpPersonNotFound = errors.New("person not found")

// signUpPersonFKey is violated when the person was deleted, or merged away,
// between the lookup and the sign-up insert.
const signUpPersonFKey = "trip_enrollments_crew_member_id_fkey"

// signUpPerson is an admin's sign-up of a person, with or without an
// account, for a planned leg. It ignores whether sign-ups are switched on,
// the deadline and publication, but starts pending or waitlisted by the
// same place rule as a self sign-up, under the same leg lock. sql.ErrNoRows
// means the trip does not exist; errTripNotPlanned that it is cancelled.
func (o enrollmentOperations) signUpPerson(ctx context.Context, tripID, personID int64, note types.NullString) (sqlcdb.ListTripEnrollmentsRow, error) {
	var out sqlcdb.ListTripEnrollmentsRow
	err := retryDeadlock(o.runTx)(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, tripID)
		if err != nil {
			return err
		}
		if trip.Status != sqlcdb.TripStatusPlanned {
			return errTripNotPlanned
		}
		if _, err := q.GetCrewMember(ctx, personID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errSignUpPersonNotFound
			}
			return &QueryError{Op: "GetCrewMember", Err: err}
		}
		status, err := initialSignUpStatus(ctx, q, trip.ID, trip.MaxCrew)
		if err != nil {
			return err
		}
		e, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
			TripID:       trip.ID,
			CrewMemberID: personID,
			Note:         note,
			Status:       status,
		})
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return errAlreadySignedUp
		case isForeignKeyViolation(err, signUpPersonFKey):
			return errSignUpPersonNotFound
		case err != nil:
			return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
		}
		row, err := q.GetTripEnrollmentDetail(ctx, sqlcdb.GetTripEnrollmentDetailParams{ID: e.ID, TripID: trip.ID})
		if err != nil {
			return &QueryError{Op: "GetTripEnrollmentDetail", Err: err}
		}
		out = sqlcdb.ListTripEnrollmentsRow(row)
		return nil
	})
	return out, err
}

func (o enrollmentOperations) enrollByTripToken(ctx context.Context, tripID int64, token string, userID int64, note types.NullString, today string) (sqlcdb.TripEnrollment, error) {
	var e sqlcdb.TripEnrollment
	err := retryDeadlock(o.runTx)(ctx, func(q sqlcdb.Querier) error {
		trip, err := q.LockTripForSignUp(ctx, tripID)
		if errors.Is(err, sql.ErrNoRows) {
			return errTripNotPlanned
		}
		if err != nil {
			return &QueryError{Op: "LockTripForSignUp", Err: err}
		}
		if !trip.Published {
			return errLegHidden
		}
		if trip.Status != sqlcdb.TripStatusPlanned {
			return errTripNotPlanned
		}
		if !trip.EnrollToken.Valid || trip.EnrollToken.String != token {
			return errLinkSwitchedOff
		}
		if deadlinePassed(trip.SignUpDeadline, today) {
			return &signUpClosedError{msg: "sign-up closed: the sign-up deadline has passed"}
		}
		initial, err := initialSignUpStatus(ctx, q, trip.ID, trip.MaxCrew)
		if err != nil {
			return err
		}
		personID, err := lockUserPerson(ctx, q, userID)
		if err != nil {
			return err
		}
		e, err = q.CreateTripEnrollment(ctx, sqlcdb.CreateTripEnrollmentParams{
			TripID:       trip.ID,
			CrewMemberID: personID,
			Note:         note,
			Status:       initial,
		})
		return err
	})
	return e, err
}

func (o enrollmentOperations) enrollByCruiseToken(ctx context.Context, cruiseID int64, tok types.NullString, userID int64, tripIDs []int64, note *string) (map[int64]dto.EnrollLegResult, error) {
	// Inserts run in ascending trip order so concurrent requests from one
	// user take the unique-index locks in the same order and cannot deadlock.
	insertOrder := slices.Clone(tripIDs)
	slices.Sort(insertOrder)
	byTrip := make(map[int64]dto.EnrollLegResult, len(insertOrder))
	err := retryDeadlock(o.runTx)(ctx, func(q sqlcdb.Querier) error {
		clear(byTrip)
		if _, err := q.LockCruiseEnrollToken(ctx, sqlcdb.LockCruiseEnrollTokenParams{ID: cruiseID, EnrollToken: tok}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errLinkSwitchedOff
			}
			return &QueryError{Op: "LockCruiseEnrollToken", Err: err}
		}
		trips, err := q.LockCruiseTripStatuses(ctx, cruiseID)
		if err != nil {
			return &QueryError{Op: "LockCruiseTripStatuses", Err: err}
		}
		legs := make(map[int64]sqlcdb.LockCruiseTripStatusesRow, len(trips))
		for i := range trips {
			legs[trips[i].ID] = trips[i]
		}
		today := o.today()
		for _, id := range tripIDs {
			leg, ok := legs[id]
			if !ok || !leg.Published {
				return &invalidLegsError{msg: fmt.Sprintf("trip %d is not a leg of this cruise", id)}
			}
			if leg.Status != sqlcdb.TripStatusPlanned {
				return &invalidLegsError{msg: fmt.Sprintf("trip %d is not open for enrollment", id)}
			}
			if deadlinePassed(leg.SignUpDeadline, today) {
				return &invalidLegsError{msg: fmt.Sprintf("trip %d: the sign-up deadline has passed", id)}
			}
		}

		personID, err := lockUserPerson(ctx, q, userID)
		if err != nil {
			return err
		}
		for _, id := range insertOrder {
			initial, err := initialSignUpStatus(ctx, q, id, legs[id].MaxCrew)
			if err != nil {
				return err
			}
			e, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
				TripID:       id,
				CrewMemberID: personID,
				Note:         nullString(note),
				Status:       initial,
			})
			result := "created"
			if errors.Is(err, sql.ErrNoRows) {
				result = "already_enrolled"
				e, err = q.GetUserTripEnrollment(ctx, sqlcdb.GetUserTripEnrollmentParams{TripID: id, UserID: userID})
			}
			if err != nil {
				return &QueryError{Op: "CreateTripEnrollmentIfAbsent", Err: err}
			}
			r := dto.EnrollLegResult{
				TripID:     id,
				Result:     result,
				Enrollment: dto.UserTripEnrollmentToDTO(e, userID),
			}
			if err := setWaitlistPosition(ctx, q, &r.Enrollment, &r.WaitlistPosition); err != nil {
				return err
			}
			byTrip[id] = r
		}
		return nil
	})
	return byTrip, err
}

type withdrawalResult int

const (
	withdrawalDone withdrawalResult = iota
	withdrawalMissing
	withdrawalCruise
	withdrawalPaid
	withdrawalReviewed
)

func withdrawSignUp(ctx context.Context, q sqlcdb.Querier, tripID, userID int64) (withdrawalResult, error) {
	_, err := q.WithdrawUserTripEnrollment(ctx, sqlcdb.WithdrawUserTripEnrollmentParams{TripID: tripID, UserID: userID})
	if err == nil {
		return withdrawalDone, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return withdrawalDone, err
	}
	e, found, err := signUpOf(ctx, q, tripID, userID)
	switch {
	case err != nil:
		return withdrawalDone, err
	case !found:
		return withdrawalMissing, nil
	case e.CruiseID != nil:
		return withdrawalCruise, nil
	case (e.Status == enrollmentPending || e.Status == enrollmentWaitlisted) && e.PaymentStatus != "unpaid":
		return withdrawalPaid, nil
	default:
		return withdrawalReviewed, nil
	}
}
