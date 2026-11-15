package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type cruiseLogistics struct {
	TravelNotes *string `json:"travel_notes"`
	MinAge      *int64  `json:"min_age"`
}

func TestCruiseHandler_Logistics(t *testing.T) {
	full := map[string]any{
		"name":         "Sycylia 2026",
		"travel_notes": "Tanie loty do Katanii",
		"min_age":      18,
	}

	t.Run("create stores and returns both", func(t *testing.T) {
		var stored sqlcdb.CreateCruiseParams
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				stored = arg
				return sqlcdb.Cruise{
					ID: 1, Name: arg.Name, TravelNotes: arg.TravelNotes, MinAge: arg.MinAge,
				}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", asEdition(full))
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if stored.TravelNotes != (types.NullString{String: "Tanie loty do Katanii", Valid: true}) ||
			stored.MinAge != (types.NullInt64{Int64: 18, Valid: true}) {
			t.Fatalf("stored = %+v", stored)
		}
		var out cruiseLogistics
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.TravelNotes == nil || *out.TravelNotes != "Tanie loty do Katanii" ||
			out.MinAge == nil || *out.MinAge != 18 {
			t.Fatalf("response = %s", resp.Body)
		}
	})

	t.Run("update stores both and get reads them back", func(t *testing.T) {
		var row sqlcdb.Cruise
		m := &mockQuerier{
			updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
				row = sqlcdb.Cruise{
					ID: arg.ID, Name: arg.Name, TravelNotes: arg.TravelNotes, MinAge: arg.MinAge,
				}
				return nil
			},
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) { return row, nil },
		}
		api := cruiseTestAPI(t, m)
		ctx := userCtx(context.Background())
		if resp := api.PutCtx(ctx, "/cruises/5", full); resp.Code != http.StatusNoContent {
			t.Fatalf("put: got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		resp := api.GetCtx(userCtxRole(context.Background(), "member"), "/cruises/5")
		if resp.Code != http.StatusOK {
			t.Fatalf("get: got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var out cruiseLogistics
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.TravelNotes == nil || *out.TravelNotes != "Tanie loty do Katanii" ||
			out.MinAge == nil || *out.MinAge != 18 {
			t.Fatalf("response = %s", resp.Body)
		}
	})

	t.Run("blank or omitted fields store null", func(t *testing.T) {
		bodies := []map[string]any{
			{"name": "X"},
			{"name": "X", "travel_notes": "\n"},
		}
		for _, body := range bodies {
			var stored sqlcdb.UpdateCruiseParams
			m := &mockQuerier{
				updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
					stored = arg
					return nil
				},
			}
			resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", body)
			if resp.Code != http.StatusNoContent {
				t.Fatalf("body %v: got %d, want 204; body=%s", body, resp.Code, resp.Body)
			}
			if stored.TravelNotes.Valid || stored.MinAge.Valid {
				t.Fatalf("body %v: stored = %+v, want nulls", body, stored)
			}
		}
	})

	for _, age := range []any{-1, 100, 18.5, "18"} {
		t.Run("rejects invalid min_age", func(t *testing.T) {
			resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/cruises",
				asEdition(map[string]any{"name": "X", "min_age": age}))
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("min_age %v: got %d, want 422; body=%s", age, resp.Code, resp.Body)
			}
		})
	}

	t.Run("member cannot set logistics", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PutCtx(userCtxRole(context.Background(), "member"), "/cruises/1", full)
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestEnrollment_GetByToken_CruiseLogistics(t *testing.T) {
	newMock := func() *mockQuerier {
		return &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{
					ID: 7, Name: "Sycylia",
					TravelNotes: types.NullString{String: "Tanie loty do Katanii", Valid: true},
					MinAge:      types.NullInt64{Int64: 18, Valid: true},
				}, nil
			},
			countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{}, nil
			},
			listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
				return nil, nil
			},
			countAcceptedByCruiseTripFn: func(context.Context, int64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
				return nil, nil
			},
			listUserCruiseTripEnrollFn: func(context.Context, sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
				return nil, nil
			},
			getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
			},
		}
	}

	callers := []struct {
		name string
		ctx  context.Context
	}{
		{"logged-in member", userCtxRole(context.Background(), "member")},
		{"anonymous preview", context.Background()},
	}
	for _, c := range callers {
		t.Run(c.name+" sees travel notes and min age", func(t *testing.T) {
			resp := enrollTestAPI(t, newMock()).GetCtx(c.ctx, "/enroll/tok")
			if resp.Code != http.StatusOK {
				t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
			}
			var info dto.EnrollInfo
			if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
				t.Fatalf("decode: %v", err)
			}
			cr := info.Cruise
			if cr == nil {
				t.Fatalf("no cruise in %s", resp.Body)
			}
			if cr.TravelNotes == nil || *cr.TravelNotes != "Tanie loty do Katanii" ||
				cr.MinAge == nil || *cr.MinAge != 18 {
				t.Fatalf("cruise = %s", resp.Body)
			}
		})
	}
}
