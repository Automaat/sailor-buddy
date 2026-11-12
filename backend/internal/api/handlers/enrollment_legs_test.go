package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// legStore is an in-memory trip_enrollments table keyed like the real one
// (UNIQUE(trip_id, user_id)), wired into a mockQuerier for the cruise with
// ID 7 and the given trips.
type legStore struct {
	rows        []sqlcdb.TripEnrollment
	nextID      int64
	inserts     int
	insertOrder []int64
}

func (s *legStore) find(tripID, userID int64) (int, bool) {
	for i := range s.rows {
		if s.rows[i].TripID == tripID && s.rows[i].UserID == userID {
			return i, true
		}
	}
	return 0, false
}

func newLegMock(s *legStore, trips []sqlcdb.Trip) *mockQuerier {
	return &mockQuerier{
		getTripByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetTripByEnrollTokenRow, error) {
			return sqlcdb.GetTripByEnrollTokenRow{}, sql.ErrNoRows
		},
		getCruiseByEnrollFn: func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
			return sqlcdb.GetCruiseByEnrollTokenRow{ID: 7, Name: "NNE 2025"}, nil
		},
		countCruiseEnrollmentsFn: func(context.Context, int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
			return sqlcdb.CountCruiseEnrollmentsRow{}, nil
		},
		listCruiseTripsFn: func(_ context.Context, cid types.NullInt64) ([]sqlcdb.Trip, error) {
			if cid.Int64 != 7 {
				return nil, nil
			}
			return trips, nil
		},
		getUserCruiseEnrollmentFn: func(context.Context, sqlcdb.GetUserCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
			return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
		},
		lockCruiseTripStatusesFn: func(_ context.Context, cid types.NullInt64) ([]sqlcdb.LockCruiseTripStatusesRow, error) {
			if cid.Int64 != 7 {
				return nil, nil
			}
			out := make([]sqlcdb.LockCruiseTripStatusesRow, len(trips))
			for i := range trips {
				out[i] = sqlcdb.LockCruiseTripStatusesRow{ID: trips[i].ID, Status: trips[i].Status}
			}
			return out, nil
		},
		createTripEnrollmentIfAbsentFn: func(_ context.Context, arg sqlcdb.CreateTripEnrollmentIfAbsentParams) (sqlcdb.TripEnrollment, error) {
			s.inserts++
			s.insertOrder = append(s.insertOrder, arg.TripID)
			if _, ok := s.find(arg.TripID, arg.UserID); ok {
				return sqlcdb.TripEnrollment{}, sql.ErrNoRows
			}
			s.nextID++
			row := sqlcdb.TripEnrollment{ID: s.nextID, TripID: arg.TripID, UserID: arg.UserID, Note: arg.Note, Status: "pending"}
			s.rows = append(s.rows, row)
			return row, nil
		},
		getUserTripEnrollmentFn: func(_ context.Context, arg sqlcdb.GetUserTripEnrollmentParams) (sqlcdb.TripEnrollment, error) {
			if i, ok := s.find(arg.TripID, arg.UserID); ok {
				return s.rows[i], nil
			}
			return sqlcdb.TripEnrollment{}, sql.ErrNoRows
		},
		listUserCruiseTripEnrollFn: func(_ context.Context, arg sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
			var out []sqlcdb.TripEnrollment
			for ti := range trips {
				if i, ok := s.find(trips[ti].ID, arg.UserID); ok {
					out = append(out, s.rows[i])
				}
			}
			return out, nil
		},
		updateTripEnrollmentStatusFn: func(_ context.Context, arg sqlcdb.UpdateTripEnrollmentStatusParams) error {
			for i := range s.rows {
				if s.rows[i].ID == arg.ID {
					s.rows[i].Status = arg.Status
				}
			}
			return nil
		},
		listTripEnrollmentsFn: func(_ context.Context, tripID int64) ([]sqlcdb.ListTripEnrollmentsRow, error) {
			var out []sqlcdb.ListTripEnrollmentsRow
			for i := range s.rows {
				if r := &s.rows[i]; r.TripID == tripID {
					out = append(out, sqlcdb.ListTripEnrollmentsRow{ID: r.ID, TripID: r.TripID, UserID: r.UserID, Status: r.Status})
				}
			}
			return out, nil
		},
	}
}

func legTrips() []sqlcdb.Trip {
	cruise := types.NullInt64{Int64: 7, Valid: true}
	return []sqlcdb.Trip{
		{ID: 11, Name: "Leg A", CruiseID: cruise, Status: sqlcdb.TripStatusPlanned},
		{ID: 12, Name: "Leg B", CruiseID: cruise, Status: sqlcdb.TripStatusPlanned},
		{ID: 13, Name: "Leg C", CruiseID: cruise, Status: sqlcdb.TripStatusCancelled},
	}
}

func postLegs(t *testing.T, m *mockQuerier, ctx context.Context, reqBody any) (code int, out dto.EnrollLegsResponse, body string) {
	t.Helper()
	resp := enrollTestAPI(t, m).PostCtx(ctx, "/enroll/tok/legs", reqBody)
	if resp.Code == http.StatusCreated {
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response: %v; body=%s", err, resp.Body)
		}
	}
	return resp.Code, out, resp.Body.String()
}

func resultSummary(r dto.EnrollLegsResponse) []string {
	out := make([]string, len(r.Results))
	for i := range r.Results {
		out[i] = fmt.Sprintf("%d:%s", r.Results[i].TripID, r.Results[i].Result)
	}
	return out
}

func TestEnrollLegs_CreatesOneEnrollmentPerLeg(t *testing.T) {
	s := &legStore{}
	code, out, body := postLegs(t, newLegMock(s, legTrips()), userCtxRole(context.Background(), "member"),
		map[string]any{"trip_ids": []int64{11, 12}, "note": "both please"})
	if code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", code, body)
	}
	if got, want := fmt.Sprint(resultSummary(out)), "[11:created 12:created]"; got != want {
		t.Fatalf("results = %s, want %s", got, want)
	}
	if len(s.rows) != 2 {
		t.Fatalf("stored %d rows, want 2", len(s.rows))
	}
	for _, r := range s.rows {
		if r.UserID != 1 || r.Status != "pending" || r.Note.String != "both please" {
			t.Errorf("unexpected row %+v", r)
		}
	}
	for _, r := range out.Results {
		if r.Enrollment.TripID == nil || *r.Enrollment.TripID != r.TripID || r.Enrollment.ID == 0 {
			t.Errorf("result enrollment does not match leg: %+v", r)
		}
	}
}

func TestEnrollLegs_ReportsAlreadyEnrolledAndCreatesOthers(t *testing.T) {
	s := &legStore{nextID: 40, rows: []sqlcdb.TripEnrollment{{ID: 40, TripID: 11, UserID: 1, Status: "accepted"}}}
	code, out, body := postLegs(t, newLegMock(s, legTrips()), userCtxRole(context.Background(), "member"),
		map[string]any{"trip_ids": []int64{11, 12}})
	if code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", code, body)
	}
	if got, want := fmt.Sprint(resultSummary(out)), "[11:already_enrolled 12:created]"; got != want {
		t.Fatalf("results = %s, want %s", got, want)
	}
	if out.Results[0].Enrollment.ID != 40 || out.Results[0].Enrollment.Status != "accepted" {
		t.Errorf("already_enrolled result should carry the existing enrollment, got %+v", out.Results[0].Enrollment)
	}
	if len(s.rows) != 2 {
		t.Fatalf("stored %d rows, want 2", len(s.rows))
	}
}

func TestEnrollLegs_InsertsInTripOrderButReportsInRequestOrder(t *testing.T) {
	s := &legStore{}
	code, out, body := postLegs(t, newLegMock(s, legTrips()), userCtxRole(context.Background(), "member"),
		map[string]any{"trip_ids": []int64{12, 11}})
	if code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body=%s", code, body)
	}
	if got := fmt.Sprint(s.insertOrder); got != "[11 12]" {
		t.Errorf("insert order = %s, want [11 12]", got)
	}
	if got := fmt.Sprint(resultSummary(out)); got != "[12:created 11:created]" {
		t.Errorf("results = %s, want request order [12:created 11:created]", got)
	}
}

func TestEnrollLegs_RejectsInvalidPicksWithoutWriting(t *testing.T) {
	cases := []struct {
		name    string
		tripIDs []int64
	}{
		{"trip of another cruise", []int64{11, 99}},
		{"cancelled leg", []int64{12, 13}},
		{"empty pick", []int64{}},
		{"null pick", nil},
		{"duplicate pick", []int64{11, 11}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &legStore{}
			code, _, body := postLegs(t, newLegMock(s, legTrips()), userCtxRole(context.Background(), "member"),
				map[string]any{"trip_ids": tc.tripIDs})
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422; body=%s", code, body)
			}
			if s.inserts != 0 || len(s.rows) != 0 {
				t.Fatalf("expected no writes, got %d inserts and %d rows", s.inserts, len(s.rows))
			}
		})
	}
}

func TestEnrollLegs_Errors(t *testing.T) {
	t.Run("unknown token", func(t *testing.T) {
		m := newLegMock(&legStore{}, legTrips())
		m.getCruiseByEnrollFn = func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
			return sqlcdb.GetCruiseByEnrollTokenRow{}, sql.ErrNoRows
		}
		code, _, _ := postLegs(t, m, userCtx(context.Background()), map[string]any{"trip_ids": []int64{11}})
		if code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", code)
		}
	})
	t.Run("cruise lookup error", func(t *testing.T) {
		m := newLegMock(&legStore{}, legTrips())
		m.getCruiseByEnrollFn = func(context.Context, types.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
			return sqlcdb.GetCruiseByEnrollTokenRow{}, errors.New("fail")
		}
		code, _, _ := postLegs(t, m, userCtx(context.Background()), map[string]any{"trip_ids": []int64{11}})
		if code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", code)
		}
	})
	t.Run("trip lock error", func(t *testing.T) {
		m := newLegMock(&legStore{}, legTrips())
		m.lockCruiseTripStatusesFn = func(context.Context, types.NullInt64) ([]sqlcdb.LockCruiseTripStatusesRow, error) {
			return nil, errors.New("fail")
		}
		code, _, _ := postLegs(t, m, userCtx(context.Background()), map[string]any{"trip_ids": []int64{11}})
		if code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", code)
		}
	})
	t.Run("insert error", func(t *testing.T) {
		m := newLegMock(&legStore{}, legTrips())
		m.createTripEnrollmentIfAbsentFn = func(context.Context, sqlcdb.CreateTripEnrollmentIfAbsentParams) (sqlcdb.TripEnrollment, error) {
			return sqlcdb.TripEnrollment{}, errors.New("fail")
		}
		code, _, _ := postLegs(t, m, userCtx(context.Background()), map[string]any{"trip_ids": []int64{11}})
		if code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", code)
		}
	})
	t.Run("unauthenticated", func(t *testing.T) {
		code, _, _ := postLegs(t, newLegMock(&legStore{}, legTrips()), context.Background(), map[string]any{"trip_ids": []int64{11}})
		if code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", code)
		}
	})
}

func TestEnrollment_GetByToken_CruiseLegEnrollments(t *testing.T) {
	s := &legStore{rows: []sqlcdb.TripEnrollment{
		{ID: 1, TripID: 12, UserID: 1, Status: "accepted"},
		{ID: 2, TripID: 11, UserID: 1, Status: "pending"},
		{ID: 3, TripID: 11, UserID: 2, Status: "pending"},
	}}
	resp := enrollTestAPI(t, newLegMock(s, legTrips())).GetCtx(userCtxRole(context.Background(), "member"), "/enroll/tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var info dto.EnrollInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if !info.Enrolled {
		t.Error("caller with only leg enrollments should be reported as enrolled")
	}
	if info.Enrollment != nil {
		t.Errorf("no cruise-level enrollment expected, got %+v", info.Enrollment)
	}
	var got []string
	for _, e := range info.LegEnrollments {
		got = append(got, fmt.Sprintf("%d:%s", *e.TripID, e.Status))
	}
	if fmt.Sprint(got) != "[11:pending 12:accepted]" {
		t.Errorf("leg enrollments = %v, want [11:pending 12:accepted]", got)
	}
}

func TestEnrollment_GetByToken_CruiseNotEnrolled(t *testing.T) {
	resp := enrollTestAPI(t, newLegMock(&legStore{}, legTrips())).GetCtx(userCtxRole(context.Background(), "member"), "/enroll/tok")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.Code)
	}
	var info dto.EnrollInfo
	if err := json.Unmarshal(resp.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Enrolled || len(info.LegEnrollments) != 0 {
		t.Errorf("expected not enrolled, got %+v", info)
	}
}

func TestEnrollment_GetByToken_LegListError(t *testing.T) {
	m := newLegMock(&legStore{}, legTrips())
	m.listUserCruiseTripEnrollFn = func(context.Context, sqlcdb.ListUserCruiseTripEnrollmentsParams) ([]sqlcdb.TripEnrollment, error) {
		return nil, errors.New("fail")
	}
	resp := enrollTestAPI(t, m).GetCtx(userCtx(context.Background()), "/enroll/tok")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestEnrollLegs_AdminReviewsEachLegSeparately(t *testing.T) {
	s := &legStore{}
	m := newLegMock(s, legTrips())
	code, out, body := postLegs(t, m, userCtxRole(context.Background(), "member"), map[string]any{"trip_ids": []int64{11, 12}})
	if code != http.StatusCreated {
		t.Fatalf("enroll: got %d; body=%s", code, body)
	}
	api := enrollTestAPI(t, m)
	admin := userCtx(context.Background())
	legA, legB := out.Results[0].Enrollment.ID, out.Results[1].Enrollment.ID
	for _, step := range []struct {
		trip, id int64
		status   string
	}{{11, legA, "accepted"}, {12, legB, "rejected"}} {
		resp := api.PutCtx(admin, fmt.Sprintf("/trips/%d/enrollments/%d/status", step.trip, step.id), map[string]string{"status": step.status})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("update %d: got %d; body=%s", step.id, resp.Code, resp.Body)
		}
	}
	for trip, want := range map[int64]string{11: "accepted", 12: "rejected"} {
		resp := api.GetCtx(admin, fmt.Sprintf("/trips/%d/enrollments", trip))
		if resp.Code != http.StatusOK {
			t.Fatalf("list trip %d: got %d", trip, resp.Code)
		}
		var rows []dto.TripEnrollmentDetail
		if err := json.Unmarshal(resp.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Status != want {
			t.Errorf("trip %d enrollments = %+v, want one %s", trip, rows, want)
		}
	}
}
