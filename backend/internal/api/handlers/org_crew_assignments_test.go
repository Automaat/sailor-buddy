package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func orgCrewTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterOrgCrewAssignmentRoutes(api, m)
	return api
}

// withOrgRole sets up the slug + membership lookups resolveOrg performs.
func withOrgRole(m *mockQuerier, role string) *mockQuerier {
	m.getOrganizationBySlugFn = func(context.Context, string) (sqlcdb.Organization, error) {
		return sqlcdb.Organization{ID: 7, Slug: "club"}, nil
	}
	m.getOrgMembershipFn = func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
		return sqlcdb.GetOrgMembershipRow{Role: role}, nil
	}
	return m
}

func TestOrgCrewAssignment_ListTripCrew(t *testing.T) {
	t.Run("non-member forbidden", func(t *testing.T) {
		m := &mockQuerier{
			getOrganizationBySlugFn: func(context.Context, string) (sqlcdb.Organization, error) {
				return sqlcdb.Organization{ID: 7, Slug: "club"}, nil
			},
			getOrgMembershipFn: func(context.Context, sqlcdb.GetOrgMembershipParams) (sqlcdb.GetOrgMembershipRow, error) {
				return sqlcdb.GetOrgMembershipRow{}, sql.ErrNoRows
			},
		}
		resp := orgCrewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/trips/3/crew")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("member ok", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgTripCrewFn: func(context.Context, sqlcdb.ListOrgTripCrewAssignmentsParams) ([]sqlcdb.ListOrgTripCrewAssignmentsRow, error) {
				return []sqlcdb.ListOrgTripCrewAssignmentsRow{{ID: 1, CrewMemberID: 2, Role: "skipper", FullName: "Jan"}}, nil
			},
		}, "crew")
		resp := orgCrewTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/trips/3/crew")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestOrgCrewAssignment_AssignTripCrew(t *testing.T) {
	t.Run("non-admin forbidden", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{}, "crew")
		resp := orgCrewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/trips/3/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("trip not found", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgTripFn: func(context.Context, sqlcdb.GetOrgTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, sql.ErrNoRows
			},
		}, "admin")
		resp := orgCrewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/trips/3/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("admin success", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			getOrgTripFn: func(_ context.Context, arg sqlcdb.GetOrgTripParams) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{ID: arg.ID}, nil
			},
			createTripCrewFn: func(_ context.Context, arg sqlcdb.CreateTripCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{ID: 1, CrewMemberID: arg.CrewMemberID, Role: arg.Role}, nil
			},
		}, "admin")
		resp := orgCrewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/trips/3/crew",
			map[string]any{"crew_member_id": 1, "role": "first_mate"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestOrgCrewAssignment_RemoveVoyageCrew(t *testing.T) {
	t.Run("non-admin forbidden", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{}, "captain")
		resp := orgCrewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/5/crew/9")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})

	t.Run("admin success", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			deleteOrgVoyageCrewFn: func(context.Context, sqlcdb.DeleteOrgVoyageCrewAssignmentParams) error { return nil },
		}, "admin")
		resp := orgCrewTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/orgs/club/voyages/5/crew/9")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestOrgCrewAssignment_AssignVoyageCrew(t *testing.T) {
	m := withOrgRole(&mockQuerier{
		getOrgVoyageFn: func(_ context.Context, arg sqlcdb.GetOrgVoyageParams) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: arg.ID}, nil
		},
		createVoyageCrewFn: func(_ context.Context, arg sqlcdb.CreateVoyageCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
			return sqlcdb.CrewAssignment{ID: 1, CrewMemberID: arg.CrewMemberID, Role: arg.Role}, nil
		},
	}, "admin")
	resp := orgCrewTestAPI(t, m).PostCtx(userCtx(context.Background()), "/orgs/club/voyages/5/crew",
		map[string]any{"crew_member_id": 2, "role": "crew"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
	}
}
