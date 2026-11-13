package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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

// slowCrewWrites makes every crew_assignments insert or delete sleep, so a
// concurrent request lands while the first one still holds its locks.
func slowCrewWrites(t *testing.T, conn *sql.DB, event string) {
	t.Helper()
	stmt := fmt.Sprintf(`
		CREATE FUNCTION slow_crew() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_sleep(1); RETURN NULL; END $$;
		CREATE TRIGGER slow_crew AFTER %s ON crew_assignments
		FOR EACH STATEMENT EXECUTE FUNCTION slow_crew();`, event)
	if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
		t.Fatalf("slow crew writes: %v", err)
	}
}

// legacyLegs seeds a cruise with two legs and one user whose cruise
// enrollment is assigned to the first leg as pending.
func legacyLegs(t *testing.T, conn *sql.DB) (legA, legB, user, cruise, enrollment int64) {
	t.Helper()
	ctx := context.Background()
	legA, users := capacityLeg(t, conn, 5, 1)
	user = users[0]
	if err := conn.QueryRowContext(ctx, "SELECT cruise_id FROM trips WHERE id = $1", legA).Scan(&cruise); err != nil {
		t.Fatalf("cruise of leg: %v", err)
	}
	trip, err := sqlcdb.New(conn).CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise, Name: "Leg 2", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create leg 2: %v", err)
	}
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status)
		VALUES ($1, $2, $3, 'pending') RETURNING id`, cruise, user, legA).Scan(&enrollment); err != nil {
		t.Fatalf("seed cruise enrollment: %v", err)
	}
	return legA, trip.ID, user, cruise, enrollment
}

// TestAcceptedCrew_MoveRacesReject rejects an accepted cruise enrollment
// while it moves to another leg. Whatever order wins, an enrollment left
// rejected has no crew entry and an accepted one has exactly its leg's.
func TestAcceptedCrew_MoveRacesReject(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	legA, legB, user, cruise, enrollment := legacyLegs(t, conn)
	api := acceptedCrewAPI(t, conn)
	admin := userCtx(ctx)
	base := fmt.Sprintf("/cruises/%d/enrollments/%d", cruise, enrollment)
	if code := api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}).Code; code != http.StatusNoContent {
		t.Fatalf("accept: got %d", code)
	}
	slowCrewWrites(t, conn, "INSERT")

	var wg sync.WaitGroup
	var move, reject int
	wg.Go(func() { move = api.PutCtx(admin, base+"/trip", map[string]any{"trip_id": legB}).Code })
	wg.Go(func() {
		time.Sleep(300 * time.Millisecond)
		reject = api.PutCtx(admin, base+"/status", map[string]any{"status": "rejected"}).Code
	})
	wg.Wait()

	var status string
	var trip int64
	if err := conn.QueryRowContext(ctx, "SELECT status, trip_id FROM cruise_enrollments WHERE id = $1", enrollment).
		Scan(&status, &trip); err != nil {
		t.Fatalf("read enrollment: %v", err)
	}
	crewA, crewB := len(tripCrew(t, conn, legA, user)), len(tripCrew(t, conn, legB, user))
	t.Logf("move=%d reject=%d status=%s trip=%d crew A=%d B=%d", move, reject, status, trip, crewA, crewB)
	if move != http.StatusNoContent || trip != legB {
		t.Fatalf("move: got %d, trip %d; want 204 onto leg %d", move, trip, legB)
	}
	wantB := 0
	if status == enrollmentAccepted {
		wantB = 1
	}
	if crewA != 0 || crewB != wantB {
		t.Fatalf("%s enrollment on leg B: crew A=%d B=%d, want 0 and %d", status, crewA, crewB, wantB)
	}
}

// TestAcceptedCrew_ConcurrentRejects rejects a user's accepted direct leg
// sign-up and their accepted cruise enrollment for the same leg at once.
// With both gone the acceptance-added crew entry must go too.
func TestAcceptedCrew_ConcurrentRejects(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	legA, _, user, cruise, enrollment := legacyLegs(t, conn)
	var direct int64
	if err := conn.QueryRowContext(ctx, `
		INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'pending') RETURNING id`,
		legA, user).Scan(&direct); err != nil {
		t.Fatalf("seed direct sign-up: %v", err)
	}
	api := acceptedCrewAPI(t, conn)
	admin := userCtx(ctx)
	base := fmt.Sprintf("/cruises/%d/enrollments/%d", cruise, enrollment)
	if code := api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}).Code; code != http.StatusNoContent {
		t.Fatalf("accept cruise enrollment: got %d", code)
	}
	if code := setTripStatus(t, api, legA, direct, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept direct sign-up: got %d", code)
	}
	slowCrewWrites(t, conn, "DELETE")

	var wg sync.WaitGroup
	var first, second int
	wg.Go(func() { first = setTripStatus(t, api, legA, direct, "rejected") })
	wg.Go(func() {
		time.Sleep(300 * time.Millisecond)
		second = api.PutCtx(admin, base+"/status", map[string]any{"status": "rejected"}).Code
	})
	wg.Wait()
	if first != http.StatusNoContent || second != http.StatusNoContent {
		t.Fatalf("rejects: got %d and %d, want 204 both", first, second)
	}
	if got := tripCrew(t, conn, legA, user); len(got) != 0 {
		t.Fatalf("both sign-ups rejected, crew = %+v, want none", got)
	}
}

// TestAcceptedCrew_StatusChangeRacesOnCruiseEnrollment runs a second change
// on an accepted cruise enrollment while a reject of it still holds its
// locks. A repeated reject and a delete both still succeed, as they did
// before leg locking, and leave no crew entry.
func TestAcceptedCrew_StatusChangeRacesOnCruiseEnrollment(t *testing.T) {
	for _, second := range []string{"reject again", "delete"} {
		t.Run(second, func(t *testing.T) {
			conn := lockTestDB(t)
			ctx := context.Background()
			legA, _, user, cruise, enrollment := legacyLegs(t, conn)
			api := acceptedCrewAPI(t, conn)
			admin := userCtx(ctx)
			base := fmt.Sprintf("/cruises/%d/enrollments/%d", cruise, enrollment)
			if code := api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}).Code; code != http.StatusNoContent {
				t.Fatalf("accept: got %d", code)
			}
			if _, err := conn.ExecContext(ctx, `
				CREATE FUNCTION slow_enrollment() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN PERFORM pg_sleep(1); RETURN NULL; END $$;
				CREATE TRIGGER slow_enrollment AFTER UPDATE ON cruise_enrollments
				FOR EACH STATEMENT EXECUTE FUNCTION slow_enrollment();`); err != nil {
				t.Fatalf("slow enrollment updates: %v", err)
			}

			var wg sync.WaitGroup
			var first, next int
			wg.Go(func() { first = api.PutCtx(admin, base+"/status", map[string]any{"status": "rejected"}).Code })
			wg.Go(func() {
				time.Sleep(300 * time.Millisecond)
				if second == "delete" {
					next = api.DeleteCtx(admin, base).Code
				} else {
					next = api.PutCtx(admin, base+"/status", map[string]any{"status": "rejected"}).Code
				}
			})
			wg.Wait()
			if first != http.StatusNoContent || next != http.StatusNoContent {
				t.Fatalf("reject then %s: got %d and %d, want 204 both", second, first, next)
			}
			var rows int
			if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM cruise_enrollments WHERE id = $1", enrollment).Scan(&rows); err != nil {
				t.Fatalf("count enrollments: %v", err)
			}
			if wantRows := map[bool]int{true: 0, false: 1}[second == "delete"]; rows != wantRows {
				t.Fatalf("enrollment rows = %d, want %d", rows, wantRows)
			}
			if got := tripCrew(t, conn, legA, user); len(got) != 0 {
				t.Fatalf("crew = %+v, want none", got)
			}
		})
	}
}

// TestAcceptedCrew_ConcurrentMovesAndChanges fires moves between two legs,
// moves off the legs and status changes at one accepted cruise enrollment
// at once. None may fail with a server error, and the crew list must end in
// step with where the enrollment ends up.
func TestAcceptedCrew_ConcurrentMovesAndChanges(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	legA, legB, user, cruise, enrollment := legacyLegs(t, conn)
	api := acceptedCrewAPI(t, conn)
	admin := userCtx(ctx)
	base := fmt.Sprintf("/cruises/%d/enrollments/%d", cruise, enrollment)
	if code := api.PutCtx(admin, base+"/status", map[string]any{"status": "accepted"}).Code; code != http.StatusNoContent {
		t.Fatalf("accept: got %d", code)
	}

	type change struct {
		path string
		body map[string]any
	}
	changes := make([]change, 0, 40)
	for range 8 {
		changes = append(changes,
			change{"/trip", map[string]any{"trip_id": legA}},
			change{"/trip", map[string]any{"trip_id": legB}},
			change{"/trip", map[string]any{"trip_id": nil}},
			change{"/status", map[string]any{"status": "accepted"}},
			change{"/status", map[string]any{"status": "rejected"}},
		)
	}
	codes := make([]int, len(changes))
	var wg sync.WaitGroup
	for i, c := range changes {
		wg.Go(func() { codes[i] = api.PutCtx(admin, base+c.path, c.body).Code })
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusNoContent && code != http.StatusConflict {
			t.Errorf("change %d: got %d, want 204 or 409", i, code)
		}
	}

	var status string
	var trip types.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT status, trip_id FROM cruise_enrollments WHERE id = $1", enrollment).
		Scan(&status, &trip); err != nil {
		t.Fatalf("read enrollment: %v", err)
	}
	for _, leg := range []int64{legA, legB} {
		want := 0
		if status == enrollmentAccepted && trip.Valid && trip.Int64 == leg {
			want = 1
		}
		if got := len(tripCrew(t, conn, leg, user)); got != want {
			t.Errorf("leg %d crew = %d, want %d (enrollment %s on %+v)", leg, got, want, status, trip)
		}
	}
}
