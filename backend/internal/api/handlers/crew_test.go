package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func crewTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterCrewRoutes(api, m)
	return api
}

func TestCrewHandler_List(t *testing.T) {
	m := &mockQuerier{
		listCrewMembersFn: func(context.Context, int64) ([]sqlcdb.CrewMember, error) {
			return []sqlcdb.CrewMember{{ID: 1, FullName: "Jan Nowak"}}, nil
		},
	}
	resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}

func TestCrewHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createCrewMemberFn: func(_ context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: 1, FullName: arg.FullName}, nil
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/crew", map[string]any{"full_name": "Anna"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing full_name", func(t *testing.T) {
		resp := crewTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/crew", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})
}

func TestCrewHandler_AssignTripCrew(t *testing.T) {
	t.Run("trip not found", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(context.Context, sqlcdb.GetTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, sql.ErrNoRows
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(_ context.Context, arg sqlcdb.GetTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: arg.ID}, nil
			},
			createTripCrewFn: func(_ context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{ID: 1, CrewMemberID: arg.CrewMemberID, Role: arg.Role, TripID: types.NullInt64{Int64: 9, Valid: true}}, nil
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		resp := crewTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})
}

func TestCrewHandler_ListTripCrew(t *testing.T) {
	m := &mockQuerier{
		listTripCrewFn: func(context.Context, sqlcdb.ListTripCrewAssignmentsParams) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
			return []sqlcdb.ListTripCrewAssignmentsRow{{ID: 1, CrewMemberID: 2, Role: "skipper", FullName: "Jan"}}, nil
		},
	}
	resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/9/crew")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}

func TestCrewHandler_RemoveTripCrew_DBError(t *testing.T) {
	m := &mockQuerier{
		deleteTripCrewFn: func(context.Context, sqlcdb.DeleteTripCrewAssignmentParams) error {
			return errors.New("fail")
		},
	}
	resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/crew/1")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}
