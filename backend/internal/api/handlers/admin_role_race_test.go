package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// pauseAfterLockAdmins holds a role change right after it locks the admin
// set, so the test can start a second role change against it.
type pauseAfterLockAdmins struct {
	sqlcdb.Querier
	locked  chan<- struct{}
	release <-chan struct{}
}

func (p pauseAfterLockAdmins) LockAdmins(ctx context.Context) ([]int64, error) {
	ids, err := p.Querier.LockAdmins(ctx)
	close(p.locked)
	<-p.release
	return ids, err
}

// seedAdmins creates n admin accounts and returns their IDs.
func seedAdmins(t *testing.T, conn *sql.DB, n int) []int64 {
	t.Helper()
	ctx := context.Background()
	q := sqlcdb.New(conn)
	ids := make([]int64, n)
	for i := range ids {
		user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{
			Email: fmt.Sprintf("admin%d@example.com", i), Name: fmt.Sprintf("Admin %d", i),
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := q.UpdateUserRole(ctx, sqlcdb.UpdateUserRoleParams{Role: "admin", ID: user.ID}); err != nil {
			t.Fatalf("make admin: %v", err)
		}
		ids[i] = user.ID
	}
	return ids
}

func countAdmins(t *testing.T, conn *sql.DB) int64 {
	t.Helper()
	n, err := sqlcdb.New(conn).CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	return n
}

func demote(ctx context.Context, h *MembersHandler, userID int64) error {
	_, err := h.updateRole(userCtx(ctx), &updateMemberRoleInput{
		UserID: userID, Body: dto.RoleBody{Role: "member"},
	})
	return err
}

// TestUpdateRole_ConcurrentDemotionsKeepAnAdmin replays two admins demoting
// each other at once. The first demotion holds the locked admin set while
// the second starts; the second must wait for it, then see one admin left
// and refuse, so the club keeps exactly one admin.
func TestUpdateRole_ConcurrentDemotionsKeepAnAdmin(t *testing.T) {
	conn := lockTestDB(t)
	admins := seedAdmins(t, conn, 2)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	locked := make(chan struct{})
	release := make(chan struct{})
	paused := &MembersHandler{q: q, runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return sqlTxRunner(conn)(ctx, func(tq sqlcdb.Querier) error {
			return fn(pauseAfterLockAdmins{Querier: tq, locked: locked, release: release})
		})
	}}
	first := make(chan error, 1)
	go func() { first <- demote(ctx, paused, admins[0]) }()
	<-locked

	second := make(chan error, 1)
	go func() { second <- demote(ctx, NewMembersHandler(q, conn), admins[1]) }()
	waitForLockWaiters(t, conn, 1)
	close(release)

	if err := <-first; err != nil {
		t.Fatalf("first demotion: %v", err)
	}
	if err := <-second; statusOf(err) != http.StatusBadRequest {
		t.Fatalf("second demotion: got %v (status %d), want 400", err, statusOf(err))
	}
	if n := countAdmins(t, conn); n != 1 {
		t.Fatalf("admins after concurrent demotions = %d, want 1", n)
	}
}

// TestUpdateRole_ParallelDemotionsKeepAnAdmin fires two demotions of the
// only two admins from a shared start barrier, over several rounds. Each
// round exactly one demotion succeeds.
func TestUpdateRole_ParallelDemotionsKeepAnAdmin(t *testing.T) {
	conn := lockTestDB(t)
	admins := seedAdmins(t, conn, 2)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	h := NewMembersHandler(q, conn)

	for round := range 10 {
		for _, id := range admins {
			if err := q.UpdateUserRole(ctx, sqlcdb.UpdateUserRoleParams{Role: "admin", ID: id}); err != nil {
				t.Fatalf("round %d: reset admin: %v", round, err)
			}
		}
		var start, done sync.WaitGroup
		start.Add(1)
		errs := make([]error, len(admins))
		for i, id := range admins {
			done.Go(func() {
				start.Wait()
				errs[i] = demote(ctx, h, id)
			})
		}
		start.Done()
		done.Wait()

		ok := 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case statusOf(err) != http.StatusBadRequest:
				t.Fatalf("round %d: demotion: %v", round, err)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: %d demotions succeeded, want 1", round, ok)
		}
		if n := countAdmins(t, conn); n != 1 {
			t.Fatalf("round %d: admins = %d, want 1", round, n)
		}
	}
}
