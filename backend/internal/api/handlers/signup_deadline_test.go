package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// enrollTestAPIAt is enrollTestAPI with the sign-up clock pinned to now.
func enrollTestAPIAt(t *testing.T, m *mockQuerier, now time.Time) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: m, runTx: directTxRunner(m), now: func() time.Time { return now }})
	return api
}

const testDeadline = "2027-05-10"

// deadlineClock is one instant relative to testDeadline. The UTC instants
// straddle the Warsaw midnight, so a UTC-based rule would fail "same day,
// late evening" or "day after, just past midnight".
var deadlineClock = []struct {
	name string
	now  time.Time
	open bool
}{
	{"day before", time.Date(2027, 5, 9, 12, 0, 0, 0, time.UTC), true},
	{"same day, late evening in Warsaw", time.Date(2027, 5, 10, 21, 30, 0, 0, time.UTC), true},
	{"day after, just past midnight in Warsaw", time.Date(2027, 5, 10, 22, 30, 0, 0, time.UTC), false},
}

func deadlineTrip(id int64) sqlcdb.Trip {
	t := openTrip(id)
	t.SignUpDeadline = types.NullString{String: testDeadline, Valid: true}
	return t
}

func TestSignUpForTrip_Deadline(t *testing.T) {
	for _, c := range deadlineClock {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(deadlineTrip(5))
			resp := enrollTestAPIAt(t, s.mock(), c.now).PostCtx(guestCtx(42), "/trips/5/sign-up")
			want := http.StatusCreated
			if !c.open {
				want = http.StatusUnprocessableEntity
			}
			if resp.Code != want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, want, resp.Body)
			}
			if !c.open && (s.inserts != 0 || len(s.rows) != 0) {
				t.Fatalf("inserts = %d, rows = %d; want nothing written", s.inserts, len(s.rows))
			}
		})
	}
}

func TestSignUpForTrip_NoDeadlineStaysOpen(t *testing.T) {
	s := newSignUpStore(openTrip(5))
	resp := enrollTestAPIAt(t, s.mock(), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
	}
}

func TestSignUpForTrip_ExistingSignUpAfterDeadlineIsReturned(t *testing.T) {
	s := newSignUpStore(deadlineTrip(5))
	s.rows = []sqlcdb.TripEnrollment{{ID: 3, TripID: 5, UserID: 42, Status: "pending"}}
	resp := enrollTestAPIAt(t, s.mock(), deadlineClock[2].now).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
}

func TestGetTripSignUp_Deadline(t *testing.T) {
	for _, c := range deadlineClock {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(deadlineTrip(5))
			resp := enrollTestAPIAt(t, s.mock(), c.now).GetCtx(guestCtx(42), "/trips/5/sign-up")
			if resp.Code != http.StatusOK {
				t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
			}
			var got dto.LegSignUp
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Open != c.open {
				t.Fatalf("open = %v, want %v", got.Open, c.open)
			}
		})
	}
}

func TestListCruiseLegSignUps_Deadline(t *testing.T) {
	trips := []sqlcdb.Trip{openTrip(11), deadlineTrip(12)}
	m := newLegMock(&legStore{}, trips)
	m.getCruiseFn = func(_ context.Context, id int64) (sqlcdb.Cruise, error) { return sqlcdb.Cruise{ID: id}, nil }
	resp := enrollTestAPIAt(t, m, deadlineClock[2].now).GetCtx(guestCtx(42), "/cruises/7/leg-sign-ups")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got []dto.LegSignUp
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || !got[0].Open || got[1].Open {
		t.Fatalf("legs = %+v, want leg 11 open and leg 12 closed by its deadline", got)
	}
}

func TestEnroll_TripToken_Deadline(t *testing.T) {
	for _, c := range deadlineClock {
		t.Run(c.name, func(t *testing.T) {
			inserts := 0
			m := &mockQuerier{
				getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
					return sqlcdb.GetTripByEnrollTokenRow{ID: 5}, nil
				},
				lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
					return sqlcdb.LockTripForSignUpRow{
						ID: id, Status: sqlcdb.TripStatusPlanned, Open: true,
						SignUpDeadline: types.NullString{String: testDeadline, Valid: true},
					}, nil
				},
				createTripEnrollmentFn: func(_ context.Context, arg sqlcdb.CreateTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
					inserts++
					return sqlcdb.TripEnrollment{ID: 1, TripID: arg.TripID, UserID: arg.UserID, Status: "pending"}, nil
				},
			}
			resp := enrollTestAPIAt(t, m, c.now).PostCtx(userCtx(context.Background()), "/enroll/tok", map[string]any{})
			want := http.StatusCreated
			if !c.open {
				want = http.StatusUnprocessableEntity
			}
			if resp.Code != want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, want, resp.Body)
			}
			if !c.open && inserts != 0 {
				t.Fatalf("inserts = %d, want none", inserts)
			}
		})
	}
}

func TestEnrollLegs_Deadline(t *testing.T) {
	for _, c := range deadlineClock {
		t.Run(c.name, func(t *testing.T) {
			trips := legTrips()
			trips[1].SignUpDeadline = types.NullString{String: testDeadline, Valid: true}
			s := &legStore{}
			resp := enrollTestAPIAt(t, newLegMock(s, trips), c.now).PostCtx(
				userCtxRole(context.Background(), "member"), "/enroll/tok/legs", map[string]any{"trip_ids": []int64{11, 12}})
			want := http.StatusCreated
			if !c.open {
				want = http.StatusUnprocessableEntity
			}
			if resp.Code != want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, want, resp.Body)
			}
			if !c.open && (s.inserts != 0 || len(s.rows) != 0) {
				t.Fatalf("inserts = %d, rows = %d; want nothing written", s.inserts, len(s.rows))
			}
		})
	}
}

func TestEnrollment_GetByToken_TripShowsDeadline(t *testing.T) {
	m := &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{ID: 5, Name: "Adriatic", SignUpDeadline: types.NullString{String: testDeadline, Valid: true}}, nil
		},
		countTripEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountTripEnrollmentsRow, error) {
			return sqlcdb.CountTripEnrollmentsRow{}, nil
		},
		countAcceptedForTripFn: func(context.Context, int64) (int64, error) { return 0, nil },
	}
	resp := enrollTestAPI(t, m).Get("/enroll/tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got dto.EnrollInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Trip == nil || got.Trip.SignUpDeadline == nil || *got.Trip.SignUpDeadline != testDeadline {
		t.Fatalf("trip = %+v, want sign_up_deadline %s", got.Trip, testDeadline)
	}
}

func TestTrip_SignUpDeadlineValidation(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
		want types.NullString
		code int
	}{
		{"no deadline", map[string]any{"cruise_id": 1, "name": "L", "embark_date": "2027-06-01"}, types.NullString{}, 0},
		{
			"deadline before embark",
			map[string]any{"cruise_id": 1, "name": "L", "embark_date": "2027-06-01", "sign_up_deadline": "2027-05-10"},
			types.NullString{String: "2027-05-10", Valid: true},
			0,
		},
		{
			"deadline on embark day",
			map[string]any{"cruise_id": 1, "name": "L", "embark_date": "2027-06-01", "sign_up_deadline": "2027-06-01"},
			types.NullString{String: "2027-06-01", Valid: true},
			0,
		},
		{
			"deadline without embark date",
			map[string]any{"cruise_id": 1, "name": "L", "sign_up_deadline": "2027-05-10"},
			types.NullString{String: "2027-05-10", Valid: true},
			0,
		},
		{
			"deadline after embark",
			map[string]any{"cruise_id": 1, "name": "L", "embark_date": "2027-06-01", "sign_up_deadline": "2027-06-02"},
			types.NullString{},
			http.StatusUnprocessableEntity,
		},
		{
			"malformed deadline",
			map[string]any{"cruise_id": 1, "name": "L", "sign_up_deadline": "10.05.2027"},
			types.NullString{},
			http.StatusUnprocessableEntity,
		},
	}
	for _, c := range cases {
		t.Run("create "+c.name, func(t *testing.T) {
			var saved *sqlcdb.CreateTripParams
			m := &mockQuerier{
				createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					saved = &arg
					return sqlcdb.Trip{ID: 1, Name: arg.Name, Status: sqlcdb.TripStatusPlanned, SignUpDeadline: arg.SignUpDeadline}, nil
				},
			}
			resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", c.body)
			if c.code != 0 {
				if resp.Code != c.code || saved != nil {
					t.Fatalf("got %d (saved %v), want %d and nothing saved; body=%s", resp.Code, saved != nil, c.code, resp.Body)
				}
				return
			}
			if resp.Code != http.StatusCreated || saved == nil || saved.SignUpDeadline != c.want {
				t.Fatalf("got %d, saved %+v; want 201 with deadline %+v; body=%s", resp.Code, saved, c.want, resp.Body)
			}
			var trip dto.Trip
			if err := json.Unmarshal(resp.Body.Bytes(), &trip); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if (trip.SignUpDeadline == nil) == c.want.Valid {
				t.Fatalf("response sign_up_deadline = %v, want %+v", trip.SignUpDeadline, c.want)
			}
		})
		t.Run("update "+c.name, func(t *testing.T) {
			var saved *sqlcdb.UpdateTripParams
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id, CruiseID: 1}, nil },
				updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
					saved = &arg
					return nil
				},
			}
			resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/1", c.body)
			if c.code != 0 {
				if resp.Code != c.code || saved != nil {
					t.Fatalf("got %d (saved %v), want %d and nothing saved; body=%s", resp.Code, saved != nil, c.code, resp.Body)
				}
				return
			}
			if resp.Code != http.StatusNoContent || saved == nil || saved.SignUpDeadline != c.want {
				t.Fatalf("got %d, saved %+v; want 204 with deadline %+v; body=%s", resp.Code, saved, c.want, resp.Body)
			}
		})
	}
}
