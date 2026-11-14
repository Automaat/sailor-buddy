package handlers

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestListPaging_AfterCursorReachesQuery(t *testing.T) {
	t.Run("name key", func(t *testing.T) {
		var got sqlcdb.ListYachtsParams
		m := &mockQuerier{
			listYachtsFn: func(_ context.Context, arg sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
				got = arg
				return nil, nil
			},
			countYachtsFn: func(context.Context) (int64, error) { return 0, nil },
		}
		after := url.QueryEscape(encodeCursor(nameKey{Name: "Alfa", ID: 5}))
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts?limit=3&after="+after)
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d; body=%s", resp.Code, resp.Body)
		}
		if !got.AfterID.Valid || got.AfterID.Int64 != 5 || !got.AfterName.Valid || got.AfterName.String != "Alfa" || got.RowLimit != 4 {
			t.Fatalf("query args = %+v, want after (Alfa, 5) and limit 4", got)
		}
	})

	t.Run("NULL date key", func(t *testing.T) {
		var got sqlcdb.ListTripsParams
		m := &mockQuerier{
			listTripsFn: func(_ context.Context, arg sqlcdb.ListTripsParams) ([]sqlcdb.Trip, error) {
				got = arg
				return nil, nil
			},
			countTripsFn: func(context.Context) (int64, error) { return 0, nil },
		}
		after := url.QueryEscape(encodeCursor(dateKey{ID: 3}))
		resp := tripTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips?after="+after)
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d; body=%s", resp.Code, resp.Body)
		}
		if !got.AfterID.Valid || got.AfterID.Int64 != 3 || got.AfterEmbarkDate.Valid {
			t.Fatalf("query args = %+v, want after (NULL, 3)", got)
		}
	})
}

func TestListPaging_InvalidCursorIs422(t *testing.T) {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for name, after := range map[string]string{
		"not base64":    "!!",
		"not JSON":      b64("nope"),
		"no id":         b64(`{"n":"Alfa"}`),
		"zero id":       b64(`{"n":"Alfa","i":0}`),
		"unknown field": b64(`{"n":"Alfa","i":1,"x":2}`),
		"trailing data": b64(`{"n":"Alfa","i":1}{"i":2}`),
		"NUL in name":   b64(`{"n":"Al\u0000fa","i":1}`),
	} {
		t.Run(name, func(t *testing.T) {
			m := &mockQuerier{}
			resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts?after="+url.QueryEscape(after))
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
			}
		})
	}
}

func TestListPaging_InvalidDateCursorIs422(t *testing.T) {
	after := base64.RawURLEncoding.EncodeToString([]byte(`{"d":"2026\u0000","i":1}`))
	resp := tripTestAPI(t, &mockQuerier{}).GetCtx(userCtx(context.Background()), "/trips?after="+after)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
	}
}
