package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func enrollTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterEnrollmentRoutes(api, m)
	return api
}

func TestEnrollment_GetByToken_Trip(t *testing.T) {
	m := &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{ID: 5, Name: "Adriatic"}, nil
		},
		countTripEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountTripEnrollmentsRow, error) {
			return sqlcdb.CountTripEnrollmentsRow{Accepted: 2, Total: 4}, nil
		},
		getUserTripEnrollmentFn: func(context.Context, sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/abc123")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}

func TestEnrollment_GetByToken_Invalid(t *testing.T) {
	m := &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
		},
		getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
			return sqlcdb.GetCruiseByEnrollTokenRow{}, sql.ErrNoRows
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/nope")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
	}
}

func TestEnrollment_GenerateToken_TripNotFound(t *testing.T) {
	m := &mockQuerier{
		getTripFn: func(context.Context, sqlcdb.GetTripParams) (sqlcdb.Trip, error) {
			return sqlcdb.Trip{}, sql.ErrNoRows
		},
	}
	resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips/9/enroll-token")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", resp.Code)
	}
}

func TestEnrollment_UpdateStatus_InvalidEnum(t *testing.T) {
	resp := enrollTestAPI(t, &mockQuerier{}).PutCtx(userCtx(context.Background()),
		"/trips/9/enrollments/1/status", map[string]any{"status": "bogus"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d, want 422", resp.Code)
	}
}

func TestEnrollment_ListEnrollments(t *testing.T) {
	m := &mockQuerier{
		listTripEnrollmentsFn: func(context.Context, sqlcdb.ListTripEnrollmentsParams) ([]sqlcdb.ListTripEnrollmentsRow, error) {
			return []sqlcdb.ListTripEnrollmentsRow{{ID: 1, TripID: 9, Status: "pending", UserName: "Jan"}}, nil
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/9/enrollments")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}
