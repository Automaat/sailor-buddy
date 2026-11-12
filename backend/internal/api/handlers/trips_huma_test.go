package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// tripTestAPI builds a huma test API with the trip routes registered against
// the given mock querier.
func tripTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterTripRoutes(api, m, nil)
	return api
}

func TestListTrips_Huma(t *testing.T) {
	m := &mockQuerier{
		listTripsFn: func(context.Context, sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
			return []sqlcdb.Trip{{ID: 7, Name: "Adriatic", Status: sqlcdb.TripStatusPlanned}}, nil
		},
		countTripsFn: func(context.Context) (int64, error) { return 1, nil },
	}
	resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var page dto.Page[dto.Trip]
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Adriatic" {
		t.Fatalf("unexpected trips: %+v", page.Items)
	}
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1", page.Total)
	}
}

func TestGetTrip_Huma_NotFound(t *testing.T) {
	m := &mockQuerier{
		getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, sql.ErrNoRows
		},
	}
	resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/99")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.Code, resp.Body)
	}
	// huma emits the RFC 9457 problem+json envelope.
	var body struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Detail != "trip not found" {
		t.Fatalf("detail = %q, want %q", body.Detail, "trip not found")
	}
}

func TestCreateTrip_Huma_MissingName(t *testing.T) {
	// huma rejects the empty body before the handler runs; the querier is
	// never called.
	resp := tripTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/trips", map[string]any{})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", resp.Code, resp.Body)
	}
}

func TestUpdateTrip_Huma(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn:    func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			updateTripFn: func(context.Context, sqlcdb.UpdateTripParams) error { return nil },
		}
		resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", map[string]any{"name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := tripTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/trips/1", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", resp.Code)
		}
	})
}

func TestDeleteTrip_Huma(t *testing.T) {
	m := &mockQuerier{
		deleteTripFn: func(context.Context, int64) error { return nil },
	}
	resp := tripTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/1")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.Code)
	}
}

func TestCancelTrip_Huma(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			cancelTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: id, Status: sqlcdb.TripStatusCancelled}, nil
			},
		}
		resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/1/cancel")
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("invalid transition", func(t *testing.T) {
		m := &mockQuerier{
			cancelTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, sql.ErrNoRows
			},
		}
		resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/1/cancel")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.Code)
		}
	})
}

func TestCreateTrip_Huma(t *testing.T) {
	m := &mockQuerier{
		createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{ID: 1, Name: arg.Name, Status: sqlcdb.TripStatusPlanned}, nil
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", map[string]any{"name": "Baltic"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var trip dto.Trip
	if err := json.Unmarshal(resp.Body.Bytes(), &trip); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if trip.Name != "Baltic" || trip.Status != "planned" {
		t.Fatalf("unexpected trip: %+v", trip)
	}
}

func TestCreateTrip_Huma_MemberForbidden(t *testing.T) {
	resp := tripTestAPI(t, &mockQuerier{}).PostCtx(
		userCtxRole(context.Background(), "member"), "/trips", map[string]any{"name": "X"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", resp.Code, resp.Body)
	}
}

func TestCreateTrip_Huma_MaxCrewFromBerths(t *testing.T) {
	yachtWithBerths := func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
		if id != 4 {
			return sqlcdb.Yacht{}, sql.ErrNoRows
		}
		return sqlcdb.Yacht{ID: 4, Name: "Ilmatar", Berths: types.NullInt64{Int64: 10, Valid: true}}, nil
	}
	cases := []struct {
		name    string
		body    map[string]any
		yacht   func(context.Context, int64) (sqlcdb.Yacht, error)
		want    types.NullInt64
		wantRes int
	}{
		{
			name: "defaults to yacht berths", body: map[string]any{"name": "B", "yacht_id": 4},
			yacht: yachtWithBerths, want: types.NullInt64{Int64: 10, Valid: true}, wantRes: http.StatusCreated,
		},
		{
			name: "explicit max_crew kept", body: map[string]any{"name": "B", "yacht_id": 4, "max_crew": 6},
			want: types.NullInt64{Int64: 6, Valid: true}, wantRes: http.StatusCreated,
		},
		{
			name: "yacht without berths leaves limit empty", body: map[string]any{"name": "B", "yacht_id": 4},
			yacht: func(context.Context, int64) (sqlcdb.Yacht, error) { return sqlcdb.Yacht{ID: 4}, nil },
			want:  types.NullInt64{}, wantRes: http.StatusCreated,
		},
		{
			name: "zero max_crew means no limit and defaults to berths", body: map[string]any{"name": "B", "yacht_id": 4, "max_crew": 0},
			yacht: yachtWithBerths, want: types.NullInt64{Int64: 10, Valid: true}, wantRes: http.StatusCreated,
		},
		{
			name: "no yacht leaves limit empty", body: map[string]any{"name": "B"},
			want: types.NullInt64{}, wantRes: http.StatusCreated,
		},
		{
			name: "unknown yacht", body: map[string]any{"name": "B", "yacht_id": 9},
			yacht: yachtWithBerths, wantRes: http.StatusUnprocessableEntity,
		},
		{
			name: "yacht lookup error", body: map[string]any{"name": "B", "yacht_id": 4},
			yacht:   func(context.Context, int64) (sqlcdb.Yacht, error) { return sqlcdb.Yacht{}, errors.New("fail") },
			wantRes: http.StatusInternalServerError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got sqlcdb.CreateTripParams
			m := &mockQuerier{
				getYachtFn: tc.yacht,
				createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					got = arg
					return sqlcdb.Trip{ID: 1, Name: arg.Name, MaxCrew: arg.MaxCrew, Status: sqlcdb.TripStatusPlanned}, nil
				},
			}
			resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", tc.body)
			if resp.Code != tc.wantRes {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tc.wantRes, resp.Body)
			}
			if tc.wantRes != http.StatusCreated {
				return
			}
			if got.MaxCrew != tc.want {
				t.Fatalf("max_crew = %+v, want %+v", got.MaxCrew, tc.want)
			}
		})
	}
}
