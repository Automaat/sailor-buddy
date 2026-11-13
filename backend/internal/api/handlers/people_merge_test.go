package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCrewMerge_MembersForbidden(t *testing.T) {
	api := crewTestAPI(t, &mockQuerier{})
	if resp := api.PostCtx(memberCtx(2), "/crew/1/merge", map[string]any{"duplicate_id": 2}); resp.Code != http.StatusForbidden {
		t.Errorf("merge: got %d, want 403; body=%s", resp.Code, resp.Body)
	}
	if resp := api.GetCtx(memberCtx(2), "/crew/1/merge-preview?duplicate_id=2"); resp.Code != http.StatusForbidden {
		t.Errorf("preview: got %d, want 403; body=%s", resp.Code, resp.Body)
	}
}

func TestCrewMerge_Refusals(t *testing.T) {
	linked := func(id, userID int64) sqlcdb.CrewMember {
		return sqlcdb.CrewMember{ID: id, FullName: "P" + strconv.FormatInt(id, 10), UserID: types.NullInt64{Int64: userID, Valid: true}}
	}
	cases := map[string]struct {
		people    []sqlcdb.CrewMember
		duplicate int64
		want      int
	}{
		"same person":          {people: []sqlcdb.CrewMember{linked(1, 5)}, duplicate: 1, want: http.StatusUnprocessableEntity},
		"missing duplicate":    {people: []sqlcdb.CrewMember{{ID: 1, FullName: "A"}}, duplicate: 2, want: http.StatusNotFound},
		"two accounts":         {people: []sqlcdb.CrewMember{linked(1, 5), linked(2, 6)}, duplicate: 2, want: http.StatusConflict},
		"too many other names": {people: []sqlcdb.CrewMember{{ID: 1, FullName: "A", OtherNames: manyNames("a", 15)}, {ID: 2, FullName: "B", OtherNames: manyNames("b", 6)}}, duplicate: 2, want: http.StatusUnprocessableEntity},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			byID := func(id int64) (sqlcdb.CrewMember, error) {
				for _, p := range c.people {
					if p.ID == id {
						return p, nil
					}
				}
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			}
			m := &mockQuerier{
				getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) { return byID(id) },
				lockCrewMembersForMergeFn: func(context.Context, sqlcdb.LockCrewMembersForMergeParams) ([]sqlcdb.CrewMember, error) {
					return c.people, nil
				},
				lockCrewMergeTripsFn:   func(context.Context, sqlcdb.LockCrewMergeTripsParams) error { return nil },
				lockCrewMergeVoyagesFn: func(context.Context, sqlcdb.LockCrewMergeVoyagesParams) error { return nil },
			}
			api := crewTestAPI(t, m)
			admin := callerCtx(1, "admin")
			if resp := api.PostCtx(admin, "/crew/1/merge", map[string]any{"duplicate_id": c.duplicate}); resp.Code != c.want {
				t.Errorf("merge: got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
			path := "/crew/1/merge-preview?duplicate_id=" + strconv.FormatInt(c.duplicate, 10)
			if resp := api.GetCtx(admin, path); resp.Code != c.want {
				t.Errorf("preview: got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
		})
	}
}

func manyNames(prefix string, n int) json.RawMessage {
	names := make([]string, n)
	for i := range names {
		names[i] = prefix + strconv.Itoa(i)
	}
	b, _ := json.Marshal(names)
	return b
}

// mergeFixture is two people with overlapping crew lists and opinions: both
// sail leg 1 and voyage 1 and have an opinion for voyage 1; only the
// duplicate sails leg 2 and voyage 2 and has an opinion for voyage 2.
type mergeFixture struct {
	conn                     *sql.DB
	keptID, duplicateID      int64
	userID                   int64
	trip1, trip2             int64
	voyage1, voyage2         int64
	sharedOpinionFile        string
	duplicateOnlyOpinionFile string
}

func seedMerge(t *testing.T) mergeFixture {
	t.Helper()
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	scalar := func(query string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := conn.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return id
	}

	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "anna@example.dev", Name: "Anna Nowak"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := q.EnsureUserPerson(ctx, user.ID); err != nil {
		t.Fatalf("ensure person: %v", err)
	}
	dup, err := q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: user.ID, Valid: true})
	if err != nil {
		t.Fatalf("account person: %v", err)
	}
	exec(`UPDATE crew_members SET other_names = '["Ania Nowak"]', phone = '+48 600 000 001' WHERE id = $1`, dup.ID)
	kept := scalar(`INSERT INTO crew_members (full_name, other_names, patent_number)
		VALUES ('Anna Kowalska', '["anna nowak", "A. Kowalska"]', 'JSM-1') RETURNING id`)

	cruise := scalar(`INSERT INTO cruises (name, kind, currency) VALUES ('Baltic', 'sea', 'PLN') RETURNING id`)
	trip1 := scalar(`INSERT INTO trips (cruise_id, name, currency) VALUES ($1, 'Leg 1', 'PLN') RETURNING id`, cruise)
	trip2 := scalar(`INSERT INTO trips (cruise_id, name, currency) VALUES ($1, 'Leg 2', 'PLN') RETURNING id`, cruise)
	voyage1 := scalar(`INSERT INTO voyages (cruise_id, name) VALUES ($1, 'Voyage 1') RETURNING id`, cruise)
	voyage2 := scalar(`INSERT INTO voyages (cruise_id, name) VALUES ($1, 'Voyage 2') RETURNING id`, cruise)

	exec(`INSERT INTO crew_assignments (trip_id, crew_member_id, role, patent_number) VALUES
		($1, $3, 'zalogant', NULL), ($1, $4, 'oficer', 'JSM-9'), ($2, $4, 'zalogant', NULL)`, trip1, trip2, kept, dup.ID)
	exec(`UPDATE trips SET captain_person_id = $1, captain_name = 'Anna Nowak' WHERE id = $2`, dup.ID, trip2)
	exec(`UPDATE voyages SET captain_person_id = $1, captain_name = 'Anna Nowak' WHERE id = $2`, dup.ID, voyage2)
	exec(`INSERT INTO crew_assignments (voyage_id, crew_member_id, role) VALUES
		($1, $3, 'kapitan'), ($1, $4, 'zalogant'), ($2, $4, 'zalogant')`, voyage1, voyage2, kept, dup.ID)

	dir := t.TempDir()
	shared := filepath.Join(dir, "shared.pdf")
	only := filepath.Join(dir, "only.pdf")
	for _, f := range []string{shared, only} {
		if err := os.WriteFile(f, []byte("pdf"), 0o600); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	exec(`INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path) VALUES
		($1, $3, $5), ($1, $4, $6), ($2, $4, $7)`,
		voyage1, voyage2, kept, dup.ID, filepath.Join(dir, "kept.pdf"), shared, only)

	return mergeFixture{
		conn: conn, keptID: kept, duplicateID: dup.ID, userID: user.ID,
		trip1: trip1, trip2: trip2, voyage1: voyage1, voyage2: voyage2,
		sharedOpinionFile: shared, duplicateOnlyOpinionFile: only,
	}
}

// snapshot lists every crew assignment and opinion as comparable strings.
func (f mergeFixture) snapshot(t *testing.T) []string {
	t.Helper()
	rows, err := f.conn.Query(`
		SELECT 'a ' || COALESCE('t' || trip_id, 'v' || voyage_id) || ' m' || crew_member_id || ' ' || role || ' ' || COALESCE(patent_number, '-')
		FROM crew_assignments
		UNION ALL
		SELECT 'o v' || voyage_id || ' m' || crew_member_id || ' ' || file_path FROM voyage_opinions
		UNION ALL
		SELECT 'p ' || id || ' ' || full_name || ' ' || other_names::TEXT || ' ' || COALESCE(user_id::TEXT, '-') || ' ' ||
		       COALESCE(phone, '-') || ' ' || COALESCE(patent_number, '-')
		FROM crew_members`)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	slices.Sort(out)
	return out
}

func (f mergeFixture) api(t *testing.T, runTx txRunner) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	q := sqlcdb.New(f.conn)
	registerCrewRoutes(api, &CrewHandler{q: q, runTx: runTx})
	return api
}

// TestCrewMerge_DB merges an account's person into a person without an
// account and checks every moved row against a real schema.
func TestCrewMerge_DB(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	api := f.api(t, sqlTxRunner(f.conn))
	admin := callerCtx(1, "admin")
	keptPath := "/crew/" + strconv.FormatInt(f.keptID, 10)
	if _, err := f.conn.ExecContext(ctx, `UPDATE crew_assignments SET added_by_acceptance = (crew_member_id = $1)
		WHERE trip_id = $2`, f.keptID, f.trip1); err != nil {
		t.Fatalf("mark accepted entry: %v", err)
	}
	if _, err := f.conn.ExecContext(ctx, `UPDATE crew_assignments SET added_by_acceptance = TRUE
		WHERE trip_id = $1 AND crew_member_id = $2`, f.trip2, f.duplicateID); err != nil {
		t.Fatalf("mark accepted entry: %v", err)
	}
	before := f.snapshot(t)

	resp := api.GetCtx(admin, keptPath+"/merge-preview?duplicate_id="+strconv.FormatInt(f.duplicateID, 10))
	if resp.Code != http.StatusOK {
		t.Fatalf("preview: got %d; body=%s", resp.Code, resp.Body)
	}
	var preview dto.PersonMerge
	if err := json.Unmarshal(resp.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.TripAssignments != 2 || preview.VoyageAssignments != 2 || preview.SharedAssignments != 2 ||
		preview.Opinions != 2 || preview.SharedOpinions != 1 || preview.CaptainedTrips != 1 || preview.CaptainedVoyages != 1 {
		t.Errorf("preview counts = %+v", preview)
	}
	if !preview.Person.HasAccount || preview.Duplicate.ID != f.duplicateID {
		t.Errorf("preview people = %+v / %+v", preview.Person, preview.Duplicate)
	}
	if after := f.snapshot(t); !slices.Equal(after, before) {
		t.Fatalf("preview changed data:\nbefore %v\nafter  %v", before, after)
	}

	resp = api.PostCtx(admin, keptPath+"/merge", map[string]any{"duplicate_id": f.duplicateID})
	if resp.Code != http.StatusOK {
		t.Fatalf("merge: got %d; body=%s", resp.Code, resp.Body)
	}
	var merged dto.PersonMerge
	if err := json.Unmarshal(resp.Body.Bytes(), &merged); err != nil {
		t.Fatalf("decode merge: %v", err)
	}
	wantNames := []string{"A. Kowalska", "Anna Kowalska", "Ania Nowak"}
	if !slices.Equal(merged.Person.OtherNames, wantNames) {
		t.Errorf("other names = %v, want %v", merged.Person.OtherNames, wantNames)
	}
	if merged.Person.UserID == nil || *merged.Person.UserID != f.userID || merged.Person.FullName != "Anna Nowak" {
		t.Errorf("kept person = %+v, want the account's name Anna Nowak linked to user %d", merged.Person, f.userID)
	}
	if !slices.Equal(preview.Person.OtherNames, wantNames) || preview.Person.FullName != "Anna Nowak" {
		t.Errorf("preview person = %+v, want the merged names", preview.Person)
	}
	if merged.Person.Phone == nil || *merged.Person.Phone != "+48 600 000 001" ||
		merged.Person.PatentNumber == nil || *merged.Person.PatentNumber != "JSM-1" {
		t.Errorf("contact fields = phone %v patent %v", merged.Person.Phone, merged.Person.PatentNumber)
	}

	q := sqlcdb.New(f.conn)
	if _, err := q.GetCrewMember(ctx, f.duplicateID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("duplicate still exists: err=%v", err)
	}
	if p, err := q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: f.userID, Valid: true}); err != nil || p.ID != f.keptID {
		t.Errorf("account person = %d (err %v), want %d", p.ID, err, f.keptID)
	}

	k := "m" + strconv.FormatInt(f.keptID, 10)
	id := func(n int64) string { return strconv.FormatInt(n, 10) }
	dir := filepath.Dir(f.sharedOpinionFile)
	want := []string{
		"a t" + id(f.trip1) + " " + k + " zalogant JSM-9",
		"a t" + id(f.trip2) + " " + k + " zalogant -",
		"a v" + id(f.voyage1) + " " + k + " kapitan -",
		"a v" + id(f.voyage2) + " " + k + " zalogant -",
		"o v" + id(f.voyage1) + " " + k + " " + filepath.Join(dir, "kept.pdf"),
		"o v" + id(f.voyage2) + " " + k + " " + f.duplicateOnlyOpinionFile,
		"p " + id(f.keptID) + ` Anna Nowak ["A. Kowalska", "Anna Kowalska", "Ania Nowak"] ` + id(f.userID) + " +48 600 000 001 JSM-1",
	}
	slices.Sort(want)
	if got := f.snapshot(t); !slices.Equal(got, want) {
		t.Errorf("after merge:\ngot  %q\nwant %q", got, want)
	}
	var tripCaptain, voyageCaptain int64
	if err := f.conn.QueryRowContext(ctx, `SELECT
		(SELECT captain_person_id FROM trips WHERE id = $1),
		(SELECT captain_person_id FROM voyages WHERE id = $2)`, f.trip2, f.voyage2).Scan(&tripCaptain, &voyageCaptain); err != nil {
		t.Fatalf("captains: %v", err)
	}
	if tripCaptain != f.keptID || voyageCaptain != f.keptID {
		t.Errorf("captains = leg %d, voyage %d; want the kept person %d", tripCaptain, voyageCaptain, f.keptID)
	}
	accepted := map[int64]bool{}
	rows, err := f.conn.QueryContext(ctx, "SELECT trip_id, added_by_acceptance FROM crew_assignments WHERE trip_id IS NOT NULL")
	if err != nil {
		t.Fatalf("list leg crew: %v", err)
	}
	for rows.Next() {
		var trip int64
		var flag bool
		if err := rows.Scan(&trip, &flag); err != nil {
			t.Fatalf("scan: %v", err)
		}
		accepted[trip] = flag
	}
	_ = rows.Close()
	if accepted[f.trip1] || !accepted[f.trip2] {
		t.Errorf("added_by_acceptance = %v: leg 1 must stay hand-added, leg 2 keeps its accepted flag", accepted)
	}
	if _, err := os.Stat(f.sharedOpinionFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("shared duplicate opinion file kept: err=%v", err)
	}
	if _, err := os.Stat(f.duplicateOnlyOpinionFile); err != nil {
		t.Errorf("moved opinion file: %v", err)
	}

	if _, err := q.UpdateUserProfile(ctx, sqlcdb.UpdateUserProfileParams{
		FirstName: "Anna", LastName: "Nowak-Wiśniewska", ID: f.userID,
	}); err != nil {
		t.Fatalf("profile save: %v", err)
	}
	found, err := q.ListCrewMembers(ctx, sqlcdb.ListCrewMembersParams{
		Search: types.NullString{String: "anna kowalska", Valid: true}, RowLimit: 10,
	})
	if err != nil || len(found) != 1 || found[0].ID != f.keptID || found[0].FullName != "Anna Nowak-Wiśniewska" {
		t.Errorf("search by the kept person's old name after a profile save = %+v (err %v)", found, err)
	}
}

// TestCrewMerge_DBRefusesTwoAccounts keeps both people when each has an
// account.
func TestCrewMerge_DBRefusesTwoAccounts(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	q := sqlcdb.New(f.conn)
	other, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "kowalska@example.dev", Name: "Anna Kowalska"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := f.conn.ExecContext(ctx, "UPDATE crew_members SET user_id = $1 WHERE id = $2", other.ID, f.keptID); err != nil {
		t.Fatalf("link kept person: %v", err)
	}
	before := f.snapshot(t)
	resp := f.api(t, sqlTxRunner(f.conn)).PostCtx(callerCtx(1, "admin"),
		"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
	if resp.Code != http.StatusConflict {
		t.Fatalf("merge: got %d, want 409; body=%s", resp.Code, resp.Body)
	}
	if after := f.snapshot(t); !slices.Equal(after, before) {
		t.Errorf("refused merge changed data:\nbefore %v\nafter  %v", before, after)
	}
}

// failLastMergeStep runs the real merge but fails its final update, after
// every row has already moved inside the transaction.
type failLastMergeStep struct {
	sqlcdb.Querier
}

func (failLastMergeStep) UpdateMergedCrewMember(context.Context, sqlcdb.UpdateMergedCrewMemberParams) (sqlcdb.CrewMember, error) {
	return sqlcdb.CrewMember{}, errors.New("injected failure")
}

func TestCrewMerge_DBAtomic(t *testing.T) {
	f := seedMerge(t)
	before := f.snapshot(t)
	tx := sqlTxRunner(f.conn)
	failing := func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return tx(ctx, func(q sqlcdb.Querier) error { return fn(failLastMergeStep{q}) })
	}
	resp := f.api(t, failing).PostCtx(callerCtx(1, "admin"),
		"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("merge: got %d, want 500; body=%s", resp.Code, resp.Body)
	}
	if after := f.snapshot(t); !slices.Equal(after, before) {
		t.Errorf("failed merge changed data:\nbefore %v\nafter  %v", before, after)
	}
	for _, file := range []string{f.sharedOpinionFile, f.duplicateOnlyOpinionFile} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("opinion file after failed merge: %v", err)
		}
	}
}

// TestCrewMerge_DBWaitsForLegCompletion completes a leg both people are on
// while the merge runs: the merge waits for it, then collapses the two
// assignments on the new voyage.
func TestCrewMerge_DBWaitsForLegCompletion(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	completion, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin completion: %v", err)
	}
	defer func() { _ = completion.Rollback() }()
	var voyage int64
	for _, step := range []string{
		"SELECT id FROM trips WHERE id = $1 FOR UPDATE",
		`INSERT INTO voyages (cruise_id, name) SELECT cruise_id, 'Leg 1 done' FROM trips WHERE id = $1 RETURNING id`,
	} {
		if err := completion.QueryRowContext(ctx, step, f.trip1).Scan(&voyage); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	if _, err := completion.ExecContext(ctx,
		"UPDATE crew_assignments SET voyage_id = $1, trip_id = NULL WHERE trip_id = $2", voyage, f.trip1); err != nil {
		t.Fatalf("repoint crew: %v", err)
	}
	if _, err := completion.ExecContext(ctx, "DELETE FROM trips WHERE id = $1", f.trip1); err != nil {
		t.Fatalf("delete trip: %v", err)
	}

	done := make(chan int, 1)
	api := f.api(t, sqlTxRunner(f.conn))
	go func() {
		resp := api.PostCtx(callerCtx(1, "admin"),
			"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
		done <- resp.Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if err := completion.Commit(); err != nil {
		t.Fatalf("commit completion: %v", err)
	}
	if code := <-done; code != http.StatusOK {
		t.Fatalf("merge: got %d, want 200", code)
	}
	var crew []int64
	rows, err := f.conn.QueryContext(ctx, "SELECT crew_member_id FROM crew_assignments WHERE voyage_id = $1", voyage)
	if err != nil {
		t.Fatalf("list crew: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		crew = append(crew, id)
	}
	if !slices.Equal(crew, []int64{f.keptID}) {
		t.Errorf("completed leg crew = %v, want only the kept person %d", crew, f.keptID)
	}
}

// TestCrewMerge_DBProfileSaveDuringMerge saves the account owner's profile
// while the merge moving that account holds its locks: the save waits and
// then updates the kept person.
func TestCrewMerge_DBProfileSaveDuringMerge(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	tx, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin merge: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := MergePeople(ctx, sqlcdb.New(tx), f.keptID, f.duplicateID); err != nil {
		t.Fatalf("merge: %v", err)
	}

	_, api := humatest.New(t)
	RegisterAuthRoutes(api, sqlcdb.New(f.conn), f.conn)
	saved := make(chan int, 1)
	go func() {
		resp := api.PutCtx(memberCtx(f.userID), "/auth/me", map[string]any{
			"first_name": "Anna", "last_name": "Zielińska", "phone": "+48 600 000 009",
		})
		saved <- resp.Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit merge: %v", err)
	}
	if code := <-saved; code != http.StatusOK {
		t.Fatalf("profile save: got %d, want 200", code)
	}
	p, err := sqlcdb.New(f.conn).GetCrewMember(ctx, f.keptID)
	if err != nil {
		t.Fatalf("kept person: %v", err)
	}
	if p.FullName != "Anna Zielińska" || p.Phone.String != "+48 600 000 009" {
		t.Errorf("kept person after profile save = %q / %q, want the saved name and phone", p.FullName, p.Phone.String)
	}
}

// TestCrewMerge_DBLegSignUpDuringMerge signs the duplicate's account up for a
// leg the duplicate is on while a merge waits for that leg: the sign-up's
// reference to the account must not wait on the merge's account lock.
func TestCrewMerge_DBLegSignUpDuringMerge(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	signUp, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin sign-up: %v", err)
	}
	defer func() { _ = signUp.Rollback() }()
	if _, err := signUp.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR NO KEY UPDATE", f.trip1); err != nil {
		t.Fatalf("lock leg: %v", err)
	}

	done := make(chan int, 1)
	api := f.api(t, sqlTxRunner(f.conn))
	go func() {
		resp := api.PostCtx(callerCtx(1, "admin"),
			"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
		done <- resp.Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if _, err := signUp.ExecContext(ctx,
		"INSERT INTO trip_enrollments (trip_id, user_id, status) VALUES ($1, $2, 'pending')", f.trip1, f.userID); err != nil {
		t.Fatalf("sign up: %v", err)
	}
	if err := signUp.Commit(); err != nil {
		t.Fatalf("commit sign-up: %v", err)
	}
	if code := <-done; code != http.StatusOK {
		t.Fatalf("merge: got %d, want 200", code)
	}
}

// TestCrewMerge_DBAcceptanceDuringMerge accepts the duplicate's account onto
// a leg the kept person is on while the merge waits for that leg. Acceptance
// locks the leg before the person, the merge the other way round; the merge
// loses the deadlock, retries and both succeed.
func TestCrewMerge_DBAcceptanceDuringMerge(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	var trip3 int64
	if err := f.conn.QueryRowContext(ctx, `INSERT INTO trips (cruise_id, name, currency)
		SELECT cruise_id, 'Leg 3', 'PLN' FROM trips WHERE id = $1 RETURNING id`, f.trip1).Scan(&trip3); err != nil {
		t.Fatalf("create leg 3: %v", err)
	}
	if _, err := f.conn.ExecContext(ctx, "INSERT INTO crew_assignments (trip_id, crew_member_id) VALUES ($1, $2)", trip3, f.keptID); err != nil {
		t.Fatalf("assign kept person: %v", err)
	}
	accept, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin acceptance: %v", err)
	}
	defer func() { _ = accept.Rollback() }()
	if _, err := accept.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR NO KEY UPDATE", trip3); err != nil {
		t.Fatalf("lock leg: %v", err)
	}

	done := make(chan int, 1)
	api := f.api(t, sqlTxRunner(f.conn))
	go func() {
		resp := api.PostCtx(callerCtx(1, "admin"),
			"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
		done <- resp.Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	if err := sqlcdb.New(accept).AddAcceptedTripCrewAssignment(ctx, sqlcdb.AddAcceptedTripCrewAssignmentParams{
		TripID: trip3, UserID: f.userID,
	}); err != nil {
		t.Fatalf("add accepted crew: %v", err)
	}
	if err := accept.Commit(); err != nil {
		t.Fatalf("commit acceptance: %v", err)
	}
	if code := <-done; code != http.StatusOK {
		t.Fatalf("merge: got %d, want 200", code)
	}
	var crew []int64
	rows, err := f.conn.QueryContext(ctx, "SELECT crew_member_id FROM crew_assignments WHERE trip_id = $1", trip3)
	if err != nil {
		t.Fatalf("list crew: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		crew = append(crew, id)
	}
	if !slices.Equal(crew, []int64{f.keptID}) {
		t.Errorf("leg 3 crew = %v, want only the kept person %d", crew, f.keptID)
	}
}

// TestCrewMerge_DBWaitsForCaptainedLegCompletion completes a leg the
// duplicate captains while the merge runs, in completion's person-then-leg
// lock order: the merge waits, then moves the new voyage's captain too.
func TestCrewMerge_DBWaitsForCaptainedLegCompletion(t *testing.T) {
	f := seedMerge(t)
	ctx := context.Background()
	completion, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin completion: %v", err)
	}
	defer func() { _ = completion.Rollback() }()
	if _, err := completion.ExecContext(ctx, "SELECT id FROM crew_members WHERE id = $1 FOR KEY SHARE", f.duplicateID); err != nil {
		t.Fatalf("lock captain: %v", err)
	}
	if _, err := completion.ExecContext(ctx, "SELECT id FROM trips WHERE id = $1 FOR UPDATE", f.trip2); err != nil {
		t.Fatalf("lock leg: %v", err)
	}

	done := make(chan int, 1)
	api := f.api(t, sqlTxRunner(f.conn))
	go func() {
		resp := api.PostCtx(callerCtx(1, "admin"),
			"/crew/"+strconv.FormatInt(f.keptID, 10)+"/merge", map[string]any{"duplicate_id": f.duplicateID})
		done <- resp.Code
	}()
	waitForLockWaiters(t, f.conn, 1)
	var voyage int64
	if err := completion.QueryRowContext(ctx, `INSERT INTO voyages (cruise_id, name, captain_person_id)
		SELECT cruise_id, 'Leg 2 done', captain_person_id FROM trips WHERE id = $1 RETURNING id`, f.trip2).Scan(&voyage); err != nil {
		t.Fatalf("create voyage: %v", err)
	}
	for _, step := range []string{
		"UPDATE crew_assignments SET voyage_id = $1, trip_id = NULL WHERE trip_id = $2",
		"DELETE FROM trips WHERE id = $2 AND $1::BIGINT > 0",
	} {
		if _, err := completion.ExecContext(ctx, step, voyage, f.trip2); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}
	if err := completion.Commit(); err != nil {
		t.Fatalf("commit completion: %v", err)
	}
	if code := <-done; code != http.StatusOK {
		t.Fatalf("merge: got %d, want 200", code)
	}
	var captain int64
	if err := f.conn.QueryRowContext(ctx, "SELECT captain_person_id FROM voyages WHERE id = $1", voyage).Scan(&captain); err != nil {
		t.Fatalf("voyage captain: %v", err)
	}
	if captain != f.keptID {
		t.Errorf("completed leg's voyage captain = %d, want the kept person %d", captain, f.keptID)
	}
}
