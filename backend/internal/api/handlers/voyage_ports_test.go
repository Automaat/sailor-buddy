package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func voyagePortTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterVoyagePortRoutes(api, m, nil)
	return api
}

// voyagePortTestAPIWithDB wires the routes with a real *sql.DB (a sqlmock)
// so the transactional reorder path can be exercised.
func voyagePortTestAPIWithDB(t *testing.T, m *mockQuerier, db *sql.DB) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterVoyagePortRoutes(api, m, db)
	return api
}

func TestVoyagePorts_List(t *testing.T) {
	m := &mockQuerier{
		listVoyagePortsFn: func(_ context.Context, voyageID int64) ([]sqlcdb.VoyagePort, error) {
			if voyageID != 5 {
				t.Fatalf("unexpected voyage id: %d", voyageID)
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
		getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: 5}, nil
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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	}

	// Each port is renumbered to its index in the request body, all inside
	// one transaction.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE voyage_ports").WithArgs(int64(3), int64(5), int64(0)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE voyage_ports").WithArgs(int64(1), int64(5), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE voyage_ports").WithArgs(int64(2), int64(5), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp := voyagePortTestAPIWithDB(t, m, db).PutCtx(userCtx(context.Background()),
		"/voyages/5/ports/order", map[string]any{"port_ids": []int64{3, 1, 2}})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var ports []dto.VoyagePort
	if err := json.Unmarshal(resp.Body.Bytes(), &ports); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ports) != 3 || ports[0].Name != "Hvar" || ports[2].Name != "Vis" {
		t.Fatalf("unexpected ports: %+v", ports)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestVoyagePorts_Reorder_RollsBackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	// A failed position write must roll the whole transaction back.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE voyage_ports").WillReturnError(errors.New("boom"))
	mock.ExpectRollback()

	resp := voyagePortTestAPIWithDB(t, m, db).PutCtx(userCtx(context.Background()),
		"/voyages/5/ports/order", map[string]any{"port_ids": []int64{3, 1}})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", resp.Code, resp.Body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
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
		deleteVoyagePortFn: func(_ context.Context, arg sqlcdb.DeleteVoyagePortParams) error {
			got = arg
			return nil
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
	resp := voyagePortTestAPI(t, &mockQuerier{}).PostCtx(
		userCtxRole(context.Background(), "member"), "/voyages/5/ports",
		map[string]any{"name": "Hvar", "latitude": 43.17, "longitude": 16.44})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", resp.Code, resp.Body)
	}
}
