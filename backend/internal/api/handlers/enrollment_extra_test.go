package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestEnrollment_GetByToken_Cruise(t *testing.T) {
	t.Run("success with child trips", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7, Name: "Baltic"}, nil
			},
			countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{Accepted: 1, Total: 3}, nil
			},
			listCruiseTripsFn: func(context.Context, types.NullInt64) ([]sqlcdb.Trip, error) {
				return []sqlcdb.Trip{{ID: 1, Name: "Leg"}}, nil
			},
			getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{ID: 4, CruiseID: 7, UserID: 1, Status: "pending"}, nil
			},
		}
		resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("child trips list error is tolerated", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7, Name: "Baltic"}, nil
			},
			countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{}, nil
			},
			listCruiseTripsFn: func(context.Context, types.NullInt64) ([]sqlcdb.Trip, error) {
				return nil, errors.New("fail")
			},
			getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
			},
		}
		resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestEnrollment_GetByToken_Errors(t *testing.T) {
	t.Run("trip lookup db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("cruise lookup db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_Enroll_Trip(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{ID: 5}, nil
			},
			getTripStatusFn: func(context.Context, int64) (sqlcdb.TripStatus, error) {
				return sqlcdb.TripStatusPlanned, nil
			},
			createTripEnrollmentFn: func(_ context.Context, arg sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{ID: 1, TripID: arg.TripID, UserID: arg.UserID, Status: "pending"}, nil
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("trip not planned is closed", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{ID: 5}, nil
			},
			getTripStatusFn: func(context.Context, int64) (sqlcdb.TripStatus, error) {
				return sqlcdb.TripStatus("cancelled"), nil
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.Code)
		}
	})

	t.Run("duplicate enrollment is a conflict", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{ID: 5}, nil
			},
			getTripStatusFn: func(context.Context, int64) (sqlcdb.TripStatus, error) {
				return sqlcdb.TripStatusPlanned, nil
			},
			createTripEnrollmentFn: func(context.Context, sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{}, &pgconn.PgError{Code: "23505"}
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.Code)
		}
	})

	t.Run("create enrollment db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{ID: 5}, nil
			},
			getTripStatusFn: func(context.Context, int64) (sqlcdb.TripStatus, error) {
				return sqlcdb.TripStatusPlanned, nil
			},
			createTripEnrollmentFn: func(context.Context, sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_Enroll_Cruise(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7}, nil
			},
			createCruiseEnrollmentFn: func(_ context.Context, arg sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{ID: 1, CruiseID: arg.CruiseID, UserID: arg.UserID, Status: "pending"}, nil
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("invalid link", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, sql.ErrNoRows
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("duplicate enrollment is a conflict", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7}, nil
			},
			createCruiseEnrollmentFn: func(context.Context, sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, &pgconn.PgError{Code: "23505"}
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409", resp.Code)
		}
	})

	t.Run("create enrollment db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7}, nil
			},
			createCruiseEnrollmentFn: func(context.Context, sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("trip lookup db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_GenerateToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn:            func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			setTripEnrollTokenFn: func(context.Context, sqlcdb.SetTripEnrollTokenParams) error { return nil },
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/enroll-token")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("get trip db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
				return sqlcdb.Trip{}, errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/enroll-token")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("set token db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			setTripEnrollTokenFn: func(context.Context, sqlcdb.SetTripEnrollTokenParams) error {
				return errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/enroll-token")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_ClearToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{clearTripEnrollTokenFn: func(context.Context, int64) error { return nil }}
		resp := enrollTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/enroll-token")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{clearTripEnrollTokenFn: func(context.Context, int64) error { return errors.New("fail") }}
		resp := enrollTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/enroll-token")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_ListEnrollments_DBError(t *testing.T) {
	m := &mockQuerier{
		listTripEnrollmentsFn: func(context.Context, int64) ([]sqlcdb.ListTripEnrollmentsRow, error) {
			return nil, errors.New("fail")
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/9/enrollments")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestEnrollment_UpdateStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateTripEnrollmentStatusFn: func(context.Context, sqlcdb.UpdateTripEnrollmentStatusParams) error { return nil },
		}
		resp := enrollTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/trips/9/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateTripEnrollmentStatusFn: func(context.Context, sqlcdb.UpdateTripEnrollmentStatusParams) error {
				return errors.New("fail")
			},
		}
		resp := enrollTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/trips/9/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestEnrollment_DeleteEnrollment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{deleteTripEnrollmentFn: func(context.Context, int64) error { return nil }}
		resp := enrollTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/enrollments/1")
		if resp.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", resp.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{deleteTripEnrollmentFn: func(context.Context, int64) error { return errors.New("fail") }}
		resp := enrollTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/trips/9/enrollments/1")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("want true for code 23505")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("want false for a non-unique pg error")
	}
	if isUniqueViolation(errors.New("plain")) {
		t.Fatal("want false for a non-pg error")
	}
}

func TestNewEnrollmentHandler(t *testing.T) {
	if NewEnrollmentHandler(&mockQuerier{}) == nil {
		t.Fatal("want non-nil handler")
	}
}
