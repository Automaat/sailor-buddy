package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func yachtTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterYachtRoutes(api, m)
	return api
}

func TestYachtHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn: func(context.Context, sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "Bavaria 46"}}, nil
			},
			countYachtsFn: func(context.Context) (int64, error) { return 1, nil },
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn: func(context.Context, sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
				return nil, errors.New("fail")
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("count error", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn:  func(context.Context, sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) { return nil, nil },
			countYachtsFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestYachtHandler_List_Pagination(t *testing.T) {
	t.Run("envelope echoes limit/offset and total", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn: func(context.Context, sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "Bavaria 46"}}, nil
			},
			countYachtsFn: func(context.Context) (int64, error) { return 42, nil },
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts?limit=10&offset=20")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var page dto.Page[dto.Yacht]
		if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if page.Total != 42 || page.Limit != 10 || page.Offset != 20 {
			t.Fatalf("unexpected page meta: %+v", page)
		}
		if !page.HasMore {
			t.Fatalf("want has_more true (21 of 42 seen)")
		}
		if len(page.Items) != 1 || page.Items[0].Name != "Bavaria 46" {
			t.Fatalf("unexpected items: %+v", page.Items)
		}
	})

	t.Run("default limit when omitted", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn:  func(context.Context, sqlcdb.ListYachtsParams) ([]sqlcdb.Yacht, error) { return nil, nil },
			countYachtsFn: func(context.Context) (int64, error) { return 0, nil },
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var page dto.Page[dto.Yacht]
		if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if page.Limit != 50 {
			t.Fatalf("default limit = %d, want 50", page.Limit)
		}
		if page.Items == nil {
			t.Fatalf("items must serialise as [], got null")
		}
	})

	t.Run("rejects out-of-range limit", func(t *testing.T) {
		for _, q := range []string{"/yachts?limit=0", "/yachts?limit=101", "/yachts?offset=-1"} {
			resp := yachtTestAPI(t, &mockQuerier{}).GetCtx(userCtx(context.Background()), q)
			if resp.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: got %d, want 422", q, resp.Code)
			}
		}
	})
}

func TestYachtHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: id, Name: "Sun Odyssey"}, nil
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(context.Context, int64) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, sql.ErrNoRows
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(context.Context, int64) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestYachtHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := yachtTestAPI(t, m).PostCtx(userCtx(context.Background()), "/yachts", map[string]any{"name": "Oceanis"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := yachtTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/yachts", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createYachtFn: func(context.Context, sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		resp := yachtTestAPI(t, m).PostCtx(userCtx(context.Background()), "/yachts", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestYachtHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateYachtFn: func(context.Context, sqlcdb.UpdateYachtParams) error { return nil },
		}
		resp := yachtTestAPI(t, m).PutCtx(userCtx(context.Background()), "/yachts/1", map[string]any{"name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := yachtTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/yachts/1", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateYachtFn: func(context.Context, sqlcdb.UpdateYachtParams) error { return errors.New("fail") },
		}
		resp := yachtTestAPI(t, m).PutCtx(userCtx(context.Background()), "/yachts/1", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestYachtHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteYachtFn: func(context.Context, int64) error { return nil },
		}
		resp := yachtTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteYachtFn: func(context.Context, int64) error { return errors.New("fail") },
		}
		resp := yachtTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	for _, constraint := range []string{"trips_yacht_id_fkey", "voyages_yacht_id_fkey"} {
		t.Run("used by "+constraint, func(t *testing.T) {
			m := &mockQuerier{
				deleteYachtFn: func(context.Context, int64) error {
					return &pgconn.PgError{Code: "23503", ConstraintName: constraint}
				},
			}
			resp := yachtTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/yachts/1")
			if resp.Code != http.StatusConflict {
				t.Fatalf("got %d, want 409; body=%s", resp.Code, resp.Body)
			}
		})
	}
}

func TestYachtHandler_Create_MemberForbidden(t *testing.T) {
	resp := yachtTestAPI(t, &mockQuerier{}).PostCtx(
		userCtxRole(context.Background(), "member"), "/yachts", map[string]any{"name": "X"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403; body=%s", resp.Code, resp.Body)
	}
}

func TestYachtHandler_Specs(t *testing.T) {
	t.Run("create stores specs and get returns them", func(t *testing.T) {
		var stored sqlcdb.Yacht
		m := &mockQuerier{
			createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				stored = sqlcdb.Yacht{
					ID: 5, Name: arg.Name, YachtType: arg.YachtType,
					Berths: arg.Berths, LengthM: arg.LengthM, BuildYear: arg.BuildYear,
					Engine: arg.Engine, OwnerName: arg.OwnerName,
				}
				return stored, nil
			},
			getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
				if id != stored.ID {
					return sqlcdb.Yacht{}, sql.ErrNoRows
				}
				return stored, nil
			},
		}
		api := yachtTestAPI(t, m)
		resp := api.PostCtx(userCtx(context.Background()), "/yachts", map[string]any{
			"name": "Ilmatar", "yacht_type": "Gib'Sea 43", "berths": 10, "length_m": 12.8,
			"build_year": 2002, "engine": "Volvo Penta D2-55", "owner_name": "Szkoła Żeglarska Kliper",
		})
		if resp.Code != http.StatusCreated {
			t.Fatalf("create: got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		resp = api.GetCtx(userCtx(context.Background()), "/yachts/5")
		if resp.Code != http.StatusOK {
			t.Fatalf("get: got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var y dto.Yacht
		if err := json.Unmarshal(resp.Body.Bytes(), &y); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if y.Berths == nil || *y.Berths != 10 ||
			y.LengthM == nil || *y.LengthM != 12.8 ||
			y.BuildYear == nil || *y.BuildYear != 2002 ||
			y.Engine == nil || *y.Engine != "Volvo Penta D2-55" ||
			y.OwnerName == nil || *y.OwnerName != "Szkoła Żeglarska Kliper" {
			t.Fatalf("specs not returned: %+v", y)
		}
	})

	t.Run("update passes specs", func(t *testing.T) {
		var got sqlcdb.UpdateYachtParams
		m := &mockQuerier{
			updateYachtFn: func(_ context.Context, arg sqlcdb.UpdateYachtParams) error {
				got = arg
				return nil
			},
		}
		resp := yachtTestAPI(t, m).PutCtx(userCtx(context.Background()), "/yachts/3", map[string]any{
			"name": "Ilmatar", "berths": 8, "length_m": 11.5,
		})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		if got.ID != 3 || got.Berths.Int64 != 8 || !got.Berths.Valid || got.LengthM.Float64 != 11.5 || got.BuildYear.Valid {
			t.Fatalf("unexpected update params: %+v", got)
		}
	})

	t.Run("specs are optional", func(t *testing.T) {
		var got sqlcdb.CreateYachtParams
		m := &mockQuerier{
			createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				got = arg
				return sqlcdb.Yacht{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := yachtTestAPI(t, m).PostCtx(userCtx(context.Background()), "/yachts", map[string]any{"name": "Oceanis"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
		if got.Berths.Valid || got.LengthM.Valid || got.BuildYear.Valid || got.Engine.Valid || got.OwnerName.Valid {
			t.Fatalf("absent specs must be NULL: %+v", got)
		}
		var y map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &y); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, k := range []string{"berths", "length_m", "build_year", "engine", "owner_name"} {
			if _, ok := y[k]; ok {
				t.Fatalf("%s must be omitted when unset: %s", k, resp.Body)
			}
		}
	})

	t.Run("rejects invalid specs", func(t *testing.T) {
		cases := map[string]map[string]any{
			"zero berths":     {"name": "X", "berths": 0},
			"negative berths": {"name": "X", "berths": -2},
			"negative length": {"name": "X", "length_m": -1.5},
			"zero length":     {"name": "X", "length_m": 0},
		}
		for name, body := range cases {
			for _, method := range []string{http.MethodPost, http.MethodPut} {
				path := "/yachts"
				if method == http.MethodPut {
					path = "/yachts/1"
				}
				resp := yachtTestAPI(t, &mockQuerier{}).DoCtx(userCtx(context.Background()), method, path, body)
				if resp.Code != http.StatusUnprocessableEntity {
					t.Fatalf("%s %s: got %d, want 422; body=%s", method, name, resp.Code, resp.Body)
				}
			}
		}
	})
}

func TestLegWriteWithUnknownYacht(t *testing.T) {
	fkErr := func(constraint string) error {
		return &pgconn.PgError{Code: "23503", ConstraintName: constraint}
	}
	body := map[string]any{"cruise_id": 1, "name": "Navigare", "yacht_id": 999}
	withCrew := map[string]any{"cruise_id": 1, "name": "Navigare", "yacht_id": 999, "max_crew": 8}
	tests := []struct {
		name string
		m    *mockQuerier
		send func(*mockQuerier) *httptest.ResponseRecorder
	}{
		{
			name: "trip create with crew limit",
			m: &mockQuerier{createTripFn: func(context.Context, sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, fkErr("trips_yacht_id_fkey")
			}},
			send: func(m *mockQuerier) *httptest.ResponseRecorder {
				return tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", withCrew)
			},
		},
		{
			name: "trip create with yacht deleted after lookup",
			m: &mockQuerier{
				getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) { return sqlcdb.Yacht{ID: id}, nil },
				createTripFn: func(context.Context, sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{}, fkErr("trips_yacht_id_fkey")
				},
			},
			send: func(m *mockQuerier) *httptest.ResponseRecorder {
				return tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", body)
			},
		},
		{
			name: "trip update",
			m: &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id, CruiseID: 1}, nil },
				updateTripFn: func(context.Context, sqlcdb.UpdateTripParams) error {
					return fkErr("trips_yacht_id_fkey")
				},
			},
			send: func(m *mockQuerier) *httptest.ResponseRecorder {
				return tripTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/trips/3", body)
			},
		},
		{
			name: "voyage create",
			m: &mockQuerier{createVoyageFn: func(context.Context, sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, fkErr("voyages_yacht_id_fkey")
			}},
			send: func(m *mockQuerier) *httptest.ResponseRecorder {
				return voyageTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages", body)
			},
		},
		{
			name: "voyage update",
			m: &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
				updateVoyageFn: func(context.Context, sqlcdb.UpdateVoyageParams) error {
					return fkErr("voyages_yacht_id_fkey")
				},
			},
			send: func(m *mockQuerier) *httptest.ResponseRecorder {
				return voyageTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/voyages/3", body)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.send(tt.m)
			if resp.Code != http.StatusUnprocessableEntity || !strings.Contains(resp.Body.String(), "yacht not found") {
				t.Fatalf("got %d %s, want 422 yacht not found", resp.Code, resp.Body)
			}
		})
	}
}
