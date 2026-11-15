package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// adminSignUpMock serves one leg (trip 5) for an admin sign-up of person 7,
// who has no account. taken and waiting are the leg's place use.
type adminSignUpMock struct {
	leg      sqlcdb.LockTripForSignUpRow
	taken    int64
	waiting  int64
	existing bool
	insertFK bool
	inserted []sqlcdb.CreateTripEnrollmentIfAbsentParams
}

func (a *adminSignUpMock) mock() *mockQuerier {
	return &mockQuerier{
		lockTripForSignUpFn: func(_ context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
			if id != a.leg.ID {
				return sqlcdb.LockTripForSignUpRow{}, sql.ErrNoRows
			}
			return a.leg, nil
		},
		lockCrewMemberForShareFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
			if id != 7 {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			}
			return sqlcdb.CrewMember{ID: 7, FullName: "Ewa bez konta"}, nil
		},
		countTripPlaceUseFn: func(context.Context, int64) (sqlcdb.CountTripPlaceUseRow, error) {
			return sqlcdb.CountTripPlaceUseRow{Taken: a.taken, Waitlisted: a.waiting}, nil
		},
		createTripEnrollmentIfAbsentFn: func(_ context.Context, arg sqlcdb.CreateTripEnrollmentIfAbsentParams) (sqlcdb.TripEnrollment, error) {
			if a.insertFK {
				return sqlcdb.TripEnrollment{}, &pgconn.PgError{Code: "23503", ConstraintName: signUpPersonFKey}
			}
			if a.existing {
				return sqlcdb.TripEnrollment{}, sql.ErrNoRows
			}
			a.inserted = append(a.inserted, arg)
			return sqlcdb.TripEnrollment{ID: 31, TripID: arg.TripID, CrewMemberID: arg.CrewMemberID, Note: arg.Note, Status: arg.Status}, nil
		},
		getTripEnrollmentDetailFn: func(_ context.Context, arg sqlcdb.GetTripEnrollmentDetailParams) (sqlcdb.GetTripEnrollmentDetailRow, error) {
			last := a.inserted[len(a.inserted)-1]
			return sqlcdb.GetTripEnrollmentDetailRow{
				ID: arg.ID, TripID: arg.TripID, CrewMemberID: last.CrewMemberID, Note: last.Note,
				Status: last.Status, PaymentStatus: "unpaid", PersonName: "Ewa bez konta",
				PersonEmail: types.NullString{String: "ewa@example.dev", Valid: true},
			}, nil
		},
	}
}

// TestCreateTripEnrollment covers an admin's sign-up of a person for a leg.
// Switched-off sign-ups, a hidden leg and a passed deadline stop a self
// sign-up but not an admin; the place rule still applies.
func TestCreateTripEnrollment(t *testing.T) {
	planned := func(open, published bool, maxCrew int64) sqlcdb.LockTripForSignUpRow {
		return sqlcdb.LockTripForSignUpRow{
			ID: 5, Status: sqlcdb.TripStatusPlanned, Open: open, Published: published,
			MaxCrew:        types.NullInt64{Int64: maxCrew, Valid: maxCrew > 0},
			SignUpDeadline: types.NullString{String: "2020-01-01", Valid: true},
		}
	}
	tests := []struct {
		name       string
		ctx        context.Context
		path       string
		body       map[string]any
		setup      adminSignUpMock
		want       int
		wantStatus string
	}{
		{name: "open leg", setup: adminSignUpMock{leg: planned(true, true, 0)}, want: http.StatusCreated, wantStatus: "pending"},
		{name: "sign-ups switched off, hidden, past the deadline", setup: adminSignUpMock{leg: planned(false, false, 0)}, want: http.StatusCreated, wantStatus: "pending"},
		{name: "full leg waitlists", setup: adminSignUpMock{leg: planned(true, true, 2), taken: 2}, want: http.StatusCreated, wantStatus: "waitlisted"},
		{name: "someone waiting waitlists", setup: adminSignUpMock{leg: planned(true, true, 4), taken: 1, waiting: 1}, want: http.StatusCreated, wantStatus: "waitlisted"},
		{name: "free place stays pending", setup: adminSignUpMock{leg: planned(true, true, 2), taken: 1}, want: http.StatusCreated, wantStatus: "pending"},
		{
			name:  "cancelled leg",
			setup: adminSignUpMock{leg: sqlcdb.LockTripForSignUpRow{ID: 5, Status: sqlcdb.TripStatusCancelled}},
			want:  http.StatusConflict,
		},
		{name: "already signed up", setup: adminSignUpMock{leg: planned(true, true, 0), existing: true}, want: http.StatusConflict},
		{name: "unknown person", body: map[string]any{"crew_member_id": 8}, setup: adminSignUpMock{leg: planned(true, true, 0)}, want: http.StatusUnprocessableEntity},
		{name: "person gone before the insert", setup: adminSignUpMock{leg: planned(true, true, 0), insertFK: true}, want: http.StatusUnprocessableEntity},
		{name: "unknown trip", path: "/trips/6/enrollments", setup: adminSignUpMock{leg: planned(true, true, 0)}, want: http.StatusNotFound},
		{name: "member", ctx: userCtxRole(context.Background(), "member"), setup: adminSignUpMock{leg: planned(true, true, 0)}, want: http.StatusForbidden},
		{name: "missing person", body: map[string]any{"note": "x"}, setup: adminSignUpMock{leg: planned(true, true, 0)}, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, path, body := tt.ctx, tt.path, tt.body
			if ctx == nil {
				ctx = userCtx(context.Background())
			}
			if path == "" {
				path = "/trips/5/enrollments"
			}
			if body == nil {
				body = map[string]any{"crew_member_id": 7, "note": "  dzwoniła  "}
			}
			s := tt.setup
			resp := enrollTestAPI(t, s.mock()).PostCtx(ctx, path, body)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusCreated {
				if len(s.inserted) != 0 {
					t.Fatalf("refused sign-up inserted %+v", s.inserted)
				}
				return
			}
			want := sqlcdb.CreateTripEnrollmentIfAbsentParams{
				TripID: 5, CrewMemberID: 7, Status: tt.wantStatus,
				Note: types.NullString{String: "dzwoniła", Valid: true},
			}
			if len(s.inserted) != 1 || s.inserted[0] != want {
				t.Fatalf("inserted %+v, want [%+v]", s.inserted, want)
			}
			var got dto.TripEnrollmentDetail
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.ID != 31 || got.Status != tt.wantStatus || got.PersonName != "Ewa bez konta" ||
				got.CrewMemberID == nil || *got.CrewMemberID != 7 || got.UserID != nil ||
				got.Email == nil || *got.Email != "ewa@example.dev" {
				t.Fatalf("response = %+v, want Ewa's %s sign-up without an account", got, tt.wantStatus)
			}
		})
	}
}

// personSignUpAPI serves the enrollment and crew routes against conn.
func personSignUpAPI(t *testing.T, conn *sql.DB) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	q := sqlcdb.New(conn)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: q, runTx: sqlTxRunner(conn)})
	registerCrewRoutes(api, &CrewHandler{q: q, runTx: sqlTxRunner(conn)})
	return api
}

// TestAdminSignUp_PersonWithoutAccountAgainstDatabase signs people without
// an account up for a two-place leg whose sign-ups are switched off, accepts
// one onto the crew list, edits its role, and checks every count treats a
// person as one place, with or without an account. An account's legacy
// cruise enrollment and its own leg sign-up for the same leg are one person.
func TestAdminSignUp_PersonWithoutAccountAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	trip, users := capacityLeg(t, conn, 2, 1)
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET enroll_token = NULL, published_at = NULL WHERE id = $1", trip); err != nil {
		t.Fatalf("close leg: %v", err)
	}
	person := func(name string) int64 {
		t.Helper()
		var id int64
		if err := conn.QueryRowContext(ctx, `INSERT INTO crew_members (full_name, email) VALUES ($1, $2) RETURNING id`,
			name, fmt.Sprintf("%d@example.dev", len(name))).Scan(&id); err != nil {
			t.Fatalf("create person: %v", err)
		}
		return id
	}
	ewa, jan, iza := person("Ewa"), person("Jan Kowal"), person("Iza Mała")
	api := personSignUpAPI(t, conn)
	admin := userCtx(ctx)
	base := fmt.Sprintf("/trips/%d", trip)
	signUp := func(personID int64) dto.TripEnrollmentDetail {
		t.Helper()
		resp := api.PostCtx(admin, base+"/enrollments", map[string]any{"crew_member_id": personID})
		if resp.Code != http.StatusCreated {
			t.Fatalf("sign up person %d: got %d; body=%s", personID, resp.Code, resp.Body)
		}
		var out dto.TripEnrollmentDetail
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	first := signUp(ewa)
	if first.Status != "pending" || first.UserID != nil || first.PersonName != "Ewa" {
		t.Fatalf("Ewa's sign-up = %+v, want pending without an account", first)
	}
	if code := api.PostCtx(admin, base+"/enrollments", map[string]any{"crew_member_id": ewa}).Code; code != http.StatusConflict {
		t.Fatalf("second sign-up of Ewa: got %d, want 409", code)
	}
	if got := signUp(jan); got.Status != "pending" {
		t.Fatalf("Jan's sign-up = %s, want pending on the second place", got.Status)
	}
	if got := signUp(iza); got.Status != "waitlisted" {
		t.Fatalf("Iza's sign-up = %s, want waitlisted on a full leg", got.Status)
	}

	if code := setTripStatus(t, api, trip, first.ID, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept Ewa: got %d", code)
	}
	var crew []dto.CrewAssignment
	if err := json.Unmarshal(api.GetCtx(admin, base+"/crew").Body.Bytes(), &crew); err != nil {
		t.Fatalf("decode crew: %v", err)
	}
	if len(crew) != 1 || crew[0].CrewMemberID != ewa || crew[0].Role != "zalogant" {
		t.Fatalf("crew = %+v, want Ewa as zalogant", crew)
	}
	resp := api.PatchCtx(admin, fmt.Sprintf("%s/crew/%d", base, crew[0].ID), map[string]any{"role": "oficer"})
	if resp.Code != http.StatusOK {
		t.Fatalf("edit role: got %d; body=%s", resp.Code, resp.Body)
	}
	if resp := api.PatchCtx(admin, fmt.Sprintf("/trips/%d/crew/%d", trip+1000, crew[0].ID), map[string]any{"role": "kapitan"}); resp.Code != http.StatusNotFound {
		t.Fatalf("edit role through another trip: got %d, want 404", resp.Code)
	}

	q := sqlcdb.New(conn)
	var cruise int64
	if err := conn.QueryRowContext(ctx, "SELECT cruise_id FROM trips WHERE id = $1", trip).Scan(&cruise); err != nil {
		t.Fatalf("cruise of leg: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status) VALUES ($1, $2, $3, 'accepted')`,
		cruise, users[0], trip); err != nil {
		t.Fatalf("seed cruise enrollment: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO trip_enrollments (trip_id, crew_member_id, status)
		SELECT $1, id, 'accepted' FROM crew_members WHERE user_id = $2`, trip, users[0]); err != nil {
		t.Fatalf("seed account sign-up: %v", err)
	}
	accepted, err := q.CountAcceptedForTrip(ctx, trip)
	if err != nil || accepted != 2 {
		t.Fatalf("accepted = %d (err %v), want 2 people", accepted, err)
	}
	byTrip, err := q.CountAcceptedByCruiseTrip(ctx, cruise)
	if err != nil || len(byTrip) != 1 || byTrip[0].Accepted != 2 {
		t.Fatalf("accepted by leg = %+v (err %v), want 2 people", byTrip, err)
	}
	use, err := q.CountTripPlaceUse(ctx, trip)
	if err != nil || use.Taken != 3 || use.Waitlisted != 1 {
		t.Fatalf("place use = %+v (err %v), want 3 taken (Ewa, Jan, the account) and 1 waiting", use, err)
	}
	other, err := q.CountOtherAcceptedForTrip(ctx, sqlcdb.CountOtherAcceptedForTripParams{TripID: trip, CrewMemberID: ewa})
	if err != nil || other != 1 {
		t.Fatalf("accepted other than Ewa = %d (err %v), want 1", other, err)
	}

	var listed []dto.TripEnrollmentDetail
	if err := json.Unmarshal(api.GetCtx(admin, base+"/enrollments").Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode sign-ups: %v", err)
	}
	if len(listed) != 4 || listed[0].PersonName != "Ewa" || listed[0].UserID != nil ||
		listed[3].UserID == nil || *listed[3].UserID != users[0] {
		t.Fatalf("sign-ups = %+v, want Ewa, Jan, Iza without an account and the account last", listed)
	}

	if code := setTripStatus(t, api, trip, first.ID, "rejected"); code != http.StatusNoContent {
		t.Fatalf("reject Ewa: got %d", code)
	}
	if got := crewOn(t, conn, "trip_id", trip); len(got) != 0 {
		t.Fatalf("crew after rejecting Ewa = %v, want none", got)
	}
}

// TestAdminSignUp_RefusesPersonPlacedByCruiseEnrollment refuses an admin
// sign-up for an account's person whose cruise enrollment is already placed
// on the leg, in any status, as the account's own sign-up check does. A
// pending duplicate would hide that enrollment from the account.
func TestAdminSignUp_RefusesPersonPlacedByCruiseEnrollment(t *testing.T) {
	for _, status := range []string{"accepted", "pending"} {
		t.Run(status, func(t *testing.T) {
			conn := lockTestDB(t)
			ctx := context.Background()
			trip, users := capacityLeg(t, conn, 4, 1)
			person := ensurePerson(t, conn, users[0])
			if _, err := conn.ExecContext(ctx, `INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status)
				SELECT cruise_id, $2, id, $3 FROM trips WHERE id = $1`, trip, users[0], status); err != nil {
				t.Fatalf("seed cruise enrollment: %v", err)
			}
			api := personSignUpAPI(t, conn)
			resp := api.PostCtx(userCtx(ctx), fmt.Sprintf("/trips/%d/enrollments", trip), map[string]any{"crew_member_id": person})
			if resp.Code != http.StatusConflict {
				t.Fatalf("admin sign-up over a %s cruise enrollment: got %d, want 409; body=%s", status, resp.Code, resp.Body)
			}
			if got := signUpsOn(t, conn, trip); len(got) != 0 {
				t.Errorf("leg sign-ups = %v, want none", got)
			}
		})
	}
}
