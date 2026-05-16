package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func orgResourceTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterOrgResourceRoutes(api, m)
	return api
}

// TestOrgYachtHandler_List_Pagination guards against the org-scoped list
// dropping its limit/offset query parameters — a regression would page with
// LIMIT 0 and return nothing.
func TestOrgYachtHandler_List_Pagination(t *testing.T) {
	t.Run("envelope echoes limit/offset and total", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgYachtsFn: func(context.Context, types.NullInt64) ([]sqlcdb.Yacht, error) {
				return []sqlcdb.Yacht{{ID: 1, Name: "Orion"}}, nil
			},
			countOrgYachtsFn: func(context.Context, types.NullInt64) (int64, error) { return 42, nil },
		}, "crew")
		resp := orgResourceTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/yachts?limit=10&offset=20")
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
		if len(page.Items) != 1 || page.Items[0].Name != "Orion" {
			t.Fatalf("unexpected items: %+v", page.Items)
		}
	})

	t.Run("default limit when omitted", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgYachtsFn:  func(context.Context, types.NullInt64) ([]sqlcdb.Yacht, error) { return nil, nil },
			countOrgYachtsFn: func(context.Context, types.NullInt64) (int64, error) { return 0, nil },
		}, "crew")
		resp := orgResourceTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/yachts")
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
	})
}

func TestOrgCrewHandler_List_Pagination(t *testing.T) {
	t.Run("envelope echoes limit/offset and total", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgCrewMembersFn: func(context.Context, types.NullInt64) ([]sqlcdb.CrewMember, error) {
				return []sqlcdb.CrewMember{{ID: 1, FullName: "Jan Nowak"}}, nil
			},
			countOrgCrewMembersFn: func(context.Context, types.NullInt64) (int64, error) { return 7, nil },
		}, "crew")
		resp := orgResourceTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/crew?limit=5&offset=5")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var page dto.Page[dto.CrewMember]
		if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if page.Total != 7 || page.Limit != 5 || page.Offset != 5 || !page.HasMore {
			t.Fatalf("unexpected page meta: %+v", page)
		}
		if len(page.Items) != 1 || page.Items[0].FullName != "Jan Nowak" {
			t.Fatalf("unexpected items: %+v", page.Items)
		}
	})

	t.Run("count error returns 500", func(t *testing.T) {
		m := withOrgRole(&mockQuerier{
			listOrgCrewMembersFn: func(context.Context, types.NullInt64) ([]sqlcdb.CrewMember, error) { return nil, nil },
			countOrgCrewMembersFn: func(context.Context, types.NullInt64) (int64, error) {
				return 0, errors.New("fail")
			},
		}, "crew")
		resp := orgResourceTestAPI(t, m).GetCtx(userCtx(context.Background()), "/orgs/club/crew")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}
