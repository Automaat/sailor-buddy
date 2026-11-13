package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// newMergeLeg adds a leg to the fixture's cruise that neither person is on.
func (f mergeFixture) newMergeLeg(t *testing.T) (tripID, cruiseID int64) {
	t.Helper()
	if err := f.conn.QueryRowContext(context.Background(), `INSERT INTO trips (cruise_id, name, currency)
		SELECT cruise_id, 'Leg 3', 'PLN' FROM trips WHERE id = $1 RETURNING id, cruise_id`, f.trip1).
		Scan(&tripID, &cruiseID); err != nil {
		t.Fatalf("create leg 3: %v", err)
	}
	return tripID, cruiseID
}

// crewOn lists the person IDs on a leg's or voyage's crew list with their
// added-by-acceptance flag.
func crewOn(t *testing.T, conn *sql.DB, column string, id int64) []string {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(),
		"SELECT crew_member_id, added_by_acceptance FROM crew_assignments WHERE "+column+" = $1 ORDER BY crew_member_id", id)
	if err != nil {
		t.Fatalf("list crew: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var person int64
		var accepted bool
		if err := rows.Scan(&person, &accepted); err != nil {
			t.Fatalf("scan crew: %v", err)
		}
		out = append(out, fmt.Sprintf("%d accepted=%t", person, accepted))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("crew rows: %v", err)
	}
	return out
}

// acceptDuringMerge seeds a pending sign-up of the duplicate's account for a
// leg neither person is on and returns the request that accepts it.
func acceptDuringMerge(t *testing.T, f mergeFixture, how string) (leg int64, accept func(humatest.TestAPI) int) {
	t.Helper()
	ctx := context.Background()
	leg, cruise := f.newMergeLeg(t)
	var id int64
	switch how {
	case "trip sign-up":
		if err := f.conn.QueryRowContext(ctx, `INSERT INTO trip_enrollments (trip_id, user_id, status)
			VALUES ($1, $2, 'pending') RETURNING id`, leg, f.userID).Scan(&id); err != nil {
			t.Fatalf("seed trip sign-up: %v", err)
		}
		return leg, func(api humatest.TestAPI) int { return setTripStatus(t, api, leg, id, "accepted") }
	case "cruise enrollment":
		if err := f.conn.QueryRowContext(ctx, `INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status)
			VALUES ($1, $2, $3, 'pending') RETURNING id`, cruise, f.userID, leg).Scan(&id); err != nil {
			t.Fatalf("seed cruise enrollment: %v", err)
		}
		return leg, func(api humatest.TestAPI) int {
			return api.PutCtx(userCtx(ctx), fmt.Sprintf("/cruises/%d/enrollments/%d/status", cruise, id),
				map[string]any{"status": "accepted"}).Code
		}
	}
	t.Fatalf("unknown sign-up path %q", how)
	return 0, nil
}

// TestAcceptedCrew_AcceptWaitsForMerge accepts a sign-up of the duplicate's
// account while a merge of that person, on no leg of the sign-up, is
// uncommitted. The acceptance waits for the merge and adds the kept person,
// which now holds the account.
func TestAcceptedCrew_AcceptWaitsForMerge(t *testing.T) {
	for _, how := range []string{"trip sign-up", "cruise enrollment"} {
		t.Run(how, func(t *testing.T) {
			f := seedMerge(t)
			ctx := context.Background()
			leg, accept := acceptDuringMerge(t, f, how)
			merge, err := f.conn.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin merge: %v", err)
			}
			defer func() { _ = merge.Rollback() }()
			if _, err := MergePeople(ctx, sqlcdb.New(merge), f.keptID, f.duplicateID); err != nil {
				t.Fatalf("merge: %v", err)
			}

			done := make(chan int, 1)
			api := acceptedCrewAPI(t, f.conn)
			go func() { done <- accept(api) }()
			waitForLockWaiters(t, f.conn, 1)
			if err := merge.Commit(); err != nil {
				t.Fatalf("commit merge: %v", err)
			}
			if code := <-done; code != http.StatusNoContent {
				t.Fatalf("accept: got %d, want 204", code)
			}
			want := []string{fmt.Sprintf("%d accepted=true", f.keptID)}
			if got := crewOn(t, f.conn, "trip_id", leg); !slices.Equal(got, want) {
				t.Errorf("leg crew = %v, want %v", got, want)
			}
		})
	}
}

// TestAcceptedCrew_AcceptLosesDeadlockToMerge lets a merge hold both people
// while the acceptance holds the leg and waits for the account's person,
// then makes the merge wait for that leg. The acceptance loses the deadlock,
// runs again after the merge commits and adds the kept person.
func TestAcceptedCrew_AcceptLosesDeadlockToMerge(t *testing.T) {
	for _, how := range []string{"trip sign-up", "cruise enrollment"} {
		t.Run(how, func(t *testing.T) {
			f := seedMerge(t)
			ctx := context.Background()
			leg, accept := acceptDuringMerge(t, f, how)
			merge, err := f.conn.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin merge: %v", err)
			}
			defer func() { _ = merge.Rollback() }()
			for _, step := range []string{
				"SET LOCAL deadlock_timeout = '10s'",
				fmt.Sprintf("SELECT id FROM crew_members WHERE id IN (%d, %d) ORDER BY id FOR UPDATE", f.keptID, f.duplicateID),
			} {
				if _, err := merge.ExecContext(ctx, step); err != nil {
					t.Fatalf("%s: %v", step, err)
				}
			}

			done := make(chan int, 1)
			api := acceptedCrewAPI(t, f.conn)
			go func() { done <- accept(api) }()
			waitForLockWaiters(t, f.conn, 1)
			if _, err := merge.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR SHARE", leg); err != nil {
				t.Fatalf("merge locks the leg: %v", err)
			}
			if _, err := MergePeople(ctx, sqlcdb.New(merge), f.keptID, f.duplicateID); err != nil {
				t.Fatalf("merge: %v", err)
			}
			waitForLockWaiters(t, f.conn, 1)
			if err := merge.Commit(); err != nil {
				t.Fatalf("commit merge: %v", err)
			}
			if code := <-done; code != http.StatusNoContent {
				t.Fatalf("accept: got %d, want 204", code)
			}
			want := []string{fmt.Sprintf("%d accepted=true", f.keptID)}
			if got := crewOn(t, f.conn, "trip_id", leg); !slices.Equal(got, want) {
				t.Errorf("leg crew = %v, want %v", got, want)
			}
		})
	}
}

// TestCrewAssign_PersonGoneDuringAssign hand-assigns a person while a
// delete or merge of that person is uncommitted: the assignment waits and
// then answers 404, and the crew list stays empty.
func TestCrewAssign_PersonGoneDuringAssign(t *testing.T) {
	for _, target := range []string{"trip", "voyage"} {
		for _, gone := range []string{"deleted", "merged"} {
			t.Run(target+" "+gone, func(t *testing.T) {
				f := seedMerge(t)
				ctx := context.Background()
				var targetID int64
				column := target + "_id"
				create := `INSERT INTO voyages (cruise_id, name) SELECT cruise_id, 'Fresh' FROM trips WHERE id = $1 RETURNING id`
				if target == "trip" {
					create = `INSERT INTO trips (cruise_id, name, currency) SELECT cruise_id, 'Fresh', 'PLN' FROM trips WHERE id = $1 RETURNING id`
				}
				if err := f.conn.QueryRowContext(ctx, create, f.trip1).Scan(&targetID); err != nil {
					t.Fatalf("create %s: %v", target, err)
				}
				tx, err := f.conn.BeginTx(ctx, nil)
				if err != nil {
					t.Fatalf("begin: %v", err)
				}
				defer func() { _ = tx.Rollback() }()
				person := f.duplicateID
				if gone == "deleted" {
					person = f.keptID
					if n, err := sqlcdb.New(tx).DeleteUnlinkedCrewMember(ctx, person); err != nil || n != 1 {
						t.Fatalf("delete person: %d rows, %v", n, err)
					}
				} else if _, err := MergePeople(ctx, sqlcdb.New(tx), f.keptID, f.duplicateID); err != nil {
					t.Fatalf("merge: %v", err)
				}

				done := make(chan int, 1)
				api := f.api(t, sqlTxRunner(f.conn))
				go func() {
					resp := api.PostCtx(userCtx(ctx), fmt.Sprintf("/%ss/%d/crew", target, targetID),
						map[string]any{"crew_member_id": person, "role": "zalogant"})
					done <- resp.Code
				}()
				waitForLockWaiters(t, f.conn, 1)
				if err := tx.Commit(); err != nil {
					t.Fatalf("commit: %v", err)
				}
				if code := <-done; code != http.StatusNotFound {
					t.Fatalf("assign: got %d, want 404", code)
				}
				if got := crewOn(t, f.conn, column, targetID); len(got) != 0 {
					t.Errorf("%s crew = %v, want none", target, got)
				}
			})
		}
	}
}
