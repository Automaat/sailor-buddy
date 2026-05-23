package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCrewHandler_List_Errors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		m := &mockQuerier{
			listCrewMembersFn: func(context.Context, sqlcdb.ListCrewMembersParams) ([]sqlcdb.CrewMember, error) {
				return nil, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("count error", func(t *testing.T) {
		m := &mockQuerier{
			listCrewMembersFn:  func(context.Context, sqlcdb.ListCrewMembersParams) ([]sqlcdb.CrewMember, error) { return nil, nil },
			countCrewMembersFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: id, FullName: "Jan"}, nil
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_Create_DBError(t *testing.T) {
	m := &mockQuerier{
		createCrewMemberFn: func(context.Context, sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
			return sqlcdb.CrewMember{}, errors.New("fail")
		},
	}
	resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/crew", map[string]any{"full_name": "Anna"})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestCrewHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateCrewMemberFn: func(context.Context, sqlcdb.UpdateCrewMemberParams) error { return nil },
		}
		resp := crewTestAPI(t, m).PutCtx(userCtx(context.Background()), "/crew/1", map[string]any{"full_name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing full_name", func(t *testing.T) {
		resp := crewTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/crew/1", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateCrewMemberFn: func(context.Context, sqlcdb.UpdateCrewMemberParams) error { return errors.New("fail") },
		}
		resp := crewTestAPI(t, m).PutCtx(userCtx(context.Background()), "/crew/1", map[string]any{"full_name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := crewTestAPI(t, &mockQuerier{}).PutCtx(
			userCtxRole(context.Background(), "member"), "/crew/1", map[string]any{"full_name": "X"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestCrewHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{deleteCrewMemberFn: func(context.Context, int64) error { return nil }}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{deleteCrewMemberFn: func(context.Context, int64) error { return errors.New("fail") }}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_ListTripCrew_DBError(t *testing.T) {
	m := &mockQuerier{
		listTripCrewFn: func(context.Context, types.NullInt64) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
			return nil, errors.New("fail")
		},
	}
	resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/9/crew")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestCrewHandler_ListVoyageCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listVoyageCrewFn: func(context.Context, types.NullInt64) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error) {
				return []sqlcdb.ListVoyageCrewAssignmentsRow{{ID: 1, CrewMemberID: 2, Role: "skipper", FullName: "Jan"}}, nil
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/9/crew")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listVoyageCrewFn: func(context.Context, types.NullInt64) ([]sqlcdb.ListVoyageCrewAssignmentsRow, error) {
				return nil, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/9/crew")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_RemoveTripCrew_Success(t *testing.T) {
	m := &mockQuerier{
		deleteTripCrewFn: func(context.Context, sqlcdb.DeleteTripCrewAssignmentParams) error { return nil },
	}
	resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/crew/1")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", resp.Code)
	}
}

func TestCrewHandler_RemoveVoyageCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteVoyageCrewFn: func(context.Context, sqlcdb.DeleteVoyageCrewAssignmentParams) error { return nil },
		}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/9/crew/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteVoyageCrewFn: func(context.Context, sqlcdb.DeleteVoyageCrewAssignmentParams) error {
				return errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/9/crew/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_AssignTripCrew_Errors(t *testing.T) {
	t.Run("verify trip db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("verify crew member db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("create assignment db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn:       func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) { return sqlcdb.CrewMember{ID: id}, nil },
			createTripCrewFn: func(context.Context, sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_AssignVoyageCrew_Errors(t *testing.T) {
	t.Run("voyage not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("verify voyage db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("create assignment db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn:     func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) { return sqlcdb.CrewMember{ID: id}, nil },
			createVoyageCrewFn: func(context.Context, sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
			map[string]any{"crew_member_id": 1, "role": "mate"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestNewCrewHandler(t *testing.T) {
	if NewCrewHandler(&mockQuerier{}) == nil {
		t.Fatal("want non-nil handler")
	}
}
