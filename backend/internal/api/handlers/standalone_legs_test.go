package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCruiseHandler_ListStandaloneLegs(t *testing.T) {
	rows := []sqlcdb.ListStandaloneLegsRow{
		{
			Kind: "trip", ID: 5, Name: "Chorwacja", Cancelled: true,
			EmbarkDate: types.NullString{String: "2026-07-01", Valid: true},
			StartPort:  types.NullString{String: "Split", Valid: true},
			CruiseID:   9, CruiseName: "Chorwacja", CruisePublishedAt: published,
		},
		{Kind: "cruise", ID: 3, Name: "Bez etapów", CruiseID: 3, CruiseName: "Bez etapów", PublishedAt: published, CruisePublishedAt: published},
	}
	tests := []struct {
		name       string
		ctx        context.Context
		rows       []sqlcdb.ListStandaloneLegsRow
		wantHidden bool
		wantBody   string
	}{
		{
			name: "admin reads hidden entries", ctx: userCtxRole(context.Background(), "admin"),
			rows: rows, wantHidden: true,
			wantBody: `[{"kind":"trip","id":5,"name":"Chorwacja","cancelled":true,"embark_date":"2026-07-01",` +
				`"start_port":{"name":"Split"},"cruise_id":9,"cruise_name":"Chorwacja",` +
				`"cruise_published_at":"2025-01-01T00:00:00Z"},` +
				`{"kind":"cruise","id":3,"name":"Bez etapów","cancelled":false,"published_at":"2025-01-01T00:00:00Z",` +
				`"cruise_id":3,"cruise_name":"Bez etapów","cruise_published_at":"2025-01-01T00:00:00Z"}]`,
		},
		{name: "captain group reads hidden entries", ctx: captainGroupCtx(7), wantHidden: true, wantBody: `[]`},
		{name: "member reads published entries", ctx: userCtxRole(context.Background(), "member"), wantBody: `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asked *bool
			m := &mockQuerier{listStandaloneLegsFn: func(_ context.Context, includeHidden bool) ([]sqlcdb.ListStandaloneLegsRow, error) {
				asked = &includeHidden
				return tt.rows, nil
			}}
			resp := cruiseTestAPI(t, m).GetCtx(tt.ctx, "/standalone-legs")
			if resp.Code != http.StatusOK {
				t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
			}
			if asked == nil || *asked != tt.wantHidden {
				t.Fatalf("include_hidden = %v, want %v", asked, tt.wantHidden)
			}
			var got, want any
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
				t.Fatalf("decode want: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("body = %s\nwant   %s", resp.Body, tt.wantBody)
			}
		})
	}

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{listStandaloneLegsFn: func(context.Context, bool) ([]sqlcdb.ListStandaloneLegsRow, error) {
			return nil, errors.New("fail")
		}}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/standalone-legs")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
