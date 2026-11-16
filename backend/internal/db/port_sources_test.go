package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

const portSourcesMigration = "037_port_sources.sql"

// TestMigrate_PortSources checks that migration 037 stores port names by
// the API name rule: blank start and end ports become NULL, the others lose
// surrounding blanks and keep one space between words. A visited port's
// required name is trimmed unless nothing would be left.
func TestMigrate_PortSources(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, portSourcesMigration)

	cruiseID := insertPortSearchCruise(t, conn, true)
	legs := []struct {
		start, end         *string
		wantStart, wantEnd types.NullString
	}{
		{new(""), new("   "), types.NullString{}, types.NullString{}},
		{new(" \t\n"), nil, types.NullString{}, types.NullString{}},
		{new("  Gdynia   Port \t"), new("Hel"), text("Gdynia Port"), text("Hel")},
		{nil, new("Visby "), types.NullString{}, text("Visby")},
	}
	type insertedLeg struct {
		table              string
		id                 int64
		wantStart, wantEnd types.NullString
	}
	var inserted []insertedLeg
	for _, table := range []string{"trips", "voyages"} {
		for _, l := range legs {
			leg := insertedLeg{table: table, wantStart: l.wantStart, wantEnd: l.wantEnd}
			mustScan(t, conn.QueryRow(`INSERT INTO `+table+` (cruise_id, name, start_port, end_port)
				VALUES ($1, 'leg', $2, $3) RETURNING id`, cruiseID, l.start, l.end), &leg.id)
			inserted = append(inserted, leg)
		}
	}
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)
	ports := map[string]string{" Hel  Bór ": "Hel Bór", "   ": "   ", "Jastarnia": "Jastarnia"}
	portIDs := map[string]int64{}
	for name := range ports {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO voyage_ports (voyage_id, name) VALUES ($1, $2) RETURNING id`,
			voyageID, name), &id)
		portIDs[name] = id
	}

	// Positions written before the API capped them, out of order and with a
	// tie: the migration keeps the listed order and renumbers from 0.
	orderVoyage := insertPortSearchVoyage(t, conn, cruiseID, true)
	legacy := []int64{9223372036854775807, 5, 5, -3}
	positioned := make([]int64, len(legacy))
	for i, pos := range legacy {
		mustScan(t, conn.QueryRow(`INSERT INTO voyage_ports (voyage_id, name, position) VALUES ($1, 'p', $2) RETURNING id`,
			orderVoyage, pos), &positioned[i])
	}
	wantOrder := []int64{positioned[3], positioned[1], positioned[2], positioned[0]}

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for i, id := range wantOrder {
		var pos int64
		mustScan(t, conn.QueryRow(`SELECT position FROM voyage_ports WHERE id = $1`, id), &pos)
		if pos != int64(i) {
			t.Errorf("port %d position = %d, want %d", id, pos, i)
		}
	}
	for _, l := range inserted {
		var start, end types.NullString
		mustScan(t, conn.QueryRow(`SELECT start_port, end_port FROM `+l.table+` WHERE id = $1`, l.id), &start, &end)
		if start != l.wantStart || end != l.wantEnd {
			t.Errorf("%s %d ports = %+v / %+v, want %+v / %+v", l.table, l.id, start, end, l.wantStart, l.wantEnd)
		}
	}
	for name, want := range ports {
		var got string
		mustScan(t, conn.QueryRow(`SELECT name FROM voyage_ports WHERE id = $1`, portIDs[name]), &got)
		if got != want {
			t.Errorf("voyage port %q = %q, want %q", name, got, want)
		}
	}
}

// errCaptured stops a query that captureDB recorded.
var errCaptured = errors.New("query captured")

// captureDB records the SQL text and arguments of a sqlc query instead of
// running it, so a test can EXPLAIN the exact query the API sends.
type captureDB struct {
	query string
	args  []any
}

func (c *captureDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errCaptured
}

func (c *captureDB) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errCaptured
}

func (c *captureDB) QueryContext(_ context.Context, query string, args ...any) (*sql.Rows, error) {
	c.query, c.args = query, args
	return nil, errCaptured
}

func (c *captureDB) QueryRowContext(context.Context, string, ...any) *sql.Row {
	return nil
}

// portSearchSeed fills every searched column with 2000 distinct port names,
// enough for the planner to prefer the trigram indexes. VACUUM moves the
// new rows out of the GIN pending lists, which the planner costs as a full
// scan.
const portSearchSeed = `
INSERT INTO cruises (name, published_at) SELECT 'event ' || i, now() FROM generate_series(1, 20) i;
INSERT INTO trips (cruise_id, name, start_port, end_port, published_at)
    SELECT c.id, 'leg', 'port ' || md5('ts' || i), 'port ' || md5('te' || i), now()
    FROM generate_series(1, 2000) i JOIN cruises c ON c.name = 'event ' || (i % 20 + 1);
INSERT INTO voyages (cruise_id, name, start_port, end_port, published_at)
    SELECT c.id, 'leg', 'port ' || md5('vs' || i), 'port ' || md5('ve' || i), now()
    FROM generate_series(1, 2000) i JOIN cruises c ON c.name = 'event ' || (i % 20 + 1);
INSERT INTO voyage_ports (voyage_id, name)
    SELECT v.id, 'port ' || md5('vp' || v.id) FROM voyages v;`

// TestSuggestPorts_UsesTrigramIndexes explains the suggestion query on
// seeded tables: every searched column must be read through its trigram
// index, not scanned, for a two-character query as for a longer one, as a
// custom and as a generic plan, with or without hidden records, both with
// the default planner settings and with sequential scans turned off.
func TestSuggestPorts_UsesTrigramIndexes(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := conn.Exec(portSearchSeed); err != nil {
		t.Fatalf("seed ports: %v", err)
	}
	if _, err := conn.Exec(`VACUUM ANALYZE cruises, trips, voyages, voyage_ports`); err != nil {
		t.Fatalf("vacuum: %v", err)
	}

	for _, seqScan := range []bool{true, false} {
		for _, planMode := range []string{"force_custom_plan", "force_generic_plan"} {
			for _, query := range []string{"he", "Świnoujście"} {
				for _, includeHidden := range []bool{false, true} {
					capture := &captureDB{}
					_, err := sqlcdb.New(capture).SuggestPorts(t.Context(), sqlcdb.SuggestPortsParams{
						Query: query, IncludeHidden: includeHidden, RowLimit: 8,
					})
					if !errors.Is(err, errCaptured) {
						t.Fatalf("capture suggestion query: %v", err)
					}
					plan := explainPlan(t, conn, seqScan, planMode, capture.query, capture.args)
					where := fmt.Sprintf("seqscan %v, %s, %q, include_hidden %v", seqScan, planMode, query, includeHidden)
					for _, index := range portSearchIndexes {
						if !strings.Contains(plan, "Bitmap Index Scan on "+index) {
							t.Errorf("%s: plan does not use %s:\n%s", where, index, plan)
						}
					}
					for _, table := range []string{"voyage_ports", "trips", "voyages"} {
						if strings.Contains(plan, "Seq Scan on "+table+" ") {
							t.Errorf("%s: plan scans %s:\n%s", where, table, plan)
						}
					}
				}
			}
		}
	}
}

// explainPlan returns the text plan of query in a transaction that sets
// enable_seqscan and plan_cache_mode.
func explainPlan(t *testing.T, conn *sql.DB, seqScan bool, planMode, query string, args []any) string {
	t.Helper()
	tx, err := conn.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`SELECT set_config('enable_seqscan', $1, true), set_config('plan_cache_mode', $2, true)`,
		strconv.FormatBool(seqScan), planMode); err != nil {
		t.Fatalf("set planner options: %v", err)
	}
	rows, err := tx.Query("EXPLAIN (COSTS OFF) "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var line string
		mustScan(t, rows, &line)
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	return plan.String()
}

// TestAddVoyagePort_Appends checks that a port added without a position
// goes after the voyage's last port, and that two writers appending at once
// get distinct positions: the second waits on the voyage's port lock the
// first holds, then sees the first one's port. A captain change on the
// voyage does not wait for that lock.
func TestAddVoyagePort_Appends(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cruiseID := insertPortSearchCruise(t, conn, true)
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)
	add := func(q sqlcdb.Querier, name string, position types.NullInt64) int64 {
		t.Helper()
		if err := q.LockVoyagePorts(t.Context(), voyageID); err != nil {
			t.Fatalf("lock voyage: %v", err)
		}
		p, err := q.AddVoyagePort(t.Context(), sqlcdb.AddVoyagePortParams{
			VoyageID: voyageID, Name: name, Position: position, CallerIsAdmin: true,
		})
		if err != nil {
			t.Fatalf("add %q: %v", name, err)
		}
		return p.Position
	}

	q := sqlcdb.New(conn)
	if got := add(q, "Hel", types.NullInt64{}); got != 0 {
		t.Fatalf("first port position = %d, want 0", got)
	}
	if got := add(q, "Jastarnia", types.NullInt64{Int64: 5, Valid: true}); got != 5 {
		t.Fatalf("explicit position = %d, want 5", got)
	}
	if got := add(q, "Puck", types.NullInt64{}); got != 6 {
		t.Fatalf("appended position = %d, want 6", got)
	}

	first, err := conn.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin first: %v", err)
	}
	defer func() { _ = first.Rollback() }()
	second, err := conn.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin second: %v", err)
	}
	defer func() { _ = second.Rollback() }()

	if got := add(sqlcdb.New(first), "Gdynia", types.NullInt64{}); got != 7 {
		t.Fatalf("first writer position = %d, want 7", got)
	}
	var secondPID int
	mustScan(t, second.QueryRow(`SELECT pg_backend_pid()`), &secondPID)
	done := make(chan int64, 1)
	go func() {
		q := sqlcdb.New(second)
		if err := q.LockVoyagePorts(context.Background(), voyageID); err != nil {
			done <- -1
			return
		}
		p, err := q.AddVoyagePort(context.Background(), sqlcdb.AddVoyagePortParams{
			VoyageID: voyageID, Name: "Gdańsk", CallerIsAdmin: true,
		})
		if err != nil {
			done <- -1
			return
		}
		done <- p.Position
	}()
	waitForLockWaiter(t, conn, secondPID)
	reassign, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(reassign, `UPDATE voyages SET captain_name = 'Nowy' WHERE id = $1`, voyageID); err != nil {
		t.Fatalf("update voyage while its ports are locked: %v", err)
	}
	if err := first.Commit(); err != nil {
		t.Fatalf("commit first: %v", err)
	}
	if got := <-done; got != 8 {
		t.Fatalf("second writer position = %d, want 8", got)
	}
}

// waitForLockWaiter waits until backend pid waits on a lock.
func waitForLockWaiter(t *testing.T, conn *sql.DB, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		mustScan(t, conn.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE pid = $1 AND wait_event_type = 'Lock')`, pid), &waiting)
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("backend %d never waited on the voyage's port lock", pid)
}
