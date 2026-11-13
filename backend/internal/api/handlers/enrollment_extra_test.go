package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
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
			listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
				return []sqlcdb.Trip{{ID: 1, Name: "Leg"}}, nil
			},
			countAcceptedByCruiseTripFn: func(_ context.Context, cid int64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
				if cid != 7 {
					t.Fatalf("cruise id = %d, want 7", cid)
				}
				return []sqlcdb.CountAcceptedByCruiseTripRow{{TripID: 1, Accepted: 2}}, nil
			},
			listUserCruiseTripEnrollFn: func(context.Context, sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
				return nil, nil
			},
			getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{ID: 4, CruiseID: 7, UserID: 1, Status: "pending"}, nil
			},
		}
		resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var info dto.EnrollInfo
		if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(info.TripAccepted) != 1 || info.TripAccepted[0] != (dto.TripAccepted{TripID: 1, AcceptedCount: 2}) {
			t.Fatalf("trip_accepted = %+v, want [{1 2}]", info.TripAccepted)
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
			listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
				return nil, errors.New("fail")
			},
			countAcceptedByCruiseTripFn: func(context.Context, int64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
				return nil, errors.New("fail")
			},
			listUserCruiseTripEnrollFn: func(context.Context, sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
				return nil, nil
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

// TestEnrollment_GetByToken_CruiseLegsArePublicSummaries checks the public
// cruise enroll response lists legs without their own enroll tokens, creator
// or total cost, while keeping what the enroll page shows.
func TestEnrollment_GetByToken_CruiseLegsArePublicSummaries(t *testing.T) {
	leg := sqlcdb.Trip{
		ID:                     1,
		Name:                   "Bodø – Tromsø",
		CreatedBy:              types.NullInt64{Int64: 42, Valid: true},
		CruiseID:               7,
		EmbarkDate:             types.NullString{String: "2025-07-01", Valid: true},
		DisembarkDate:          types.NullString{String: "2025-07-10", Valid: true},
		StartPort:              types.NullString{String: "Bodø", Valid: true},
		EndPort:                types.NullString{String: "Tromsø", Valid: true},
		CaptainName:            types.NullString{String: "Jan Kowalski", Valid: true},
		CaptainUserID:          types.NullInt64{Int64: 9, Valid: true},
		MaxCrew:                types.NullInt64{Int64: 8, Valid: true},
		CostTotal:              types.NullFloat64{Float64: 20000, Valid: true},
		CostPerPerson:          types.NullFloat64{Float64: 2500, Valid: true},
		CostPerPersonNonMember: types.NullFloat64{Float64: 3000, Valid: true},
		YachtFund:              types.NullFloat64{Float64: 500, Valid: true},
		Deposit:                types.NullFloat64{Float64: 200, Valid: true},
		Currency:               "EUR",
		Status:                 sqlcdb.TripStatusCancelled,
		EnrollToken:            types.NullString{String: "leg-secret", Valid: true},
	}
	m := &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
		},
		getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
			return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7, Name: "NNE 2025"}, nil
		},
		countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
			return sqlcdb.CountCruiseEnrollmentsRow{}, nil
		},
		listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) {
			return []sqlcdb.Trip{leg}, nil
		},
		countAcceptedByCruiseTripFn: func(context.Context, int64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
			return nil, nil
		},
	}
	// Anonymous caller: the enroll link needs no login.
	resp := enrollTestAPI(t, m).GetCtx(context.Background(), "/enroll/tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var raw struct {
		Trips []map[string]any `json:"trips"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw.Trips) != 1 {
		t.Fatalf("trips = %d, want 1; body=%s", len(raw.Trips), resp.Body)
	}
	got := raw.Trips[0]
	for _, k := range []string{"enroll_token", "created_by", "cost_total", "captain_user_id"} {
		if _, ok := got[k]; ok {
			t.Errorf("trips[0] exposes %q: %v", k, got[k])
		}
	}
	want := map[string]any{
		"id":                         float64(1),
		"name":                       "Bodø – Tromsø",
		"status":                     "cancelled",
		"embark_date":                "2025-07-01",
		"disembark_date":             "2025-07-10",
		"start_port":                 "Bodø",
		"end_port":                   "Tromsø",
		"captain_name":               "Jan Kowalski",
		"max_crew":                   float64(8),
		"currency":                   "EUR",
		"cost_per_person":            float64(2500),
		"cost_per_person_non_member": float64(3000),
		"yacht_fund":                 float64(500),
		"deposit":                    float64(200),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("trips[0][%q] = %v, want %v", k, got[k], v)
		}
	}
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
	t.Run("cruise token takes no whole-cruise sign-up", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7}, nil
			},
		}
		resp := enrollTestAPI(t, m).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{"note": "x"})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
		}
		if !strings.Contains(resp.Body.String(), "/enroll/{token}/legs") {
			t.Fatalf("body must point to the legs operation: %s", resp.Body)
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

	t.Run("cruise lookup db error", func(t *testing.T) {
		m := &mockQuerier{
			getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
				return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
			},
			getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, errors.New("fail")
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
	if NewEnrollmentHandler(&mockQuerier{}, nil) == nil {
		t.Fatal("want non-nil handler")
	}
}
