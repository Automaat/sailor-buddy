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

// TestUpdateRole_DemoteLastAdminWhilePromoting replays the only admin
// promoting a member while demoting themself from another tab. The demotion
// waits for the promotion and must then see the new admin and succeed,
// rather than refuse with a stale view of the admin set.
func TestUpdateRole_DemoteLastAdminWhilePromoting(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	admin := seedAdmins(t, conn, 1)[0]
	member, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "member@example.com", Name: "Member"})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	locked := make(chan struct{})
	release := make(chan struct{})
	paused := &MembersHandler{q: q, runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return sqlTxRunner(conn)(ctx, func(tq sqlcdb.Querier) error {
			return fn(pauseAfterLockAdmins{Querier: tq, locked: locked, release: release})
		})
	}}
	promoted := make(chan error, 1)
	go func() {
		_, err := paused.updateRole(userCtx(ctx), &updateMemberRoleInput{
			UserID: member.ID, Body: dto.RoleBody{Role: "admin"},
		})
		promoted <- err
	}()
	<-locked

	demoted := make(chan error, 1)
	go func() { demoted <- demote(ctx, NewMembersHandler(q, conn), admin) }()
	waitForLockWaiters(t, conn, 1)
	close(release)

	if err := <-promoted; err != nil {
		t.Fatalf("promote member: %v", err)
	}
	if err := <-demoted; err != nil {
		t.Fatalf("demote admin: %v", err)
	}
	ids, err := q.LockAdmins(ctx)
	if err != nil {
		t.Fatalf("list admins: %v", err)
	}
	if len(ids) != 1 || ids[0] != member.ID {
		t.Fatalf("admins = %v, want only %d", ids, member.ID)
	}
}

// TestUpdateRole_DoesNotDeadlockWithAccountLink replays a role change of a
// user whose row another transaction already holds, the way linkAccount
// does before it writes decided_by, a foreign key to the acting admin. The
// admin-set lock must not block that foreign-key check, or the two would
// deadlock. The handler runs without deadlock retry, so a deadlock fails.
func TestUpdateRole_DoesNotDeadlockWithAccountLink(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	admin := seedAdmins(t, conn, 1)[0]
	member, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "member@example.com", Name: "Member"})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	link, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin link: %v", err)
	}
	defer func() { _ = link.Rollback() }()
	if _, err := sqlcdb.New(link).LockUserForUpdate(ctx, member.ID); err != nil {
		t.Fatalf("lock member: %v", err)
	}

	h := &MembersHandler{q: q, runTx: sqlTxRunner(conn)}
	promoted := make(chan error, 1)
	go func() {
		_, err := h.updateRole(userCtx(ctx), &updateMemberRoleInput{
			UserID: member.ID, Body: dto.RoleBody{Role: "admin"},
		})
		promoted <- err
	}()
	waitForLockWaiters(t, conn, 1)

	if _, err := link.ExecContext(ctx,
		"INSERT INTO yachts (name, created_by) VALUES ('Link', $1)", admin); err != nil {
		t.Fatalf("write row referencing the admin: %v", err)
	}
	if err := link.Commit(); err != nil {
		t.Fatalf("commit link: %v", err)
	}
	if err := <-promoted; err != nil {
		t.Fatalf("promote member: %v", err)
	}
}
