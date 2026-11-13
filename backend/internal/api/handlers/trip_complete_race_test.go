package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// pauseAfterTripLock holds a completion transaction right after it locks the
// trip, so the test can interleave a cruise delete.
type pauseAfterTripLock struct {
	sqlcdb.Querier
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterTripLock) GetTripForUpdate(ctx context.Context, id int64) (sqlcdb.Trip, error) {
	trip, err := p.Querier.GetTripForUpdate(ctx, id)
	close(p.locked)
	<-p.release
	return trip, err
}

// assertVoyageKeepsCruise checks the trip became a voyage still linked to its
// cruise, which the refused delete left in place.
func assertVoyageKeepsCruise(t *testing.T, conn *sql.DB, leg legFixture, voyage sqlcdb.Voyage) {
	t.Helper()
	var trips, cruises int
	var voyageCruise int64
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM trips WHERE id = $1),
		(SELECT count(*) FROM cruises WHERE id = $2),
		(SELECT cruise_id FROM voyages WHERE id = $3)`,
		leg.tripID, leg.cruiseID, voyage.ID).Scan(&trips, &cruises, &voyageCruise); err != nil {
		t.Fatalf("read final state: %v", err)
	}
	if trips != 0 || cruises != 1 || voyageCruise != leg.cruiseID {
		t.Fatalf("trips=%d cruises=%d voyage cruise=%d, want 0 1 %d", trips, cruises, voyageCruise, leg.cruiseID)
	}
}

// assertDeleteRefused checks the cruise delete failed on a leg foreign key, the
// clean refusal the API maps to 409, rather than a deadlock abort.
func assertDeleteRefused(t *testing.T, err error) {
	t.Helper()
	if !isForeignKeyViolation(err, "trips_cruise_id_fkey") && !isForeignKeyViolation(err, "voyages_cruise_id_fkey") {
		t.Fatalf("delete cruise: got %v, want a trips/voyages foreign-key violation", err)
	}
}

// TestCompleteTrip_ConcurrentCruiseDelete replays a trip completion racing the
// delete of the trip's cruise. In either order completion commits and the
// delete is refused cleanly, without a deadlock abort on either side.
func TestCompleteTrip_ConcurrentCruiseDelete(t *testing.T) {
	t.Run("completion locks first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		ctx := context.Background()

		locked := make(chan struct{})
		release := make(chan struct{})
		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
				return pauseAfterTripLock{Querier: q, locked: locked, release: release}
			}, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		<-locked

		deleted := make(chan error, 1)
		go func() { deleted <- sqlcdb.New(conn).DeleteCruise(ctx, leg.cruiseID) }()
		waitForLockWaiters(t, conn, 1)
		close(release)

		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertDeleteRefused(t, <-deleted)
		assertVoyageKeepsCruise(t, conn, leg, done.val)
	})

	t.Run("delete locks the cruise first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		ctx := context.Background()

		// The delete's ON DELETE CASCADE into cruise_enrollments stalls on this
		// row lock while the delete already holds the cruise row, so
		// completion starts against a delete that is still in flight.
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO cruise_enrollments (cruise_id, user_id) VALUES ($1, $2)", leg.cruiseID, leg.userID); err != nil {
			t.Fatalf("seed cruise enrollment: %v", err)
		}
		holder, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin holder: %v", err)
		}
		defer func() { _ = holder.Rollback() }()
		if _, err := holder.ExecContext(ctx,
			"SELECT id FROM cruise_enrollments WHERE cruise_id = $1 FOR UPDATE", leg.cruiseID); err != nil {
			t.Fatalf("hold cruise enrollment: %v", err)
		}

		deleted := make(chan error, 1)
		go func() { deleted <- sqlcdb.New(conn).DeleteCruise(ctx, leg.cruiseID) }()
		waitForLockWaiters(t, conn, 1)

		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTx(ctx, conn, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		waitForLockWaiters(t, conn, 2)
		if err := holder.Commit(); err != nil {
			t.Fatalf("release cruise enrollment: %v", err)
		}

		assertDeleteRefused(t, <-deleted)
		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertVoyageKeepsCruise(t, conn, leg, done.val)
	})
}

// pauseAfterCruiseLock holds the first completion attempt right after it
// locks the trip's cruise, so the test can move the trip to another cruise.
type pauseAfterCruiseLock struct {
	sqlcdb.Querier
	calls   *atomic.Int32
	once    *sync.Once
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterCruiseLock) LockTripCruise(ctx context.Context, tripID int64) (int64, error) {
	id, err := p.Querier.LockTripCruise(ctx, tripID)
	p.calls.Add(1)
	p.once.Do(func() {
		close(p.locked)
		<-p.release
	})
	return id, err
}

// TestCompleteTrip_TripMovedToAnotherCruise moves the trip to another cruise
// between the completion's cruise lock and trip lock. Completion must retry
// and attach the voyage to the trip's new cruise.
func TestCompleteTrip_TripMovedToAnotherCruise(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	ctx := context.Background()

	other, err := sqlcdb.New(conn).CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "North Sea", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create other cruise: %v", err)
	}

	var once sync.Once
	var lockCalls atomic.Int32
	locked := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan result[sqlcdb.Voyage], 1)
	go func() {
		v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
			return pauseAfterCruiseLock{Querier: q, calls: &lockCalls, once: &once, locked: locked, release: release}
		}, leg.tripID, dto.CompleteTripBody{})
		completed <- result[sqlcdb.Voyage]{v, err}
	}()
	<-locked
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET cruise_id = $1 WHERE id = $2", other.ID, leg.tripID); err != nil {
		t.Fatalf("move trip: %v", err)
	}
	close(release)

	done := <-completed
	if done.err != nil {
		t.Fatalf("complete trip: %v", done.err)
	}
	if done.val.CruiseID != other.ID {
		t.Fatalf("voyage cruise = %d, want %d", done.val.CruiseID, other.ID)
	}
	if got := lockCalls.Load(); got != 2 {
		t.Fatalf("cruise lock attempts = %d, want 2 (one retry after the move)", got)
	}
}

// TestEnroll_TripCompletedDuringEnroll replays a member enrolling through a
// trip link while an admin completes that trip. The enrollment insert waits
// on the completion's trip lock and must then fail with a 409, not a 500.
func TestEnroll_TripCompletedDuringEnroll(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	const tripToken = "trip-token"
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET enroll_token = $1 WHERE id = $2", tripToken, leg.tripID); err != nil {
		t.Fatalf("set trip token: %v", err)
	}
	member, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "late@example.com", Name: "Late"})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

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

	h := NewEnrollmentHandler(q, conn)
	enrolled := make(chan result[*enrollmentOutput], 1)
	go func() {
		out, err := h.enroll(memberCtx(member.ID), &enrollInput{Token: tripToken})
		enrolled <- result[*enrollmentOutput]{out, err}
	}()
	waitForLockWaiters(t, conn, 2)
	if err := blocker.Commit(); err != nil {
		t.Fatalf("release blocker: %v", err)
	}

	done := <-completed
	if done.err != nil {
		t.Fatalf("complete trip: %v", done.err)
	}
	enroll := <-enrolled
	if got := statusOf(enroll.err); got != http.StatusConflict {
		t.Fatalf("enroll status = %d (err=%v), want 409", got, enroll.err)
	}
	assertTripCompleted(t, conn, leg, done.val)
}

// seedCaptainPerson gives the leg a captain person without an account.
func seedCaptainPerson(t *testing.T, conn *sql.DB, leg legFixture) int64 {
	t.Helper()
	var person int64
	if err := conn.QueryRow(`INSERT INTO crew_members (full_name) VALUES ('Kpt. Bez Konta') RETURNING id`).Scan(&person); err != nil {
		t.Fatalf("seed captain person: %v", err)
	}
	if _, err := conn.Exec(`UPDATE trips SET captain_person_id = $1, captain_name = 'Kpt. Bez Konta' WHERE id = $2`,
		person, leg.tripID); err != nil {
		t.Fatalf("assign captain person: %v", err)
	}
	return person
}

// TestCompleteTrip_ConcurrentCaptainDelete replays a trip completion racing
// the delete of the trip's captain person. In either order both commit,
// without a deadlock abort on either side. In the second case a share lock
// on the trip stalls the delete's ON DELETE SET NULL while it already holds
// the person row, so completion starts against a delete still in flight.
func TestCompleteTrip_ConcurrentCaptainDelete(t *testing.T) {
	t.Run("completion locks first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		person := seedCaptainPerson(t, conn, leg)
		ctx := context.Background()

		locked := make(chan struct{})
		release := make(chan struct{})
		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
				return pauseAfterTripLock{Querier: q, locked: locked, release: release}
			}, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		<-locked

		deleted := make(chan result[int64], 1)
		go func() {
			n, err := sqlcdb.New(conn).DeleteUnlinkedCrewMember(ctx, person)
			deleted <- result[int64]{n, err}
		}()
		waitForLockWaiters(t, conn, 1)
		close(release)

		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		if done.val.CaptainPersonID.Int64 != person {
			t.Fatalf("voyage captain = %v, want person %d", done.val.CaptainPersonID, person)
		}
		if del := <-deleted; del.err != nil || del.val != 1 {
			t.Fatalf("delete captain person = (%d, %v), want (1, nil)", del.val, del.err)
		}
		var captain sql.NullInt64
		var name string
		if err := conn.QueryRow(`SELECT captain_person_id, captain_name FROM voyages WHERE id = $1`, done.val.ID).
			Scan(&captain, &name); err != nil {
			t.Fatalf("read voyage: %v", err)
		}
		if captain.Valid || name != "Kpt. Bez Konta" {
			t.Fatalf("voyage captain after delete = (%v, %q), want (NULL, the kept name)", captain, name)
		}
	})

	t.Run("delete locks the person first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		person := seedCaptainPerson(t, conn, leg)
		ctx := context.Background()

		holder, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin holder: %v", err)
		}
		defer func() { _ = holder.Rollback() }()
		if _, err := holder.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR SHARE", leg.tripID); err != nil {
			t.Fatalf("hold trip: %v", err)
		}

		deleted := make(chan result[int64], 1)
		go func() {
			n, err := sqlcdb.New(conn).DeleteUnlinkedCrewMember(ctx, person)
			deleted <- result[int64]{n, err}
		}()
		waitForLockWaiters(t, conn, 1)

		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTx(ctx, conn, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		waitForLockWaiters(t, conn, 2)
		if err := holder.Commit(); err != nil {
			t.Fatalf("release trip: %v", err)
		}

		if del := <-deleted; del.err != nil || del.val != 1 {
			t.Fatalf("delete captain person = (%d, %v), want (1, nil)", del.val, del.err)
		}
		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		if done.val.CaptainPersonID.Valid || done.val.CaptainName.String != "Kpt. Bez Konta" {
			t.Fatalf("voyage captain = (%v, %q), want (NULL, the kept name)", done.val.CaptainPersonID, done.val.CaptainName.String)
		}
	})
}

// pauseAfterCaptainLock holds the first completion attempt right after it
// locks the trip's captain person, so the test can change the captain.
type pauseAfterCaptainLock struct {
	sqlcdb.Querier
	calls   *atomic.Int32
	once    *sync.Once
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterCaptainLock) LockTripCaptain(ctx context.Context, tripID int64) (int64, error) {
	id, err := p.Querier.LockTripCaptain(ctx, tripID)
	p.calls.Add(1)
	p.once.Do(func() {
		close(p.locked)
		<-p.release
	})
	return id, err
}

// TestCompleteTrip_CaptainChanged changes the trip's captain person between
// the completion's captain lock and trip lock. Completion must retry and give
// the voyage the new captain.
func TestCompleteTrip_CaptainChanged(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	seedCaptainPerson(t, conn, leg)
	ctx := context.Background()

	var next int64
	if err := conn.QueryRow(`INSERT INTO crew_members (full_name) VALUES ('Kpt. Nowy') RETURNING id`).Scan(&next); err != nil {
		t.Fatalf("seed new captain: %v", err)
	}

	var once sync.Once
	var lockCalls atomic.Int32
	locked := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan result[sqlcdb.Voyage], 1)
	go func() {
		v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
			return pauseAfterCaptainLock{Querier: q, calls: &lockCalls, once: &once, locked: locked, release: release}
		}, leg.tripID, dto.CompleteTripBody{})
		completed <- result[sqlcdb.Voyage]{v, err}
	}()
	<-locked
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET captain_person_id = $1 WHERE id = $2", next, leg.tripID); err != nil {
		t.Fatalf("change captain: %v", err)
	}
	close(release)

	done := <-completed
	if done.err != nil {
		t.Fatalf("complete trip: %v", done.err)
	}
	if done.val.CaptainPersonID.Int64 != next {
		t.Fatalf("voyage captain = %v, want %d", done.val.CaptainPersonID, next)
	}
	if got := lockCalls.Load(); got != 2 {
		t.Fatalf("captain lock attempts = %d, want 2 (one retry after the change)", got)
	}
}
