package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestAuthHandler_MyLegs(t *testing.T) {
	nextDayInWarsaw := func() time.Time { return time.Date(2026, 11, 15, 23, 30, 0, 0, time.UTC) }
	rows := []sqlcdb.ListUserUpcomingLegsRow{
		{
			ID: 7, Name: "Etap 1", CruiseID: 3, CruiseName: "Sycylia 2027",
			SeriesID: types.NullInt64{Int64: 2, Valid: true}, SeriesName: types.NullString{String: "Sycylia", Valid: true},
			Edition:    types.NullString{String: "2027", Valid: true},
			EmbarkDate: types.NullString{String: "2027-05-01", Valid: true}, DisembarkDate: types.NullString{String: "2027-05-08", Valid: true},
			StartPort: types.NullString{String: "Palermo", Valid: true}, EndPort: types.NullString{String: "Trapani", Valid: true},
			Relation: "captain",
		},
		{ID: 8, Name: "Etap 2", CruiseID: 3, CruiseName: "Sycylia 2027", Relation: "crew"},
		{ID: 9, Name: "Bałtyk", CruiseID: 4, CruiseName: "Bałtyk", Relation: "accepted"},
		{ID: 10, Name: "Mazury", CruiseID: 5, CruiseName: "Mazury", Relation: "pending"},
		{ID: 11, Name: "Chorwacja", CruiseID: 6, CruiseName: "Chorwacja", Relation: "waitlisted"},
	}
	want := `{"legs":[` +
		`{"id":7,"name":"Etap 1","cruise_id":3,"cruise_name":"Sycylia 2027","series_id":2,"series_name":"Sycylia",` +
		`"edition":"2027","embark_date":"2027-05-01","disembark_date":"2027-05-08","start_port":"Palermo",` +
		`"end_port":"Trapani","relation":"captain"},` +
		`{"id":8,"name":"Etap 2","cruise_id":3,"cruise_name":"Sycylia 2027","relation":"crew"},` +
		`{"id":9,"name":"Bałtyk","cruise_id":4,"cruise_name":"Bałtyk","relation":"sign_up","sign_up_status":"accepted"},` +
		`{"id":10,"name":"Mazury","cruise_id":5,"cruise_name":"Mazury","relation":"sign_up","sign_up_status":"pending"},` +
		`{"id":11,"name":"Chorwacja","cruise_id":6,"cruise_name":"Chorwacja","relation":"sign_up","sign_up_status":"waitlisted"}` +
		`]}`

	tests := []struct {
		name       string
		ctx        context.Context
		rows       []sqlcdb.ListUserUpcomingLegsRow
		wantHidden bool
		wantBody   string
	}{
		{name: "member sees only visible legs", ctx: userCtxRole(context.Background(), "member"), rows: rows, wantBody: want},
		{name: "admin sees hidden legs", ctx: userCtx(context.Background()), rows: rows, wantHidden: true, wantBody: want},
		{
			name: "captain group sees hidden legs",
			ctx: context.WithValue(context.Background(), middleware.UserCtxKey, &auth.Claims{
				EmailVerified: true, UserID: 1, Role: "member", IsCaptain: true,
			}),
			rows: rows, wantHidden: true, wantBody: want,
		},
		{name: "none is an empty list", ctx: userCtxRole(context.Background(), "member"), wantBody: `{"legs":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			var asked sqlcdb.ListUserUpcomingLegsParams
			h := authHandlerFor(&mockQuerier{
				listUserLegsFn: func(_ context.Context, arg sqlcdb.ListUserUpcomingLegsParams) ([]sqlcdb.ListUserUpcomingLegsRow, error) {
					asked = arg
					return tt.rows, nil
				},
			})
			h.now = nextDayInWarsaw
			registerAuthRoutes(api, h)

			resp := api.GetCtx(tt.ctx, "/auth/me/legs")
			if resp.Code != http.StatusOK {
				t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
			}
			wantArgs := sqlcdb.ListUserUpcomingLegsParams{UserID: 1, Today: "2026-11-16", IncludeHidden: tt.wantHidden}
			if asked != wantArgs {
				t.Fatalf("query args = %+v, want %+v", asked, wantArgs)
			}
			var got, wantJSON any
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantBody), &wantJSON); err != nil {
				t.Fatalf("decode want: %v", err)
			}
			if !reflect.DeepEqual(got, wantJSON) {
				t.Fatalf("body = %s\nwant   %s", resp.Body, tt.wantBody)
			}
		})
	}

	t.Run("not authenticated", func(t *testing.T) {
		_, api := humatest.New(t)
		registerAuthRoutes(api, authHandlerFor(&mockQuerier{}))
		if resp := api.Get("/auth/me/legs"); resp.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.Code)
		}
	})

	t.Run("query failure is a 500", func(t *testing.T) {
		_, api := humatest.New(t)
		registerAuthRoutes(api, authHandlerFor(&mockQuerier{
			listUserLegsFn: func(context.Context, sqlcdb.ListUserUpcomingLegsParams) ([]sqlcdb.ListUserUpcomingLegsRow, error) {
				return nil, context.DeadlineExceeded
			},
		}))
		if resp := api.GetCtx(userCtx(context.Background()), "/auth/me/legs"); resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
