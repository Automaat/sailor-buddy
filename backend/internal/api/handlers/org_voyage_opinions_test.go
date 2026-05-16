package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func orgOpinionTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterOrgVoyageOpinionRoutes(api, m, t.TempDir())
	return api
}

func TestOrgVoyageOpinion_List(t *testing.T) {
	t.Run("non-member forbidden", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 7, Slug: "club"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("member ok", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID}, nil
			},
			listVoyageVoyageOpinionsFn: func(context.Context, int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error) {
				return []sqlcdb.ListVoyageVoyageOpinionsRow{{ID: 1, VoyageID: 3, FileFormat: "pdf", FullName: "Jan"}}, nil
			},
		}, "crew")
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("voyage not found", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(context.Context, sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}, "crew")
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})
}

func TestOrgVoyageOpinion_Download(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "opinion.pdf")
		if err := os.WriteFile(file, []byte("%PDF-1.4 test"), 0o644); err != nil {
			t.Fatal(err)
		}
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 3, FilePath: file, FileFormat: "pdf"}, nil
			},
		}, "crew")
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1/download")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if ct := resp.Header().Get("Content-Type"); ct != "application/pdf" {
			t.Fatalf("content-type: got %q", ct)
		}
		if cd := resp.Header().Get("Content-Disposition"); cd == "" {
			t.Fatal("missing Content-Disposition header")
		}
	})

	t.Run("opinion belongs to another voyage", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 99}, nil
			},
		}, "crew")
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1/download")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("non-member forbidden", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 7, Slug: "club"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		resp := orgOpinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1/download")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestOrgVoyageOpinion_Generate_RequiresAdmin(t *testing.T) {
	m := withOrgRole(&mockQuerier{}, "crew")
	resp := orgOpinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions",
		map[string]any{"crew_member_id": 1})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", resp.Code)
	}
}

func TestOrgVoyageOpinion_Delete(t *testing.T) {
	t.Run("opinion belongs to another voyage", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 99}, nil
			},
		}, "admin")
		resp := orgOpinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{}, "captain")
		resp := orgOpinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("admin success", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: arg.ID}, nil
			},
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 1, VoyageID: 3, FilePath: "/nonexistent/x.pdf"}, nil
			},
			deleteVoyageOpinionFn: func(context.Context, int64) error { return nil },
		}, "admin")
		resp := orgOpinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/3/opinions/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})
}
