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

func TestYachtHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn: func(context.Context, int64) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "SY Odyssey"}}, nil
			},
		}
		h := NewYachtHandler(m)
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
			listYachtsFn: func(context.Context, int64) ([]sqlcdb.Yacht, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestYachtHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(_ context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: arg.ID, Name: "SY Odyssey"}, nil
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
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
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
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
			getYachtFn: func(context.Context, sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, sql.ErrNoRows
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
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
			getYachtFn: func(context.Context, sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
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

func TestYachtHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"SY Odyssey"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createYachtFn: func(context.Context, sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestYachtHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateYachtFn: func(context.Context, sqlcdb.UpdateYachtParams) error { return nil },
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"Updated"}`))
		req = req.WithContext(userCtx(req.Context()))
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
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/abc", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
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
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
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
			updateYachtFn: func(context.Context, sqlcdb.UpdateYachtParams) error { return errors.New("fail") },
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
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

func TestYachtHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteYachtFn: func(context.Context, sqlcdb.DeleteYachtParams) error { return nil },
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
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
		h := NewYachtHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
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
			deleteYachtFn: func(context.Context, sqlcdb.DeleteYachtParams) error { return errors.New("fail") },
		}
		h := NewYachtHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
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
