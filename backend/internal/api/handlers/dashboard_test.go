package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func dashboardTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterDashboardRoutes(api, m)
	return api
}

func TestDashboardHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardFn: func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{VoyageCount: 3, TotalMiles: 420}, nil
			},
			getVoyagesByYearFn: func(context.Context) ([]sqlcdb.GetVoyagesByYearRow, error) {
				return []sqlcdb.GetVoyagesByYearRow{{VoyageCount: 3, TotalMiles: 420}}, nil
			},
		}
		resp := dashboardTestAPI(t, m).GetCtx(userCtx(context.Background()), "/dashboard")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("stats db error", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardFn: func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, errors.New("fail")
			},
		}
		resp := dashboardTestAPI(t, m).GetCtx(userCtx(context.Background()), "/dashboard")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("yearly db error", func(t *testing.T) {
		m := &mockQuerier{
			getDashboardFn: func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, nil
			},
			getVoyagesByYearFn: func(context.Context) ([]sqlcdb.GetVoyagesByYearRow, error) {
				return nil, errors.New("fail")
			},
		}
		resp := dashboardTestAPI(t, m).GetCtx(userCtx(context.Background()), "/dashboard")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
