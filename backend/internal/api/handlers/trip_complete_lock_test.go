package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// lockTestDB creates a throwaway database on the server named by
// SAILOR_TEST_DATABASE_URL and migrates it. A separate database keeps this
// package's tests apart from the schema resets in internal/db, which run in
// parallel against the same server.
func lockTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SAILOR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAILOR_TEST_DATABASE_URL not set; skipping database lock test")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random db name: %v", err)
	}
	name := "handlers_lock_" + hex.EncodeToString(b)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.Path = "/" + name
	conn, err := db.Open(u.String())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

// legFixture is a planned leg of a cruise that the user is already enrolled
// in, with one crew assignment on the leg.
type legFixture struct {
	userID   int64
	tripID   int64
	cruiseID int64
	token    string
}

func seedLeg(t *testing.T, conn *sql.DB) legFixture {
	t.Helper()
	ctx := context.Background()
	q := sqlcdb.New(conn)
	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "crew@example.com", Name: "Crew"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Baltic", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	const token = "legs-token"
	if _, err := conn.ExecContext(ctx, "UPDATE cruises SET enroll_token = $1 WHERE id = $2", token, cruise.ID); err != nil {
		t.Fatalf("set cruise token: %v", err)
	}
	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID,
		Name:     "Leg 1",
		Status:   sqlcdb.TripStatusPlanned,
		Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if _, err := q.CreateTripEnrollmentIfAbsent(ctx, sqlcdb.CreateTripEnrollmentIfAbsentParams{
		TripID: trip.ID, UserID: user.ID, Status: enrollmentPending,
	}); err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `WITH m AS (
		INSERT INTO crew_members (full_name) VALUES ('Crew') RETURNING id
	) INSERT INTO crew_assignments (trip_id, crew_member_id) SELECT $1, id FROM m`, trip.ID); err != nil {
		t.Fatalf("seed crew assignment: %v", err)
	}
	return legFixture{userID: user.ID, tripID: trip.ID, cruiseID: cruise.ID, token: token}
}

// waitForLockWaiters blocks until at least n sessions of the test database
// are waiting on a lock.
func waitForLockWaiters(t *testing.T, conn *sql.DB, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := conn.QueryRow(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("count lock waiters: %v", err)
		}
		if waiting >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d lock waiters, have %d", n, waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// pausingQuerier holds an enrollment transaction right after it share-locks
// the cruise's trips, so the test can interleave a trip completion.
type pausingQuerier struct {
	sqlcdb.Querier
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pausingQuerier) LockCruiseTripStatuses(ctx context.Context, cruiseID int64) ([]sqlcdb.LockCruiseTripStatusesRow, error) {
	rows, err := p.Querier.LockCruiseTripStatuses(ctx, cruiseID)
	close(p.locked)
	<-p.release
	return rows, err
}

func memberCtx(userID int64) context.Context {
	return context.WithValue(context.Background(), middleware.UserCtxKey, &auth.Claims{
		UserID: userID, Email: "crew@example.com", Name: "Crew", Role: "member",
	})
}

func statusOf(err error) int {
	if se, ok := errors.AsType[huma.StatusError](err); ok {
		return se.GetStatus()
	}
	return 0
}

type result[T any] struct {
	val T
	err error
}

// TestCompleteTrip_ConcurrentLegEnrollment replays a trip completion racing a
// member's leg re-enrollment against a real database. Completion must commit
// in both orders; the enrollment either finishes first or gets a 422.
//
// In the completion-first order, a blocker transaction row-locks the leg's
// crew assignment, so completion pauses at its crew repoint while holding the
// trip lock it took up front.
func TestCompleteTrip_ConcurrentLegEnrollment(t *testing.T) {
	t.Run("enrollment locks the legs first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)

		locked := make(chan struct{})
		release := make(chan struct{})
		h := &EnrollmentHandler{q: sqlcdb.New(conn), runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
			return sqlTxRunner(conn)(ctx, func(q sqlcdb.Querier) error {
				return fn(pausingQuerier{Querier: q, locked: locked, release: release})
			})
		}}

		enrolled := make(chan result[*enrollLegsOutput], 1)
		go func() {
			out, err := h.enrollLegs(memberCtx(leg.userID), &enrollLegsInput{
				Token: leg.token, Body: dto.EnrollLegsBody{TripIDs: []int64{leg.tripID}},
			})
			enrolled <- result[*enrollLegsOutput]{out, err}
		}()
		<-locked

		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTx(context.Background(), conn, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		waitForLockWaiters(t, conn, 1)
		close(release)

		enroll := <-enrolled
		if enroll.err != nil {
			t.Fatalf("enroll legs: %v", enroll.err)
		}
		if got := enroll.val.Body.Results[0].Result; got != "already_enrolled" {
			t.Fatalf("enroll result = %q, want already_enrolled", got)
		}
		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertTripCompleted(t, conn, leg, done.val)
	})

	t.Run("completion locks the trip first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		ctx := context.Background()

		blocker, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin blocker: %v", err)
		}
		defer func() { _ = blocker.Rollback() }()
		if _, err := blocker.ExecContext(ctx,
			"SELECT id FROM crew_assignments WHERE trip_id = $1 FOR UPDATE", leg.tripID); err != nil {
			t.Fatalf("lock crew assignment: %v", err)
		}

		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTx(ctx, conn, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		waitForLockWaiters(t, conn, 1)

		h := NewEnrollmentHandler(sqlcdb.New(conn), conn)
		enrolled := make(chan result[*enrollLegsOutput], 1)
		go func() {
			out, err := h.enrollLegs(memberCtx(leg.userID), &enrollLegsInput{
				Token: leg.token, Body: dto.EnrollLegsBody{TripIDs: []int64{leg.tripID}},
			})
			enrolled <- result[*enrollLegsOutput]{out, err}
		}()
		select {
		case enroll := <-enrolled:
			t.Fatalf("enrollment finished while completion held the trip: err=%v", enroll.err)
		case <-time.After(300 * time.Millisecond):
		}
		waitForLockWaiters(t, conn, 2)
		if err := blocker.Commit(); err != nil {
			t.Fatalf("release blocker: %v", err)
		}

		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		enroll := <-enrolled
		if got := statusOf(enroll.err); got != http.StatusUnprocessableEntity {
			t.Fatalf("enroll status = %d (err=%v), want 422", got, enroll.err)
		}
		assertTripCompleted(t, conn, leg, done.val)
	})
}

func assertTripCompleted(t *testing.T, conn *sql.DB, leg legFixture, voyage sqlcdb.Voyage) {
	t.Helper()
	var trips, enrollments, onVoyage int
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM trips WHERE id = $1),
		(SELECT count(*) FROM trip_enrollments WHERE trip_id = $1),
		(SELECT count(*) FROM crew_assignments WHERE voyage_id = $2)`,
		leg.tripID, voyage.ID).Scan(&trips, &enrollments, &onVoyage); err != nil {
		t.Fatalf("read completion state: %v", err)
	}
	if trips != 0 || enrollments != 0 || onVoyage != 1 {
		t.Fatalf("trips=%d enrollments=%d voyage crew=%d, want 0 0 1", trips, enrollments, onVoyage)
	}
	if voyage.CruiseID != leg.cruiseID {
		t.Fatalf("voyage cruise = %+v, want %d", voyage.CruiseID, leg.cruiseID)
	}
}
