package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

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
			listYachtsFn: func(context.Context, int64) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "Bavaria 46"}}, nil
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listYachtsFn: func(context.Context, int64) ([]sqlcdb.Yacht, error) {
				return nil, errors.New("fail")
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestYachtHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(_ context.Context, arg sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: arg.ID, Name: "Sun Odyssey"}, nil
			},
		}
		resp := yachtTestAPI(t, m).GetCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getYachtFn: func(context.Context, sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
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
			getYachtFn: func(context.Context, sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
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
			deleteYachtFn: func(context.Context, sqlcdb.DeleteYachtParams) error { return nil },
		}
		resp := yachtTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteYachtFn: func(context.Context, sqlcdb.DeleteYachtParams) error { return errors.New("fail") },
		}
		resp := yachtTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/yachts/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
