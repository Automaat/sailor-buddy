package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func membersTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerMembersRoutes(api, &MembersHandler{q: m, runTx: directTxRunner(m)})
	return api
}

func TestMembersHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listUsersFn: func(context.Context) ([]sqlcdb.User, error) {
				return []sqlcdb.User{{ID: 1, Name: "Ann", Email: "ann@x", Role: "admin"}}, nil
			},
		}
		resp := membersTestAPI(t, m).GetCtx(userCtx(context.Background()), "/members")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listUsersFn: func(context.Context) ([]sqlcdb.User, error) { return nil, errors.New("fail") },
		}
		resp := membersTestAPI(t, m).GetCtx(userCtx(context.Background()), "/members")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestMembersHandler_UpdateRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "member"}, nil
			},
			lockAdminsFn:     func(context.Context) ([]int64, error) { return []int64{1}, nil },
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return nil },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("member not found", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) {
				return sqlcdb.User{}, sql.ErrNoRows
			},
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("get member db error", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) {
				return sqlcdb.User{}, errors.New("fail")
			},
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("refuses to demote the last admin", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "admin"}, nil
			},
			lockAdminsFn: func(context.Context) ([]int64, error) { return []int64{2}, nil },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "member"})
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.Code)
		}
	})

	t.Run("demotes admin when others remain", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "admin"}, nil
			},
			lockAdminsFn:     func(context.Context) ([]int64, error) { return []int64{1, 2}, nil },
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return nil },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "member"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("lock admins error", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "admin"}, nil
			},
			lockAdminsFn: func(context.Context) ([]int64, error) { return nil, errors.New("fail") },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "member"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("update role db error", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "member"}, nil
			},
			lockAdminsFn:     func(context.Context) ([]int64, error) { return []int64{1}, nil },
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return errors.New("fail") },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("demotes a non-admin while one admin remains", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "member"}, nil
			},
			lockAdminsFn:     func(context.Context) ([]int64, error) { return []int64{1}, nil },
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return nil },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "member"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := membersTestAPI(t, &mockQuerier{}).PutCtx(
			userCtxRole(context.Background(), "member"), "/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestMembersHandler_UpdateAccess(t *testing.T) {
	accessMock := func(target sqlcdb.User, saved *[]sqlcdb.UpdateUserSuspendedParams) *mockQuerier {
		return &mockQuerier{
			lockUserForUpdateFn: func(_ context.Context, id int64) (int64, error) {
				if id != target.ID {
					return 0, sql.ErrNoRows
				}
				return id, nil
			},
			getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) { return target, nil },
			updateUserSuspendedFn: func(_ context.Context, arg sqlcdb.UpdateUserSuspendedParams) error {
				*saved = append(*saved, arg)
				return nil
			},
		}
	}
	tests := []struct {
		name      string
		role      string
		target    sqlcdb.User
		path      string
		suspended bool
		want      int
		wantSaved []sqlcdb.UpdateUserSuspendedParams
	}{
		{
			name: "suspends a member", role: "admin",
			target: sqlcdb.User{ID: 2, Role: "member"}, path: "/members/2/access", suspended: true,
			want: http.StatusNoContent, wantSaved: []sqlcdb.UpdateUserSuspendedParams{{ID: 2, Suspended: true}},
		},
		{
			name: "restores a suspended member", role: "admin",
			target: sqlcdb.User{ID: 2, Role: "member", Suspended: true}, path: "/members/2/access", suspended: false,
			want: http.StatusNoContent, wantSaved: []sqlcdb.UpdateUserSuspendedParams{{ID: 2, Suspended: false}},
		},
		{
			name: "refuses to suspend an admin", role: "admin",
			target: sqlcdb.User{ID: 2, Role: "admin"}, path: "/members/2/access", suspended: true,
			want: http.StatusConflict,
		},
		{
			name: "member forbidden", role: "member",
			target: sqlcdb.User{ID: 2, Role: "member"}, path: "/members/2/access", suspended: true,
			want: http.StatusForbidden,
		},
		{
			name: "unknown account", role: "admin",
			target: sqlcdb.User{ID: 2, Role: "member"}, path: "/members/9/access", suspended: true,
			want: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saved []sqlcdb.UpdateUserSuspendedParams
			resp := membersTestAPI(t, accessMock(tt.target, &saved)).PutCtx(
				userCtxRole(context.Background(), tt.role), tt.path, map[string]any{"suspended": tt.suspended})
			if resp.Code != tt.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if !slices.Equal(saved, tt.wantSaved) {
				t.Errorf("saved %+v, want %+v", saved, tt.wantSaved)
			}
		})
	}

	t.Run("rejects the removed approved field", func(t *testing.T) {
		var saved []sqlcdb.UpdateUserSuspendedParams
		resp := membersTestAPI(t, accessMock(sqlcdb.User{ID: 2, Role: "member"}, &saved)).PutCtx(
			userCtx(context.Background()), "/members/2/access", map[string]any{"approved": true, "suspended": false})
		if resp.Code != http.StatusUnprocessableEntity || len(saved) != 0 {
			t.Fatalf("got %d saved=%+v, want 422 and no write; body=%s", resp.Code, saved, resp.Body)
		}
	})
}

func TestNewMembersHandler(t *testing.T) {
	if NewMembersHandler(&mockQuerier{}, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
