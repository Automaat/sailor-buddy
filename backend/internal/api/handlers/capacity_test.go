package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// limitedTrip is an open leg with the given max_crew; 0 leaves it unset.
func limitedTrip(id, maxCrew int64) sqlcdb.Trip {
	t := openTrip(id)
	if maxCrew > 0 {
		t.MaxCrew = types.NullInt64{Int64: maxCrew, Valid: true}
	}
	return t
}

// enrollmentRows builds sign-ups of trip 5, one per user starting at 100,
// with the given statuses, in sign-up order.
func enrollmentRows(statuses ...string) []sqlcdb.TripEnrollment {
	rows := make([]sqlcdb.TripEnrollment, len(statuses))
	for i, st := range statuses {
		rows[i] = sqlcdb.TripEnrollment{ID: int64(i + 1), TripID: 5, UserID: int64(100 + i), Status: st, PaymentStatus: "unpaid"}
	}
	return rows
}

func TestUpdateTripEnrollmentStatus_Capacity(t *testing.T) {
	cases := []struct {
		name       string
		trip       sqlcdb.Trip
		rows       []string
		target     int64
		status     string
		want       int
		wantStatus string
	}{
		{"last free place is accepted", limitedTrip(5, 3), []string{"accepted", "accepted", "pending"}, 3, "accepted", http.StatusNoContent, "accepted"},
		{"full leg refuses an accept", limitedTrip(5, 3), []string{"accepted", "accepted", "accepted", "waitlisted"}, 4, "accepted", http.StatusConflict, "waitlisted"},
		{"over-full leg refuses an accept", limitedTrip(5, 1), []string{"accepted", "accepted", "pending"}, 3, "accepted", http.StatusConflict, "pending"},
		{"unset max_crew has no limit", limitedTrip(5, 0), []string{"accepted", "accepted", "accepted", "pending"}, 4, "accepted", http.StatusNoContent, "accepted"},
		{"re-accepting an accepted sign-up on a full leg", limitedTrip(5, 2), []string{"accepted", "accepted"}, 2, "accepted", http.StatusNoContent, "accepted"},
		{"full leg still takes a waitlist move", limitedTrip(5, 2), []string{"accepted", "accepted", "pending"}, 3, "waitlisted", http.StatusNoContent, "waitlisted"},
		{"full leg still takes a rejection", limitedTrip(5, 2), []string{"accepted", "accepted"}, 1, "rejected", http.StatusNoContent, "rejected"},
		{"rejected and waitlisted hold no place", limitedTrip(5, 2), []string{"accepted", "rejected", "waitlisted", "pending"}, 4, "accepted", http.StatusNoContent, "accepted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(c.trip)
			s.rows = enrollmentRows(c.rows...)
			resp := enrollTestAPI(t, s.mock()).PutCtx(userCtx(context.Background()),
				fmt.Sprintf("/trips/5/enrollments/%d/status", c.target), map[string]any{"status": c.status})
			if resp.Code != c.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			if got := s.rows[c.target-1].Status; got != c.wantStatus {
				t.Fatalf("status = %q, want %q", got, c.wantStatus)
			}
		})
	}

	t.Run("accepted cruise enrollment assigned to the leg takes a place", func(t *testing.T) {
		s := newSignUpStore(limitedTrip(5, 2))
		s.rows = enrollmentRows("accepted", "pending")
		s.legacy = []sqlcdb.CruiseEnrollment{{ID: 1, CruiseID: 7, UserID: 300, TripID: types.NullInt64{Int64: 5, Valid: true}, Status: "accepted"}}
		resp := enrollTestAPI(t, s.mock()).PutCtx(userCtx(context.Background()),
			"/trips/5/enrollments/2/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("enrollment of another trip is not found", func(t *testing.T) {
		s := newSignUpStore(limitedTrip(5, 3), limitedTrip(6, 3))
		s.rows = enrollmentRows("pending")
		resp := enrollTestAPI(t, s.mock()).PutCtx(userCtx(context.Background()),
			"/trips/6/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusNotFound || s.rows[0].Status != "pending" {
			t.Fatalf("got %d with status %q, want 404 and pending kept", resp.Code, s.rows[0].Status)
		}
	})

	t.Run("unknown trip is not found", func(t *testing.T) {
		s := newSignUpStore()
		resp := enrollTestAPI(t, s.mock()).PutCtx(userCtx(context.Background()),
			"/trips/9/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("members cannot change a status", func(t *testing.T) {
		s := newSignUpStore(limitedTrip(5, 3))
		s.rows = enrollmentRows("pending")
		resp := enrollTestAPI(t, s.mock()).PutCtx(guestCtx(100),
			"/trips/5/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusForbidden || s.rows[0].Status != "pending" {
			t.Fatalf("got %d with status %q, want 403 and pending kept", resp.Code, s.rows[0].Status)
		}
	})

	t.Run("db error", func(t *testing.T) {
		s := newSignUpStore(limitedTrip(5, 3))
		s.rows = enrollmentRows("pending")
		m := s.mock()
		m.updateTripEnrollmentStatusFn = func(context.Context, sqlcdb.UpdateTripEnrollmentStatusParams) error {
			return errors.New("fail")
		}
		resp := enrollTestAPI(t, m).PutCtx(userCtx(context.Background()),
			"/trips/5/enrollments/1/status", map[string]any{"status": "accepted"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestSignUpForTrip_Waitlist(t *testing.T) {
	cases := []struct {
		name         string
		trip         sqlcdb.Trip
		rows         []string
		wantStatus   string
		wantPosition int64
	}{
		{"free place starts pending", limitedTrip(5, 3), []string{"accepted", "pending"}, "pending", 0},
		{"accepted plus pending fill the leg", limitedTrip(5, 3), []string{"accepted", "pending", "pending"}, "waitlisted", 1},
		{"joins the end of the waiting list", limitedTrip(5, 1), []string{"accepted", "waitlisted", "waitlisted"}, "waitlisted", 3},
		{"rejected sign-ups hold no place", limitedTrip(5, 2), []string{"accepted", "rejected", "rejected"}, "pending", 0},
		{"no one jumps an earlier waitlisted sign-up", limitedTrip(5, 3), []string{"accepted", "rejected", "waitlisted"}, "waitlisted", 2},
		{"unset max_crew never waitlists", limitedTrip(5, 0), []string{"accepted", "accepted", "pending"}, "pending", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(c.trip)
			s.rows = enrollmentRows(c.rows...)
			s.nextID = int64(len(s.rows))
			resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), "/trips/5/sign-up")
			if resp.Code != http.StatusCreated {
				t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
			}
			got := decodeSignUp(t, resp.Body.Bytes())
			if got.Enrollment.Status != c.wantStatus {
				t.Fatalf("status = %q, want %q", got.Enrollment.Status, c.wantStatus)
			}
			switch {
			case c.wantPosition == 0 && got.WaitlistPosition != nil:
				t.Fatalf("position = %d, want none", *got.WaitlistPosition)
			case c.wantPosition != 0 && (got.WaitlistPosition == nil || *got.WaitlistPosition != c.wantPosition):
				t.Fatalf("position = %v, want %d", got.WaitlistPosition, c.wantPosition)
			}
		})
	}

	t.Run("signing up again keeps the position", func(t *testing.T) {
		s := newSignUpStore(limitedTrip(5, 1))
		s.rows = append(enrollmentRows("accepted", "waitlisted"),
			sqlcdb.TripEnrollment{ID: 3, TripID: 5, UserID: 42, Status: "waitlisted"})
		resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), "/trips/5/sign-up")
		got := decodeSignUp(t, resp.Body.Bytes())
		if resp.Code != http.StatusOK || got.Result != "already_enrolled" ||
			got.WaitlistPosition == nil || *got.WaitlistPosition != 2 {
			t.Fatalf("got %d %+v, want 200 already_enrolled at position 2", resp.Code, got)
		}
	})
}

func TestGetTripSignUp_WaitlistPosition(t *testing.T) {
	s := newSignUpStore(limitedTrip(5, 1))
	s.rows = append(enrollmentRows("accepted", "waitlisted", "rejected"),
		sqlcdb.TripEnrollment{ID: 4, TripID: 5, UserID: 42, Status: "waitlisted"})
	resp := enrollTestAPI(t, s.mock()).GetCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got dto.LegSignUp
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Enrollment == nil || got.Enrollment.Status != "waitlisted" || got.WaitlistPosition == nil || *got.WaitlistPosition != 2 {
		t.Fatalf("sign-up = %+v, want waitlisted at position 2", got)
	}
}

func TestEnrollLegs_WaitlistsFullLegs(t *testing.T) {
	trips := legTrips()
	trips[0].MaxCrew = types.NullInt64{Int64: 1, Valid: true}
	s := &legStore{rows: []sqlcdb.TripEnrollment{{ID: 1, TripID: 11, UserID: 99, Status: "pending"}}, nextID: 1}
	m := newLegMock(s, trips)
	m.countTripPlaceUseFn = func(_ context.Context, tripID int64) (sqlcdb.CountTripPlaceUseRow, error) {
		var n int64
		for i := range s.rows {
			if r := &s.rows[i]; r.TripID == tripID && (r.Status == "pending" || r.Status == "accepted") {
				n++
			}
		}
		return sqlcdb.CountTripPlaceUseRow{Taken: n}, nil
	}
	m.getWaitlistPositionFn = func(context.Context, int64) (int64, error) { return 1, nil }
	code, out, body := postLegs(t, m, userCtxRole(context.Background(), "member"), map[string]any{"trip_ids": []int64{11, 12}})
	if code != http.StatusCreated {
		t.Fatalf("got %d; body=%s", code, body)
	}
	byTrip := map[int64]dto.EnrollLegResult{}
	for _, r := range out.Results {
		byTrip[r.TripID] = r
	}
	if r := byTrip[11]; r.Enrollment.Status != "waitlisted" || r.WaitlistPosition == nil || *r.WaitlistPosition != 1 {
		t.Errorf("full leg 11 = %+v, want waitlisted at position 1", r)
	}
	if r := byTrip[12]; r.Enrollment.Status != "pending" || r.WaitlistPosition != nil {
		t.Errorf("unlimited leg 12 = %+v, want pending without a position", r)
	}
}

func TestInitialSignUpStatus_CountError(t *testing.T) {
	m := &mockQuerier{countTripPlaceUseFn: func(context.Context, int64) (sqlcdb.CountTripPlaceUseRow, error) {
		return sqlcdb.CountTripPlaceUseRow{}, sql.ErrConnDone
	}}
	if _, err := initialSignUpStatus(context.Background(), m, 5, types.NullInt64{Int64: 2, Valid: true}); !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("err = %v, want the count error", err)
	}
}

// cruiseEnrollmentMock adds cruise 7's enrollments (s.legacy) to the
// sign-up store's fake, so cruise enrollment changes see the legs' crew.
func cruiseEnrollmentMock(s *signUpStore) *mockQuerier {
	m := s.mock()
	m.getCruiseEnrollmentOfCruiseFn = func(_ context.Context, arg sqlcdb.GetCruiseEnrollmentOfCruiseParams) (sqlcdb.CruiseEnrollment, error) {
		for i := range s.legacy {
			if s.legacy[i].ID == arg.ID && s.legacy[i].CruiseID == arg.CruiseID {
				return s.legacy[i], nil
			}
		}
		return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
	}
	m.updateCruiseEnrollmentStatusFn = func(_ context.Context, arg sqlcdb.UpdateCruiseEnrollmentStatusParams) error {
		for i := range s.legacy {
			if s.legacy[i].ID == arg.ID {
				s.legacy[i].Status = arg.Status
			}
		}
		return nil
	}
	m.assignCruiseEnrollmentFn = func(_ context.Context, arg sqlcdb.AssignCruiseEnrollmentToTripParams) error {
		for i := range s.legacy {
			if s.legacy[i].ID == arg.ID {
				s.legacy[i].TripID = arg.TripID
			}
		}
		return nil
	}
	return m
}

func cruiseEnrollmentAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerCruiseRoutes(api, &CruiseHandler{q: m, runTx: directTxRunner(m)})
	return api
}

// TestCruiseEnrollment_Capacity runs against two legs: leg 5 has one place,
// already taken, and leg 6 has a free place.
func TestCruiseEnrollment_Capacity(t *testing.T) {
	onLeg := func(tripID int64) types.NullInt64 { return types.NullInt64{Int64: tripID, Valid: true} }
	cases := []struct {
		name       string
		legacy     sqlcdb.CruiseEnrollment
		path       string
		body       map[string]any
		want       int
		wantStatus string
		wantTrip   types.NullInt64
	}{
		{
			name:   "accepting on a full leg is refused",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, TripID: onLeg(5), Status: "pending"},
			path:   "/cruises/7/enrollments/1/status", body: map[string]any{"status": "accepted"},
			want: http.StatusConflict, wantStatus: "pending", wantTrip: onLeg(5),
		},
		{
			name:   "accepting on a leg with a free place",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, TripID: onLeg(6), Status: "pending"},
			path:   "/cruises/7/enrollments/1/status", body: map[string]any{"status": "accepted"},
			want: http.StatusNoContent, wantStatus: "accepted", wantTrip: onLeg(6),
		},
		{
			name:   "accepting without a leg needs no place",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, Status: "pending"},
			path:   "/cruises/7/enrollments/1/status", body: map[string]any{"status": "accepted"},
			want: http.StatusNoContent, wantStatus: "accepted",
		},
		{
			name:   "moving an accepted enrollment onto a full leg is refused",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, TripID: onLeg(6), Status: "accepted"},
			path:   "/cruises/7/enrollments/1/trip", body: map[string]any{"trip_id": 5},
			want: http.StatusConflict, wantStatus: "accepted", wantTrip: onLeg(6),
		},
		{
			name:   "moving a pending enrollment onto a full leg is allowed",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, Status: "pending"},
			path:   "/cruises/7/enrollments/1/trip", body: map[string]any{"trip_id": 5},
			want: http.StatusNoContent, wantStatus: "pending", wantTrip: onLeg(5),
		},
		{
			name:   "unassigning an accepted enrollment is allowed",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 7, UserID: 300, TripID: onLeg(5), Status: "accepted"},
			path:   "/cruises/7/enrollments/1/trip", body: map[string]any{"trip_id": nil},
			want: http.StatusNoContent, wantStatus: "accepted",
		},
		{
			name:   "enrollment of another cruise is not found",
			legacy: sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 8, UserID: 300, TripID: onLeg(6), Status: "pending"},
			path:   "/cruises/7/enrollments/1/status", body: map[string]any{"status": "accepted"},
			want: http.StatusNotFound, wantStatus: "pending", wantTrip: onLeg(6),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(limitedTrip(5, 1), limitedTrip(6, 2))
			s.rows = enrollmentRows("accepted")
			s.legacy = []sqlcdb.CruiseEnrollment{c.legacy}
			resp := cruiseEnrollmentAPI(t, cruiseEnrollmentMock(s)).PutCtx(userCtx(context.Background()), c.path, c.body)
			if resp.Code != c.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			if got := s.legacy[0]; got.Status != c.wantStatus || got.TripID != c.wantTrip {
				t.Fatalf("enrollment = %s on %+v, want %s on %+v", got.Status, got.TripID, c.wantStatus, c.wantTrip)
			}
		})
	}
}
