package handlers

import (
	"context"
	"database/sql"
	"errors"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// personLockAttempts bounds how often lockUserPerson looks the account's
// person up again after it was merged away or deleted under the lookup.
const personLockAttempts = 3

// errPersonChanged means the account's person kept changing during
// acceptance: the caller should answer 409 and let the admin retry.
var errPersonChanged = errors.New("the account's person changed during acceptance")

// errPersonChangedMessage is the 409 detail for errPersonChanged and for an
// acceptance that kept losing deadlocks.
const errPersonChangedMessage = "the person changed meanwhile: reload and try again"

// lockUserPerson returns the id of userID's person, creating it when the
// account has none yet, and key-share-locks it, so a person merge or delete
// waits for the caller's transaction. When a merge deleted the person while
// the lock waited, the lookup runs again and finds the kept person, which
// now holds the account link. errPersonChanged means it kept changing.
func lockUserPerson(ctx context.Context, q sqlcdb.Querier, userID int64) (int64, error) {
	user := types.NullInt64{Int64: userID, Valid: true}
	for range personLockAttempts {
		if err := q.EnsureUserPerson(ctx, userID); err != nil {
			return 0, &QueryError{Op: "EnsureUserPerson", Err: err}
		}
		personID, err := q.LockUserPerson(ctx, user)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, &QueryError{Op: "LockUserPerson", Err: err}
		}
		return personID, nil
	}
	return 0, errPersonChanged
}

// addAcceptedCrew puts userID's person on the trip's crew list as a
// załogant, marked as added by acceptance, for an accepted cruise
// enrollment. Run it in the transaction that accepts the enrollment.
func addAcceptedCrew(ctx context.Context, q sqlcdb.Querier, tripID, userID int64) error {
	personID, err := lockUserPerson(ctx, q, userID)
	if err != nil {
		return err
	}
	return addAcceptedPersonCrew(ctx, q, tripID, personID)
}

// addAcceptedPersonCrew puts the person on the trip's crew list as a
// załogant, marked as added by acceptance. A person already on the list
// keeps their entry. Run it in the transaction that accepts the sign-up.
func addAcceptedPersonCrew(ctx context.Context, q sqlcdb.Querier, tripID, personID int64) error {
	if err := q.AddAcceptedTripCrewAssignment(ctx, sqlcdb.AddAcceptedTripCrewAssignmentParams{
		TripID:       tripID,
		CrewMemberID: personID,
	}); err != nil {
		return &QueryError{Op: "AddAcceptedTripCrewAssignment", Err: err}
	}
	return nil
}

// removeAcceptedCrew takes userID's person off the trip's crew list when
// acceptance added them and they hold no accepted sign-up for the trip any
// more. Crew added by hand stays. Run it after the sign-up change, in the
// same transaction.
func removeAcceptedCrew(ctx context.Context, q sqlcdb.Querier, tripID, userID int64) error {
	return removeAcceptedCrewOf(ctx, q, sqlcdb.RemoveAcceptedTripCrewAssignmentParams{
		TripID: tripID,
		UserID: types.NullInt64{Int64: userID, Valid: true},
	})
}

// removeAcceptedPersonCrew is removeAcceptedCrew for a person.
func removeAcceptedPersonCrew(ctx context.Context, q sqlcdb.Querier, tripID, personID int64) error {
	return removeAcceptedCrewOf(ctx, q, sqlcdb.RemoveAcceptedTripCrewAssignmentParams{
		TripID:       tripID,
		CrewMemberID: types.NullInt64{Int64: personID, Valid: true},
	})
}

func removeAcceptedCrewOf(ctx context.Context, q sqlcdb.Querier, arg sqlcdb.RemoveAcceptedTripCrewAssignmentParams) error {
	if err := q.RemoveAcceptedTripCrewAssignment(ctx, arg); err != nil {
		return &QueryError{Op: "RemoveAcceptedTripCrewAssignment", Err: err}
	}
	return nil
}

// syncCruiseEnrollmentCrew updates the crew lists after a cruise enrollment
// changed from before to after: the leg it left loses the acceptance-added
// entry, and the leg it is accepted on gains one.
func syncCruiseEnrollmentCrew(ctx context.Context, q sqlcdb.Querier, before, after sqlcdb.CruiseEnrollment) error {
	if before.TripID.Valid && (before.TripID != after.TripID || after.Status != enrollmentAccepted) {
		if err := removeAcceptedCrew(ctx, q, before.TripID.Int64, before.UserID); err != nil {
			return err
		}
	}
	if after.TripID.Valid && after.Status == enrollmentAccepted {
		return addAcceptedCrew(ctx, q, after.TripID.Int64, after.UserID)
	}
	return nil
}
