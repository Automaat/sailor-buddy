package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// These tests pin the limit/offset query parameters on every org-scoped
// collection list. huma does not promote params from an embedded struct, so
// each org list input declares them inline — a regression pages with LIMIT 0.

func TestOrgVoyageHandler_List_Pagination(t *testing.T) {
	m := withOrgRole(&mockQuerier{
		listOrgVoyagesFn: func(context.Context, types.NullInt64) ([]sqlcdb.Voyage, error) {
			return []sqlcdb.Voyage{{ID: 1, Name: "Adriatic 2025"}}, nil
		},
		countOrgVoyagesFn: func(context.Context, types.NullInt64) (int64, error) { return 9, nil },
	}, "crew")
	_, api := humatest.New(t)
	RegisterOrgVoyageRoutes(api, m)

	resp := api.GetCtx(userCtx(context.Background()), "/orgs/club/voyages?limit=3&offset=6")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var page dto.Page[dto.Voyage]
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 9 || page.Limit != 3 || page.Offset != 6 || !page.HasMore {
		t.Fatalf("unexpected page meta: %+v", page)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Adriatic 2025" {
		t.Fatalf("unexpected items: %+v", page.Items)
	}
}

func TestOrgTripHandler_List_Pagination(t *testing.T) {
	m := withOrgRole(&mockQuerier{
		listOrgTripsFn: func(context.Context, types.NullInt64) ([]sqlcdb.Trip, error) {
			return []sqlcdb.Trip{{ID: 1, Name: "Baltic", Status: sqlcdb.TripStatusPlanned}}, nil
		},
		countOrgTripsFn: func(context.Context, types.NullInt64) (int64, error) { return 1, nil },
	}, "crew")
	_, api := humatest.New(t)
	RegisterOrgTripRoutes(api, m, nil)

	resp := api.GetCtx(userCtx(context.Background()), "/orgs/club/trips?limit=10&offset=0")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var page dto.Page[dto.Trip]
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 1 || page.Limit != 10 || page.HasMore {
		t.Fatalf("unexpected page meta: %+v", page)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Baltic" {
		t.Fatalf("unexpected items: %+v", page.Items)
	}
}

func TestOrgCruiseHandler_List_Pagination(t *testing.T) {
	m := withOrgRole(&mockQuerier{
		listCruisesFn: func(context.Context, int64) ([]sqlcdb.Cruise, error) {
			return []sqlcdb.Cruise{{ID: 1, Name: "Summer Regatta"}}, nil
		},
		countCruisesFn: func(context.Context, int64) (int64, error) { return 12, nil },
	}, "crew")
	_, api := humatest.New(t)
	RegisterOrgCruiseRoutes(api, m)

	resp := api.GetCtx(userCtx(context.Background()), "/orgs/club/cruises?limit=4&offset=8")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var page dto.Page[dto.Cruise]
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if page.Total != 12 || page.Limit != 4 || page.Offset != 8 {
		t.Fatalf("unexpected page meta: %+v", page)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Summer Regatta" {
		t.Fatalf("unexpected items: %+v", page.Items)
	}
}
