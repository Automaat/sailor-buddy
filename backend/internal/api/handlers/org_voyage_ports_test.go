package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func orgVoyagePortTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterOrgVoyagePortRoutes(api, m)
	return api
}

func TestOrgVoyagePorts_List(t *testing.T) {
	t.Run("non-member forbidden", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 7, Slug: "club"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		resp := orgVoyagePortTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("member ok", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgVoyagePortsFn: func(context.Context, sqlcdb.ListOrgVoyagePortsParams) ([]sqlcdb.VoyagePort, error) {
				return []sqlcdb.VoyagePort{{ID: 1, VoyageID: 3, Name: "Split", Latitude: 43.5, Longitude: 16.4}}, nil
			},
		}, "crew")
		resp := orgVoyagePortTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var ports []dto.VoyagePort
		if err := json.Unmarshal(resp.Body.Bytes(), &ports); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(ports) != 1 || ports[0].Name != "Split" {
			t.Fatalf("unexpected ports: %+v", ports)
		}
	})
}

func TestOrgVoyagePorts_Add(t *testing.T) {
	t.Run("non-admin forbidden", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{}, "crew")
		resp := orgVoyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports",
			map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("admin ok", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(context.Context, sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: 3}, nil
			},
			createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
				return sqlcdb.VoyagePort{ID: 9, VoyageID: arg.VoyageID, Name: arg.Name}, nil
			},
		}, "admin")
		resp := orgVoyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports",
			map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("voyage not found", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgVoyageFn: func(context.Context, sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}, "admin")
		resp := orgVoyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports",
			map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestOrgVoyagePorts_Remove(t *testing.T) {
	t.Run("non-admin forbidden", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{}, "crew")
		resp := orgVoyagePortTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports/9")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("admin ok", func(t *testing.T) {
		var got sqlcdb.DeleteOrgVoyagePortParams
		m := withOrgRole(&mockQuerier{
			deleteOrgVoyagePortFn: func(_ context.Context, arg sqlcdb.DeleteOrgVoyagePortParams) error {
				got = arg
				return nil
			},
		}, "admin")
		resp := orgVoyagePortTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/3/ports/9")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
		if got.ID != 9 || got.VoyageID != 3 || !got.OrgID.Valid {
			t.Fatalf("unexpected delete params: %+v", got)
		}
	})
}
