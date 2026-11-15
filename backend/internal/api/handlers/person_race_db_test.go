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

// signUpsOn lists a leg's sign-ups as person IDs with their status.
func signUpsOn(t *testing.T, conn *sql.DB, tripID int64) []string {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(),
		"SELECT crew_member_id, status FROM trip_enrollments WHERE trip_id = $1 ORDER BY crew_member_id", tripID)
	if err != nil {
		t.Fatalf("list sign-ups: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var person int64
		var status string
		if err := rows.Scan(&person, &status); err != nil {
			t.Fatalf("scan sign-up: %v", err)
		}
		out = append(out, fmt.Sprintf("%d %s", person, status))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("sign-up rows: %v", err)
	}
	return out
}

// acceptDuringMerge seeds a pending sign-up of the duplicate's person, or a
// cruise enrollment of its account, for a leg neither person is on and
// returns the request that accepts it.
func acceptDuringMerge(t *testing.T, f mergeFixture, how string) (leg int64, accept func(humatest.TestAPI) int) {
	t.Helper()
	ctx := context.Background()
	leg, cruise := f.newMergeLeg(t)
	var id int64
	switch how {
	case "trip sign-up":
		if err := f.conn.QueryRowContext(ctx, `INSERT INTO trip_enrollments (trip_id, crew_member_id, status)
			VALUES ($1, $2, 'pending') RETURNING id`, leg, f.duplicateID).Scan(&id); err != nil {
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

// TestAcceptedCrew_AcceptWaitsForMerge accepts a sign-up of the duplicate
// while a merge of that person, on no crew of the sign-up's leg, is
// uncommitted. The acceptance waits for the merge and adds the kept person,
// which now holds the sign-up and the account.
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

// TestCrewAssign_PersonGoneDuringAssign hand-assigns a person to a voyage,
// or signs them up for a leg, while a delete or merge of that person is
// uncommitted: the write waits and then answers 404 (voyage crew) or 422
// (leg sign-up), and nothing is added.
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
				path, want := fmt.Sprintf("/voyages/%d/crew", targetID), http.StatusNotFound
				body := map[string]any{"crew_member_id": person, "role": "zalogant"}
				if target == "trip" {
					path, want = fmt.Sprintf("/trips/%d/enrollments", targetID), http.StatusUnprocessableEntity
					body = map[string]any{"crew_member_id": person}
				}
				go func() {
					resp := api.PostCtx(userCtx(ctx), path, body)
					done <- resp.Code
				}()
				waitForLockWaiters(t, f.conn, 1)
				if err := tx.Commit(); err != nil {
					t.Fatalf("commit: %v", err)
				}
				if code := <-done; code != want {
					t.Fatalf("POST %s: got %d, want %d", path, code, want)
				}
				if got := crewOn(t, f.conn, column, targetID); len(got) != 0 {
					t.Errorf("%s crew = %v, want none", target, got)
				}
				if got := signUpsOn(t, f.conn, targetID); target == "trip" && len(got) != 0 {
					t.Errorf("leg sign-ups = %v, want none", got)
				}
			})
		}
	}
}

// TestCrewAssign_KeptPersonDuringMerge hand-assigns the kept person to a
// voyage the duplicate is on, or signs them up for a leg the duplicate is
// signed up for, while the merge waits for another leg the kept person is
// on. The insert then waits for the merge, the merge moves the duplicate's
// entry onto the same slot and waits for the insert, and the insert loses
// the deadlock. It runs again after the merge commits and answers 409,
// since the kept person now holds the slot.
func TestCrewAssign_KeptPersonDuringMerge(t *testing.T) {
	for _, target := range []string{"trip", "voyage"} {
		t.Run(target, func(t *testing.T) {
			f := seedMerge(t)
			ctx := context.Background()
			create := `INSERT INTO voyages (cruise_id, name) SELECT cruise_id, 'Fresh' FROM trips WHERE id = $1 RETURNING id`
			if target == "trip" {
				create = `INSERT INTO trips (cruise_id, name, currency) SELECT cruise_id, 'Fresh', 'PLN' FROM trips WHERE id = $1 RETURNING id`
			}
			var targetID int64
			if err := f.conn.QueryRowContext(ctx, create, f.trip1).Scan(&targetID); err != nil {
				t.Fatalf("create %s: %v", target, err)
			}
			column := target + "_id"
			path := fmt.Sprintf("/voyages/%d/crew", targetID)
			body := map[string]any{"crew_member_id": f.keptID, "role": "zalogant"}
			seed := "INSERT INTO crew_assignments (voyage_id, crew_member_id) VALUES ($1, $2)"
			if target == "trip" {
				path = fmt.Sprintf("/trips/%d/enrollments", targetID)
				body = map[string]any{"crew_member_id": f.keptID}
				seed = "INSERT INTO trip_enrollments (trip_id, crew_member_id) VALUES ($1, $2)"
			}
			if _, err := f.conn.ExecContext(ctx, seed, targetID, f.duplicateID); err != nil {
				t.Fatalf("seed duplicate: %v", err)
			}
			holder, err := f.conn.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin holder: %v", err)
			}
			defer func() { _ = holder.Rollback() }()
			if _, err := holder.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR UPDATE", f.trip1); err != nil {
				t.Fatalf("hold leg 1: %v", err)
			}

			api := f.api(t, sqlTxRunner(f.conn))
			merged := make(chan int, 1)
			go func() {
				resp := api.PostCtx(callerCtx(1, "admin"),
					fmt.Sprintf("/crew/%d/merge", f.keptID), map[string]any{"duplicate_id": f.duplicateID})
				merged <- resp.Code
			}()
			waitForLockWaiters(t, f.conn, 1)
			assigned := make(chan int, 1)
			go func() {
				resp := api.PostCtx(userCtx(ctx), path, body)
				assigned <- resp.Code
			}()
			waitForLockWaiters(t, f.conn, 2)
			if err := holder.Commit(); err != nil {
				t.Fatalf("commit holder: %v", err)
			}
			if code := <-merged; code != http.StatusOK {
				t.Fatalf("merge: got %d, want 200", code)
			}
			if code := <-assigned; code != http.StatusConflict {
				t.Fatalf("POST %s: got %d, want 409", path, code)
			}
			if target == "trip" {
				want := []string{fmt.Sprintf("%d pending", f.keptID)}
				if got := signUpsOn(t, f.conn, targetID); !slices.Equal(got, want) {
					t.Errorf("leg sign-ups = %v, want %v", got, want)
				}
				return
			}
			want := []string{fmt.Sprintf("%d accepted=false", f.keptID)}
			if got := crewOn(t, f.conn, column, targetID); !slices.Equal(got, want) {
				t.Errorf("%s crew = %v, want %v", target, got, want)
			}
		})
	}
}

// TestMergePeople_PaymentChangeWaitsForMerge records a payment on the
// duplicate's sign-up once a merge holds its locks but has not yet read the
// two sign-ups that both people have for the leg. The payment must wait for
// the merge: run between the merge's payment copy and its delete, it would
// answer 200 for a payment the merge then throws away. After the merge
// commits, the payment finds the sign-up gone.
func TestMergePeople_PaymentChangeWaitsForMerge(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	leg, _ := f.newMergeLeg(t)
	var duplicateSignUp int64
	if _, err := f.conn.ExecContext(ctx, `INSERT INTO trip_enrollments (trip_id, crew_member_id, status)
		VALUES ($1, $2, 'pending')`, leg, f.keptID); err != nil {
		t.Fatalf("seed kept sign-up: %v", err)
	}
	if err := f.conn.QueryRowContext(ctx, `INSERT INTO trip_enrollments (trip_id, crew_member_id, status)
		VALUES ($1, $2, 'pending') RETURNING id`, leg, f.duplicateID).Scan(&duplicateSignUp); err != nil {
		t.Fatalf("seed duplicate sign-up: %v", err)
	}

	merge, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin merge: %v", err)
	}
	defer func() { _ = merge.Rollback() }()
	mq := sqlcdb.New(merge)
	if _, _, err := lockMergePeople(ctx, mq, f.keptID, f.duplicateID); err != nil {
		t.Fatalf("lock merge: %v", err)
	}

	done := make(chan int, 1)
	api := acceptedCrewAPI(t, f.conn)
	go func() {
		done <- api.PutCtx(userCtx(ctx), fmt.Sprintf("/trips/%d/enrollments/%d/payment-status", leg, duplicateSignUp),
			map[string]any{"payment_status": "paid"}).Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if _, err := MergePeople(ctx, mq, f.keptID, f.duplicateID); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if err := merge.Commit(); err != nil {
		t.Fatalf("commit merge: %v", err)
	}
	if code := <-done; code != http.StatusNotFound {
		t.Fatalf("payment on the merged-away sign-up: got %d, want 404", code)
	}
	var payment string
	if err := f.conn.QueryRowContext(ctx, `SELECT payment_status FROM trip_enrollments
		WHERE trip_id = $1 AND crew_member_id = $2`, leg, f.keptID).Scan(&payment); err != nil {
		t.Fatalf("kept sign-up: %v", err)
	}
	if payment != "unpaid" {
		t.Errorf("kept sign-up payment = %s, want unpaid", payment)
	}
}

// TestAdminSignUp_WaitsForMergeOfPerson signs up a person without an account
// while a merge that gives it the account of the duplicate is uncommitted.
// That account's cruise enrollment is already placed on the leg. The sign-up
// waits for the merge, then sees the account and refuses the duplicate.
func TestAdminSignUp_WaitsForMergeOfPerson(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	leg, cruise := f.newMergeLeg(t)
	if _, err := f.conn.ExecContext(ctx, `INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status)
		VALUES ($1, $2, $3, 'pending')`, cruise, f.userID, leg); err != nil {
		t.Fatalf("seed cruise enrollment: %v", err)
	}

	merge, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin merge: %v", err)
	}
	defer func() { _ = merge.Rollback() }()
	mq := sqlcdb.New(merge)
	if _, _, err := lockMergePeople(ctx, mq, f.keptID, f.duplicateID); err != nil {
		t.Fatalf("lock merge: %v", err)
	}

	done := make(chan int, 1)
	api := personSignUpAPI(t, f.conn)
	go func() {
		done <- api.PostCtx(userCtx(ctx), fmt.Sprintf("/trips/%d/enrollments", leg),
			map[string]any{"crew_member_id": f.keptID}).Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if _, err := MergePeople(ctx, mq, f.keptID, f.duplicateID); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if err := merge.Commit(); err != nil {
		t.Fatalf("commit merge: %v", err)
	}
	if code := <-done; code != http.StatusConflict {
		t.Fatalf("admin sign-up of the merged person: got %d, want 409", code)
	}
	if got := signUpsOn(t, f.conn, leg); len(got) != 0 {
		t.Errorf("leg sign-ups = %v, want none", got)
	}
}
