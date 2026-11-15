package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCrewHandler_List_Errors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		m := &mockQuerier{
			listCrewMembersFn: func(context.Context, sqlcdb.ListCrewMembersParams) ([]sqlcdb.ListCrewMembersRow, error) {
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
			listCrewMembersFn: func(context.Context, sqlcdb.ListCrewMembersParams) ([]sqlcdb.ListCrewMembersRow, error) {
				return nil, nil
			},
			countCrewMembersFn: func(context.Context, types.NullString) (int64, error) { return 0, errors.New("fail") },
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
			getCrewMemberForCallerFn: func(_ context.Context, arg sqlcdb.GetCrewMemberForCallerParams) (sqlcdb.GetCrewMemberForCallerRow, error) {
				return sqlcdb.GetCrewMemberForCallerRow{CrewMember: sqlcdb.CrewMember{ID: arg.ID, FullName: "Jan"}}, nil
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberForCallerFn: func(context.Context, sqlcdb.GetCrewMemberForCallerParams) (sqlcdb.GetCrewMemberForCallerRow, error) {
				return sqlcdb.GetCrewMemberForCallerRow{}, sql.ErrNoRows
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberForCallerFn: func(context.Context, sqlcdb.GetCrewMemberForCallerParams) (sqlcdb.GetCrewMemberForCallerRow, error) {
				return sqlcdb.GetCrewMemberForCallerRow{}, errors.New("fail")
			},
		}
		resp := crewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

// TestCrewHandler_EmergencyContactsFollowRow checks the person detail and
// list pass the caller's admin flag and account to the query and keep the
// emergency contact only where the row allows it.
func TestCrewHandler_EmergencyContactsFollowRow(t *testing.T) {
	person := sqlcdb.CrewMember{
		ID: 7, FullName: "Jan", Phone: types.NullString{String: "+48 600", Valid: true},
		EmergencyContactName:  types.NullString{String: "Ewa", Valid: true},
		EmergencyContactPhone: types.NullString{String: "+48 700", Valid: true},
	}
	for _, tt := range []struct {
		name    string
		caller  context.Context
		admin   bool
		visible bool
	}{
		{name: "admin", caller: callerCtx(4, "admin"), admin: true, visible: true},
		{name: "captain", caller: callerCtx(4, "member"), visible: true},
		{name: "member", caller: callerCtx(4, "member"), visible: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var matched []bool
			want := types.NullInt64{Int64: 4, Valid: true}
			m := &mockQuerier{
				getCrewMemberForCallerFn: func(_ context.Context, arg sqlcdb.GetCrewMemberForCallerParams) (sqlcdb.GetCrewMemberForCallerRow, error) {
					matched = append(matched, arg.IsAdmin == tt.admin && arg.CallerUserID == want)
					return sqlcdb.GetCrewMemberForCallerRow{CrewMember: person, ContactsVisible: tt.visible}, nil
				},
				listCrewMembersFn: func(_ context.Context, arg sqlcdb.ListCrewMembersParams) ([]sqlcdb.ListCrewMembersRow, error) {
					matched = append(matched, arg.IsAdmin == tt.admin && arg.CallerUserID == want)
					return []sqlcdb.ListCrewMembersRow{{CrewMember: person, ContactsVisible: tt.visible}}, nil
				},
				countCrewMembersFn: func(context.Context, types.NullString) (int64, error) { return 1, nil },
			}
			api := crewTestAPI(t, m)
			var one dto.CrewMember
			var page dto.Page[dto.CrewMember]
			if err := json.Unmarshal(api.GetCtx(tt.caller, "/crew/7").Body.Bytes(), &one); err != nil {
				t.Fatalf("decode person: %v", err)
			}
			if err := json.Unmarshal(api.GetCtx(tt.caller, "/crew").Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
				t.Fatalf("decode people: %v", err)
			}
			if !slices.Equal(matched, []bool{true, true}) {
				t.Fatalf("query caller params matched = %v, want admin = %v and account 4 on both", matched, tt.admin)
			}
			for _, got := range []dto.CrewMember{one, page.Items[0]} {
				present := got.EmergencyContactName != nil && got.EmergencyContactPhone != nil
				absent := got.EmergencyContactName == nil && got.EmergencyContactPhone == nil
				if tt.visible && !present || !tt.visible && !absent || got.Phone == nil {
					t.Errorf("person = %+v, want emergency contact present = %v and the phone kept", got, tt.visible)
				}
			}
		})
	}
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
		m := &mockQuerier{deleteCrewMemberFn: func(context.Context, int64) (int64, error) { return 1, nil }}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("retries a deadlock victim", func(t *testing.T) {
		calls := 0
		m := &mockQuerier{deleteCrewMemberFn: func(context.Context, int64) (int64, error) {
			calls++
			if calls == 1 {
				return 0, &pgconn.PgError{Code: "40P01"}
			}
			return 1, nil
		}}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusNoContent || calls != 2 {
			t.Fatalf("got %d after %d calls, want 204 after 2", resp.Code, calls)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{deleteCrewMemberFn: func(context.Context, int64) (int64, error) { return 0, errors.New("fail") }}
		resp := crewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/crew/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCrewHandler_ListTripCrew_DBError(t *testing.T) {
	m := &mockQuerier{
		getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
		listTripCrewFn: func(context.Context, sqlcdb.ListTripCrewAssignmentsParams) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
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
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
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
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
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
	if NewCrewHandler(&mockQuerier{}, nil, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
