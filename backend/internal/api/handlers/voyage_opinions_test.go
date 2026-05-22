package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func opinionTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterVoyageOpinionRoutes(api, m, t.TempDir())
	return api
}

func TestVoyageOpinion_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id}, nil
			},
			listVoyageVoyageOpinionsFn: func(context.Context, int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error) {
				return []sqlcdb.ListVoyageVoyageOpinionsRow{{ID: 1, VoyageID: 3, FileFormat: "pdf", FullName: "Jan"}}, nil
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("voyage not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})
}

func TestVoyageOpinion_Delete(t *testing.T) {
	t.Run("opinion belongs to another voyage", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 99}, nil
			},
		}
		resp := opinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/3/opinions/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 3, FilePath: "/nonexistent/x.pdf"}, nil
			},
			deleteVoyageOpinionFn: func(context.Context, int64) error { return nil },
		}
		resp := opinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/3/opinions/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})
}
