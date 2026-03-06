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

func TestTrainingHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listTrainingsFn: func(context.Context, int64) ([]sqlcdb.Training, error) {
				return []sqlcdb.Training{{ID: 1, Name: "RYA Day Skipper"}}, nil
			},
		}
		h := NewTrainingHandler(m)
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
			listTrainingsFn: func(context.Context, int64) ([]sqlcdb.Training, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewTrainingHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestTrainingHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTrainingFn: func(_ context.Context, arg sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: arg.ID, Name: "RYA"}, nil
			},
		}
		h := NewTrainingHandler(m)
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
		h := NewTrainingHandler(&mockQuerier{})
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
			getTrainingFn: func(context.Context, sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, sql.ErrNoRows
			},
		}
		h := NewTrainingHandler(m)
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
			getTrainingFn: func(context.Context, sqlcdb.GetTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("fail")
			},
		}
		h := NewTrainingHandler(m)
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

func TestTrainingHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createTrainingFn: func(_ context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewTrainingHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"RYA"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewTrainingHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewTrainingHandler(&mockQuerier{})
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
			createTrainingFn: func(context.Context, sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("fail")
			},
		}
		h := NewTrainingHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestTrainingHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateTrainingFn: func(context.Context, sqlcdb.UpdateTrainingParams) error { return nil },
		}
		h := NewTrainingHandler(m)
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
		h := NewTrainingHandler(&mockQuerier{})
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
		h := NewTrainingHandler(&mockQuerier{})
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
			updateTrainingFn: func(context.Context, sqlcdb.UpdateTrainingParams) error { return errors.New("fail") },
		}
		h := NewTrainingHandler(m)
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

func TestTrainingHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteTrainingFn: func(context.Context, sqlcdb.DeleteTrainingParams) error { return nil },
		}
		h := NewTrainingHandler(m)
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
		h := NewTrainingHandler(&mockQuerier{})
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
			deleteTrainingFn: func(context.Context, sqlcdb.DeleteTrainingParams) error { return errors.New("fail") },
		}
		h := NewTrainingHandler(m)
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
