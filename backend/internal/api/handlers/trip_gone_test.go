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
		{name: "trip completed or deleted underneath", err: tripGone("crew_assignments_trip_id_fkey"), want: http.StatusConflict},
		{name: "other foreign key", err: tripGone("crew_assignments_crew_member_id_fkey"), want: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run("crew/"+tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getTripFn:       func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
				getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) { return sqlcdb.CrewMember{ID: id}, nil },
				createTripCrewFn: func(context.Context, sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
					return sqlcdb.CrewAssignment{}, tt.err
				},
			}
			resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
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
				assignCruiseEnrollmentFn: func(context.Context, sqlcdb.AssignCruiseEnrollmentToTripParams) error { return tt.err },
			}
			resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
				"/cruises/1/enrollments/2/trip", map[string]any{"trip_id": 5})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}
