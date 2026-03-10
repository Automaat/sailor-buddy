package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestOrgHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listUserOrganizationsFn: func(_ context.Context, userID int64) ([]sqlcdb.ListUserOrganizationsRow, error) {
				return []sqlcdb.ListUserOrganizationsRow{{ID: 1, Name: "Org One", Slug: "org-one", Role: "admin"}}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listUserOrganizationsFn: func(context.Context, int64) ([]sqlcdb.ListUserOrganizationsRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(_ context.Context, arg sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Name: arg.Name, Slug: arg.Slug}, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{ID: 1}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Sailing Club","slug":"sailing-club"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"slug":"abc"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing slug", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Org"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid slug", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Org","slug":"has spaces!"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("slug conflict", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(context.Context, sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, &pgconn.PgError{Code: "23505"}
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Org","slug":"taken"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("got %d, want %d", w.Code, http.StatusConflict)
		}
	})

	t.Run("db error on create", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(context.Context, sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Org","slug":"org"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("add member db error", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(context.Context, sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Name: "Org", Slug: "org"}, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Org","slug":"org"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(_ context.Context, slug string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Name: "Org", Slug: slug}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/test-org", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", "test-org")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, sql.ErrNoRows
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/nope", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", "nope")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/test-org", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", "test-org")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateOrganizationFn: func(context.Context, sqlcdb.UpdateOrganizationParams) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"name":"Updated Org"}`))
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateOrganizationFn: func(context.Context, sqlcdb.UpdateOrganizationParams) error { return errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrganizationFn: func(context.Context, int64) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrganizationFn: func(context.Context, int64) error { return errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_ListMembers(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 1, Role: "admin", UserName: "Alice"}}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.ListMembers(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.ListMembers(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_UpdateMemberRole(t *testing.T) {
	t.Parallel()

	t.Run("success admin role", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgMemberRoleFn: func(context.Context, sqlcdb.UpdateOrgMemberRoleParams) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"admin"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("success non-admin to non-last", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				// member 2 is not admin, so no admin check needed
				return []sqlcdb.ListOrgMembersRow{
					{ID: 1, Role: "admin"},
					{ID: 2, Role: "captain"},
				}, nil
			},
			updateOrgMemberRoleFn: func(context.Context, sqlcdb.UpdateOrgMemberRoleParams) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid member id", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"superuser"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("demote last admin", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 2, Role: "admin"}}, nil
			},
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 1, nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("list members db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("count admins db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 2, Role: "admin"}}, nil
			},
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 0, errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("update role db error", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgMemberRoleFn: func(context.Context, sqlcdb.UpdateOrgMemberRoleParams) error { return errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"role":"admin"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.UpdateMemberRole(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_RemoveMember(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 2, nil },
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{
					{ID: 1, Role: "admin"},
					{ID: 2, Role: "crew"},
				}, nil
			},
			removeOrgMemberFn: func(context.Context, sqlcdb.RemoveOrgMemberParams) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid member id", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("remove last admin", func(t *testing.T) {
		m := &mockQuerier{
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 1, nil },
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 2, Role: "admin"}}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("count admins db error", func(t *testing.T) {
		m := &mockQuerier{
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 0, errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("list members db error", func(t *testing.T) {
		m := &mockQuerier{
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 2, nil },
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("remove db error", func(t *testing.T) {
		m := &mockQuerier{
			countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 2, nil },
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 2, Role: "crew"}}, nil
			},
			removeOrgMemberFn: func(context.Context, sqlcdb.RemoveOrgMemberParams) error { return errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("memberID", "2")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveMember(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_CreateInvite(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrgInviteFn: func(_ context.Context, arg sqlcdb.CreateOrgInviteParams) (sqlcdb.OrgInvite, error) {
				return sqlcdb.OrgInvite{ID: 1, OrgID: arg.OrgID, Token: arg.Token, Role: arg.Role}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.CreateInvite(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.CreateInvite(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid expires_in", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"role":"crew","expires_in_hours":0}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.CreateInvite(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid max_uses", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"role":"crew","max_uses":0}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.CreateInvite(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createOrgInviteFn: func(context.Context, sqlcdb.CreateOrgInviteParams) (sqlcdb.OrgInvite, error) {
				return sqlcdb.OrgInvite{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"role":"crew"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.CreateInvite(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_ListInvites(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listOrgInvitesFn: func(context.Context, int64) ([]sqlcdb.ListOrgInvitesRow, error) {
				return []sqlcdb.ListOrgInvitesRow{{ID: 1, Role: "crew"}}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.ListInvites(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgInvitesFn: func(context.Context, int64) ([]sqlcdb.ListOrgInvitesRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.ListInvites(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_DeleteInvite(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgInviteFn: func(context.Context, sqlcdb.DeleteOrgInviteParams) error { return nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("inviteID", "5")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.DeleteInvite(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid invite id", func(t *testing.T) {
		h := NewOrgHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("inviteID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.DeleteInvite(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgInviteFn: func(context.Context, sqlcdb.DeleteOrgInviteParams) error { return errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("inviteID", "5")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.DeleteInvite(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_AcceptInvite(t *testing.T) {
	t.Parallel()

	validInvite := sqlcdb.GetOrgInviteByTokenRow{
		ID:    1,
		OrgID: 1,
		Token: "abc123",
		Role:  "crew",
	}

	t.Run("success no max uses", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return validInvite, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{ID: 1}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("success with max uses", func(t *testing.T) {
		invite := validInvite
		invite.MaxUses = sql.NullInt64{Int64: 5, Valid: true}
		invite.UseCount = 4
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
			incrementInviteUseCountFn: func(context.Context, int64) (int64, error) { return 1, nil },
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{ID: 1}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{}, sql.ErrNoRows
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "nope")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired", func(t *testing.T) {
		invite := validInvite
		invite.ExpiresAt = sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusGone {
			t.Fatalf("got %d, want %d", w.Code, http.StatusGone)
		}
	})

	t.Run("max uses reached rows=0", func(t *testing.T) {
		invite := validInvite
		invite.MaxUses = sql.NullInt64{Int64: 5, Valid: true}
		invite.UseCount = 5
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
			incrementInviteUseCountFn: func(context.Context, int64) (int64, error) { return 0, nil },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusGone {
			t.Fatalf("got %d, want %d", w.Code, http.StatusGone)
		}
	})

	t.Run("already member", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return validInvite, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{}, &pgconn.PgError{Code: "23505"}
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("got %d, want %d", w.Code, http.StatusConflict)
		}
	})

	t.Run("get invite db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("increment use count db error", func(t *testing.T) {
		invite := validInvite
		invite.MaxUses = sql.NullInt64{Int64: 5, Valid: true}
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
			incrementInviteUseCountFn: func(context.Context, int64) (int64, error) { return 0, errors.New("fail") },
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("add member db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return validInvite, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{}, errors.New("fail")
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AcceptInvite(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgHandler_GetInviteInfo(t *testing.T) {
	t.Parallel()

	validInvite := sqlcdb.GetOrgInviteByTokenRow{
		ID:      1,
		OrgID:   1,
		Token:   "abc123",
		Role:    "crew",
		OrgName: "Test Org",
		OrgSlug: "test-org",
	}

	t.Run("success not member", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return validInvite, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetInviteInfo(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("success already member", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return validInvite, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{ID: 1, Role: "crew"}, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetInviteInfo(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{}, sql.ErrNoRows
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "nope")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetInviteInfo(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired", func(t *testing.T) {
		invite := validInvite
		invite.ExpiresAt = sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetInviteInfo(w, req)
		if w.Code != http.StatusGone {
			t.Fatalf("got %d, want %d", w.Code, http.StatusGone)
		}
	})

	t.Run("max uses reached", func(t *testing.T) {
		invite := validInvite
		invite.MaxUses = sql.NullInt64{Int64: 5, Valid: true}
		invite.UseCount = 5
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return invite, nil
			},
		}
		h := NewOrgHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("token", "abc123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetInviteInfo(w, req)
		if w.Code != http.StatusGone {
			t.Fatalf("got %d, want %d", w.Code, http.StatusGone)
		}
	})
}
