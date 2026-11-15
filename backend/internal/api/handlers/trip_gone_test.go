package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestAssignToCompletedTrip_Conflict(t *testing.T) {
	tripGone := func(constraint string) error {
		return &pgconn.PgError{Code: "23503", ConstraintName: constraint}
	}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "person deleted or merged underneath", err: tripGone("crew_assignments_crew_member_id_fkey"), want: http.StatusNotFound},
		{name: "person already on the crew", err: &pgconn.PgError{Code: "23505", ConstraintName: "crew_assignments_voyage_member_uniq"}, want: http.StatusConflict},
		{name: "deadlock on every attempt", err: &pgconn.PgError{Code: "40P01"}, want: http.StatusConflict},
		{name: "other foreign key", err: tripGone("crew_assignments_voyage_id_fkey"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run("voyage crew/"+tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getVoyageFn:     func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
				getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) { return sqlcdb.CrewMember{ID: id}, nil },
				createVoyageCrewFn: func(context.Context, sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
					return sqlcdb.CrewAssignment{}, tt.err
				},
			}
			resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
				map[string]any{"crew_member_id": 1, "role": "mate"})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}

	enrollTests := []struct {
		name string
		err  error
		want int
	}{
		{name: "trip completed or deleted underneath", err: tripGone("cruise_enrollments_trip_id_fkey"), want: http.StatusConflict},
		{name: "other foreign key", err: tripGone("cruise_enrollments_user_id_fkey"), want: http.StatusInternalServerError},
	}
	for _, tt := range enrollTests {
		t.Run("enrollment/"+tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
				assignCruiseEnrollmentFn:      func(context.Context, sqlcdb.AssignCruiseEnrollmentToTripParams) error { return tt.err },
				lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
					return sqlcdb.LockTripForSignUpRow{ID: id}, nil
				},
			}
			resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
				"/cruises/1/enrollments/2/trip", map[string]any{"trip_id": 5})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}
