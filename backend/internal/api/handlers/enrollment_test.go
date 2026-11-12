package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
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
		countAcceptedForTripFn: func(_ context.Context, tripID int64) (int64, error) {
			if tripID != 5 {
				t.Fatalf("trip id = %d, want 5", tripID)
			}
			return 3, nil
		},
		getUserTripEnrollmentFn: func(context.Context, sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/abc123")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var info dto.EnrollInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.AcceptedCount != 2 || len(info.TripAccepted) != 1 || info.TripAccepted[0] != (dto.TripAccepted{TripID: 5, AcceptedCount: 3}) {
		t.Fatalf("accepted_count = %d, trip_accepted = %+v; want 2 and [{5 3}]", info.AcceptedCount, info.TripAccepted)
	}
}

func TestEnrollment_GetByToken_TripAcceptedCountErrorTolerated(t *testing.T) {
	m := &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{ID: 5, Name: "Adriatic"}, nil
		},
		countTripEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountTripEnrollmentsRow, error) {
			return sqlcdb.CountTripEnrollmentsRow{Accepted: 2, Total: 4}, nil
		},
		countAcceptedForTripFn: func(context.Context, int64) (int64, error) { return 0, errors.New("fail") },
	}
	resp := enrollTestAPI(t, m).GetCtx(context.Background(), "/enroll/abc123")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	if strings.Contains(resp.Body.String(), "trip_accepted") {
		t.Fatalf("trip_accepted must be omitted on count error: %s", resp.Body)
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
		getTripFn: func(context.Context, int64) (sqlcdb.Trip, error) {
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
		listTripEnrollmentsFn: func(context.Context, int64) ([]sqlcdb.ListTripEnrollmentsRow, error) {
			return []sqlcdb.ListTripEnrollmentsRow{{ID: 1, TripID: 9, Status: "pending", UserName: "Jan"}}, nil
		},
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/trips/9/enrollments")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}
