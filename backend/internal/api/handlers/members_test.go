package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func membersTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterMembersRoutes(api, m)
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
			countAdminsFn: func(context.Context) (int64, error) { return 1, nil },
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
			countAdminsFn:    func(context.Context) (int64, error) { return 2, nil },
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return nil },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "member"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("count admins error", func(t *testing.T) {
		m := &mockQuerier{
			getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
				return sqlcdb.User{ID: id, Role: "admin"}, nil
			},
			countAdminsFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
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
			updateUserRoleFn: func(context.Context, sqlcdb.UpdateUserRoleParams) error { return errors.New("fail") },
		}
		resp := membersTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/members/2/role", map[string]any{"role": "admin"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
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

func TestNewMembersHandler(t *testing.T) {
	if NewMembersHandler(&mockQuerier{}) == nil {
		t.Fatal("want non-nil handler")
	}
}
