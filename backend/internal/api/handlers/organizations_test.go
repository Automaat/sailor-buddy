package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func orgTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterOrgRoutes(api, m)
	return api
}

func TestOrgHandler_List(t *testing.T) {
	m := &mockQuerier{
		listUserOrganizationsFn: func(context.Context, int64) ([]sqlcdb.ListUserOrganizationsRow, error) {
			return []sqlcdb.ListUserOrganizationsRow{{ID: 1, Name: "Warsaw SC", Slug: "warsaw", Role: "admin"}}, nil
		},
	}
	resp := orgTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}

func TestOrgHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(_ context.Context, arg sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Name: arg.Name, Slug: arg.Slug}, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{}, nil
			},
		}
		resp := orgTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs",
			map[string]any{"name": "Warsaw SC", "slug": "warsaw-sc"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := orgTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/orgs",
			map[string]any{"slug": "x"})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("invalid slug", func(t *testing.T) {
		resp := orgTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/orgs",
			map[string]any{"name": "X", "slug": "Bad Slug!"})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("slug conflict", func(t *testing.T) {
		m := &mockQuerier{
			createOrganizationFn: func(context.Context, sqlcdb.CreateOrganizationParams) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, &pgconn.PgError{Code: "23505"}
			},
		}
		resp := orgTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs",
			map[string]any{"name": "X", "slug": "taken"})
		if resp.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.Code)
		}
	})
}

func TestOrgHandler_Get(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{}, sql.ErrNoRows
			},
		}
		resp := orgTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/missing")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("not a member", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Slug: "warsaw"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		resp := orgTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/warsaw")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 1, Name: "Warsaw SC", Slug: "warsaw"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{Role: "crew"}, nil
			},
		}
		resp := orgTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/warsaw")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestOrgHandler_Update_RequiresAdmin(t *testing.T) {
	m := &mockQuerier{
		getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
			return sqlcdb.Organization{ID: 1, Slug: "warsaw"}, nil
		},
		getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
			return sqlcdb.GetOrgMembershipRow{Role: "crew"}, nil
		},
	}
	resp := orgTestAPI(t, m).PutCtx(userCtx(context.Background()), "/orgs/warsaw", map[string]any{"name": "New"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", resp.Code)
	}
}

func TestOrgHandler_UpdateMemberRole_LastAdmin(t *testing.T) {
	m := &mockQuerier{
		getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
			return sqlcdb.Organization{ID: 1, Slug: "warsaw"}, nil
		},
		getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
			return sqlcdb.GetOrgMembershipRow{Role: "admin"}, nil
		},
		listOrgMembersFn: func(context.Context, int64) ([]sqlcdb.ListOrgMembersRow, error) {
			return []sqlcdb.ListOrgMembersRow{{ID: 5, Role: "admin"}}, nil
		},
		countOrgAdminsFn: func(context.Context, int64) (int64, error) { return 1, nil },
	}
	resp := orgTestAPI(t, m).PutCtx(userCtx(context.Background()), "/orgs/warsaw/members/5/role",
		map[string]any{"role": "crew"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
	}
}

func TestOrgHandler_AcceptInvite(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{
					ID: 1, ExpiresAt: types.NullTime{Time: time.Now().Add(-time.Hour), Valid: true},
				}, nil
			},
		}
		resp := orgTestAPI(t, m).PostCtx(userCtx(context.Background()), "/join/tok")
		if resp.Code != http.StatusGone {
			t.Fatalf("got %d, want 410", resp.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{}, sql.ErrNoRows
			},
		}
		resp := orgTestAPI(t, m).PostCtx(userCtx(context.Background()), "/join/tok")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getOrgInviteByTokenFn: func(context.Context, string) (sqlcdb.GetOrgInviteByTokenRow, error) {
				return sqlcdb.GetOrgInviteByTokenRow{ID: 1, OrgID: 2, Role: "crew", OrgName: "Warsaw", OrgSlug: "warsaw"}, nil
			},
			addOrgMemberFn: func(context.Context, sqlcdb.AddOrgMemberParams) (sqlcdb.OrgMember, error) {
				return sqlcdb.OrgMember{}, nil
			},
		}
		resp := orgTestAPI(t, m).PostCtx(userCtx(context.Background()), "/join/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})
}
