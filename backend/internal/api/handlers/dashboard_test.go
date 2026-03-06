package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestDashboardHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardStatsFn: func(context.Context, int64) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{CruiseCount: 5, TotalMiles: 100}, nil
			},
			getCruisesByYearFn: func(context.Context, int64) ([]sqlcdb.GetCruisesByYearRow, error) {
				return []sqlcdb.GetCruisesByYearRow{{CruiseCount: 2}}, nil
			},
		}
		h := NewDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("nil by_year", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardStatsFn: func(context.Context, int64) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, nil
			},
			getCruisesByYearFn: func(context.Context, int64) ([]sqlcdb.GetCruisesByYearRow, error) {
				return nil, nil
			},
		}
		h := NewDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("stats db error", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardStatsFn: func(context.Context, int64) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, errors.New("fail")
			},
		}
		h := NewDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("yearly db error", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardStatsFn: func(context.Context, int64) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, nil
			},
			getCruisesByYearFn: func(context.Context, int64) ([]sqlcdb.GetCruisesByYearRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewDashboardHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
