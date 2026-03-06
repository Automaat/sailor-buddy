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

func TestCruiseHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCruisesFn: func(_ context.Context, ownerID int64) ([]sqlcdb.Cruise, error) {
				return []sqlcdb.Cruise{{ID: 1, Name: "Med Trip"}}, nil
			},
		}
		h := NewCruiseHandler(m)
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
			listCruisesFn: func(context.Context, int64) ([]sqlcdb.Cruise, error) {
				return nil, errors.New("db down")
			},
		}
		h := NewCruiseHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCruiseHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: arg.ID, Name: "Trip"}, nil
			},
		}
		h := NewCruiseHandler(m)
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
		h := NewCruiseHandler(&mockQuerier{})
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
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		h := NewCruiseHandler(m)
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
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewCruiseHandler(m)
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

func TestCruiseHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name, OwnerID: arg.OwnerID}, nil
			},
		}
		h := NewCruiseHandler(m)
		body := `{"name":"Baltic Cruise"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewCruiseHandler(&mockQuerier{})
		body := `{"year":2024}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewCruiseHandler(&mockQuerier{})
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
			createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewCruiseHandler(m)
		body := `{"name":"Test"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCruiseHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateCruiseFn: func(context.Context, sqlcdb.UpdateCruiseParams) error {
				return nil
			},
		}
		h := NewCruiseHandler(m)
		body := `{"name":"Updated"}`
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(body))
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
		h := NewCruiseHandler(&mockQuerier{})
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
		h := NewCruiseHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"year":2024}`))
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
			updateCruiseFn: func(context.Context, sqlcdb.UpdateCruiseParams) error {
				return errors.New("fail")
			},
		}
		h := NewCruiseHandler(m)
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

func TestCruiseHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteCruiseFn: func(context.Context, sqlcdb.DeleteCruiseParams) error {
				return nil
			},
		}
		h := NewCruiseHandler(m)
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
		h := NewCruiseHandler(&mockQuerier{})
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
			deleteCruiseFn: func(context.Context, sqlcdb.DeleteCruiseParams) error {
				return errors.New("fail")
			},
		}
		h := NewCruiseHandler(m)
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
