package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func voyagePortTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerVoyagePortRoutes(api, &VoyagePortHandler{q: m, runTx: directTxRunner(m)})
	return api
}

func TestVoyagePorts_List(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
		listVoyagePortsFn: func(_ context.Context, voyageID int64) ([]sqlcdb.VoyagePort, error) {
			if voyageID != 5 {
				t.Fatalf("unexpected voyage id: %d", voyageID)
			}
			return []sqlcdb.VoyagePort{
				{ID: 1, VoyageID: 5, Name: "Split", Latitude: types.NullFloat64{Float64: 43.5, Valid: true}, Longitude: types.NullFloat64{Float64: 16.4, Valid: true}, Position: 0},
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
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
		},
		addVoyagePortFn: func(_ context.Context, arg sqlcdb.AddVoyagePortParams) (sqlcdb.VoyagePort, error) {
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
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{}, sql.ErrNoRows
		},
	}
	resp := voyagePortTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/5/ports",
		map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.Code, resp.Body)
	}
}

func TestVoyagePorts_Reorder_VoyageNotFound(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{}, sql.ErrNoRows
		},
	}
	resp := voyagePortTestAPI(t, m).PutCtx(userCtx(context.Background()), "/voyages/5/ports/order",
		map[string]any{"port_ids": []int64{3, 1, 2}})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", resp.Code, resp.Body)
	}
}

func TestVoyagePorts_Reorder(t *testing.T) {
	var got []sqlcdb.SetVoyagePortPositionParams
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
		},
		listVoyagePortsFn: func(context.Context, int64) ([]sqlcdb.VoyagePort, error) {
			return []sqlcdb.VoyagePort{
				{ID: 3, VoyageID: 5, Name: "Hvar", Position: 0},
				{ID: 1, VoyageID: 5, Name: "Split", Position: 1},
				{ID: 2, VoyageID: 5, Name: "Vis", Position: 2},
			}, nil
		},
		setPortPositionFn: func(_ context.Context, arg sqlcdb.SetVoyagePortPositionParams) (int64, error) {
			got = append(got, arg)
			return 1, nil
		},
	}

	resp := voyagePortTestAPI(t, m).PutCtx(userCtx(context.Background()),
		"/voyages/5/ports/order", map[string]any{"port_ids": []int64{3, 1, 2}})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	want := []sqlcdb.SetVoyagePortPositionParams{
		{ID: 3, VoyageID: 5, Position: 0, CallerIsAdmin: true, CallerID: 1},
		{ID: 1, VoyageID: 5, Position: 1, CallerIsAdmin: true, CallerID: 1},
		{ID: 2, VoyageID: 5, Position: 2, CallerIsAdmin: true, CallerID: 1},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("position writes = %+v, want %+v", got, want)
	}
	var ports []dto.VoyagePort
	if err := json.Unmarshal(resp.Body.Bytes(), &ports); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ports) != 3 || ports[0].Name != "Hvar" || ports[2].Name != "Vis" {
		t.Fatalf("unexpected ports: %+v", ports)
	}
}

func TestVoyagePorts_Reorder_StopsOnError(t *testing.T) {
	writes := 0
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
		},
		listVoyagePortsFn: func(context.Context, int64) ([]sqlcdb.VoyagePort, error) {
			return []sqlcdb.VoyagePort{
				{ID: 3, VoyageID: 5, Name: "Hvar"},
				{ID: 1, VoyageID: 5, Name: "Split"},
			}, nil
		},
		setPortPositionFn: func(context.Context, sqlcdb.SetVoyagePortPositionParams) (int64, error) {
			writes++
			return 0, errors.New("boom")
		},
	}
	resp := voyagePortTestAPI(t, m).PutCtx(userCtx(context.Background()),
		"/voyages/5/ports/order", map[string]any{"port_ids": []int64{3, 1}})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", resp.Code, resp.Body)
	}
	if writes != 1 {
		t.Fatalf("position writes = %d, want 1", writes)
	}
}

// TestVoyagePorts_Reorder_PortGoneMeanwhile covers a position write that
// changes no row while the caller still passes the captain check: a port
// went away after the list was read, so the reorder no longer matches it.
func TestVoyagePorts_Reorder_PortGoneMeanwhile(t *testing.T) {
	writes := 0
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
		},
		listVoyagePortsFn: func(context.Context, int64) ([]sqlcdb.VoyagePort, error) {
			return []sqlcdb.VoyagePort{{ID: 3, VoyageID: 5}, {ID: 1, VoyageID: 5}}, nil
		},
		setPortPositionFn: func(context.Context, sqlcdb.SetVoyagePortPositionParams) (int64, error) {
			writes++
			return 0, nil
		},
	}
	resp := voyagePortTestAPI(t, m).PutCtx(userCtx(context.Background()),
		"/voyages/5/ports/order", map[string]any{"port_ids": []int64{3, 1}})
	if resp.Code != http.StatusUnprocessableEntity || writes != 1 {
		t.Fatalf("status = %d after %d writes, want 422 after 1; body=%s", resp.Code, writes, resp.Body)
	}
}

func TestVoyagePorts_Reorder_RejectsMismatch(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
		},
		listVoyagePortsFn: func(context.Context, int64) ([]sqlcdb.VoyagePort, error) {
			return []sqlcdb.VoyagePort{
				{ID: 3, VoyageID: 5, Name: "Hvar"},
				{ID: 1, VoyageID: 5, Name: "Split"},
			}, nil
		},
	}
	// Body omits port 3 and invents port 9 — must be rejected, not committed.
	resp := voyagePortTestAPI(t, m).PutCtx(userCtx(context.Background()), "/voyages/5/ports/order",
		map[string]any{"port_ids": []int64{1, 9}})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", resp.Code, resp.Body)
	}
}

func TestVoyagePorts_Remove(t *testing.T) {
	var got sqlcdb.DeleteVoyagePortParams
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: 5}, nil },
		deleteVoyagePortFn: func(_ context.Context, arg sqlcdb.DeleteVoyagePortParams) (bool, error) {
			got = arg
			return true, nil
		},
	}
	resp := voyagePortTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/5/ports/9")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if got.ID != 9 || got.VoyageID != 5 {
		t.Fatalf("unexpected delete params: %+v", got)
	}
}

func TestVoyagePorts_Add_MemberForbidden(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: 5}, nil },
	}
	resp := voyagePortTestAPI(t, m).PostCtx(
		userCtxRole(context.Background(), "member"), "/voyages/5/ports",
		map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", resp.Code, resp.Body)
	}
}
