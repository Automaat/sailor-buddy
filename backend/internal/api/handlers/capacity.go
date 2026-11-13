package handlers

import (
	"context"
	"fmt"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Enrollment statuses that carry capacity rules.
const (
	enrollmentPending    = "pending"
	enrollmentAccepted   = "accepted"
	enrollmentWaitlisted = "waitlisted"
)

// crewLimit returns a leg's crew limit. A NULL or 0 max_crew means "no
// limit" (the trip forms store 0 for it), so ok is false.
func crewLimit(maxCrew types.NullInt64) (limit int64, ok bool) {
	if maxCrew.Valid && maxCrew.Int64 > 0 {
		return maxCrew.Int64, true
	}
	return 0, false
}

// initialSignUpStatus is the status a new sign-up for a leg starts with:
// pending while accepted plus pending sign-ups hold fewer places than the
// leg's crew limit and nobody is waiting, waitlisted otherwise, so a
// newcomer never gets ahead of an earlier waitlisted sign-up. A leg without
// a limit never waitlists. Run it in the transaction that locked the leg, so
// concurrent sign-ups count one another.
func initialSignUpStatus(ctx context.Context, q sqlcdb.Querier, tripID int64, maxCrew types.NullInt64) (string, error) {
	limit, ok := crewLimit(maxCrew)
	if !ok {
		return enrollmentPending, nil
	}
	use, err := q.CountTripPlaceUse(ctx, tripID)
	if err != nil {
		return "", &QueryError{Op: "CountTripPlaceUse", Err: err}
	}
	if use.Taken >= limit || use.Waitlisted > 0 {
		return enrollmentWaitlisted, nil
	}
	return enrollmentPending, nil
}

// legFullError aborts an accept when the leg has no free place.
type legFullError struct{ limit int64 }

func (e *legFullError) Error() string {
	return fmt.Sprintf("leg is full: all %d places are taken", e.limit)
}

// checkPlaceFree returns a legFullError when accepting userID's sign-up
// would put the leg over its crew limit. Accepted crew of other users is
// counted, so re-accepting someone who already holds a place passes. Run it
// in the transaction that locked the leg, so concurrent accepts serialize.
func checkPlaceFree(ctx context.Context, q sqlcdb.Querier, tripID, userID int64, maxCrew types.NullInt64) error {
	limit, ok := crewLimit(maxCrew)
	if !ok {
		return nil
	}
	accepted, err := q.CountOtherAcceptedForTrip(ctx, sqlcdb.CountOtherAcceptedForTripParams{TripID: tripID, UserID: userID})
	if err != nil {
		return &QueryError{Op: "CountOtherAcceptedForTrip", Err: err}
	}
	if accepted >= limit {
		return &legFullError{limit: limit}
	}
	return nil
}

// waitlistPosition returns the 1-based waiting-list position of a waitlisted
// leg sign-up, ordered by sign-up time; ok is false for any other sign-up. A
// cruise enrollment assigned to the leg has no position: it predates leg
// sign-ups and admins manage it.
func waitlistPosition(ctx context.Context, q sqlcdb.Querier, e *dto.Enrollment) (pos int64, ok bool, err error) {
	if e == nil || e.Status != enrollmentWaitlisted || e.CruiseID != nil {
		return 0, false, nil
	}
	pos, err = q.GetWaitlistPosition(ctx, e.ID)
	if err != nil {
		return 0, false, &QueryError{Op: "GetWaitlistPosition", Err: err}
	}
	return pos, true, nil
}

// setWaitlistPosition fills *dst with e's waiting-list position when it has one.
func setWaitlistPosition(ctx context.Context, q sqlcdb.Querier, e *dto.Enrollment, dst **int64) error {
	pos, ok, err := waitlistPosition(ctx, q, e)
	if err != nil {
		return err
	}
	if ok {
		*dst = &pos
	}
	return nil
}
