package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func seriesTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterSeriesRoutes(api, m)
	return api
}

func TestSeriesHandler_Reads(t *testing.T) {
	member := userCtxRole(context.Background(), "member")

	t.Run("member lists series", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseSeriesFn: func(context.Context, sqlcdb.ListCruiseSeriesParams) ([]sqlcdb.CruiseSeries, error) {
				return []sqlcdb.CruiseSeries{{ID: 1, Name: "May I Sail"}}, nil
			},
			countCruiseSeriesFn: func(context.Context) (int64, error) { return 1, nil },
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var page dto.Page[dto.CruiseSeries]
		if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "May I Sail" {
			t.Fatalf("unexpected page: %+v", page)
		}
	})

	t.Run("member gets a series", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{ID: id, Name: "AGH WSE", Description: types.NullString{String: "since 2012", Valid: true}}, nil
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/3")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var out dto.CruiseSeries
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.ID != 3 || out.Description == nil || *out.Description != "since 2012" {
			t.Fatalf("unexpected series: %+v", out)
		}
	})

	t.Run("unknown series is 404", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: func(context.Context, int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{}, sql.ErrNoRows
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/9")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestSeriesHandler_AdminWrites(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		var got sqlcdb.CreateCruiseSeriesParams
		m := &mockQuerier{
			createCruiseSeriesFn: func(_ context.Context, arg sqlcdb.CreateCruiseSeriesParams) (sqlcdb.CruiseSeries, error) {
				got = arg
				return sqlcdb.CruiseSeries{ID: 7, Name: arg.Name, Description: arg.Description}, nil
			},
		}
		resp := seriesTestAPI(t, m).PostCtx(userCtx(context.Background()), "/series",
			map[string]any{"name": "May I Sail", "description": "since 2018"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if got.Name != "May I Sail" || got.Description.String != "since 2018" || !got.CreatedBy.Valid {
			t.Fatalf("unexpected params: %+v", got)
		}
	})

	t.Run("create requires a name", func(t *testing.T) {
		resp := seriesTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/series",
			map[string]any{"name": ""})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("update", func(t *testing.T) {
		var got sqlcdb.UpdateCruiseSeriesParams
		m := &mockQuerier{
			getCruiseSeriesFn: func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{ID: id}, nil
			},
			updateCruiseSeriesFn: func(_ context.Context, arg sqlcdb.UpdateCruiseSeriesParams) error {
				got = arg
				return nil
			},
		}
		resp := seriesTestAPI(t, m).PutCtx(userCtx(context.Background()), "/series/4",
			map[string]any{"name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		if got.ID != 4 || got.Name != "Renamed" || got.Description.Valid {
			t.Fatalf("unexpected params: %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var deleted int64
		m := &mockQuerier{
			getCruiseSeriesFn: func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{ID: id}, nil
			},
			deleteCruiseSeriesFn: func(_ context.Context, id int64) error {
				deleted = id
				return nil
			},
		}
		resp := seriesTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/series/5")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		if deleted != 5 {
			t.Fatalf("deleted id = %d, want 5", deleted)
		}
	})
}

func TestSeriesHandler_MemberWritesForbidden(t *testing.T) {
	member := userCtxRole(context.Background(), "member")
	body := map[string]any{"name": "X"}
	cases := []struct {
		name string
		do   func(api humatest.TestAPI) *httptest.ResponseRecorder
	}{
		{"create", func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.PostCtx(member, "/series", body) }},
		{"update", func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.PutCtx(member, "/series/1", body) }},
		{"delete", func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.DeleteCtx(member, "/series/1") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := tc.do(seriesTestAPI(t, &mockQuerier{}))
			if resp.Code != http.StatusForbidden {
				t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
			}
		})
	}
}

func TestSeriesHandler_Editions(t *testing.T) {
	member := userCtxRole(context.Background(), "member")
	knownSeries := func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
		return sqlcdb.CruiseSeries{ID: id, Name: "May I Sail"}, nil
	}

	t.Run("lists editions with totals in query order", func(t *testing.T) {
		var gotSeries types.NullInt64
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			listCruiseSeriesEditionsFn: func(_ context.Context, seriesID types.NullInt64) ([]sqlcdb.ListCruiseSeriesEditionsRow, error) {
				gotSeries = seriesID
				return []sqlcdb.ListCruiseSeriesEditionsRow{
					{ID: 20, Name: "MIS 2025", Edition: types.NullString{String: "2025", Valid: true}, Kind: "sea", VoyageCount: 2, TotalMiles: 200, TotalHours: 40},
					{ID: 10, Name: "MIS 2024", Edition: types.NullString{String: "2024", Valid: true}, Kind: "inland", VoyageCount: 1, TotalMiles: 100.5, TotalHours: 20},
				}, nil
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/2/editions")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if !gotSeries.Valid || gotSeries.Int64 != 2 {
			t.Fatalf("queried series %+v, want 2", gotSeries)
		}
		var out []dto.CruiseSeriesEdition
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("got %d editions, want 2", len(out))
		}
		first, second := out[0], out[1]
		if first.CruiseID != 20 || first.Edition == nil || *first.Edition != "2025" ||
			first.TotalMiles != 200 || first.TotalHours != 40 || first.VoyageCount != 2 {
			t.Errorf("first edition = %+v", first)
		}
		if second.CruiseID != 10 || second.Kind != "inland" || second.TotalMiles != 100.5 || second.TotalHours != 20 {
			t.Errorf("second edition = %+v", second)
		}
	})

	t.Run("empty series returns []", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			listCruiseSeriesEditionsFn: func(context.Context, types.NullInt64) ([]sqlcdb.ListCruiseSeriesEditionsRow, error) {
				return nil, nil
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/2/editions")
		if resp.Code != http.StatusOK || strings.TrimSpace(resp.Body.String()) != "[]" {
			t.Fatalf("got %d %q, want 200 []", resp.Code, resp.Body.String())
		}
	})

	t.Run("unknown series is 404", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: func(context.Context, int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{}, sql.ErrNoRows
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/2/editions")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("query error is 500", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			listCruiseSeriesEditionsFn: func(context.Context, types.NullInt64) ([]sqlcdb.ListCruiseSeriesEditionsRow, error) {
				return nil, errors.New("boom")
			},
		}
		resp := seriesTestAPI(t, m).GetCtx(member, "/series/2/editions")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_Series(t *testing.T) {
	knownSeries := func(context.Context, int64) (sqlcdb.CruiseSeries, error) {
		return sqlcdb.CruiseSeries{ID: 3, Name: "May I Sail"}, nil
	}

	t.Run("create links a series and edition and reads them back", func(t *testing.T) {
		var got sqlcdb.CreateCruiseParams
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				got = arg
				return sqlcdb.Cruise{ID: 1, Name: arg.Name, Kind: arg.Kind, SeriesID: arg.SeriesID, Edition: arg.Edition}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises",
			map[string]any{"name": "MIS 2026/27", "series_id": 3, "edition": " 2026/27 "})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if got.SeriesID != (types.NullInt64{Int64: 3, Valid: true}) || got.Edition != (types.NullString{String: "2026/27", Valid: true}) {
			t.Fatalf("stored series=%+v edition=%+v", got.SeriesID, got.Edition)
		}
		var out dto.Cruise
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.SeriesID == nil || *out.SeriesID != 3 || out.Edition == nil || *out.Edition != "2026/27" {
			t.Fatalf("response series=%v edition=%v", out.SeriesID, out.Edition)
		}
	})

	t.Run("get reads series and edition back", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{
					ID: id, Name: "MIS 2025", Kind: "sea",
					SeriesID: types.NullInt64{Int64: 3, Valid: true},
					Edition:  types.NullString{String: "2025", Valid: true},
				}, nil
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtxRole(context.Background(), "member"), "/cruises/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var out dto.Cruise
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.SeriesID == nil || *out.SeriesID != 3 || out.Edition == nil || *out.Edition != "2025" {
			t.Fatalf("response series=%v edition=%v", out.SeriesID, out.Edition)
		}
	})

	t.Run("update links a series", func(t *testing.T) {
		var got sqlcdb.UpdateCruiseParams
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
				got = arg
				return nil
			},
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/8",
			map[string]any{"name": "MIS 2025", "series_id": 3, "edition": "2025"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		if got.ID != 8 || got.SeriesID.Int64 != 3 || !got.SeriesID.Valid || got.Edition.String != "2025" {
			t.Fatalf("unexpected params: %+v", got)
		}
	})

	noSeries := []struct {
		name string
		body map[string]any
	}{
		{"omitted", map[string]any{"name": "Solo"}},
		{"null", map[string]any{"name": "Solo", "series_id": nil, "edition": nil}},
		{"blank edition", map[string]any{"name": "Solo", "edition": "   "}},
	}
	for _, tc := range noSeries {
		t.Run("cruise without series: "+tc.name, func(t *testing.T) {
			var created sqlcdb.CreateCruiseParams
			var updated sqlcdb.UpdateCruiseParams
			m := &mockQuerier{
				createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
					created = arg
					return sqlcdb.Cruise{ID: 1, Name: arg.Name, Kind: arg.Kind}, nil
				},
				updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
					updated = arg
					return nil
				},
			}
			api := cruiseTestAPI(t, m)
			if resp := api.PostCtx(userCtx(context.Background()), "/cruises", tc.body); resp.Code != http.StatusCreated {
				t.Fatalf("create: got %d, want 201; body=%s", resp.Code, resp.Body)
			}
			if resp := api.PutCtx(userCtx(context.Background()), "/cruises/1", tc.body); resp.Code != http.StatusNoContent {
				t.Fatalf("update: got %d, want 204; body=%s", resp.Code, resp.Body)
			}
			if created.SeriesID.Valid || created.Edition.Valid || updated.SeriesID.Valid || updated.Edition.Valid {
				t.Fatalf("want null series and edition; created=%+v updated=%+v", created, updated)
			}
		})
	}

	t.Run("unknown series_id is 422", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: func(context.Context, int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{}, sql.ErrNoRows
			},
		}
		api := cruiseTestAPI(t, m)
		body := map[string]any{"name": "Ghost", "series_id": 404}
		if resp := api.PostCtx(userCtx(context.Background()), "/cruises", body); resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("create: got %d, want 422; body=%s", resp.Code, resp.Body)
		}
		if resp := api.PutCtx(userCtx(context.Background()), "/cruises/1", body); resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("update: got %d, want 422; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("series deleted after the check is 422", func(t *testing.T) {
		fk := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23503", ConstraintName: cruiseSeriesFKey})
		m := &mockQuerier{
			getCruiseSeriesFn: knownSeries,
			createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, fk
			},
			updateCruiseFn: func(context.Context, sqlcdb.UpdateCruiseParams) error { return fk },
		}
		api := cruiseTestAPI(t, m)
		body := map[string]any{"name": "Race", "series_id": 3}
		if resp := api.PostCtx(userCtx(context.Background()), "/cruises", body); resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("create: got %d, want 422; body=%s", resp.Code, resp.Body)
		}
		if resp := api.PutCtx(userCtx(context.Background()), "/cruises/1", body); resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("update: got %d, want 422; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("series lookup failure is 500", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseSeriesFn: func(context.Context, int64) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{}, errors.New("boom")
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises",
			map[string]any{"name": "X", "series_id": 3})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("edition longer than 50 is 422", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/cruises",
			map[string]any{"name": "X", "edition": strings.Repeat("x", 51)})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestSeriesHandler_DBErrors(t *testing.T) {
	boom := errors.New("boom")
	admin := userCtx(context.Background())
	failingGet := func(context.Context, int64) (sqlcdb.CruiseSeries, error) { return sqlcdb.CruiseSeries{}, boom }
	knownSeries := func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
		return sqlcdb.CruiseSeries{ID: id}, nil
	}
	cases := []struct {
		name string
		m    *mockQuerier
		do   func(api humatest.TestAPI) *httptest.ResponseRecorder
	}{
		{
			name: "list",
			m: &mockQuerier{listCruiseSeriesFn: func(context.Context, sqlcdb.ListCruiseSeriesParams) ([]sqlcdb.CruiseSeries, error) {
				return nil, boom
			}},
			do: func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.GetCtx(admin, "/series") },
		},
		{
			name: "get",
			m:    &mockQuerier{getCruiseSeriesFn: failingGet},
			do:   func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.GetCtx(admin, "/series/1") },
		},
		{
			name: "create",
			m: &mockQuerier{createCruiseSeriesFn: func(context.Context, sqlcdb.CreateCruiseSeriesParams) (sqlcdb.CruiseSeries, error) {
				return sqlcdb.CruiseSeries{}, boom
			}},
			do: func(api humatest.TestAPI) *httptest.ResponseRecorder {
				return api.PostCtx(admin, "/series", map[string]any{"name": "X"})
			},
		},
		{
			name: "update",
			m: &mockQuerier{
				getCruiseSeriesFn:    knownSeries,
				updateCruiseSeriesFn: func(context.Context, sqlcdb.UpdateCruiseSeriesParams) error { return boom },
			},
			do: func(api humatest.TestAPI) *httptest.ResponseRecorder {
				return api.PutCtx(admin, "/series/1", map[string]any{"name": "X"})
			},
		},
		{
			name: "delete",
			m: &mockQuerier{
				getCruiseSeriesFn:    knownSeries,
				deleteCruiseSeriesFn: func(context.Context, int64) error { return boom },
			},
			do: func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.DeleteCtx(admin, "/series/1") },
		},
		{
			name: "editions series lookup",
			m:    &mockQuerier{getCruiseSeriesFn: failingGet},
			do:   func(api humatest.TestAPI) *httptest.ResponseRecorder { return api.GetCtx(admin, "/series/1/editions") },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := tc.do(seriesTestAPI(t, tc.m))
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf("got %d, want 500; body=%s", resp.Code, resp.Body)
			}
		})
	}
}
