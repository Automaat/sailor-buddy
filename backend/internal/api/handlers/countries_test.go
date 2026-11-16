package handlers

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestListCountries(t *testing.T) {
	_, api := humatest.New(t)
	RegisterCountryRoutes(api)
	resp := api.GetCtx(userCtxRole(context.Background(), "member"), "/countries")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var list []dto.Country
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 249 {
		t.Errorf("len = %d, want 249", len(list))
	}
	for _, want := range []dto.Country{{Code: "DK", Name: "Dania"}, {Code: "NO", Name: "Norwegia"}} {
		if !slices.Contains(list, want) {
			t.Errorf("list lacks %+v", want)
		}
	}
}

// TestCountries_SavedAsCodes checks every form's endpoint stores the codes
// it is sent, in order, and an omitted list as empty.
func TestCountries_SavedAsCodes(t *testing.T) {
	var stored types.TextArray
	m := &mockQuerier{
		createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			stored = arg.Countries
			return sqlcdb.Trip{ID: 1, Name: arg.Name, Countries: arg.Countries}, nil
		},
		updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
			stored = arg.Countries
			return nil
		},
		createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
			stored = arg.Countries
			return sqlcdb.Voyage{ID: 1, Name: arg.Name, Countries: arg.Countries}, nil
		},
		createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
			stored = arg.Countries
			return sqlcdb.Cruise{ID: 1, Name: arg.Name, Countries: arg.Countries}, nil
		},
		updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
			stored = arg.Countries
			return nil
		},
		getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{ID: id, CruiseID: 1, Status: sqlcdb.TripStatusPlanned}, nil
		},
	}
	tests := []struct {
		name   string
		call   func(body map[string]any) *httptest.ResponseRecorder
		body   map[string]any
		status int
	}{
		{"create trip", func(b map[string]any) *httptest.ResponseRecorder {
			return tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", b)
		}, map[string]any{"cruise_id": 1}, http.StatusCreated},
		{"update trip", func(b map[string]any) *httptest.ResponseRecorder {
			return tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", b)
		}, map[string]any{"cruise_id": 1}, http.StatusNoContent},
		{"create voyage", func(b map[string]any) *httptest.ResponseRecorder {
			return voyageTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages", b)
		}, map[string]any{"cruise_id": 1}, http.StatusCreated},
		{"create cruise", func(b map[string]any) *httptest.ResponseRecorder {
			return cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", asEdition(b))
		}, map[string]any{}, http.StatusCreated},
		{"update cruise", func(b map[string]any) *httptest.ResponseRecorder {
			return cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", b)
		}, map[string]any{}, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, c := range []struct {
				countries any
				want      []string
			}{
				{[]string{"NO", "DK"}, []string{"NO", "DK"}},
				{nil, nil},
			} {
				body := map[string]any{"name": "Rejs"}
				maps.Copy(body, tt.body)
				if c.countries != nil {
					body["countries"] = c.countries
				}
				stored = types.TextArray{"stale"}
				resp := tt.call(body)
				if resp.Code != tt.status {
					t.Fatalf("countries %v: status = %d, want %d; body=%s", c.countries, resp.Code, tt.status, resp.Body)
				}
				if !slices.Equal(stored, c.want) {
					t.Errorf("countries %v: stored %q, want %q", c.countries, stored, c.want)
				}
				if v, err := stored.Value(); err != nil || (c.want == nil && v != "{}") {
					t.Errorf("countries %v: stored as %v (%v), want {}", c.countries, v, err)
				}
			}
		})
	}
}

// TestCountries_RejectsUnknownCodes checks every form's endpoint answers 422,
// naming the bad item, for a code that is not on the list, a lower-case or
// three-letter code and a repeated code, and writes nothing.
func TestCountries_RejectsUnknownCodes(t *testing.T) {
	m := &mockQuerier{}
	endpoints := []struct {
		name string
		call func(body map[string]any) *httptest.ResponseRecorder
	}{
		{"create trip", func(b map[string]any) *httptest.ResponseRecorder {
			b["cruise_id"] = 1
			return tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", b)
		}},
		{"update trip", func(b map[string]any) *httptest.ResponseRecorder {
			b["cruise_id"] = 1
			return tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", b)
		}},
		{"create voyage", func(b map[string]any) *httptest.ResponseRecorder {
			b["cruise_id"] = 1
			return voyageTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages", b)
		}},
		{"update voyage", func(b map[string]any) *httptest.ResponseRecorder {
			b["cruise_id"] = 1
			return voyageTestAPI(t, m).PutCtx(userCtx(context.Background()), "/voyages/1", b)
		}},
		{"create cruise", func(b map[string]any) *httptest.ResponseRecorder {
			return cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", asEdition(b))
		}},
		{"update cruise", func(b map[string]any) *httptest.ResponseRecorder {
			return cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", b)
		}},
	}
	bad := []struct {
		name      string
		countries []string
		location  string
	}{
		{"unknown code", []string{"NO", "XX"}, "body.countries[1]"},
		{"lower case", []string{"no"}, "body.countries[0]"},
		{"three letters", []string{"NOR"}, "body.countries[0]"},
		{"repeated", []string{"NO", "NO"}, "body.countries"},
	}
	for _, e := range endpoints {
		for _, b := range bad {
			t.Run(e.name+"/"+b.name, func(t *testing.T) {
				resp := e.call(map[string]any{"name": "Rejs", "countries": b.countries})
				if resp.Code != http.StatusUnprocessableEntity {
					t.Fatalf("status = %d, want 422; body=%s", resp.Code, resp.Body)
				}
				if !strings.Contains(resp.Body.String(), `"location":"`+b.location+`"`) {
					t.Errorf("body = %s, want an error at %s", resp.Body, b.location)
				}
			})
		}
	}
}

// TestCompleteTrip_CopiesCountries checks completing a leg gives the voyage
// the leg's countries.
func TestCompleteTrip_CopiesCountries(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	if _, err := conn.Exec(`UPDATE trips SET countries = '{NO,SE}' WHERE id = $1`, leg.tripID); err != nil {
		t.Fatalf("set trip countries: %v", err)
	}
	voyage, err := completeTripTx(context.Background(), conn, leg.tripID, dto.CompleteTripBody{})
	if err != nil {
		t.Fatalf("complete trip: %v", err)
	}
	if !slices.Equal(voyage.Countries, types.TextArray{"NO", "SE"}) {
		t.Errorf("voyage countries = %q, want [NO SE]", voyage.Countries)
	}
}
