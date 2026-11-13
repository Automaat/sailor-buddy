package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// linkSwitches are the admin changes that stop an old share link: clearing
// the token (sign-ups off) and regenerating it.
var linkSwitches = []struct {
	name  string
	token sql.NullString
}{
	{"cleared", sql.NullString{}},
	{"regenerated", sql.NullString{String: "new-tok", Valid: true}},
}

func countRows(t *testing.T, conn *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// TestEnrollLink_TripTokenSwitchedOffBeforeLockAgainstDatabase switches the
// trip link off in a transaction that holds the leg's row lock, so the enroll
// finds the old token, then waits on the lock. Once the switch commits, the
// enroll must see it: 409 and no sign-up.
func TestEnrollLink_TripTokenSwitchedOffBeforeLockAgainstDatabase(t *testing.T) {
	for _, sw := range linkSwitches {
		t.Run(sw.name, func(t *testing.T) {
			conn := lockTestDB(t)
			ctx := context.Background()
			trip, users := capacityLeg(t, conn, 0, 1)
			_, api := humatest.New(t)
			registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})

			admin, err := conn.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin admin: %v", err)
			}
			defer func() { _ = admin.Rollback() }()
			if _, err := admin.ExecContext(ctx, "UPDATE trips SET enroll_token = $1 WHERE id = $2", sw.token, trip); err != nil {
				t.Fatalf("switch link: %v", err)
			}

			code := make(chan int, 1)
			go func() {
				code <- api.PostCtx(guestCtx(users[0]), "/enroll/leg-tok", map[string]any{}).Code
			}()
			waitForLockWaiters(t, conn, 1)
			if err := admin.Commit(); err != nil {
				t.Fatalf("commit switch: %v", err)
			}
			if got := <-code; got != http.StatusConflict {
				t.Fatalf("enroll: got %d, want 409", got)
			}
			if n := countRows(t, conn, "SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1", trip); n != 0 {
				t.Fatalf("sign-ups = %d, want 0", n)
			}
		})
	}
}

// pausingTripLock holds a trip-token enroll right after it locks the leg.
type pausingTripLock struct {
	sqlcdb.Querier
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pausingTripLock) LockTripForSignUp(ctx context.Context, id int64) (sqlcdb.LockTripForSignUpRow, error) {
	row, err := p.Querier.LockTripForSignUp(ctx, id)
	close(p.locked)
	<-p.release
	return row, err
}

// TestEnrollLink_TripTokenClearWaitsForEnrollAgainstDatabase pauses a
// trip-token enroll after it locked the leg: the admin switch-off must wait
// until the enroll commits, so the sign-up is written before the clear.
func TestEnrollLink_TripTokenClearWaitsForEnrollAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	trip, users := capacityLeg(t, conn, 0, 1)
	locked := make(chan struct{})
	release := make(chan struct{})
	enrolls := &EnrollmentHandler{q: sqlcdb.New(conn), runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return sqlTxRunner(conn)(ctx, func(q sqlcdb.Querier) error {
			return fn(pausingTripLock{Querier: q, locked: locked, release: release})
		})
	}}
	_, enrollAPI := humatest.New(t)
	registerEnrollmentRoutes(enrollAPI, enrolls)
	_, adminAPI := humatest.New(t)
	registerEnrollmentRoutes(adminAPI, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})

	enrolled := make(chan int, 1)
	go func() {
		enrolled <- enrollAPI.PostCtx(guestCtx(users[0]), "/enroll/leg-tok", map[string]any{}).Code
	}()
	<-locked
	cleared := make(chan int, 1)
	go func() {
		cleared <- adminAPI.DeleteCtx(userCtx(context.Background()), fmt.Sprintf("/trips/%d/enroll-token", trip)).Code
	}()
	waitForLockWaiters(t, conn, 1)
	close(release)

	if code := <-enrolled; code != http.StatusCreated {
		t.Fatalf("enroll: got %d, want 201", code)
	}
	if code := <-cleared; code != http.StatusNoContent {
		t.Fatalf("clear: got %d, want 204", code)
	}
	if n := countRows(t, conn, "SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1", trip); n != 1 {
		t.Fatalf("sign-ups = %d, want 1", n)
	}
	if n := countRows(t, conn, "SELECT COUNT(*) FROM trips WHERE id = $1 AND enroll_token IS NULL", trip); n != 1 {
		t.Fatal("link still on after the clear")
	}
}

// TestEnrollLink_CruiseTokenSwitchedOffBeforeLockAgainstDatabase switches the
// cruise link off in a transaction that holds the cruise row, so the leg
// enroll finds the old token, then waits on the cruise lock. Once the switch
// commits, the enroll must see it: 404 and no sign-up.
func TestEnrollLink_CruiseTokenSwitchedOffBeforeLockAgainstDatabase(t *testing.T) {
	for _, sw := range linkSwitches {
		t.Run(sw.name, func(t *testing.T) {
			conn := lockTestDB(t)
			ctx := context.Background()
			leg := seedLeg(t, conn)
			if _, err := conn.ExecContext(ctx, "DELETE FROM trip_enrollments WHERE trip_id = $1", leg.tripID); err != nil {
				t.Fatalf("clear seeded sign-up: %v", err)
			}
			h := &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)}

			admin, err := conn.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin admin: %v", err)
			}
			defer func() { _ = admin.Rollback() }()
			if _, err := admin.ExecContext(ctx, "UPDATE cruises SET enroll_token = $1 WHERE id = $2", sw.token, leg.cruiseID); err != nil {
				t.Fatalf("switch link: %v", err)
			}

			enrolled := make(chan error, 1)
			go func() {
				_, err := h.enrollLegs(memberCtx(leg.userID), &enrollLegsInput{
					Token: leg.token, Body: dto.EnrollLegsBody{TripIDs: []int64{leg.tripID}},
				})
				enrolled <- err
			}()
			waitForLockWaiters(t, conn, 1)
			if err := admin.Commit(); err != nil {
				t.Fatalf("commit switch: %v", err)
			}
			if got := statusOf(<-enrolled); got != http.StatusNotFound {
				t.Fatalf("enroll: got %d, want 404", got)
			}
			if n := countRows(t, conn, "SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1", leg.tripID); n != 0 {
				t.Fatalf("sign-ups = %d, want 0", n)
			}
		})
	}
}

// TestEnrollLink_CruiseTokenClearWaitsForLegEnrollAgainstDatabase pauses a
// leg enroll after it took its locks: the cruise switch-off must wait until
// the enroll commits, so the sign-up is written before the clear.
func TestEnrollLink_CruiseTokenClearWaitsForLegEnrollAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	leg := seedLeg(t, conn)
	if _, err := conn.ExecContext(ctx, "DELETE FROM trip_enrollments WHERE trip_id = $1", leg.tripID); err != nil {
		t.Fatalf("clear seeded sign-up: %v", err)
	}
	locked := make(chan struct{})
	release := make(chan struct{})
	h := &EnrollmentHandler{q: sqlcdb.New(conn), runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return sqlTxRunner(conn)(ctx, func(q sqlcdb.Querier) error {
			return fn(pausingQuerier{Querier: q, locked: locked, release: release})
		})
	}}

	enrolled := make(chan error, 1)
	go func() {
		_, err := h.enrollLegs(memberCtx(leg.userID), &enrollLegsInput{
			Token: leg.token, Body: dto.EnrollLegsBody{TripIDs: []int64{leg.tripID}},
		})
		enrolled <- err
	}()
	<-locked
	cleared := make(chan error, 1)
	go func() {
		cleared <- sqlcdb.New(conn).ClearCruiseEnrollToken(ctx, leg.cruiseID)
	}()
	waitForLockWaiters(t, conn, 1)
	close(release)

	if err := <-enrolled; err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := <-cleared; err != nil {
		t.Fatalf("clear: %v", err)
	}
	if n := countRows(t, conn, "SELECT COUNT(*) FROM trip_enrollments WHERE trip_id = $1", leg.tripID); n != 1 {
		t.Fatalf("sign-ups = %d, want 1", n)
	}
}
