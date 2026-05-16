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

// applyMigrationsThrough applies migrations in order up to and including the
// named file, recording each in schema_migrations so a later Migrate call
// continues from the next one.
func applyMigrationsThrough(t *testing.T, conn *sql.DB, last string) {
	t.Helper()
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, name := range embeddedMigrations(t) {
		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := conn.Exec(string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
		if _, err := conn.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			t.Fatalf("record migration %s: %v", name, err)
		}
		if name == last {
			return
		}
	}
	t.Fatalf("migration %s not found", last)
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
		"organizations", "org_members", "org_invites",
		"trips", "voyages", "cruises", "crew_assignments",
		"trip_enrollments", "cruise_enrollments", "voyage_opinions",
	} {
		assertTableExists(t, conn, table)
	}

	for _, index := range []string{
		"crew_assignments_trip_member_uniq",
		"crew_assignments_voyage_member_uniq",
		"idx_trips_org_id",
		"idx_voyages_org_id",
		"idx_cruises_org_id",
		"idx_trips_cruise_id",
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

// TestMigrate_PreservesDataAcrossSplit seeds pre-015 cruise, crew, assignment
// and opinion rows, then runs the cruise-splitting migrations and checks the
// data lands in the new trips/voyages tables.
func TestMigrate_PreservesDataAcrossSplit(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	applyMigrationsThrough(t, conn, "014_cruise_status.sql")

	var ownerID int64
	if err := conn.QueryRow(
		`INSERT INTO users (email, name, password_hash)
		 VALUES ('skipper@example.com', 'Skipper', 'x') RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var crewID int64
	if err := conn.QueryRow(
		`INSERT INTO crew_members (owner_id, full_name)
		 VALUES ($1, 'Jan Nowak') RETURNING id`, ownerID,
	).Scan(&crewID); err != nil {
		t.Fatalf("seed crew member: %v", err)
	}

	var voyageCruiseID int64
	if err := conn.QueryRow(
		`INSERT INTO cruises (owner_id, name, status)
		 VALUES ($1, 'Old Voyage', 'completed') RETURNING id`, ownerID,
	).Scan(&voyageCruiseID); err != nil {
		t.Fatalf("seed completed cruise: %v", err)
	}

	var tripCruiseID int64
	if err := conn.QueryRow(
		`INSERT INTO cruises (owner_id, name, status)
		 VALUES ($1, 'Old Trip', 'planned') RETURNING id`, ownerID,
	).Scan(&tripCruiseID); err != nil {
		t.Fatalf("seed planned cruise: %v", err)
	}

	if _, err := conn.Exec(
		`INSERT INTO crew_assignments (cruise_id, crew_member_id, role)
		 VALUES ($1, $2, 'skipper')`, voyageCruiseID, crewID,
	); err != nil {
		t.Fatalf("seed crew assignment: %v", err)
	}

	if _, err := conn.Exec(
		`INSERT INTO voyage_opinions (cruise_id, crew_member_id, file_path)
		 VALUES ($1, $2, 'opinions/old.pdf')`, voyageCruiseID, crewID,
	); err != nil {
		t.Fatalf("seed voyage opinion: %v", err)
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate through split: %v", err)
	}

	var voyageName string
	if err := conn.QueryRow(
		"SELECT name FROM voyages WHERE id = $1", voyageCruiseID,
	).Scan(&voyageName); err != nil {
		t.Fatalf("completed cruise did not become a voyage: %v", err)
	}
	if voyageName != "Old Voyage" {
		t.Errorf("voyage name = %q, want %q", voyageName, "Old Voyage")
	}

	var tripName string
	if err := conn.QueryRow(
		"SELECT name FROM trips WHERE id = $1", tripCruiseID,
	).Scan(&tripName); err != nil {
		t.Fatalf("planned cruise did not become a trip: %v", err)
	}
	if tripName != "Old Trip" {
		t.Errorf("trip name = %q, want %q", tripName, "Old Trip")
	}

	var (
		assignedVoyage sql.NullInt64
		assignedTrip   sql.NullInt64
	)
	if err := conn.QueryRow(
		"SELECT voyage_id, trip_id FROM crew_assignments WHERE crew_member_id = $1", crewID,
	).Scan(&assignedVoyage, &assignedTrip); err != nil {
		t.Fatalf("crew assignment lost across split: %v", err)
	}
	if !assignedVoyage.Valid || assignedVoyage.Int64 != voyageCruiseID {
		t.Errorf("crew assignment voyage_id = %v, want %d", assignedVoyage, voyageCruiseID)
	}
	if assignedTrip.Valid {
		t.Errorf("crew assignment trip_id = %v, want NULL", assignedTrip)
	}

	var opinionVoyage int64
	if err := conn.QueryRow(
		"SELECT voyage_id FROM voyage_opinions WHERE crew_member_id = $1", crewID,
	).Scan(&opinionVoyage); err != nil {
		t.Fatalf("voyage opinion lost across split: %v", err)
	}
	if opinionVoyage != voyageCruiseID {
		t.Errorf("voyage opinion voyage_id = %d, want %d", opinionVoyage, voyageCruiseID)
	}
}
