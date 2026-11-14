package db

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDB connects to the Postgres instance named by SAILOR_TEST_DATABASE_URL.
// The migration smoke test needs a real server; without the env var (a local
// `go test ./...` run with no database) the test is skipped. CI sets it.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SAILOR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAILOR_TEST_DATABASE_URL not set; skipping migration smoke test")
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// resetSchema drops every object in the public schema so each test starts
// from an empty database.
func resetSchema(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

// embeddedMigrations returns every embedded migration filename in apply order.
func embeddedMigrations(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

func assertTableExists(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	if !exists {
		t.Errorf("expected table %q to exist", name)
	}
}

func assertTableAbsent(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_name = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	if exists {
		t.Errorf("expected table %q to be dropped", name)
	}
}

func assertColumnAbsent(t *testing.T, conn *sql.DB, table, column string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1
		   AND column_name = $2)`, table, column,
	).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if exists {
		t.Errorf("expected column %s.%s to be dropped", table, column)
	}
}

func assertIndexExists(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM pg_indexes
		 WHERE schemaname = 'public' AND indexname = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("check index %s: %v", name, err)
	}
	if !exists {
		t.Errorf("expected index %q to exist", name)
	}
}

func assertConstraintExists(t *testing.T, conn *sql.DB, name string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, name,
	).Scan(&exists); err != nil {
		t.Fatalf("check constraint %s: %v", name, err)
	}
	if !exists {
		t.Errorf("expected constraint %q to exist", name)
	}
}

// TestMigrate_FreshSchema runs every migration against an empty database and
// verifies the full embedded set is recorded and the expected schema lands.
func TestMigrate_FreshSchema(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate fresh schema: %v", err)
	}

	rows, err := conn.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var applied []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		applied = append(applied, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}

	want := embeddedMigrations(t)
	if strings.Join(applied, ",") != strings.Join(want, ",") {
		t.Fatalf("schema_migrations = %v, want %v", applied, want)
	}

	for _, table := range []string{
		"users", "yachts", "crew_members", "trainings",
		"trips", "voyages", "cruises", "crew_assignments",
		"trip_enrollments", "cruise_enrollments", "voyage_opinions",
		"voyage_ports", "cruise_series",
	} {
		assertTableExists(t, conn, table)
	}

	for _, index := range []string{
		"crew_assignments_trip_member_uniq",
		"crew_assignments_voyage_member_uniq",
		"idx_trips_cruise_id",
		"idx_voyages_cruise_id",
		"voyage_ports_voyage_id_idx",
		"idx_cruises_series_id",
	} {
		assertIndexExists(t, conn, index)
	}

	assertConstraintExists(t, conn, "crew_assignments_one_parent")

	assertTableAbsent(t, conn, "refresh_tokens")
	assertColumnAbsent(t, conn, "users", "password_hash")
}

// TestMigrate_Idempotent verifies a second Migrate call against an
// already-migrated database is a no-op rather than an error.
func TestMigrate_Idempotent(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}
}

// openPool opens a separate *sql.DB on the test database, standing in for a
// second process that shares the database.
func openPool(t *testing.T) *sql.DB {
	t.Helper()
	pool, err := sql.Open("pgx", os.Getenv("SAILOR_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

// appliedCounts maps each recorded migration version to its row count.
func appliedCounts(t *testing.T, conn *sql.DB) map[string]int {
	t.Helper()
	rows, err := conn.Query("SELECT version, COUNT(*) FROM schema_migrations GROUP BY version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	counts := map[string]int{}
	for rows.Next() {
		var version string
		var n int
		if err := rows.Scan(&version, &n); err != nil {
			t.Fatalf("scan schema_migrations: %v", err)
		}
		counts[version] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}
	return counts
}

// TestMigrate_Concurrent runs Migrate from several processes at once against
// an empty schema: every run succeeds and each migration lands exactly once.
func TestMigrate_Concurrent(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	const runners = 4
	pools := make([]*sql.DB, runners)
	for i := range pools {
		pools[i] = openPool(t)
	}

	start := make(chan struct{})
	errs := make(chan error, runners)
	var wg sync.WaitGroup
	for _, pool := range pools {
		wg.Go(func() {
			<-start
			errs <- Migrate(t.Context(), pool)
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent migrate: %v", err)
		}
	}

	counts := appliedCounts(t, conn)
	want := embeddedMigrations(t)
	if len(counts) != len(want) {
		t.Errorf("schema_migrations has %d versions, want %d", len(counts), len(want))
	}
	for _, version := range want {
		if counts[version] != 1 {
			t.Errorf("version %s recorded %d times, want 1", version, counts[version])
		}
	}
}

// TestMigrate_WaitsForLockHolder holds the migration lock from another
// session and checks Migrate applies nothing until that session lets go,
// then releases the lock itself when done.
func TestMigrate_WaitsForLockHolder(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	holder, err := openPool(t).Conn(t.Context())
	if err != nil {
		t.Fatalf("holder conn: %v", err)
	}
	defer func() { _ = holder.Close() }()
	if _, err := holder.ExecContext(t.Context(), "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		t.Fatalf("hold lock: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- Migrate(t.Context(), openPool(t)) }()

	waitForAdvisoryWaiter(t, conn)
	var tables int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public'`).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 0 {
		t.Fatalf("Migrate created %d tables while the lock was held elsewhere", tables)
	}

	var released bool
	if err := holder.QueryRowContext(t.Context(), "SELECT pg_advisory_unlock($1)", migrationLockID).Scan(&released); err != nil || !released {
		t.Fatalf("release lock: released=%v err=%v", released, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("migrate after lock release: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Migrate did not finish after the lock was released")
	}

	var acquired bool
	if err := holder.QueryRowContext(t.Context(), "SELECT pg_try_advisory_lock($1)", migrationLockID).Scan(&acquired); err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if !acquired {
		t.Fatal("Migrate kept the migration lock after returning")
	}
	if _, err := holder.ExecContext(t.Context(), "SELECT pg_advisory_unlock($1)", migrationLockID); err != nil {
		t.Fatalf("release probe lock: %v", err)
	}
}

// TestMigrate_CanceledWhileWaiting cancels the context of a Migrate blocked
// on the lock: it returns the error instead of hanging.
func TestMigrate_CanceledWhileWaiting(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	holder, err := openPool(t).Conn(t.Context())
	if err != nil {
		t.Fatalf("holder conn: %v", err)
	}
	defer func() { _ = holder.Close() }()
	if _, err := holder.ExecContext(t.Context(), "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		t.Fatalf("hold lock: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- Migrate(ctx, openPool(t)) }()
	waitForAdvisoryWaiter(t, conn)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Migrate returned nil after its context was canceled")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Migrate ignored context cancellation while waiting for the lock")
	}
}

// waitForAdvisoryWaiter blocks until some session waits on the migration
// advisory lock.
func waitForAdvisoryWaiter(t *testing.T, conn *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM pg_locks
			WHERE locktype = 'advisory' AND NOT granted AND objid = $1`,
			migrationLockID&0xffffffff).Scan(&waiting); err != nil {
			t.Fatalf("query pg_locks: %v", err)
		}
		if waiting > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no session waited on the migration lock")
}
