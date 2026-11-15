package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// captainVoyageDB is a test database with a voyage captain's account and
// another person to hand the captaincy to. Each subtest seeds its own
// voyage on it.
type captainVoyageDB struct {
	conn          *sql.DB
	uploads       string
	captainUser   int64
	captainPerson int64
	otherPerson   int64
	crewPerson    int64
}

func seedCaptainVoyageDB(t *testing.T) captainVoyageDB {
	t.Helper()
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	user := func(email string) int64 {
		u, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: email, Name: email})
		if err != nil {
			t.Fatalf("create user %s: %v", email, err)
		}
		return u.ID
	}
	d := captainVoyageDB{conn: conn, uploads: t.TempDir()}
	d.captainUser = user("kapitan@example.dev")
	d.captainPerson = ensurePerson(t, conn, d.captainUser)
	d.otherPerson = ensurePerson(t, conn, user("inny@example.dev"))
	if err := conn.QueryRowContext(ctx,
		"INSERT INTO crew_members (full_name) VALUES ('Jan Nowak') RETURNING id").Scan(&d.crewPerson); err != nil {
		t.Fatalf("create crew member: %v", err)
	}
	return d
}

// seededVoyage is one seeded voyage: captained by the fixture's captain,
// with the crew member aboard, two ports and one opinion with its file.
// A published voyage has its cruise published too; else both are hidden.
type seededVoyage struct {
	id, port, port2, opinion int64
}

func (d captainVoyageDB) voyage(t *testing.T, published bool) seededVoyage {
	t.Helper()
	ctx := context.Background()
	var v seededVoyage
	if err := d.conn.QueryRowContext(ctx, `WITH c AS (
			INSERT INTO cruises (name, published_at) VALUES ('Bałtyk', CASE WHEN $2 THEN now() END) RETURNING id)
		INSERT INTO voyages (cruise_id, name, captain_person_id, published_at)
		SELECT id, 'Rejs', $1, CASE WHEN $2 THEN now() END FROM c RETURNING id`,
		d.captainPerson, published).Scan(&v.id); err != nil {
		t.Fatalf("create voyage: %v", err)
	}
	if _, err := d.conn.ExecContext(ctx,
		"INSERT INTO crew_assignments (voyage_id, crew_member_id) VALUES ($1, $2)", v.id, d.crewPerson); err != nil {
		t.Fatalf("add crew: %v", err)
	}
	for i, p := range []*int64{&v.port, &v.port2} {
		if err := d.conn.QueryRowContext(ctx,
			"INSERT INTO voyage_ports (voyage_id, name, position) VALUES ($1, $2, $3) RETURNING id",
			v.id, fmt.Sprintf("Port %d", i), i).Scan(p); err != nil {
			t.Fatalf("add port: %v", err)
		}
	}
	rel := filepath.Join(opinionsDir, fmt.Sprintf("%d_%d.pdf", v.id, d.crewPerson))
	writeUpload(t, d.uploads, rel, "opinion")
	if err := d.conn.QueryRowContext(ctx, `INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format)
		VALUES ($1, $2, $3, 'pdf') RETURNING id`, v.id, d.crewPerson, filepath.Join(d.uploads, rel)).Scan(&v.opinion); err != nil {
		t.Fatalf("add opinion: %v", err)
	}
	return v
}

// interleave runs around each captain write statement, so a test can change
// the voyage's captain after the handler's own check and before or while
// the statement runs. calls counts the statements.
type interleave struct {
	around func(statement func() error) error
	calls  int
}

// interleavedQuerier routes the captain write statements of q through il.
type interleavedQuerier struct {
	sqlcdb.Querier
	il *interleave
}

func (q interleavedQuerier) run(statement func() error) error {
	q.il.calls++
	if q.il.around == nil {
		return statement()
	}
	return q.il.around(statement)
}

func (q interleavedQuerier) UpdateVoyageLog(ctx context.Context, arg sqlcdb.UpdateVoyageLogParams) (n int64, err error) {
	err = q.run(func() (err error) { n, err = q.Querier.UpdateVoyageLog(ctx, arg); return err })
	return n, err
}

func (q interleavedQuerier) AddVoyageYacht(ctx context.Context, arg sqlcdb.AddVoyageYachtParams) (y sqlcdb.Yacht, err error) {
	err = q.run(func() (err error) { y, err = q.Querier.AddVoyageYacht(ctx, arg); return err })
	return y, err
}

func (q interleavedQuerier) AddVoyagePort(ctx context.Context, arg sqlcdb.AddVoyagePortParams) (p sqlcdb.VoyagePort, err error) {
	err = q.run(func() (err error) { p, err = q.Querier.AddVoyagePort(ctx, arg); return err })
	return p, err
}

func (q interleavedQuerier) DeleteVoyagePort(ctx context.Context, arg sqlcdb.DeleteVoyagePortParams) (ok bool, err error) {
	err = q.run(func() (err error) { ok, err = q.Querier.DeleteVoyagePort(ctx, arg); return err })
	return ok, err
}

func (q interleavedQuerier) SetVoyagePortPosition(ctx context.Context, arg sqlcdb.SetVoyagePortPositionParams) (n int64, err error) {
	err = q.run(func() (err error) { n, err = q.Querier.SetVoyagePortPosition(ctx, arg); return err })
	return n, err
}

func (q interleavedQuerier) UpsertVoyageOpinion(ctx context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (r sqlcdb.UpsertVoyageOpinionRow, err error) {
	err = q.run(func() (err error) { r, err = q.Querier.UpsertVoyageOpinion(ctx, arg); return err })
	return r, err
}

func (q interleavedQuerier) DeleteVoyageOpinion(ctx context.Context, arg sqlcdb.DeleteVoyageOpinionParams) (r sqlcdb.DeleteVoyageOpinionRow, err error) {
	err = q.run(func() (err error) { r, err = q.Querier.DeleteVoyageOpinion(ctx, arg); return err })
	return r, err
}

func (d captainVoyageDB) api(t *testing.T, il *interleave) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	q := interleavedQuerier{Querier: sqlcdb.New(d.conn), il: il}
	RegisterVoyageRoutes(api, q)
	registerVoyagePortRoutes(api, &VoyagePortHandler{q: q, runTx: func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return sqlTxRunner(d.conn)(ctx, func(tq sqlcdb.Querier) error {
			return fn(interleavedQuerier{Querier: tq, il: il})
		})
	}})
	RegisterVoyageOpinionRoutes(api, q, testUploadRoot(t, d.uploads))
	return api
}

// state renders everything a captain write on the voyage may change: the
// log, the ports, the opinions, the opinion files and the club yachts.
func (d captainVoyageDB) state(t *testing.T, v seededVoyage) string {
	t.Helper()
	var b strings.Builder
	row := func(query string, args ...any) {
		rows, err := d.conn.QueryContext(context.Background(), query, args...)
		if err != nil {
			t.Fatalf("read state: %v", err)
		}
		defer func() { _ = rows.Close() }()
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatalf("scan state: %v", err)
			}
			fmt.Fprintln(&b, vals...)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("state rows: %v", err)
		}
	}
	row("SELECT miles, captain_notes, yacht_id FROM voyages WHERE id = $1", v.id)
	row("SELECT id, name, position FROM voyage_ports WHERE voyage_id = $1 ORDER BY id", v.id)
	row("SELECT id, crew_member_id, file_path, file_format FROM voyage_opinions WHERE voyage_id = $1 ORDER BY id", v.id)
	row("SELECT count(*) FROM yachts")
	entries, err := os.ReadDir(filepath.Join(d.uploads, opinionsDir))
	if err != nil {
		t.Fatalf("read opinions dir: %v", err)
	}
	for _, e := range entries {
		fmt.Fprintln(&b, "file", e.Name())
	}
	return b.String()
}

// captainVoyageWrite is one captain write request. waits marks a statement
// that writes the voyage row, which an open reassignment holds; rechecks
// marks a write whose later statements check the captain again.
type captainVoyageWrite struct {
	name            string
	ok              int
	waits, rechecks bool
	send            func(v seededVoyage) *httptest.ResponseRecorder
}

// TestCaptainVoyageWrites_CheckCaptainInStatement hands a voyage's captaincy
// to another person after the captain's write passed the handler's check,
// just before its write statement runs. No lock is held, so the statement's
// own captain check decides:
//
//   - A reassignment committed before the statement refuses the write with
//     403, or 404 for a hidden voyage the former captain no longer sees, and
//     leaves rows and opinion files as they were.
//   - A reassignment still open while the statement runs and then rolled
//     back lets the write through, hidden voyage included.
//   - A reassignment still open while the statement runs and committed after
//     it is the accepted race: the write is checked against the captain as
//     it stood when the statement started and succeeds. Two writes differ:
//     the log update writes the voyage row the reassignment holds, so it
//     waits for the commit and then refuses; the port reorder checks again
//     in each later statement of its transaction, so it refuses and rolls
//     back.
func TestCaptainVoyageWrites_CheckCaptainInStatement(t *testing.T) {
	d := seedCaptainVoyageDB(t)
	il := &interleave{}
	api := d.api(t, il)
	captain := callerCtx(d.captainUser, "member")
	writes := []captainVoyageWrite{
		{name: "log", ok: http.StatusNoContent, waits: true, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PutCtx(captain, fmt.Sprintf("/voyages/%d/log", v.id), map[string]any{"miles": 120, "captain_notes": "Wiało"})
		}},
		{name: "add yacht", ok: http.StatusCreated, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PostCtx(captain, fmt.Sprintf("/voyages/%d/yachts", v.id), map[string]any{"name": "Czarter"})
		}},
		{name: "add port", ok: http.StatusCreated, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PostCtx(captain, fmt.Sprintf("/voyages/%d/ports", v.id), map[string]any{"name": "Jastarnia"})
		}},
		{name: "remove port", ok: http.StatusNoContent, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.DeleteCtx(captain, fmt.Sprintf("/voyages/%d/ports/%d", v.id, v.port))
		}},
		{name: "reorder ports", ok: http.StatusOK, rechecks: true, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PutCtx(captain, fmt.Sprintf("/voyages/%d/ports/order", v.id), map[string]any{"port_ids": []int64{v.port2, v.port}})
		}},
		{name: "generate opinion", ok: http.StatusCreated, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PostCtx(captain, fmt.Sprintf("/voyages/%d/opinions", v.id), map[string]any{"crew_member_id": d.crewPerson, "format": "docx"})
		}},
		{name: "delete opinion", ok: http.StatusNoContent, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.DeleteCtx(captain, fmt.Sprintf("/voyages/%d/opinions/%d", v.id, v.opinion))
		}},
	}
	for _, w := range writes {
		for _, tc := range []struct {
			name      string
			published bool
			open      bool
			commit    bool
		}{
			{name: "committed before the statement", published: true, commit: true},
			{name: "open during the statement, rolled back", published: true, open: true},
			{name: "open during the statement, committed", published: true, open: true, commit: true},
			{name: "hidden/committed before the statement", commit: true},
			{name: "hidden/open during the statement, rolled back", open: true},
			{name: "hidden/open during the statement, committed", open: true, commit: true},
		} {
			t.Run(w.name+"/"+tc.name, func(t *testing.T) {
				v := d.voyage(t, tc.published)
				before := d.state(t, v)
				il.calls = 0
				il.around = d.reassignAround(t, il, v, w, tc.open, tc.commit)
				defer func() { il.around = nil }()

				resp := w.send(v)
				refused := tc.commit && (!tc.open || w.waits || w.rechecks)
				want := w.ok
				if refused {
					want = http.StatusForbidden
					if !tc.published {
						want = http.StatusNotFound
					}
				}
				if resp.Code != want {
					t.Fatalf("status = %d, want %d; body=%s", resp.Code, want, resp.Body)
				}
				if il.calls == 0 {
					t.Fatal("the write statement never ran")
				}
				after := d.state(t, v)
				if refused && after != before {
					t.Fatalf("state changed by a refused write:\nbefore:\n%s\nafter:\n%s", before, after)
				}
				if !refused && after == before {
					t.Fatalf("write left no change:\n%s", after)
				}
			})
		}
	}
}

// reassignAround returns an interleave hook that hands v's captaincy to the
// other person around the write's first statement. Without open the change
// commits before the statement; with open it stays uncommitted while the
// statement runs and is committed or rolled back after it, or, for a
// statement that waits on it, once that statement blocks.
func (d captainVoyageDB) reassignAround(t *testing.T, il *interleave, v seededVoyage, w captainVoyageWrite, open, commit bool) func(func() error) error {
	t.Helper()
	ctx := context.Background()
	const reassign = "UPDATE voyages SET captain_person_id = $1 WHERE id = $2"
	return func(statement func() error) error {
		if il.calls > 1 {
			return statement()
		}
		if !open {
			if _, err := d.conn.ExecContext(ctx, reassign, d.otherPerson, v.id); err != nil {
				t.Errorf("reassign captain: %v", err)
			}
			return statement()
		}
		tx, err := d.conn.BeginTx(ctx, nil)
		if err != nil {
			t.Errorf("begin reassignment: %v", err)
			return statement()
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(ctx, reassign, d.otherPerson, v.id); err != nil {
			t.Errorf("reassign captain: %v", err)
		}
		done := make(chan error, 1)
		go func() { done <- statement() }()
		if w.waits {
			waitForLockWaiters(t, d.conn, 1)
		} else {
			select {
			case err := <-done:
				done <- err
			case <-time.After(10 * time.Second):
				t.Errorf("statement waited for the open reassignment")
			}
		}
		if commit {
			err = tx.Commit()
		} else {
			err = tx.Rollback()
		}
		if err != nil {
			t.Errorf("end reassignment: %v", err)
		}
		return <-done
	}
}

// TestVoyageOpinionGenerate_RemovesReplacedFile regenerates a seeded
// opinion: the upsert returns the replaced row's file, which is removed once
// the row points at the new one.
func TestVoyageOpinionGenerate_RemovesReplacedFile(t *testing.T) {
	d := seedCaptainVoyageDB(t)
	api := d.api(t, &interleave{})
	v := d.voyage(t, true)
	resp := api.PostCtx(callerCtx(d.captainUser, "member"), fmt.Sprintf("/voyages/%d/opinions", v.id),
		map[string]any{"crew_member_id": d.crewPerson, "format": "docx"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var stored string
	if err := d.conn.QueryRow("SELECT file_path FROM voyage_opinions WHERE id = $1", v.opinion).Scan(&stored); err != nil {
		t.Fatalf("read opinion: %v", err)
	}
	if got := opinionFiles(t, d.uploads); len(got) != 1 || filepath.Join(d.uploads, opinionsDir, got[0]) != stored {
		t.Fatalf("opinion files = %v, want only %s", got, stored)
	}
}
