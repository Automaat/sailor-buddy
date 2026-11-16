package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// TestLegPortNames checks that every write of a trip's or voyage's start
// and end port applies the location name rule: blanks around and inside
// the name are normalized, a blank or absent port is stored as NULL, and a
// name longer than 100 characters is refused before the write.
func TestLegPortNames(t *testing.T) {
	long := strings.Repeat("ś", 101)
	longest := strings.Repeat("ś", 100)
	tests := []struct {
		name      string
		ports     map[string]any
		accepted  bool
		wantStart types.NullString
		wantEnd   types.NullString
	}{
		{
			"normalized",
			map[string]any{"start_port": "  Gdynia \u00a0 Port\t", "end_port": "Visby"},
			true,
			types.NullString{String: "Gdynia Port", Valid: true},
			types.NullString{String: "Visby", Valid: true},
		},
		{
			"empty is no port",
			map[string]any{"start_port": "", "end_port": "Visby"},
			true,
			types.NullString{},
			types.NullString{String: "Visby", Valid: true},
		},
		{
			"blank is no port",
			map[string]any{"start_port": "Hel", "end_port": " \u200b\t "},
			true,
			types.NullString{String: "Hel", Valid: true},
			types.NullString{},
		},
		{"absent is no port", map[string]any{}, true, types.NullString{}, types.NullString{}},
		{
			"longest name",
			map[string]any{"start_port": " " + longest + " "},
			true,
			types.NullString{String: longest, Valid: true},
			types.NullString{},
		},
		{"start too long", map[string]any{"start_port": long}, false, types.NullString{}, types.NullString{}},
		{"end too long", map[string]any{"end_port": long}, false, types.NullString{}, types.NullString{}},
	}
	type stored struct{ start, end types.NullString }
	writes := []struct {
		name string
		send func(t *testing.T, body map[string]any) (int, *stored)
		ok   int
	}{
		{"trip create", func(t *testing.T, body map[string]any) (int, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
				got = &stored{arg.StartPort, arg.EndPort}
				return sqlcdb.Trip{ID: 1, Name: arg.Name}, nil
			}}
			return tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", body).Code, got
		}, http.StatusCreated},
		{"trip update", func(t *testing.T, body map[string]any) (int, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id, CruiseID: 1}, nil },
				updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
					got = &stored{arg.StartPort, arg.EndPort}
					return nil
				},
			}
			return tripTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/trips/3", body).Code, got
		}, http.StatusNoContent},
		{"voyage create", func(t *testing.T, body map[string]any) (int, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				got = &stored{arg.StartPort, arg.EndPort}
				return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
			}}
			return voyageTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages", body).Code, got
		}, http.StatusCreated},
		{"voyage update", func(t *testing.T, body map[string]any) (int, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
				updateVoyageFn: func(_ context.Context, arg sqlcdb.UpdateVoyageParams) error {
					got = &stored{arg.StartPort, arg.EndPort}
					return nil
				},
			}
			return voyageTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/voyages/3", body).Code, got
		}, http.StatusNoContent},
	}
	for _, w := range writes {
		for _, tt := range tests {
			t.Run(w.name+" "+tt.name, func(t *testing.T) {
				code, got := w.send(t, captainBody(tt.ports))
				if !tt.accepted {
					if code != http.StatusUnprocessableEntity || got != nil {
						t.Fatalf("status = %d, written = %v; want 422 and no write", code, got != nil)
					}
					return
				}
				if code != w.ok || got == nil {
					t.Fatalf("status = %d, written = %v; want %d and a write", code, got != nil, w.ok)
				}
				if got.start != tt.wantStart || got.end != tt.wantEnd {
					t.Fatalf("stored ports = %+v / %+v, want %+v / %+v", got.start, got.end, tt.wantStart, tt.wantEnd)
				}
			})
		}
	}
}
