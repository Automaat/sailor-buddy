package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
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
		listTripsFn: func(_ context.Context, ownerID int64) ([]sqlcdb.Trip, error) {
			return []sqlcdb.Trip{{ID: 7, OwnerID: ownerID, Name: "Adriatic", Status: sqlcdb.TripStatusPlanned}}, nil
		},
		countTripsFn: func(context.Context, int64) (int64, error) { return 1, nil },
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
		getTripFn: func(context.Context, sqlcdb.GetTripParams) (sqlcdb.Trip, error) {
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
		deleteTripFn: func(context.Context, sqlcdb.DeleteTripParams) error { return nil },
	}
	resp := tripTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/1")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.Code)
	}
}

func TestCancelTrip_Huma(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			cancelTripFn: func(_ context.Context, arg sqlcdb.CancelTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: arg.ID, Status: sqlcdb.TripStatusCancelled}, nil
			},
		}
		resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/1/cancel")
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("invalid transition", func(t *testing.T) {
		m := &mockQuerier{
			cancelTripFn: func(context.Context, sqlcdb.CancelTripParams) (sqlcdb.Trip, error) {
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
			return sqlcdb.Trip{ID: 1, OwnerID: arg.OwnerID, Name: arg.Name, Status: sqlcdb.TripStatusPlanned}, nil
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
