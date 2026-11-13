package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestTripAndVoyageWrites_RequireCruise(t *testing.T) {
	missing := func(context.Context, int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{}, sql.ErrNoRows }
	broken := func(context.Context, int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{}, errors.New("db down") }
	tests := []struct {
		name      string
		method    string
		path      string
		body      map[string]any
		getCruise func(context.Context, int64) (sqlcdb.Cruise, error)
		want      int
	}{
		{name: "trip create without cruise", method: http.MethodPost, path: "/trips", body: map[string]any{"name": "X"}, want: http.StatusUnprocessableEntity},
		{name: "trip create with cruise 0", method: http.MethodPost, path: "/trips", body: map[string]any{"name": "X", "cruise_id": 0}, want: http.StatusUnprocessableEntity},
		{name: "trip create with unknown cruise", method: http.MethodPost, path: "/trips", body: map[string]any{"name": "X", "cruise_id": 99}, getCruise: missing, want: http.StatusUnprocessableEntity},
		{name: "trip update with unknown cruise", method: http.MethodPut, path: "/trips/1", body: map[string]any{"name": "X", "cruise_id": 99}, getCruise: missing, want: http.StatusUnprocessableEntity},
		{name: "trip create when cruise lookup fails", method: http.MethodPost, path: "/trips", body: map[string]any{"name": "X", "cruise_id": 5}, getCruise: broken, want: http.StatusInternalServerError},
		{name: "voyage create without cruise", method: http.MethodPost, path: "/voyages", body: map[string]any{"name": "X"}, want: http.StatusUnprocessableEntity},
		{name: "voyage create with unknown cruise", method: http.MethodPost, path: "/voyages", body: map[string]any{"name": "X", "cruise_id": 99}, getCruise: missing, want: http.StatusUnprocessableEntity},
		{name: "voyage update with unknown cruise", method: http.MethodPut, path: "/voyages/1", body: map[string]any{"name": "X", "cruise_id": 99}, getCruise: missing, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{getCruiseFn: tt.getCruise}
			api := tripTestAPI(t, m)
			if tt.path == "/voyages" || tt.path == "/voyages/1" {
				api = voyageTestAPI(t, m)
			}
			resp := api.DoCtx(userCtx(context.Background()), tt.method, tt.path, tt.body)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}

func TestCruiseDelete_RefusedWhileLegsRemain(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "planned trips remain", err: &pgconn.PgError{Code: "23503", ConstraintName: "trips_cruise_id_fkey"}, want: http.StatusConflict},
		{name: "logged voyages remain", err: &pgconn.PgError{Code: "23503", ConstraintName: "voyages_cruise_id_fkey"}, want: http.StatusConflict},
		{name: "other foreign key", err: &pgconn.PgError{Code: "23503", ConstraintName: "something_else"}, want: http.StatusInternalServerError},
		{name: "no legs", err: nil, want: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{deleteCruiseFn: func(context.Context, int64) error { return tt.err }}
			resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/3")
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}

func TestTripUpdate_MoveToAnotherCruise(t *testing.T) {
	tests := []struct {
		name       string
		cruiseID   int64
		linked     int64
		countErr   error
		want       int
		wantCount  bool
		wantUpdate bool
	}{
		{name: "same cruise skips the check", cruiseID: 1, want: http.StatusNoContent, wantUpdate: true},
		{name: "move without enrollments", cruiseID: 2, want: http.StatusNoContent, wantCount: true, wantUpdate: true},
		{name: "move with enrollments refused", cruiseID: 2, linked: 3, want: http.StatusConflict, wantCount: true},
		{name: "count failure", cruiseID: 2, countErr: errors.New("db down"), want: http.StatusInternalServerError, wantCount: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counted, updated := false, false
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{ID: id, CruiseID: 1}, nil
				},
				countTripLinkedFn: func(context.Context, int64) (int64, error) {
					counted = true
					return tt.linked, tt.countErr
				},
				updateTripFn: func(context.Context, sqlcdb.UpdateTripParams) error {
					updated = true
					return nil
				},
			}
			resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/5",
				map[string]any{"name": "Etap", "cruise_id": tt.cruiseID})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if counted != tt.wantCount || updated != tt.wantUpdate {
				t.Fatalf("counted=%v updated=%v, want %v %v", counted, updated, tt.wantCount, tt.wantUpdate)
			}
		})
	}
}
