package handlers

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func cruiseTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	if m.getCruiseSeriesFn == nil {
		m.getCruiseSeriesFn = func(_ context.Context, id int64) (sqlcdb.CruiseSeries, error) {
			return sqlcdb.CruiseSeries{ID: id, Name: "Seria"}, nil
		}
	}
	_, api := humatest.New(t)
	registerCruiseRoutes(api, &CruiseHandler{q: m, runTx: directTxRunner(m)})
	return api
}

// asDBEdition returns body as a valid new cruise against a real database: an
// edition of a series it inserts.
func asDBEdition(t *testing.T, conn *sql.DB, body map[string]any) map[string]any {
	t.Helper()
	var seriesID int64
	if err := conn.QueryRow("INSERT INTO cruise_series(name) VALUES ('Seria') RETURNING id").Scan(&seriesID); err != nil {
		t.Fatalf("insert series: %v", err)
	}
	out := asEdition(body)
	out["series_id"] = seriesID
	return out
}

// asEdition returns body as a valid new cruise: an edition of series 1.
func asEdition(body map[string]any) map[string]any {
	out := map[string]any{"series_id": 1, "edition": "2026"}
	maps.Copy(out, body)
	return out
}

func TestCruiseHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCruisesFn: func(context.Context, sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error) {
				return []sqlcdb.Cruise{{ID: 1, Name: "Baltic"}}, nil
			},
			countCruisesFn: func(context.Context) (int64, error) { return 1, nil },
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("list error", func(t *testing.T) {
		m := &mockQuerier{
			listCruisesFn: func(context.Context, sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error) {
				return nil, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("count error", func(t *testing.T) {
		m := &mockQuerier{
			listCruisesFn:  func(context.Context, sqlcdb.ListCruisesParams) ([]sqlcdb.Cruise, error) { return nil, nil },
			countCruisesFn: func(context.Context) (int64, error) { return 0, errors.New("fail") },
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: id, Name: "Adriatic"}, nil
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", asEdition(map[string]any{"name": "New"}))
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/cruises", asEdition(map[string]any{}))
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", asEdition(map[string]any{"name": "X"}))
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PostCtx(
			userCtxRole(context.Background(), "member"), "/cruises", map[string]any{"name": "X"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestCruiseHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateCruiseFn: func(context.Context, sqlcdb.UpdateCruiseParams) error { return nil },
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", map[string]any{"name": "Renamed"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateCruiseFn: func(context.Context, sqlcdb.UpdateCruiseParams) error { return errors.New("fail") },
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/1", map[string]any{"name": "X"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).PutCtx(
			userCtxRole(context.Background(), "member"), "/cruises/1", map[string]any{"name": "X"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestCruiseHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{deleteCruiseFn: func(context.Context, int64) error { return nil }}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{deleteCruiseFn: func(context.Context, int64) error { return errors.New("fail") }}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_GenerateEnrollToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: id}, nil
			},
			setCruiseEnrollToken: func(context.Context, sqlcdb.SetCruiseEnrollTokenParams) error { return nil },
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises/1/enroll-token")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("cruise not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises/1/enroll-token")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("set token error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn:          func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil },
			setCruiseEnrollToken: func(context.Context, sqlcdb.SetCruiseEnrollTokenParams) error { return errors.New("fail") },
		}
		resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises/1/enroll-token")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_ClearEnrollToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{clearCruiseEnrollToken: func(context.Context, int64) error { return nil }}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1/enroll-token")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{clearCruiseEnrollToken: func(context.Context, int64) error { return errors.New("fail") }}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1/enroll-token")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_ListChildTrips(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil },
			listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
				return []sqlcdb.Trip{{ID: 1, Name: "Leg 1"}}, nil
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/trips")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("cruise not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{}, sql.ErrNoRows },
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/trips")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil },
			listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
				return nil, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("verify cruise db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/trips")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_ListChildVoyages(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil },
			listCruiseVoyagesFn: func(context.Context, int64) ([]sqlcdb.Voyage, error) {
				return []sqlcdb.Voyage{{ID: 1, Name: "Logged leg"}}, nil
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/voyages")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil },
			listCruiseVoyagesFn: func(context.Context, int64) ([]sqlcdb.Voyage, error) {
				return nil, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/voyages")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_ListEnrollments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseEnrollmentsFn: func(context.Context, int64) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
				return []sqlcdb.ListCruiseEnrollmentsRow{{ID: 1, CruiseID: 1, UserID: 2, Status: "pending"}}, nil
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/enrollments")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseEnrollmentsFn: func(context.Context, int64) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
				return nil, errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1/enrollments")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := cruiseTestAPI(t, &mockQuerier{}).GetCtx(
			userCtxRole(context.Background(), "member"), "/cruises/1/enrollments")
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

// unassignedCruiseEnrollment is a pending enrollment 2 of cruise 1 with no
// leg, so status and assign changes need no capacity check.
func unassignedCruiseEnrollment(context.Context, sqlcdb.GetCruiseEnrollmentOfCruiseParams) (sqlcdb.CruiseEnrollment, error) {
	return sqlcdb.CruiseEnrollment{ID: 2, CruiseID: 1, UserID: 9, Status: "pending"}, nil
}

func TestCruiseHandler_UpdateEnrollmentStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn:  unassignedCruiseEnrollment,
			updateCruiseEnrollmentStatusFn: func(context.Context, sqlcdb.UpdateCruiseEnrollmentStatusParams) error { return nil },
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/cruises/1/enrollments/2/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
			updateCruiseEnrollmentStatusFn: func(context.Context, sqlcdb.UpdateCruiseEnrollmentStatusParams) error {
				return errors.New("fail")
			},
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/cruises/1/enrollments/2/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_AssignEnrollmentToTrip(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
			assignCruiseEnrollmentFn:      func(context.Context, sqlcdb.AssignCruiseEnrollmentToTripParams) error { return nil },
			lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
				return sqlcdb.LockTripForSignUpRow{ID: id}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/cruises/1/enrollments/2/trip", map[string]any{"trip_id": 5})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
			assignCruiseEnrollmentFn: func(context.Context, sqlcdb.AssignCruiseEnrollmentToTripParams) error {
				return errors.New("fail")
			},
			lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
				return sqlcdb.LockTripForSignUpRow{ID: id}, nil
			},
		}
		resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/cruises/1/enrollments/2/trip", map[string]any{"trip_id": 5})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestCruiseHandler_DeleteEnrollment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
			deleteCruiseEnrollmentFn:      func(context.Context, int64) error { return nil },
		}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1/enrollments/2")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseEnrollmentOfCruiseFn: unassignedCruiseEnrollment,
			deleteCruiseEnrollmentFn:      func(context.Context, int64) error { return errors.New("fail") },
		}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1/enrollments/2")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
	t.Run("missing enrollment is a no-op", func(t *testing.T) {
		m := &mockQuerier{getCruiseEnrollmentOfCruiseFn: func(context.Context, sqlcdb.GetCruiseEnrollmentOfCruiseParams) (sqlcdb.CruiseEnrollment, error) {
			return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
		}}
		resp := cruiseTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/cruises/1/enrollments/2")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})
}

func TestNewCruiseHandler(t *testing.T) {
	if NewCruiseHandler(&mockQuerier{}, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
