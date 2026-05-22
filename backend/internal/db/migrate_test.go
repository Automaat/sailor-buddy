package db

import (
	"database/sql"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

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

	if err := Migrate(conn); err != nil {
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
		"users", "refresh_tokens", "yachts", "crew_members", "trainings",
		"trips", "voyages", "cruises", "crew_assignments",
		"trip_enrollments", "cruise_enrollments", "voyage_opinions",
		"voyage_ports",
	} {
		assertTableExists(t, conn, table)
	}

	for _, index := range []string{
		"crew_assignments_trip_member_uniq",
		"crew_assignments_voyage_member_uniq",
		"idx_trips_cruise_id",
		"idx_voyages_cruise_id",
		"voyage_ports_voyage_id_idx",
	} {
		assertIndexExists(t, conn, index)
	}

	assertConstraintExists(t, conn, "crew_assignments_one_parent")
}

// TestMigrate_Idempotent verifies a second Migrate call against an
// already-migrated database is a no-op rather than an error.
func TestMigrate_Idempotent(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	if err := Migrate(conn); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("second migrate should be a no-op: %v", err)
	}
}
