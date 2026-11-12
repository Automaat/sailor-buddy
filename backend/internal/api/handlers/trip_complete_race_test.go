package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
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

// assertDetachedVoyage checks the trip became a voyage whose cruise link was
// cleared by the cruise delete, and that the cruise is gone.
func assertDetachedVoyage(t *testing.T, conn *sql.DB, leg legFixture, voyage sqlcdb.Voyage) {
	t.Helper()
	var trips, cruises int
	var voyageCruise sql.NullInt64
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM trips WHERE id = $1),
		(SELECT count(*) FROM cruises WHERE id = $2),
		(SELECT cruise_id FROM voyages WHERE id = $3)`,
		leg.tripID, leg.cruiseID, voyage.ID).Scan(&trips, &cruises, &voyageCruise); err != nil {
		t.Fatalf("read final state: %v", err)
	}
	if trips != 0 || cruises != 0 || voyageCruise.Valid {
		t.Fatalf("trips=%d cruises=%d voyage cruise=%+v, want 0 0 NULL", trips, cruises, voyageCruise)
	}
}

// TestCompleteTrip_ConcurrentCruiseDelete replays a trip completion racing the
// delete of the trip's cruise. Both must commit in either order, without a
// deadlock abort on either side.
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
		if err := <-deleted; err != nil {
			t.Fatalf("delete cruise: %v", err)
		}
		assertDetachedVoyage(t, conn, leg, done.val)
	})

	t.Run("delete locks the cruise first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		ctx := context.Background()

		deleter, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin delete: %v", err)
		}
		defer func() { _ = deleter.Rollback() }()
		if err := sqlcdb.New(deleter).DeleteCruise(ctx, leg.cruiseID); err != nil {
			t.Fatalf("delete cruise: %v", err)
		}

		completed := make(chan result[sqlcdb.Voyage], 1)
		go func() {
			v, err := completeTripTx(ctx, conn, leg.tripID, dto.CompleteTripBody{})
			completed <- result[sqlcdb.Voyage]{v, err}
		}()
		waitForLockWaiters(t, conn, 1)
		select {
		case done := <-completed:
			t.Fatalf("completion finished while the delete held the cruise: err=%v", done.err)
		case <-time.After(300 * time.Millisecond):
		}
		if err := deleter.Commit(); err != nil {
			t.Fatalf("commit delete: %v", err)
		}

		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertDetachedVoyage(t, conn, leg, done.val)
	})
}

// pauseAfterCruiseLock holds the first completion attempt right after it
// locks the trip's cruise, so the test can move the trip to another cruise.
type pauseAfterCruiseLock struct {
	sqlcdb.Querier
	once    *sync.Once
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterCruiseLock) LockTripCruise(ctx context.Context, tripID int64) (int64, error) {
	id, err := p.Querier.LockTripCruise(ctx, tripID)
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
	locked := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan result[sqlcdb.Voyage], 1)
	go func() {
		v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
			return pauseAfterCruiseLock{Querier: q, once: &once, locked: locked, release: release}
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
	if want := (types.NullInt64{Int64: other.ID, Valid: true}); done.val.CruiseID != want {
		t.Fatalf("voyage cruise = %+v, want %+v", done.val.CruiseID, want)
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
