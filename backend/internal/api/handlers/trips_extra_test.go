package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestTripHandler_List_Errors(t *testing.T) {
	t.Run("list error", func(t *testing.T) {
		m := &mockQuerier{
			listTripsFn: func(context.Context, sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
				return nil, errors.New("fail")
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("count error", func(t *testing.T) {
		m := &mockQuerier{
			listTripsFn:  func(context.Context, sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) { return nil, nil },
			countTripsFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTripHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: id, Name: "Adriatic", Status: sqlcdb.TripStatusPlanned}, nil
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, errors.New("fail")
			},
		}
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestTripHandler_Create_DBError(t *testing.T) {
	m := &mockQuerier{
		createTripFn: func(context.Context, sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, errors.New("fail")
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", map[string]any{"name": "X"})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Update_DBError(t *testing.T) {
	m := &mockQuerier{
		updateTripFn: func(context.Context, sqlcdb.UpdateTripParams) error { return errors.New("fail") },
	}
	resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", map[string]any{"name": "X"})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Update_MemberForbidden(t *testing.T) {
	resp := tripTestAPI(t, &mockQuerier{}).PutCtx(
		userCtxRole(context.Background(), "member"), "/trips/1", map[string]any{"name": "X"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", resp.Code)
	}
}

func TestTripHandler_Delete_DBError(t *testing.T) {
	m := &mockQuerier{deleteTripFn: func(context.Context, int64) error { return errors.New("fail") }}
	resp := tripTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/1")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestTripHandler_Cancel_DBError(t *testing.T) {
	m := &mockQuerier{
		cancelTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, errors.New("fail")
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/1/cancel")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestNewTripHandler(t *testing.T) {
	if NewTripHandler(&mockQuerier{}, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
