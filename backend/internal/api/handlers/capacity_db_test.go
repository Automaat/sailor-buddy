package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// capacityLeg seeds an open leg with max_crew places and n users, and
// returns the leg and the users' IDs in creation order.
func capacityLeg(t *testing.T, conn *sql.DB, maxCrew, n int) (tripID int64, users []int64) {
	t.Helper()
	ctx := context.Background()
	q := sqlcdb.New(conn)
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "WSEX", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "Leg 1", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET enroll_token = 'leg-tok', max_crew = $1 WHERE id = $2", maxCrew, trip.ID); err != nil {
		t.Fatalf("open leg: %v", err)
	}
	users = make([]int64, n)
	for i := range n {
		u, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: fmt.Sprintf("crew%d@example.com", i), Name: ""})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		users[i] = u.ID
	}
	return trip.ID, users
}

// TestCapacity_SignUpAndWaitlistAgainstDatabase walks a two-place leg through
// the waiting list against real tables: sign-ups past the places held by
// accepted plus pending crew are waitlisted in sign-up order, withdrawing
// moves the list up, a full leg refuses an accept, and once an accepted
// sign-up is rejected the admin promotes the first waitlisted by hand.
func TestCapacity_SignUpAndWaitlistAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	trip, users := capacityLeg(t, conn, 2, 4)
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})
	path := "/trips/" + strconv.FormatInt(trip, 10) + "/sign-up"

	signUp := func(user int64) dto.EnrollLegResult {
		t.Helper()
		resp := api.PostCtx(guestCtx(user), path)
		if resp.Code != http.StatusCreated {
			t.Fatalf("sign up %d: got %d; body=%s", user, resp.Code, resp.Body)
		}
		return decodeSignUp(t, resp.Body.Bytes())
	}
	position := func(user int64) (status string, pos int64) {
		t.Helper()
		var got dto.LegSignUp
		if err := json.Unmarshal(api.GetCtx(guestCtx(user), path).Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Enrollment == nil {
			return "", 0
		}
		if got.WaitlistPosition != nil {
			pos = *got.WaitlistPosition
		}
		return got.Enrollment.Status, pos
	}
	setStatus := func(id int64, status string) int {
		t.Helper()
		return api.PutCtx(userCtx(context.Background()),
			fmt.Sprintf("/trips/%d/enrollments/%d/status", trip, id), map[string]string{"status": status}).Code
	}

	first, second := signUp(users[0]), signUp(users[1])
	if first.Enrollment.Status != "pending" || second.Enrollment.Status != "pending" {
		t.Fatalf("first two = %s, %s; want pending", first.Enrollment.Status, second.Enrollment.Status)
	}
	if code := setStatus(first.Enrollment.ID, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept first: got %d", code)
	}
	third, fourth := signUp(users[2]), signUp(users[3])
	if third.Enrollment.Status != "waitlisted" || third.WaitlistPosition == nil || *third.WaitlistPosition != 1 {
		t.Fatalf("third = %+v, want waitlisted at 1", third)
	}
	if fourth.Enrollment.Status != "waitlisted" || fourth.WaitlistPosition == nil || *fourth.WaitlistPosition != 2 {
		t.Fatalf("fourth = %+v, want waitlisted at 2", fourth)
	}

	if code := setStatus(second.Enrollment.ID, "accepted"); code != http.StatusNoContent {
		t.Fatalf("accept second: got %d", code)
	}
	if code := setStatus(third.Enrollment.ID, "accepted"); code != http.StatusConflict {
		t.Fatalf("accept on a full leg: got %d, want 409", code)
	}

	if resp := api.DeleteCtx(guestCtx(users[2]), path); resp.Code != http.StatusNoContent {
		t.Fatalf("withdraw waitlisted: got %d; body=%s", resp.Code, resp.Body)
	}
	if st, pos := position(users[3]); st != "waitlisted" || pos != 1 {
		t.Fatalf("fourth after withdraw = %s at %d, want waitlisted at 1", st, pos)
	}

	if code := setStatus(first.Enrollment.ID, "rejected"); code != http.StatusNoContent {
		t.Fatalf("reject first: got %d", code)
	}
	if st, _ := position(users[3]); st != "waitlisted" {
		t.Fatalf("fourth after a free place = %s, want still waitlisted: promotion is manual", st)
	}
	if code := setStatus(fourth.Enrollment.ID, "accepted"); code != http.StatusNoContent {
		t.Fatalf("promote fourth: got %d, want 204", code)
	}
	var accepted int64
	if err := conn.QueryRow("SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1 AND status = 'accepted'", trip).Scan(&accepted); err != nil || accepted != 2 {
		t.Fatalf("accepted = %d (err %v), want 2", accepted, err)
	}
}

// TestCapacity_ConcurrentAcceptsAgainstDatabase holds the leg's row lock
// while two admins accept different sign-ups for its last place, then lets
// both run: the accepts serialize on the lock and exactly one gets 409.
func TestCapacity_ConcurrentAcceptsAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	trip, users := capacityLeg(t, conn, 1, 2)
	var ids []int64
	for _, u := range users {
		var id int64
		if err := conn.QueryRow("INSERT INTO trip_enrollments (trip_id, user_id) VALUES ($1, $2) RETURNING id", trip, u).Scan(&id); err != nil {
			t.Fatalf("seed enrollment: %v", err)
		}
		ids = append(ids, id)
	}
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})

	holder, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback() }()
	if _, err := holder.ExecContext(ctx, "SELECT 1 FROM trips WHERE id = $1 FOR NO KEY UPDATE", trip); err != nil {
		t.Fatalf("hold trip lock: %v", err)
	}

	codes := make(chan int, len(ids))
	for _, id := range ids {
		go func() {
			codes <- api.PutCtx(userCtx(ctx), fmt.Sprintf("/trips/%d/enrollments/%d/status", trip, id),
				map[string]string{"status": "accepted"}).Code
		}()
	}
	waitForLockWaiters(t, conn, 2)
	if err := holder.Commit(); err != nil {
		t.Fatalf("release trip lock: %v", err)
	}
	got := []int{<-codes, <-codes}
	slices.Sort(got)
	if got[0] != http.StatusNoContent || got[1] != http.StatusConflict {
		t.Fatalf("codes = %v, want one 204 and one 409", got)
	}
	var accepted int64
	if err := conn.QueryRow("SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1 AND status = 'accepted'", trip).Scan(&accepted); err != nil || accepted != 1 {
		t.Fatalf("accepted = %d (err %v), want 1", accepted, err)
	}
}

// TestCapacity_CruiseEnrollmentAgainstDatabase checks the cruise enrollment
// routes against real tables: on a one-place leg already taken by a trip
// enrollment, neither accepting an assigned cruise enrollment nor moving an
// accepted one onto the leg goes past max_crew.
func TestCapacity_CruiseEnrollmentAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	trip, users := capacityLeg(t, conn, 1, 3)
	if _, err := conn.ExecContext(ctx, "INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'accepted')", trip, users[0]); err != nil {
		t.Fatalf("seed accepted: %v", err)
	}
	var cruise, assigned, unassigned int64
	if err := conn.QueryRow("SELECT cruise_id FROM trips WHERE id = $1", trip).Scan(&cruise); err != nil {
		t.Fatalf("cruise: %v", err)
	}
	if err := conn.QueryRow("INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id) VALUES ($1, $2, $3) RETURNING id",
		cruise, users[1], trip).Scan(&assigned); err != nil {
		t.Fatalf("seed assigned: %v", err)
	}
	if err := conn.QueryRow("INSERT INTO cruise_enrollments (cruise_id, user_id, status) VALUES ($1, $2, 'accepted') RETURNING id",
		cruise, users[2]).Scan(&unassigned); err != nil {
		t.Fatalf("seed unassigned: %v", err)
	}
	_, api := humatest.New(t)
	registerCruiseRoutes(api, NewCruiseHandler(sqlcdb.New(conn), conn))
	base := fmt.Sprintf("/cruises/%d/enrollments/", cruise)

	if code := api.PutCtx(userCtx(ctx), fmt.Sprintf("%s%d/status", base, assigned), map[string]string{"status": "accepted"}).Code; code != http.StatusConflict {
		t.Fatalf("accept assigned on a full leg: got %d, want 409", code)
	}
	if code := api.PutCtx(userCtx(ctx), fmt.Sprintf("%s%d/trip", base, unassigned), map[string]any{"trip_id": trip}).Code; code != http.StatusConflict {
		t.Fatalf("move accepted onto a full leg: got %d, want 409", code)
	}
	if code := api.PutCtx(userCtx(ctx), fmt.Sprintf("%s%d/status", base, assigned), map[string]string{"status": "rejected"}).Code; code != http.StatusNoContent {
		t.Fatalf("reject assigned: got %d, want 204", code)
	}
	var accepted int64
	if err := conn.QueryRow(`SELECT COUNT(*) FROM (
		SELECT user_id FROM trip_enrollments WHERE trip_id = $1 AND status = 'accepted'
		UNION SELECT user_id FROM cruise_enrollments WHERE trip_id = $1 AND status = 'accepted') u`, trip).Scan(&accepted); err != nil || accepted != 1 {
		t.Fatalf("accepted on the leg = %d (err %v), want 1", accepted, err)
	}
}

// TestCapacity_ConcurrentTripTokenEnrollsAgainstDatabase holds the leg's row
// lock while six members enroll through the trip share link of a two-place
// leg, then lets them all run: the enrolls serialize on the lock, so exactly
// two start pending and the rest are waitlisted.
func TestCapacity_ConcurrentTripTokenEnrollsAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	trip, users := capacityLeg(t, conn, 2, 6)
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})

	holder, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback() }()
	if _, err := holder.ExecContext(ctx, "SELECT 1 FROM trips WHERE id = $1 FOR NO KEY UPDATE", trip); err != nil {
		t.Fatalf("hold trip lock: %v", err)
	}

	codes := make(chan int, len(users))
	for _, u := range users {
		go func() {
			codes <- api.PostCtx(guestCtx(u), "/enroll/leg-tok", map[string]any{}).Code
		}()
	}
	waitForLockWaiters(t, conn, len(users))
	if err := holder.Commit(); err != nil {
		t.Fatalf("release trip lock: %v", err)
	}
	for range users {
		if code := <-codes; code != http.StatusCreated {
			t.Fatalf("enroll: got %d, want 201", code)
		}
	}
	var pending, waitlisted int64
	if err := conn.QueryRow(`SELECT COUNT(*) FILTER (WHERE status = 'pending'), COUNT(*) FILTER (WHERE status = 'waitlisted')
		FROM trip_enrollments WHERE trip_id = $1`, trip).Scan(&pending, &waitlisted); err != nil {
		t.Fatalf("count sign-ups: %v", err)
	}
	if pending != 2 || waitlisted != 4 {
		t.Fatalf("pending = %d, waitlisted = %d; want 2 and 4", pending, waitlisted)
	}
}
