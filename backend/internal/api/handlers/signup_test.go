package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// signUpStore is an in-memory trips and trip_enrollments pair keyed like
// the real tables (UNIQUE(trip_id, user_id)).
type signUpStore struct {
	trips   map[int64]sqlcdb.Trip
	rows    []sqlcdb.TripEnrollment
	legacy  []sqlcdb.CruiseEnrollment
	nextID  int64
	inserts int
	// crewCalls records crew list changes as "add trip/user" or
	// "remove trip/user", in call order.
	crewCalls []string
}

func (s *signUpStore) find(tripID, userID int64) (int, bool) {
	for i := range s.rows {
		if s.rows[i].TripID == tripID && s.rows[i].UserID == userID {
			return i, true
		}
	}
	return 0, false
}

func newSignUpStore(trips ...sqlcdb.Trip) *signUpStore {
	s := &signUpStore{trips: map[int64]sqlcdb.Trip{}}
	for i := range trips {
		s.trips[trips[i].ID] = trips[i]
	}
	return s
}

func (s *signUpStore) mock() *mockQuerier {
	return &mockQuerier{
		ensureUserPersonFn: func(context.Context, int64) error { return nil },
		addAcceptedTripCrewAssignmentFn: func(_ context.Context, arg sqlcdb.AddAcceptedTripCrewAssignmentParams) error {
			s.crewCalls = append(s.crewCalls, fmt.Sprintf("add %d/%d", arg.TripID, arg.UserID))
			return nil
		},
		removeAcceptedTripCrewAssignmentFn: func(_ context.Context, arg sqlcdb.RemoveAcceptedTripCrewAssignmentParams) error {
			s.crewCalls = append(s.crewCalls, fmt.Sprintf("remove %d/%d", arg.TripID, arg.UserID))
			return nil
		},
		getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
			t, ok := s.trips[id]
			if !ok {
				return sqlcdb.Trip{}, sql.ErrNoRows
			}
			return t, nil
		},
		lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
			t, ok := s.trips[id]
			if !ok {
				return sqlcdb.LockTripForSignUpRow{}, sql.ErrNoRows
			}
			return sqlcdb.LockTripForSignUpRow{ID: t.ID, Status: t.Status, Open: t.EnrollToken.Valid, MaxCrew: t.MaxCrew, SignUpDeadline: t.SignUpDeadline}, nil
		},
		getUserTripEnrollmentFn: func(_ context.Context, arg sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
			if i, ok := s.find(arg.TripID, arg.UserID); ok {
				return s.rows[i], nil
			}
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		},
		createTripEnrollmentIfAbsentFn: func(_ context.Context, arg sqlcdb.CreateTripEnrollmentIfAbsentParams) (sqlcdb.TripEnrollment, error) {
			if _, ok := s.find(arg.TripID, arg.UserID); ok {
				return sqlcdb.TripEnrollment{}, sql.ErrNoRows
			}
			s.inserts++
			s.nextID++
			row := sqlcdb.TripEnrollment{ID: s.nextID, TripID: arg.TripID, UserID: arg.UserID, Note: arg.Note, Status: arg.Status, PaymentStatus: "unpaid"}
			s.rows = append(s.rows, row)
			return row, nil
		},
		updateUserTripEnrollNoteFn: func(_ context.Context, arg sqlcdb.UpdateUserTripEnrollmentNoteParams) (sqlcdb.TripEnrollment, error) {
			i, ok := s.find(arg.TripID, arg.UserID)
			if !ok {
				return sqlcdb.TripEnrollment{}, sql.ErrNoRows
			}
			s.rows[i].Note = arg.Note
			return s.rows[i], nil
		},
		deletePendingUserTripEnrollFn: func(_ context.Context, arg sqlcdb.WithdrawUserTripEnrollmentParams) (int64, error) {
			i, ok := s.find(arg.TripID, arg.UserID)
			if !ok || (s.rows[i].Status != "pending" && s.rows[i].Status != "waitlisted") || s.rows[i].PaymentStatus != "unpaid" {
				return 0, sql.ErrNoRows
			}
			id := s.rows[i].ID
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return id, nil
		},
		getUserCruiseEnrollForTripFn: func(_ context.Context, arg sqlcdb.GetUserCruiseEnrollmentForTripParams) (sqlcdb.CruiseEnrollment, error) {
			for i := range s.legacy {
				if s.legacy[i].TripID == arg.TripID && s.legacy[i].UserID == arg.UserID {
					return s.legacy[i], nil
				}
			}
			return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
		},
		countAcceptedForTripFn: func(_ context.Context, tripID int64) (int64, error) {
			return s.count(tripID, 0, "accepted"), nil
		},
		countOtherAcceptedForTripFn: func(_ context.Context, arg sqlcdb.CountOtherAcceptedForTripParams) (int64, error) {
			return s.count(arg.TripID, arg.UserID, "accepted"), nil
		},
		countTripPlaceUseFn: func(_ context.Context, tripID int64) (sqlcdb.CountTripPlaceUseRow, error) {
			var waiting int64
			for i := range s.rows {
				if s.rows[i].TripID == tripID && s.rows[i].Status == "waitlisted" {
					waiting++
				}
			}
			return sqlcdb.CountTripPlaceUseRow{Taken: s.count(tripID, 0, "accepted", "pending"), Waitlisted: waiting}, nil
		},
		getWaitlistPositionFn: func(_ context.Context, id int64) (int64, error) {
			// Rows are kept in sign-up order, like ORDER BY created_at, id.
			var pos int64
			for i := range s.rows {
				if s.rows[i].Status == "waitlisted" {
					pos++
				}
				if s.rows[i].ID == id {
					return pos, nil
				}
			}
			return 0, sql.ErrNoRows
		},
		lockTripEnrollmentOfTripFn: func(_ context.Context, arg sqlcdb.LockTripEnrollmentOfTripParams) (sqlcdb.TripEnrollment, error) {
			for i := range s.rows {
				if s.rows[i].ID == arg.ID && s.rows[i].TripID == arg.TripID {
					return s.rows[i], nil
				}
			}
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		},
		updateTripEnrollmentStatusFn: func(_ context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error {
			for i := range s.rows {
				if s.rows[i].ID == arg.ID {
					s.rows[i].Status = arg.Status
				}
			}
			return nil
		},
	}
}

// count returns the distinct users of a trip's sign-ups in one of statuses,
// skipping exceptUser (0 skips no one). Users are unique per trip, as in
// UNIQUE(trip_id, user_id); cruise enrollments assigned to the trip count too.
func (s *signUpStore) count(tripID, exceptUser int64, statuses ...string) int64 {
	users := map[int64]bool{}
	for i := range s.rows {
		r := &s.rows[i]
		if r.TripID == tripID && r.UserID != exceptUser && slices.Contains(statuses, r.Status) {
			users[r.UserID] = true
		}
	}
	for i := range s.legacy {
		r := &s.legacy[i]
		if r.TripID.Valid && r.TripID.Int64 == tripID && r.UserID != exceptUser && slices.Contains(statuses, r.Status) {
			users[r.UserID] = true
		}
	}
	return int64(len(users))
}

func openTrip(id int64) sqlcdb.Trip {
	return sqlcdb.Trip{ID: id, CruiseID: 7, Name: "Leg", Status: sqlcdb.TripStatusPlanned, EnrollToken: types.NullString{String: "tok", Valid: true}}
}

// guestCtx is a self-registered user who has not finished onboarding: the
// sign-up operations must not depend on profile completeness.
func guestCtx(userID int64) context.Context {
	return context.WithValue(context.Background(), middleware.UserCtxKey, &auth.Claims{
		UserID: userID, Email: "new@example.com", Role: "member",
	})
}

func decodeSignUp(t *testing.T, body []byte) dto.EnrollLegResult {
	t.Helper()
	var out dto.EnrollLegResult
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, body)
	}
	return out
}

func TestSignUpForTrip_CreatesPendingEnrollment(t *testing.T) {
	s := newSignUpStore(openTrip(5))
	resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
	}
	got := decodeSignUp(t, resp.Body.Bytes())
	if got.Result != "created" || got.TripID != 5 || got.Enrollment.Status != "pending" || got.Enrollment.UserID != 42 {
		t.Fatalf("result = %+v, want created pending enrollment of user 42 on trip 5", got)
	}
	if len(s.rows) != 1 || s.rows[0].Note.Valid {
		t.Fatalf("rows = %+v, want one row without a note", s.rows)
	}
}

func TestSignUpForTrip_TwiceReturnsExisting(t *testing.T) {
	s := newSignUpStore(openTrip(5))
	api := enrollTestAPI(t, s.mock())
	first := api.PostCtx(guestCtx(42), "/trips/5/sign-up")
	if first.Code != http.StatusCreated {
		t.Fatalf("first: got %d, want 201; body=%s", first.Code, first.Body)
	}
	second := api.PostCtx(guestCtx(42), "/trips/5/sign-up")
	if second.Code != http.StatusOK {
		t.Fatalf("second: got %d, want 200; body=%s", second.Code, second.Body)
	}
	a, b := decodeSignUp(t, first.Body.Bytes()), decodeSignUp(t, second.Body.Bytes())
	if b.Result != "already_enrolled" || b.Enrollment.ID != a.Enrollment.ID {
		t.Fatalf("second = %+v, want already_enrolled with enrollment %d", b, a.Enrollment.ID)
	}
	if len(s.rows) != 1 || s.inserts != 1 {
		t.Fatalf("rows = %d, inserts = %d; want 1 and 1", len(s.rows), s.inserts)
	}
}

func TestSignUpForTrip_LostInsertRaceReturnsExisting(t *testing.T) {
	s := newSignUpStore(openTrip(5))
	m := s.mock()
	calls := 0
	m.getUserTripEnrollmentFn = func(context.Context, sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
		calls++
		if calls == 1 {
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		}
		return sqlcdb.TripEnrollment{ID: 99, TripID: 5, UserID: 42, Status: "pending"}, nil
	}
	m.createTripEnrollmentIfAbsentFn = func(context.Context, sqlcdb.CreateTripEnrollmentIfAbsentParams) (sqlcdb.TripEnrollment, error) {
		return sqlcdb.TripEnrollment{}, sql.ErrNoRows
	}
	resp := enrollTestAPI(t, m).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	if got := decodeSignUp(t, resp.Body.Bytes()); got.Result != "already_enrolled" || got.Enrollment.ID != 99 {
		t.Fatalf("result = %+v, want already_enrolled 99", got)
	}
}

func TestSignUpForTrip_Rejected(t *testing.T) {
	cancelled := openTrip(5)
	cancelled.Status = sqlcdb.TripStatusCancelled
	closed := openTrip(5)
	closed.EnrollToken = types.NullString{}

	cases := []struct {
		name string
		trip *sqlcdb.Trip
		path string
		want int
	}{
		{"cancelled leg", &cancelled, "/trips/5/sign-up", http.StatusUnprocessableEntity},
		{"sign-ups switched off", &closed, "/trips/5/sign-up", http.StatusUnprocessableEntity},
		{"unknown or completed trip", nil, "/trips/404/sign-up", http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore()
			if c.trip != nil {
				s = newSignUpStore(*c.trip)
			}
			resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), c.path)
			if resp.Code != c.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			if s.inserts != 0 || len(s.rows) != 0 {
				t.Fatalf("inserts = %d, rows = %d; want nothing written", s.inserts, len(s.rows))
			}
		})
	}
}

func TestSignUpForTrip_ExistingSignUpOnClosedLegIsReturned(t *testing.T) {
	trip := openTrip(5)
	trip.EnrollToken = types.NullString{}
	s := newSignUpStore(trip)
	s.rows = []sqlcdb.TripEnrollment{{ID: 3, TripID: 5, UserID: 42, Status: "accepted"}}
	resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	if got := decodeSignUp(t, resp.Body.Bytes()); got.Enrollment.ID != 3 || got.Enrollment.Status != "accepted" {
		t.Fatalf("result = %+v, want existing accepted enrollment 3", got)
	}
}

func TestSignUpForTrip_DBError(t *testing.T) {
	m := newSignUpStore(openTrip(5)).mock()
	m.lockTripForSignUpFn = func(context.Context, int64) (sqlcdb.LockTripForSignUpRow, error) {
		return sqlcdb.LockTripForSignUpRow{}, errors.New("fail")
	}
	resp := enrollTestAPI(t, m).PostCtx(guestCtx(42), "/trips/5/sign-up")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500; body=%s", resp.Code, resp.Body)
	}
}

func TestSignUpOperations_RequireSignedInUser(t *testing.T) {
	api := enrollTestAPI(t, &mockQuerier{})
	cases := []struct {
		name string
		do   func() *httptest.ResponseRecorder
	}{
		{"sign up", func() *httptest.ResponseRecorder { return api.Post("/trips/5/sign-up") }},
		{"get", func() *httptest.ResponseRecorder { return api.Get("/trips/5/sign-up") }},
		{"note", func() *httptest.ResponseRecorder {
			return api.Put("/trips/5/sign-up/note", map[string]any{"note": "x"})
		}},
		{"withdraw", func() *httptest.ResponseRecorder { return api.Delete("/trips/5/sign-up") }},
		{"cruise legs", func() *httptest.ResponseRecorder { return api.Get("/cruises/7/leg-sign-ups") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if resp := c.do(); resp.Code != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401; body=%s", resp.Code, resp.Body)
			}
		})
	}
}

func TestSignUpRoutes_RejectRequestsWithoutToken(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })
	h := middleware.Auth(nil, &mockQuerier{})(next)
	for _, r := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/trips/5/sign-up", http.NoBody),
		httptest.NewRequest(http.MethodGet, "/api/cruises/7/leg-sign-ups", http.NoBody),
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: got %d, want 401", r.Method, r.URL.Path, rec.Code)
		}
	}
	if reached {
		t.Fatal("handler reached without a token")
	}
}

func TestGetTripSignUp(t *testing.T) {
	t.Run("not signed up, open leg", func(t *testing.T) {
		s := newSignUpStore(openTrip(5))
		s.rows = []sqlcdb.TripEnrollment{{ID: 1, TripID: 5, UserID: 8, Status: "accepted"}}
		resp := enrollTestAPI(t, s.mock()).GetCtx(guestCtx(42), "/trips/5/sign-up")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		var got dto.LegSignUp
		if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !got.Open || got.AcceptedCount != 1 || got.Enrollment != nil {
			t.Fatalf("got %+v, want open, 1 accepted, no enrollment", got)
		}
	})
	t.Run("signed up on a closed leg", func(t *testing.T) {
		trip := openTrip(5)
		trip.EnrollToken = types.NullString{}
		s := newSignUpStore(trip)
		s.rows = []sqlcdb.TripEnrollment{{ID: 2, TripID: 5, UserID: 42, Status: "pending"}}
		resp := enrollTestAPI(t, s.mock()).GetCtx(guestCtx(42), "/trips/5/sign-up")
		var got dto.LegSignUp
		if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Open || got.Enrollment == nil || got.Enrollment.ID != 2 {
			t.Fatalf("got %+v, want closed with enrollment 2", got)
		}
	})
	t.Run("unknown trip", func(t *testing.T) {
		resp := enrollTestAPI(t, newSignUpStore().mock()).GetCtx(guestCtx(42), "/trips/5/sign-up")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})
}

func TestUpdateTripSignUpNote(t *testing.T) {
	cases := []struct {
		name     string
		signedUp bool
		body     map[string]any
		want     int
		wantNote types.NullString
	}{
		{"sets the note", true, map[string]any{"note": "  vegetarian  "}, http.StatusOK, types.NullString{String: "vegetarian", Valid: true}},
		{"blank clears it", true, map[string]any{"note": "  "}, http.StatusOK, types.NullString{}},
		{"omitted clears it", true, map[string]any{}, http.StatusOK, types.NullString{}},
		{"not signed up", false, map[string]any{"note": "hi"}, http.StatusNotFound, types.NullString{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(openTrip(5))
			if c.signedUp {
				s.rows = []sqlcdb.TripEnrollment{{ID: 1, TripID: 5, UserID: 42, Status: "pending", Note: types.NullString{String: "old", Valid: true}}}
			}
			resp := enrollTestAPI(t, s.mock()).PutCtx(guestCtx(42), "/trips/5/sign-up/note", c.body)
			if resp.Code != c.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			if c.signedUp && s.rows[0].Note != c.wantNote {
				t.Fatalf("note = %+v, want %+v", s.rows[0].Note, c.wantNote)
			}
		})
	}
}

func TestWithdrawTripSignUp(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		payment  string
		want     int
		wantRows int
	}{
		{"pending is withdrawn", "pending", "unpaid", http.StatusNoContent, 0},
		{"accepted stays", "accepted", "unpaid", http.StatusConflict, 1},
		{"waitlisted is withdrawn", "waitlisted", "unpaid", http.StatusNoContent, 0},
		{"rejected stays", "rejected", "unpaid", http.StatusConflict, 1},
		{"pending with deposit stays", "pending", "deposit", http.StatusConflict, 1},
		{"pending and paid stays", "pending", "paid", http.StatusConflict, 1},
		{"waitlisted with deposit stays", "waitlisted", "deposit", http.StatusConflict, 1},
		{"not signed up", "", "", http.StatusNotFound, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSignUpStore(openTrip(5))
			if c.status != "" {
				s.rows = []sqlcdb.TripEnrollment{{ID: 1, TripID: 5, UserID: 42, Status: c.status, PaymentStatus: c.payment}}
			}
			resp := enrollTestAPI(t, s.mock()).DeleteCtx(guestCtx(42), "/trips/5/sign-up")
			if resp.Code != c.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			if len(s.rows) != c.wantRows {
				t.Fatalf("rows = %d, want %d", len(s.rows), c.wantRows)
			}
		})
	}
	t.Run("other users' sign-ups are untouched", func(t *testing.T) {
		s := newSignUpStore(openTrip(5))
		s.rows = []sqlcdb.TripEnrollment{{ID: 1, TripID: 5, UserID: 8, Status: "pending", PaymentStatus: "unpaid"}}
		resp := enrollTestAPI(t, s.mock()).DeleteCtx(guestCtx(42), "/trips/5/sign-up")
		if resp.Code != http.StatusNotFound || len(s.rows) != 1 {
			t.Fatalf("got %d with %d rows, want 404 and the other row kept", resp.Code, len(s.rows))
		}
	})
}

func TestListCruiseLegSignUps(t *testing.T) {
	closed := openTrip(12)
	closed.EnrollToken = types.NullString{}
	cancelled := openTrip(13)
	cancelled.Status = sqlcdb.TripStatusCancelled
	trips := []sqlcdb.Trip{openTrip(11), closed, cancelled}
	m := &mockQuerier{
		getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
			if id != 7 {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			}
			return sqlcdb.Cruise{ID: 7}, nil
		},
		listCruiseTripsFn: func(context.Context, int64) ([]sqlcdb.Trip, error) { return trips, nil },
		countAcceptedByCruiseTripFn: func(context.Context, int64) ([]sqlcdb.CountAcceptedByCruiseTripRow, error) {
			return []sqlcdb.CountAcceptedByCruiseTripRow{{TripID: 11, Accepted: 3}, {TripID: 12, Accepted: 0}, {TripID: 13, Accepted: 0}}, nil
		},
		getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
			return sqlcdb.CruiseEnrollment{ID: 5, CruiseID: 7, UserID: 42, TripID: types.NullInt64{Int64: 11, Valid: true}, Status: "accepted"}, nil
		},
		listUserCruiseTripEnrollFn: func(_ context.Context, arg sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
			if arg.UserID != 42 {
				t.Fatalf("user = %d, want 42", arg.UserID)
			}
			return []sqlcdb.TripEnrollment{{ID: 9, TripID: 12, UserID: 42, Status: "waitlisted"}}, nil
		},
		getWaitlistPositionFn: func(_ context.Context, id int64) (int64, error) {
			if id != 9 {
				t.Fatalf("waitlist position asked for enrollment %d, want 9", id)
			}
			return 2, nil
		},
	}
	api := enrollTestAPI(t, m)
	resp := api.GetCtx(guestCtx(42), "/cruises/7/leg-sign-ups")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got []dto.LegSignUp
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d legs, want 3", len(got))
	}
	if e := got[0].Enrollment; !got[0].Open || got[0].AcceptedCount != 3 || e == nil || e.CruiseID == nil || e.Status != "accepted" {
		t.Errorf("leg 11 = %+v, want open with 3 accepted and the caller's cruise enrollment assigned to it", got[0])
	}
	if got[1].Open || got[1].Enrollment == nil || got[1].Enrollment.Status != "waitlisted" ||
		got[1].WaitlistPosition == nil || *got[1].WaitlistPosition != 2 {
		t.Errorf("leg 12 = %+v, want closed with the caller's waitlisted sign-up at position 2", got[1])
	}
	if got[0].WaitlistPosition != nil {
		t.Errorf("leg 11 position = %d, want none for an accepted sign-up", *got[0].WaitlistPosition)
	}
	if got[2].Open {
		t.Errorf("leg 13 = %+v, want cancelled leg closed", got[2])
	}

	if resp := api.GetCtx(guestCtx(42), "/cruises/8/leg-sign-ups"); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown cruise: got %d, want 404", resp.Code)
	}
}

func TestListCruiseTripEnrollments(t *testing.T) {
	m := &mockQuerier{
		getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
			if id != 7 {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			}
			return sqlcdb.Cruise{ID: 7}, nil
		},
		listCruiseTripEnrollmentsFn: func(_ context.Context, cruiseID int64) ([]sqlcdb.ListCruiseTripEnrollmentsRow, error) {
			if cruiseID != 7 {
				t.Fatalf("cruise = %d, want 7", cruiseID)
			}
			return []sqlcdb.ListCruiseTripEnrollmentsRow{{ID: 1, TripID: 11, UserID: 42, Status: "pending", UserName: "Ola"}}, nil
		},
	}
	api := enrollTestAPI(t, m)
	resp := api.GetCtx(userCtx(context.Background()), "/cruises/7/trip-enrollments")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got []dto.TripEnrollmentDetail
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].TripID == nil || *got[0].TripID != 11 || got[0].UserName != "Ola" {
		t.Fatalf("got %+v, want Ola's sign-up on leg 11", got)
	}
	if resp := api.GetCtx(guestCtx(42), "/cruises/7/trip-enrollments"); resp.Code != http.StatusForbidden {
		t.Fatalf("member: got %d, want 403", resp.Code)
	}
	if resp := api.GetCtx(userCtx(context.Background()), "/cruises/8/trip-enrollments"); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown cruise: got %d, want 404", resp.Code)
	}
}

// legacyStore holds one cruise enrollment of user 42 assigned to leg 5, made
// before sign-ups moved to legs.
func legacyStore(status string) *signUpStore {
	s := newSignUpStore(openTrip(5))
	s.legacy = []sqlcdb.CruiseEnrollment{{
		ID: 3, CruiseID: 7, UserID: 42, TripID: types.NullInt64{Int64: 5, Valid: true}, Status: status,
	}}
	return s
}

func TestSignUp_CruiseEnrollmentAssignedToLegCountsAsSignUp(t *testing.T) {
	t.Run("get shows it", func(t *testing.T) {
		resp := enrollTestAPI(t, legacyStore("accepted").mock()).GetCtx(guestCtx(42), "/trips/5/sign-up")
		var got dto.LegSignUp
		if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v; body=%s", err, resp.Body)
		}
		if e := got.Enrollment; e == nil || e.ID != 3 || e.CruiseID == nil || e.Status != "accepted" {
			t.Fatalf("got %+v, want the cruise enrollment 3", got)
		}
	})
	t.Run("sign-up returns it and writes nothing", func(t *testing.T) {
		s := legacyStore("accepted")
		resp := enrollTestAPI(t, s.mock()).PostCtx(guestCtx(42), "/trips/5/sign-up")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if got := decodeSignUp(t, resp.Body.Bytes()); got.Result != "already_enrolled" || got.Enrollment.ID != 3 {
			t.Fatalf("result = %+v, want already_enrolled cruise enrollment 3", got)
		}
		if s.inserts != 0 || len(s.rows) != 0 {
			t.Fatalf("inserts = %d, rows = %d; want nothing written", s.inserts, len(s.rows))
		}
	})
	t.Run("withdraw and note are refused", func(t *testing.T) {
		api := enrollTestAPI(t, legacyStore("pending").mock())
		if resp := api.DeleteCtx(guestCtx(42), "/trips/5/sign-up"); resp.Code != http.StatusConflict {
			t.Fatalf("withdraw: got %d, want 409; body=%s", resp.Code, resp.Body)
		}
		if resp := api.PutCtx(guestCtx(42), "/trips/5/sign-up/note", map[string]any{"note": "x"}); resp.Code != http.StatusConflict {
			t.Fatalf("note: got %d, want 409; body=%s", resp.Code, resp.Body)
		}
	})
}
