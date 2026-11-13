package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// crewEntry is one crew list row of a person on a trip.
type crewEntry struct {
	role              string
	addedByAcceptance bool
}

// tripCrew returns the crew entries of userID's person on the trip.
func tripCrew(t *testing.T, conn *sql.DB, tripID, userID int64) []crewEntry {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), `
		SELECT ca.role, ca.added_by_acceptance
		FROM crew_assignments ca
		JOIN crew_members cm ON cm.id = ca.crew_member_id
		WHERE ca.trip_id = $1 AND cm.user_id = $2`, tripID, userID)
	if err != nil {
		t.Fatalf("query crew: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []crewEntry
	for rows.Next() {
		var e crewEntry
		if err := rows.Scan(&e.role, &e.addedByAcceptance); err != nil {
			t.Fatalf("scan crew: %v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("crew rows: %v", err)
	}
	return out
}

// acceptedCrewAPI serves the enrollment and cruise routes against conn.
func acceptedCrewAPI(t *testing.T, conn *sql.DB) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	q := sqlcdb.New(conn)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: q, runTx: sqlTxRunner(conn)})
	registerCruiseRoutes(api, &CruiseHandler{q: q, runTx: sqlTxRunner(conn)})
	return api
}

// signUpVia signs userID up for the leg through the leg's sign-up button or
// its share link, and returns the trip enrollment ID.
func signUpVia(t *testing.T, conn *sql.DB, api humatest.TestAPI, how string, tripID, userID int64) int64 {
	t.Helper()
	var resp *httptest.ResponseRecorder
	switch how {
	case "sign-up":
		resp = api.PostCtx(guestCtx(userID), fmt.Sprintf("/trips/%d/sign-up", tripID))
	case "share-link":
		token := fmt.Sprintf("leg-tok-%d", tripID)
		if _, err := conn.ExecContext(context.Background(), "UPDATE trips SET enroll_token = $1 WHERE id = $2", token, tripID); err != nil {
			t.Fatalf("set share token: %v", err)
		}
		resp = api.PostCtx(guestCtx(userID), "/enroll/"+token, map[string]any{})
	default:
		t.Fatalf("unknown sign-up path %q", how)
	}
	if resp.Code != http.StatusCreated {
		t.Fatalf("%s for user %d: got %d; body=%s", how, userID, resp.Code, resp.Body)
	}
	var id int64
	if err := conn.QueryRowContext(context.Background(),
		"SELECT id FROM trip_enrollments WHERE trip_id = $1 AND user_id = $2", tripID, userID).Scan(&id); err != nil {
		t.Fatalf("find enrollment: %v", err)
	}
	return id
}

func setTripStatus(t *testing.T, api humatest.TestAPI, tripID, id int64, status string) int {
	t.Helper()
	return api.PutCtx(userCtx(context.Background()),
		fmt.Sprintf("/trips/%d/enrollments/%d/status", tripID, id), map[string]any{"status": status}).Code
}

func ensurePerson(t *testing.T, conn *sql.DB, userID int64) int64 {
	t.Helper()
	q := sqlcdb.New(conn)
	if err := q.EnsureUserPerson(context.Background(), userID); err != nil {
		t.Fatalf("ensure person: %v", err)
	}
	p, err := q.GetCrewMemberByUserID(context.Background(), types.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	return p.ID
}

// TestAcceptedCrew_TripSignUp accepts a leg sign-up made through either
// path, checks the person joins the crew list once as a załogant, then takes
// the sign-up out of accepted and checks the entry goes.
func TestAcceptedCrew_TripSignUp(t *testing.T) {
	leaves := []string{"rejected", "pending", "waitlisted", "deleted"}
	for _, how := range []string{"sign-up", "share-link"} {
		for _, leave := range leaves {
			t.Run(how+" then "+leave, func(t *testing.T) {
				conn := lockTestDB(t)
				trip, _ := capacityLeg(t, conn, 3, 0)
				user, err := sqlcdb.New(conn).CreateUser(context.Background(), sqlcdb.CreateUserParams{
					Email: fmt.Sprintf("%s-%s-%d@example.com", how, leave, trip), Name: "Ola",
				})
				if err != nil {
					t.Fatalf("create user: %v", err)
				}
				ensurePerson(t, conn, user.ID)
				api := acceptedCrewAPI(t, conn)
				id := signUpVia(t, conn, api, how, trip, user.ID)

				if got := tripCrew(t, conn, trip, user.ID); len(got) != 0 {
					t.Fatalf("pending sign-up has crew %+v, want none", got)
				}
				for range 2 {
					if code := setTripStatus(t, api, trip, id, "accepted"); code != http.StatusNoContent {
						t.Fatalf("accept: got %d", code)
					}
				}
				want := []crewEntry{{role: "zalogant", addedByAcceptance: true}}
				if got := tripCrew(t, conn, trip, user.ID); len(got) != 1 || got[0] != want[0] {
					t.Fatalf("crew after accept = %+v, want %+v", got, want)
				}

				if leave == "deleted" {
					resp := api.DeleteCtx(userCtx(context.Background()), fmt.Sprintf("/trips/%d/enrollments/%d", trip, id))
					if resp.Code != http.StatusNoContent {
						t.Fatalf("delete: got %d", resp.Code)
					}
				} else if code := setTripStatus(t, api, trip, id, leave); code != http.StatusNoContent {
					t.Fatalf("set %s: got %d", leave, code)
				}
				if got := tripCrew(t, conn, trip, user.ID); len(got) != 0 {
					t.Fatalf("crew after %s = %+v, want none", leave, got)
				}
			})
		}
	}
}

// TestAcceptedCrew_HandAddedEntryStays checks that accepting a person the
// admin already put on the crew list keeps one hand-added entry, and that
// rejecting or deleting the sign-up leaves it.
func TestAcceptedCrew_HandAddedEntryStays(t *testing.T) {
	for _, leave := range []string{"rejected", "deleted"} {
		t.Run(leave, func(t *testing.T) {
			conn := lockTestDB(t)
			trip, _ := capacityLeg(t, conn, 0, 0)
			user, err := sqlcdb.New(conn).CreateUser(context.Background(), sqlcdb.CreateUserParams{
				Email: fmt.Sprintf("hand-%s@example.com", leave), Name: "Ola",
			})
			if err != nil {
				t.Fatalf("create user: %v", err)
			}
			person := ensurePerson(t, conn, user.ID)
			if _, err := sqlcdb.New(conn).CreateTripCrewAssignment(context.Background(), sqlcdb.CreateTripCrewAssignmentParams{
				TripID: types.NullInt64{Int64: trip, Valid: true}, CrewMemberID: person, Role: "oficer",
			}); err != nil {
				t.Fatalf("hand-add crew: %v", err)
			}
			api := acceptedCrewAPI(t, conn)
			id := signUpVia(t, conn, api, "sign-up", trip, user.ID)
			if code := setTripStatus(t, api, trip, id, "accepted"); code != http.StatusNoContent {
				t.Fatalf("accept: got %d", code)
			}
			want := crewEntry{role: "oficer"}
			if got := tripCrew(t, conn, trip, user.ID); len(got) != 1 || got[0] != want {
				t.Fatalf("crew after accept = %+v, want [%+v]", got, want)
			}
			if leave == "deleted" {
				api.DeleteCtx(userCtx(context.Background()), fmt.Sprintf("/trips/%d/enrollments/%d", trip, id))
			} else {
				setTripStatus(t, api, trip, id, leave)
			}
			if got := tripCrew(t, conn, trip, user.ID); len(got) != 1 || got[0] != want {
				t.Fatalf("crew after %s = %+v, want [%+v]", leave, got, want)
			}
		})
	}
}

// TestAcceptedCrew_FullLegAddsNothing checks a refused accept on a full leg
// leaves the crew list untouched.
func TestAcceptedCrew_FullLegAddsNothing(t *testing.T) {
	conn := lockTestDB(t)
	trip, users := capacityLeg(t, conn, 1, 2)
	api := acceptedCrewAPI(t, conn)
	first := signUpVia(t, conn, api, "sign-up", trip, users[0])
	var second int64
	if err := conn.QueryRowContext(context.Background(), `
		INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'pending') RETURNING id`,
		trip, users[1]).Scan(&second); err != nil {
		t.Fatalf("seed second sign-up: %v", err)
	}
	if code := setTripStatus(t, api, trip, first, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept first: got %d", code)
	}
	if code := setTripStatus(t, api, trip, second, "accepted"); code != http.StatusConflict {
		t.Fatalf("accept on full leg: got %d, want 409", code)
	}
	if got := tripCrew(t, conn, trip, users[1]); len(got) != 0 {
		t.Fatalf("refused accept added crew %+v", got)
	}
	if got := tripCrew(t, conn, trip, users[0]); len(got) != 1 {
		t.Fatalf("accepted user crew = %+v, want one entry", got)
	}
}

// TestAcceptedCrew_LegacyCruiseEnrollment walks a cruise-level enrollment
// assigned to a leg through accept, a move to another leg, rejection and
// deletion, and checks a second accepted sign-up keeps the entry.
func TestAcceptedCrew_LegacyCruiseEnrollment(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	legA, users := capacityLeg(t, conn, 2, 1)
	user := users[0]
	var cruise int64
	if err := conn.QueryRowContext(ctx, "SELECT cruise_id FROM trips WHERE id = $1", legA).Scan(&cruise); err != nil {
		t.Fatalf("cruise of leg: %v", err)
	}
	legBTrip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise, Name: "Leg 2", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create leg 2: %v", err)
	}
	legB := legBTrip.ID
	var enrollment int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status)
		VALUES ($1, $2, $3, 'pending') RETURNING id`, cruise, user, legA).Scan(&enrollment); err != nil {
		t.Fatalf("seed cruise enrollment: %v", err)
	}
	api := acceptedCrewAPI(t, conn)
	admin := userCtx(ctx)
	base := fmt.Sprintf("/cruises/%d/enrollments/%d", cruise, enrollment)
	step := func(name string, resp *httptest.ResponseRecorder, wantA, wantB int) {
		t.Helper()
		if resp.Code != http.StatusNoContent {
			t.Fatalf("%s: got %d; body=%s", name, resp.Code, resp.Body)
		}
		if got := len(tripCrew(t, conn, legA, user)); got != wantA {
			t.Fatalf("%s: leg A crew entries = %d, want %d", name, got, wantA)
		}
		if got := len(tripCrew(t, conn, legB, user)); got != wantB {
			t.Fatalf("%s: leg B crew entries = %d, want %d", name, got, wantB)
		}
	}

	step("accept", api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}), 1, 0)
	step("move to leg B", api.PutCtx(admin, base+"/trip", map[string]any{"trip_id": legB}), 0, 1)
	step("move off the legs", api.PutCtx(admin, base+"/trip", map[string]any{"trip_id": nil}), 0, 0)
	step("back to leg A", api.PutCtx(admin, base+"/trip", map[string]any{"trip_id": legA}), 1, 0)
	step("reject", api.PutCtx(admin, base+"/status", map[string]any{"status": "rejected"}), 0, 0)
	step("accept again", api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}), 1, 0)

	var direct int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'pending') RETURNING id`,
		legA, user).Scan(&direct); err != nil {
		t.Fatalf("seed direct sign-up: %v", err)
	}
	if code := setTripStatus(t, api, legA, direct, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept direct sign-up: got %d", code)
	}
	step("reject the cruise enrollment while the direct sign-up holds", api.PutCtx(admin, base+"/status",
		map[string]any{"status": "rejected"}), 1, 0)
	step("accept the cruise enrollment again", api.PutCtx(admin, base+"/status",
		map[string]any{"status": "accepted"}), 1, 0)
	if code := setTripStatus(t, api, legA, direct, "rejected"); code != http.StatusNoContent {
		t.Fatalf("reject direct sign-up: got %d", code)
	}
	if got := len(tripCrew(t, conn, legA, user)); got != 1 {
		t.Fatalf("crew after rejecting the direct sign-up = %d, want 1 (cruise enrollment holds)", got)
	}
	step("delete the cruise enrollment", api.DeleteCtx(admin, base), 0, 0)
}
