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
// with the crew member aboard, one port and one opinion with its file.
// A published voyage has its cruise published too; else both are hidden.
type seededVoyage struct {
	id, port, opinion int64
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
	if err := d.conn.QueryRowContext(ctx,
		"INSERT INTO voyage_ports (voyage_id, name, position) VALUES ($1, 'Hel', 0) RETURNING id", v.id).Scan(&v.port); err != nil {
		t.Fatalf("add port: %v", err)
	}
	rel := filepath.Join(opinionsDir, fmt.Sprintf("%d_%d.pdf", v.id, d.crewPerson))
	writeUpload(t, d.uploads, rel, "opinion")
	if err := d.conn.QueryRowContext(ctx, `INSERT INTO voyage_opinions (voyage_id, crew_member_id, file_path, file_format)
		VALUES ($1, $2, $3, 'pdf') RETURNING id`, v.id, d.crewPerson, filepath.Join(d.uploads, rel)).Scan(&v.opinion); err != nil {
		t.Fatalf("add opinion: %v", err)
	}
	return v
}

func (d captainVoyageDB) api(t *testing.T) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	q := sqlcdb.New(d.conn)
	RegisterVoyageRoutes(api, q, d.conn)
	RegisterVoyagePortRoutes(api, q, d.conn)
	RegisterVoyageOpinionRoutes(api, q, d.conn, testUploadRoot(t, d.uploads))
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

// TestCaptainVoyageWrites_RecheckCaptainUnderLock hands a voyage's captaincy
// to another person in an open transaction while the captain sends a write.
// The write passes the check before its transaction, then waits on the
// voyage lock. A reassignment that commits there refuses the write, with 403
// or with 404 for a hidden voyage the former captain no longer sees, and
// leaves everything as it was; one that rolls back lets it through, hidden
// voyage included.
func TestCaptainVoyageWrites_RecheckCaptainUnderLock(t *testing.T) {
	d := seedCaptainVoyageDB(t)
	api := d.api(t)
	captain := callerCtx(d.captainUser, "member")
	writes := []struct {
		name string
		ok   int
		send func(v seededVoyage) *httptest.ResponseRecorder
	}{
		{name: "log", ok: http.StatusNoContent, send: func(v seededVoyage) *httptest.ResponseRecorder {
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
		{name: "reorder ports", ok: http.StatusOK, send: func(v seededVoyage) *httptest.ResponseRecorder {
			return api.PutCtx(captain, fmt.Sprintf("/voyages/%d/ports/order", v.id), map[string]any{"port_ids": []int64{v.port}})
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
			name              string
			published, revoke bool
			refused           int
		}{
			{name: "revocation commits", published: true, revoke: true, refused: http.StatusForbidden},
			{name: "revocation rolls back", published: true},
			{name: "hidden/revocation commits", revoke: true, refused: http.StatusNotFound},
			{name: "hidden/revocation rolls back"},
		} {
			revoke := tc.revoke
			t.Run(w.name+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				v := d.voyage(t, tc.published)
				before := d.state(t, v)

				admin, err := d.conn.BeginTx(ctx, nil)
				if err != nil {
					t.Fatalf("begin reassignment: %v", err)
				}
				defer func() { _ = admin.Rollback() }()
				if _, err := admin.ExecContext(ctx,
					"UPDATE voyages SET captain_person_id = $1 WHERE id = $2", d.otherPerson, v.id); err != nil {
					t.Fatalf("reassign captain: %v", err)
				}

				done := make(chan *httptest.ResponseRecorder, 1)
				go func() { done <- w.send(v) }()
				waitForLockWaiters(t, d.conn, 1)
				if revoke {
					err = admin.Commit()
				} else {
					err = admin.Rollback()
				}
				if err != nil {
					t.Fatalf("end reassignment: %v", err)
				}

				resp := <-done
				want := w.ok
				if revoke {
					want = tc.refused
				}
				if resp.Code != want {
					t.Fatalf("status = %d, want %d; body=%s", resp.Code, want, resp.Body)
				}
				after := d.state(t, v)
				if revoke && after != before {
					t.Fatalf("state changed by a refused write:\nbefore:\n%s\nafter:\n%s", before, after)
				}
				if !revoke && w.name != "reorder ports" && after == before {
					t.Fatalf("write left no change:\n%s", after)
				}
			})
		}
	}
}
