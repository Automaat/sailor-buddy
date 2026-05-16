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

func voyageTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterVoyageRoutes(api, m)
	return api
}

func TestVoyageHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listVoyagesFn: func(context.Context, int64) ([]sqlcdb.Voyage, error) {
				return []sqlcdb.Voyage{{ID: 1, Name: "Adriatic 2025"}}, nil
			},
			countVoyagesFn: func(context.Context, int64) (int64, error) { return 1, nil },
		}
		resp := voyageTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listVoyagesFn: func(context.Context, int64) ([]sqlcdb.Voyage, error) {
				return nil, errors.New("fail")
			},
		}
		resp := voyageTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestVoyageHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, arg sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID, Name: "Baltic"}, nil
			},
		}
		resp := voyageTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}
		resp := voyageTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, errors.New("fail")
			},
		}
		resp := voyageTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestVoyageHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := voyageTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages", map[string]any{"name": "Ionian"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := voyageTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/voyages", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createVoyageFn: func(context.Context, sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, errors.New("fail")
			},
		}
		resp := voyageTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestVoyageHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateVoyageFn: func(context.Context, sqlcdb.UpdateVoyageParams) error { return nil },
		}
		resp := voyageTestAPI(t, m).PutCtx(userCtx(context.Background()), "/voyages/1", map[string]any{"name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := voyageTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()), "/voyages/1", map[string]any{})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateVoyageFn: func(context.Context, sqlcdb.UpdateVoyageParams) error { return errors.New("fail") },
		}
		resp := voyageTestAPI(t, m).PutCtx(userCtx(context.Background()), "/voyages/1", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestVoyageHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteVoyageFn: func(context.Context, sqlcdb.DeleteVoyageParams) error { return nil },
		}
		resp := voyageTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteVoyageFn: func(context.Context, sqlcdb.DeleteVoyageParams) error { return errors.New("fail") },
		}
		resp := voyageTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
