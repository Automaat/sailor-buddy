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

func voyagePortTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterVoyagePortRoutes(api, m)
	return api
}

func TestVoyagePorts_List(t *testing.T) {
	m := &mockQuerier{
		listVoyagePortsFn: func(_ context.Context, arg sqlcdb.ListVoyagePortsParams) ([]sqlcdb.VoyagePort, error) {
			if arg.VoyageID != 5 || arg.OwnerID != 1 {
				t.Fatalf("unexpected params: %+v", arg)
			}
			return []sqlcdb.VoyagePort{
				{ID: 1, VoyageID: 5, Name: "Split", Latitude: 43.5, Longitude: 16.4, Position: 0},
			}, nil
		},
	}
	resp := voyagePortTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/5/ports")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var ports []dto.VoyagePort
	if err := json.Unmarshal(resp.Body.Bytes(), &ports); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ports) != 1 || ports[0].Name != "Split" {
		t.Fatalf("unexpected ports: %+v", ports)
	}
}

func TestVoyagePorts_Add(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(context.Context, sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5, OwnerID: 1}, nil
		},
		createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
			return sqlcdb.VoyagePort{
				ID: 9, VoyageID: arg.VoyageID, Name: arg.Name,
				Latitude: arg.Latitude, Longitude: arg.Longitude, Position: arg.Position,
			}, nil
		},
	}
	resp := voyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/5/ports",
		map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var port dto.VoyagePort
	if err := json.Unmarshal(resp.Body.Bytes(), &port); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if port.Name != "Hvar" || port.VoyageID != 5 {
		t.Fatalf("unexpected port: %+v", port)
	}
}

func TestVoyagePorts_Add_VoyageNotFound(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(context.Context, sqlcdb.GetVoyageParams) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{}, sql.ErrNoRows
		},
	}
	resp := voyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/5/ports",
		map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.Code, resp.Body)
	}
}

func TestVoyagePorts_Remove(t *testing.T) {
	var got sqlcdb.DeleteVoyagePortParams
	m := &mockQuerier{
		deleteVoyagePortFn: func(_ context.Context, arg sqlcdb.DeleteVoyagePortParams) error {
			got = arg
			return nil
		},
	}
	resp := voyagePortTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/5/ports/9")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if got.ID != 9 || got.VoyageID != 5 || got.OwnerID != 1 {
		t.Fatalf("unexpected delete params: %+v", got)
	}
}
