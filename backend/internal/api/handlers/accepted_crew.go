package handlers

import (
	"context"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// addAcceptedCrew puts userID's person on the trip's crew list as a
// załogant, marked as added by acceptance. A person already on the list
// keeps their entry. Run it in the transaction that accepts the sign-up.
func addAcceptedCrew(ctx context.Context, q sqlcdb.Querier, tripID, userID int64) error {
	if err := q.EnsureUserPerson(ctx, userID); err != nil {
		return &QueryError{Op: "EnsureUserPerson", Err: err}
	}
	if err := q.AddAcceptedTripCrewAssignment(ctx, sqlcdb.AddAcceptedTripCrewAssignmentParams{
		TripID: tripID,
		UserID: userID,
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
	if err := q.RemoveAcceptedTripCrewAssignment(ctx, sqlcdb.RemoveAcceptedTripCrewAssignmentParams{
		TripID: tripID,
		UserID: userID,
	}); err != nil {
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
