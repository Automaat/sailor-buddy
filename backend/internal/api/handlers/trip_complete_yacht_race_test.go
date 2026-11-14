package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// seedLegYacht gives the leg a yacht and returns the yacht's ID.
func seedLegYacht(t *testing.T, conn *sql.DB, leg legFixture, name string) int64 {
	t.Helper()
	yacht := seedYacht(t, conn, name)
	if _, err := conn.Exec(`UPDATE trips SET yacht_id = $1 WHERE id = $2`, yacht, leg.tripID); err != nil {
		t.Fatalf("assign yacht: %v", err)
	}
	return yacht
}

func seedYacht(t *testing.T, conn *sql.DB, name string) int64 {
	t.Helper()
	var yacht int64
	if err := conn.QueryRow(`INSERT INTO yachts (name) VALUES ($1) RETURNING id`, name).Scan(&yacht); err != nil {
		t.Fatalf("seed yacht: %v", err)
	}
	return yacht
}

// assertYachtDeleteRefused checks the yacht delete failed on a leg or voyage
// foreign key, the clean refusal the API maps to 409, rather than a deadlock
// abort.
func assertYachtDeleteRefused(t *testing.T, err error) {
	t.Helper()
	if !isForeignKeyViolation(err, "trips_yacht_id_fkey") && !isForeignKeyViolation(err, "voyages_yacht_id_fkey") {
		t.Fatalf("delete yacht: got %v, want a trips/voyages foreign-key violation", err)
	}
}

// assertVoyageKeepsYacht checks the trip became a voyage still on its yacht,
// which the refused delete left in place.
func assertVoyageKeepsYacht(t *testing.T, conn *sql.DB, leg legFixture, voyage sqlcdb.Voyage, yacht int64) {
	t.Helper()
	assertTripCompleted(t, conn, leg, voyage)
	var yachts int
	var voyageYacht sql.NullInt64
	if err := conn.QueryRow(`SELECT
		(SELECT count(*) FROM yachts WHERE id = $1),
		(SELECT yacht_id FROM voyages WHERE id = $2)`,
		yacht, voyage.ID).Scan(&yachts, &voyageYacht); err != nil {
		t.Fatalf("read final state: %v", err)
	}
	if yachts != 1 || voyageYacht.Int64 != yacht {
		t.Fatalf("yachts=%d voyage yacht=%v, want 1 %d", yachts, voyageYacht, yacht)
	}
}

// TestCompleteTrip_ConcurrentYachtDelete replays a trip completion racing the
// delete of the trip's yacht. In either order completion commits and the
// delete is refused cleanly, without a deadlock abort on either side. In the
// second case a row lock on the trip stalls the delete's foreign-key check
// while it already holds the yacht row, so completion starts against a
// delete still in flight.
func TestCompleteTrip_ConcurrentYachtDelete(t *testing.T) {
	t.Run("completion locks first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		yacht := seedLegYacht(t, conn, leg, "Ilmatar")
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
		go func() { deleted <- sqlcdb.New(conn).DeleteYacht(ctx, yacht) }()
		waitForLockWaiters(t, conn, 1)
		close(release)

		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertYachtDeleteRefused(t, <-deleted)
		assertVoyageKeepsYacht(t, conn, leg, done.val, yacht)
	})

	t.Run("delete locks the yacht first", func(t *testing.T) {
		conn := lockTestDB(t)
		leg := seedLeg(t, conn)
		yacht := seedLegYacht(t, conn, leg, "Ilmatar")
		ctx := context.Background()

		holder, err := conn.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin holder: %v", err)
		}
		defer func() { _ = holder.Rollback() }()
		if _, err := holder.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR UPDATE", leg.tripID); err != nil {
			t.Fatalf("hold trip: %v", err)
		}

		deleted := make(chan error, 1)
		go func() { deleted <- sqlcdb.New(conn).DeleteYacht(ctx, yacht) }()
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

		assertYachtDeleteRefused(t, <-deleted)
		done := <-completed
		if done.err != nil {
			t.Fatalf("complete trip: %v", done.err)
		}
		assertVoyageKeepsYacht(t, conn, leg, done.val, yacht)
	})
}

// pauseAfterYachtLock holds the first completion attempt right after it
// locks the trip's yacht, so the test can change the yacht.
type pauseAfterYachtLock struct {
	sqlcdb.Querier
	calls   *atomic.Int32
	once    *sync.Once
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterYachtLock) LockTripYacht(ctx context.Context, tripID int64) (int64, error) {
	id, err := p.Querier.LockTripYacht(ctx, tripID)
	p.calls.Add(1)
	p.once.Do(func() {
		close(p.locked)
		<-p.release
	})
	return id, err
}

// TestCompleteTrip_YachtChanged changes the trip's yacht between the
// completion's yacht lock and trip lock. Completion must retry and give the
// voyage the new yacht.
func TestCompleteTrip_YachtChanged(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	seedLegYacht(t, conn, leg, "Ilmatar")
	next := seedYacht(t, conn, "Zawisza")
	ctx := context.Background()

	var once sync.Once
	var lockCalls atomic.Int32
	locked := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan result[sqlcdb.Voyage], 1)
	go func() {
		v, err := completeTripTxWith(ctx, conn, func(q sqlcdb.Querier) sqlcdb.Querier {
			return pauseAfterYachtLock{Querier: q, calls: &lockCalls, once: &once, locked: locked, release: release}
		}, leg.tripID, dto.CompleteTripBody{})
		completed <- result[sqlcdb.Voyage]{v, err}
	}()
	<-locked
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET yacht_id = $1 WHERE id = $2", next, leg.tripID); err != nil {
		t.Fatalf("change yacht: %v", err)
	}
	close(release)

	done := <-completed
	if done.err != nil {
		t.Fatalf("complete trip: %v", done.err)
	}
	if done.val.YachtID.Int64 != next {
		t.Fatalf("voyage yacht = %v, want %d", done.val.YachtID, next)
	}
	if got := lockCalls.Load(); got != 2 {
		t.Fatalf("yacht lock attempts = %d, want 2 (one retry after the change)", got)
	}
}

// TestYachtHandler_DeleteUsedYacht deletes a yacht a leg uses, then the same
// yacht once the leg is a voyage, against a real database. Both are refused
// with 409; an unused yacht still deletes.
func TestYachtHandler_DeleteUsedYacht(t *testing.T) {
	conn := lockTestDB(t)
	leg := seedLeg(t, conn)
	yacht := seedLegYacht(t, conn, leg, "Ilmatar")
	unused := seedYacht(t, conn, "Zawisza")
	ctx := userCtx(context.Background())
	_, api := humatest.New(t)
	RegisterYachtRoutes(api, sqlcdb.New(conn))

	if resp := api.DeleteCtx(ctx, fmt.Sprintf("/yachts/%d", yacht)); resp.Code != http.StatusConflict {
		t.Fatalf("delete yacht used by a leg: got %d, want 409; body=%s", resp.Code, resp.Body)
	}

	if _, err := completeTripTx(context.Background(), conn, leg.tripID, dto.CompleteTripBody{}); err != nil {
		t.Fatalf("complete trip: %v", err)
	}
	if resp := api.DeleteCtx(ctx, fmt.Sprintf("/yachts/%d", yacht)); resp.Code != http.StatusConflict {
		t.Fatalf("delete yacht used by a voyage: got %d, want 409; body=%s", resp.Code, resp.Body)
	}

	if resp := api.DeleteCtx(ctx, fmt.Sprintf("/yachts/%d", unused)); resp.Code != http.StatusNoContent {
		t.Fatalf("delete unused yacht: got %d, want 204; body=%s", resp.Code, resp.Body)
	}
	var left int
	if err := conn.QueryRow(`SELECT count(*) FROM yachts`).Scan(&left); err != nil {
		t.Fatalf("count yachts: %v", err)
	}
	if left != 1 {
		t.Fatalf("yachts left = %d, want 1 (the used one)", left)
	}
}
