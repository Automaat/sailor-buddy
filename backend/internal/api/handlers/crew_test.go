package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func crewTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerCrewRoutes(api, &CrewHandler{q: m, runTx: directTxRunner(m)})
	return api
}

func TestCrewHandler_List(t *testing.T) {
	m := &mockQuerier{
		listCrewMembersFn: func(context.Context, sqlcdb.ListCrewMembersParams) ([]sqlcdb.ListCrewMembersRow, error) {
			return []sqlcdb.ListCrewMembersRow{{CrewMember: sqlcdb.CrewMember{ID: 1, FullName: "Jan Nowak"}}}, nil
		},
		countCrewMembersFn: func(context.Context, types.NullString) (int64, error) { return 1, nil },
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

// TestCrewHandler_TripCrewHasNoAddRoute checks a planned leg's crew cannot
// be added by hand: it comes only from accepted sign-ups.
func TestCrewHandler_TripCrewHasNoAddRoute(t *testing.T) {
	resp := crewTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/trips/9/crew",
		map[string]any{"crew_member_id": 1, "role": "first_mate"})
	if resp.Code != http.StatusMethodNotAllowed && resp.Code != http.StatusNotFound {
		t.Fatalf("POST /trips/9/crew = %d, want 405 or 404", resp.Code)
	}
}

func TestCrewHandler_UpdateTripCrewRole(t *testing.T) {
	row := func(arg sqlcdb.UpdateTripCrewAssignmentRoleParams) sqlcdb.UpdateTripCrewAssignmentRoleRow {
		return sqlcdb.UpdateTripCrewAssignmentRoleRow{
			ID: arg.ID, TripID: types.NullInt64{Int64: arg.TripID, Valid: true}, CrewMemberID: 4,
			Role: arg.Role, FullName: "Ola Nowak",
		}
	}
	tests := []struct {
		name     string
		ctx      context.Context
		body     map[string]any
		found    bool
		want     int
		wantRole string
	}{
		{name: "admin sets a trimmed role", ctx: userCtx(context.Background()), body: map[string]any{"role": "  oficer "}, found: true, want: http.StatusOK, wantRole: "oficer"},
		{name: "blank role", ctx: userCtx(context.Background()), body: map[string]any{"role": "   "}, found: true, want: http.StatusUnprocessableEntity},
		{name: "empty role", ctx: userCtx(context.Background()), body: map[string]any{"role": ""}, found: true, want: http.StatusUnprocessableEntity},
		{name: "missing role", ctx: userCtx(context.Background()), body: map[string]any{}, found: true, want: http.StatusUnprocessableEntity},
		{name: "too long role", ctx: userCtx(context.Background()), body: map[string]any{"role": strings.Repeat("ż", dto.MaxCrewRoleLen+1)}, found: true, want: http.StatusUnprocessableEntity},
		{name: "entry of another trip", ctx: userCtx(context.Background()), body: map[string]any{"role": "oficer"}, want: http.StatusNotFound},
		{name: "member", ctx: userCtxRole(context.Background(), "member"), body: map[string]any{"role": "oficer"}, found: true, want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []sqlcdb.UpdateTripCrewAssignmentRoleParams
			m := &mockQuerier{
				updateTripCrewRoleFn: func(_ context.Context, arg sqlcdb.UpdateTripCrewAssignmentRoleParams) (sqlcdb.UpdateTripCrewAssignmentRoleRow, error) {
					got = append(got, arg)
					if !tt.found {
						return sqlcdb.UpdateTripCrewAssignmentRoleRow{}, sql.ErrNoRows
					}
					return row(arg), nil
				},
			}
			resp := crewTestAPI(t, m).PatchCtx(tt.ctx, "/trips/9/crew/3", tt.body)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusOK && tt.want != http.StatusNotFound {
				if len(got) != 0 {
					t.Fatalf("refused change wrote %+v", got)
				}
				return
			}
			want := sqlcdb.UpdateTripCrewAssignmentRoleParams{Role: "oficer", ID: 3, TripID: 9}
			if len(got) != 1 || got[0] != want {
				t.Fatalf("update = %+v, want [%+v]", got, want)
			}
			if tt.want != http.StatusOK {
				return
			}
			var out dto.CrewAssignment
			if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if out.ID != 3 || out.Role != tt.wantRole || out.FullName != "Ola Nowak" {
				t.Fatalf("response = %+v, want entry 3 of Ola Nowak as %s", out, tt.wantRole)
			}
		})
	}
}

func TestCrewHandler_AssignVoyageCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id}, nil
			},
			getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: id}, nil
			},
			createVoyageCrewFn: func(_ context.Context, arg sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{ID: 1, CrewMemberID: arg.CrewMemberID, Role: arg.Role, VoyageID: types.NullInt64{Int64: 9, Valid: true}}, nil
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("crew member of another owner rejected", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id}, nil
			},
			getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
		}
		resp := crewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/9/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestCrewHandler_ListTripCrew(t *testing.T) {
	m := &mockQuerier{
		getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
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

func TestCrewHandler_Create_MemberForbidden(t *testing.T) {
	resp := crewTestAPI(t, &mockQuerier{}).PostCtx(
		userCtxRole(context.Background(), "member"), "/crew", map[string]any{"full_name": "Anna"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
	}
}
