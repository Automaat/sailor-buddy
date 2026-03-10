package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// ---- OrgYachtHandler ----

func TestOrgYachtHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listOrgYachtsFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "SY Club"}}, nil
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgYachtsFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Yacht, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgYachtHandler_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrgYachtFn: func(_ context.Context, arg sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: arg.ID, Name: "SY Club"}, nil
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrgYachtFn: func(context.Context, sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, sql.ErrNoRows
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgYachtFn: func(context.Context, sqlcdb.GetOrgYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgYachtHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrgYachtFn: func(_ context.Context, arg sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"SY Club"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createOrgYachtFn: func(context.Context, sqlcdb.CreateOrgYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"SY Club"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgYachtHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgYachtFn: func(context.Context, sqlcdb.UpdateOrgYachtParams) error { return nil },
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"Updated"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/abc", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgYachtFn: func(context.Context, sqlcdb.UpdateOrgYachtParams) error { return errors.New("fail") },
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgYachtHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgYachtFn: func(context.Context, sqlcdb.DeleteOrgYachtParams) error { return nil },
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgYachtFn: func(context.Context, sqlcdb.DeleteOrgYachtParams) error { return errors.New("fail") },
		}
		h := NewOrgYachtHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

// ---- OrgCruiseHandler ----

func TestOrgCruiseHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listOrgCruisesFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Cruise, error) {
				return []sqlcdb.Cruise{{ID: 1, Name: "Baltic 2024"}}, nil
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgCruisesFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Cruise, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCruiseHandler_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCruiseFn: func(_ context.Context, arg sqlcdb.GetOrgCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: arg.ID, Name: "Baltic 2024"}, nil
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCruiseFn: func(context.Context, sqlcdb.GetOrgCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCruiseFn: func(context.Context, sqlcdb.GetOrgCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCruiseHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrgCruiseFn: func(_ context.Context, arg sqlcdb.CreateOrgCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Baltic 2024"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createOrgCruiseFn: func(context.Context, sqlcdb.CreateOrgCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Baltic 2024"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCruiseHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgCruiseFn: func(context.Context, sqlcdb.UpdateOrgCruiseParams) error { return nil },
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"Updated"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/abc", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgCruiseFn: func(context.Context, sqlcdb.UpdateOrgCruiseParams) error { return errors.New("fail") },
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCruiseHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgCruiseFn: func(context.Context, sqlcdb.DeleteOrgCruiseParams) error { return nil },
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgCruiseFn: func(context.Context, sqlcdb.DeleteOrgCruiseParams) error { return errors.New("fail") },
		}
		h := NewOrgCruiseHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

// ---- OrgCrewHandler ----

func TestOrgCrewHandler_List(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listOrgCrewMembersFn: func(context.Context, sql.NullInt64) ([]sqlcdb.CrewMember, error) {
				return []sqlcdb.CrewMember{{ID: 1, FullName: "Alice"}}, nil
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listOrgCrewMembersFn: func(context.Context, sql.NullInt64) ([]sqlcdb.CrewMember, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCrewHandler_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCrewMemberFn: func(_ context.Context, arg sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: arg.ID, FullName: "Alice"}, nil
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCrewMemberFn: func(context.Context, sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgCrewMemberFn: func(context.Context, sqlcdb.GetOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCrewHandler_Create(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrgCrewMemberFn: func(_ context.Context, arg sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: 1, FullName: arg.FullName}, nil
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"full_name":"Alice"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createOrgCrewMemberFn: func(context.Context, sqlcdb.CreateOrgCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"full_name":"Alice"}`))
		req = req.WithContext(orgCtx(userCtx(req.Context())))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCrewHandler_Update(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgCrewMemberFn: func(context.Context, sqlcdb.UpdateOrgCrewMemberParams) error { return nil },
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"full_name":"Alice"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/abc", strings.NewReader(`{"full_name":"Alice"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader("{bad"))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateOrgCrewMemberFn: func(context.Context, sqlcdb.UpdateOrgCrewMemberParams) error { return errors.New("fail") },
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"full_name":"Alice"}`))
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestOrgCrewHandler_Delete(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgCrewMemberFn: func(context.Context, sqlcdb.DeleteOrgCrewMemberParams) error { return nil },
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewOrgCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteOrgCrewMemberFn: func(context.Context, sqlcdb.DeleteOrgCrewMemberParams) error { return errors.New("fail") },
		}
		h := NewOrgCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(orgCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

// ---- OrgDashboardHandler ----

func TestOrgDashboardHandler_Get(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrgDashboardStatsFn: func(context.Context, sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
				return sqlcdb.GetOrgDashboardStatsRow{}, nil
			},
			getOrgCruisesByYearFn: func(context.Context, sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error) {
				return []sqlcdb.GetOrgCruisesByYearRow{}, nil
			},
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{{ID: 1, Role: "admin"}}, nil
			},
			listOrgYachtsFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "SY Club"}}, nil
			},
		}
		h := NewOrgDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("stats error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgDashboardStatsFn: func(context.Context, sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
				return sqlcdb.GetOrgDashboardStatsRow{}, errors.New("fail")
			},
		}
		h := NewOrgDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("by-year error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgDashboardStatsFn: func(context.Context, sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
				return sqlcdb.GetOrgDashboardStatsRow{}, nil
			},
			getOrgCruisesByYearFn: func(context.Context, sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("members error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgDashboardStatsFn: func(context.Context, sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
				return sqlcdb.GetOrgDashboardStatsRow{}, nil
			},
			getOrgCruisesByYearFn: func(context.Context, sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error) {
				return []sqlcdb.GetOrgCruisesByYearRow{}, nil
			},
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("yachts error", func(t *testing.T) {
		m := &mockQuerier{
			getOrgDashboardStatsFn: func(context.Context, sql.NullInt64) (sqlcdb.GetOrgDashboardStatsRow, error) {
				return sqlcdb.GetOrgDashboardStatsRow{}, nil
			},
			getOrgCruisesByYearFn: func(context.Context, sql.NullInt64) ([]sqlcdb.GetOrgCruisesByYearRow, error) {
				return []sqlcdb.GetOrgCruisesByYearRow{}, nil
			},
			listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
				return []sqlcdb.ListOrgMembersRow{}, nil
			},
			listOrgYachtsFn: func(context.Context, sql.NullInt64) ([]sqlcdb.Yacht, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewOrgDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(orgCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
