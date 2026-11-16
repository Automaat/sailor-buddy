package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// storedPort is a start or end port as a write hands it to the database.
type storedPort struct {
	name     types.NullString
	lat, lon types.NullFloat64
}

func named(name string) storedPort {
	return storedPort{name: types.NullString{String: name, Valid: true}}
}

func located(name string, lat, lon float64) storedPort {
	p := named(name)
	p.lat = types.NullFloat64{Float64: lat, Valid: true}
	p.lon = types.NullFloat64{Float64: lon, Valid: true}
	return p
}

// TestLegPorts checks that every write of a trip's or voyage's start and
// end port applies the location rules: the name's blanks are normalized,
// coordinates are stored with it, an absent port or a blank name without
// coordinates is stored as NULL, and a too long name, unpaired or out of
// range coordinates, or coordinates without a name are refused before the
// write, at the field they are about (refusedAt; empty when accepted).
func TestLegPorts(t *testing.T) {
	long := strings.Repeat("ś", 101)
	longest := strings.Repeat("ś", 100)
	tests := []struct {
		name      string
		ports     map[string]any
		wantStart storedPort
		wantEnd   storedPort
		refusedAt string
	}{
		{
			name: "normalized name",
			ports: map[string]any{
				"start_port": map[string]any{"name": "  Gdynia \u00a0 Port\t"},
				"end_port":   map[string]any{"name": "Visby"},
			},
			wantStart: named("Gdynia Port"), wantEnd: named("Visby"),
		},
		{
			name: "coordinates",
			ports: map[string]any{
				"start_port": map[string]any{"name": "Hel", "latitude": 54.6, "longitude": 18.8},
				"end_port":   map[string]any{"name": "Visby", "latitude": -90, "longitude": 180},
			},
			wantStart: located("Hel", 54.6, 18.8), wantEnd: located("Visby", -90, 180),
		},
		{
			name:      "empty name is no port",
			ports:     map[string]any{"start_port": map[string]any{"name": ""}, "end_port": map[string]any{"name": "Visby"}},
			wantStart: storedPort{}, wantEnd: named("Visby"),
		},
		{
			name:      "blank name is no port",
			ports:     map[string]any{"start_port": map[string]any{"name": "Hel"}, "end_port": map[string]any{"name": " \u200b\t "}},
			wantStart: named("Hel"), wantEnd: storedPort{},
		},
		{name: "absent is no port", ports: map[string]any{}},
		{
			name:      "longest name",
			ports:     map[string]any{"start_port": map[string]any{"name": " " + longest + " "}},
			wantStart: named(longest),
		},
		{
			name:      "start name too long",
			ports:     map[string]any{"start_port": map[string]any{"name": long}},
			refusedAt: "body.start_port.name",
		},
		{
			name:      "end name too long",
			ports:     map[string]any{"end_port": map[string]any{"name": long}},
			refusedAt: "body.end_port.name",
		},
		{
			name:      "latitude without longitude",
			ports:     map[string]any{"start_port": map[string]any{"name": "Hel", "latitude": 54.6}},
			refusedAt: "body.start_port.latitude",
		},
		{
			name:      "longitude without latitude",
			ports:     map[string]any{"end_port": map[string]any{"name": "Hel", "longitude": 18.8}},
			refusedAt: "body.end_port.latitude",
		},
		{
			name:      "latitude out of range",
			ports:     map[string]any{"start_port": map[string]any{"name": "Hel", "latitude": 90.5, "longitude": 18.8}},
			refusedAt: "body.start_port.latitude",
		},
		{
			name:      "longitude out of range",
			ports:     map[string]any{"end_port": map[string]any{"name": "Hel", "latitude": 54.6, "longitude": -180.5}},
			refusedAt: "body.end_port.longitude",
		},
		{
			name:      "coordinates without a name",
			ports:     map[string]any{"start_port": map[string]any{"name": " ", "latitude": 54.6, "longitude": 18.8}},
			refusedAt: "body.start_port.name",
		},
		{
			name:      "port without a name field",
			ports:     map[string]any{"start_port": map[string]any{"latitude": 54.6, "longitude": 18.8}},
			refusedAt: "body.start_port.name",
		},
		{
			name:      "port as a bare string",
			ports:     map[string]any{"start_port": "Hel"},
			refusedAt: "body.start_port",
		},
	}
	type stored struct{ start, end storedPort }
	writes := []struct {
		name string
		send func(t *testing.T, body map[string]any) (int, string, *stored)
		ok   int
	}{
		{"trip create", func(t *testing.T, body map[string]any) (int, string, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
				got = &stored{
					storedPort{arg.StartPort, arg.StartPortLatitude, arg.StartPortLongitude},
					storedPort{arg.EndPort, arg.EndPortLatitude, arg.EndPortLongitude},
				}
				return sqlcdb.Trip{ID: 1, Name: arg.Name}, nil
			}}
			resp := tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", body)
			return resp.Code, resp.Body.String(), got
		}, http.StatusCreated},
		{"trip update", func(t *testing.T, body map[string]any) (int, string, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id, CruiseID: 1}, nil },
				updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
					got = &stored{
						storedPort{arg.StartPort, arg.StartPortLatitude, arg.StartPortLongitude},
						storedPort{arg.EndPort, arg.EndPortLatitude, arg.EndPortLongitude},
					}
					return nil
				},
			}
			resp := tripTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/trips/3", body)
			return resp.Code, resp.Body.String(), got
		}, http.StatusNoContent},
		{"voyage create", func(t *testing.T, body map[string]any) (int, string, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				got = &stored{
					storedPort{arg.StartPort, arg.StartPortLatitude, arg.StartPortLongitude},
					storedPort{arg.EndPort, arg.EndPortLatitude, arg.EndPortLongitude},
				}
				return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
			}}
			resp := voyageTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages", body)
			return resp.Code, resp.Body.String(), got
		}, http.StatusCreated},
		{"voyage update", func(t *testing.T, body map[string]any) (int, string, *stored) {
			t.Helper()
			var got *stored
			m := &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
				updateVoyageFn: func(_ context.Context, arg sqlcdb.UpdateVoyageParams) error {
					got = &stored{
						storedPort{arg.StartPort, arg.StartPortLatitude, arg.StartPortLongitude},
						storedPort{arg.EndPort, arg.EndPortLatitude, arg.EndPortLongitude},
					}
					return nil
				},
			}
			resp := voyageTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/voyages/3", body)
			return resp.Code, resp.Body.String(), got
		}, http.StatusNoContent},
	}
	for _, w := range writes {
		for _, tt := range tests {
			t.Run(w.name+" "+tt.name, func(t *testing.T) {
				code, body, got := w.send(t, captainBody(tt.ports))
				if tt.refusedAt != "" {
					if code != http.StatusUnprocessableEntity || got != nil {
						t.Fatalf("status = %d, written = %v; want 422 and no write", code, got != nil)
					}
					if !strings.Contains(body, `"location":"`+tt.refusedAt+`"`) {
						t.Fatalf("error not reported at %s: %s", tt.refusedAt, body)
					}
					return
				}
				if code != w.ok || got == nil {
					t.Fatalf("status = %d, written = %v; want %d and a write; body=%s", code, got != nil, w.ok, body)
				}
				if got.start != tt.wantStart || got.end != tt.wantEnd {
					t.Fatalf("stored ports = %+v / %+v, want %+v / %+v", got.start, got.end, tt.wantStart, tt.wantEnd)
				}
			})
		}
	}
}

// TestLegPortsRead checks that a leg's start and end ports are read back as
// port objects: a name with the coordinates stored for it, the name alone
// for a port without them.
func TestLegPortsRead(t *testing.T) {
	want := map[string]any{"name": "Hel", "latitude": 54.6, "longitude": 18.8}
	reads := []struct {
		name string
		get  func(t *testing.T) []byte
	}{
		{"trip", func(t *testing.T) []byte {
			t.Helper()
			m := &mockQuerier{getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{
					ID: id, CruiseID: 1, Name: "Etap", Status: sqlcdb.TripStatusPlanned,
					StartPort:          types.NullString{String: "Hel", Valid: true},
					StartPortLatitude:  types.NullFloat64{Float64: 54.6, Valid: true},
					StartPortLongitude: types.NullFloat64{Float64: 18.8, Valid: true},
					EndPort:            types.NullString{String: "Visby", Valid: true},
				}, nil
			}}
			resp := tripTestAPI(t, m).GetCtx(callerCtx(1, "admin"), "/trips/3")
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", resp.Code, resp.Body)
			}
			return resp.Body.Bytes()
		}},
		{"voyage", func(t *testing.T) []byte {
			t.Helper()
			m := &mockQuerier{getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{
					ID: id, CruiseID: 1, Name: "Etap",
					StartPort:          types.NullString{String: "Hel", Valid: true},
					StartPortLatitude:  types.NullFloat64{Float64: 54.6, Valid: true},
					StartPortLongitude: types.NullFloat64{Float64: 18.8, Valid: true},
					EndPort:            types.NullString{String: "Visby", Valid: true},
				}, nil
			}}
			resp := voyageTestAPI(t, m).GetCtx(callerCtx(1, "admin"), "/voyages/3")
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", resp.Code, resp.Body)
			}
			return resp.Body.Bytes()
		}},
	}
	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			var got map[string]any
			if err := json.Unmarshal(r.get(t), &got); err != nil {
				t.Fatal(err)
			}
			start, ok := got["start_port"].(map[string]any)
			if !ok || len(start) != len(want) {
				t.Fatalf("start_port = %v, want %v", got["start_port"], want)
			}
			for k, v := range want {
				if start[k] != v {
					t.Fatalf("start_port = %v, want %v", start, want)
				}
			}
			end, ok := got["end_port"].(map[string]any)
			if !ok || len(end) != 1 || end["name"] != "Visby" {
				t.Fatalf("end_port = %v, want only the name Visby", got["end_port"])
			}
		})
	}
}
