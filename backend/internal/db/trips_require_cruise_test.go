package db

import (
	"database/sql"
	"errors"
	"io/fs"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

const requireCruiseMigration = "020_trips_require_cruise.sql"

func TestMigrate_WrapsStandaloneTripsAndVoyages(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)

	applyMigrationsBefore(t, conn, requireCruiseMigration)

	var eventID, linkedTrip, standaloneTrip, standaloneVoyage int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ('NNE 2025') RETURNING id`), &eventID)
	mustScan(t, conn.QueryRow(`INSERT INTO trips (name, cruise_id) VALUES ('Leg 1', $1) RETURNING id`, eventID), &linkedTrip)
	mustScan(t, conn.QueryRow(`INSERT INTO trips (name, embark_date, start_port, currency,
		cost_per_person, cost_per_person_non_member, yacht_fund, deposit)
		VALUES ('Weekend', '2026-05-01', 'Gdynia', 'EUR', 760, 800, 150, 100) RETURNING id`), &standaloneTrip)
	mustScan(t, conn.QueryRow(`INSERT INTO voyages (name, year, miles)
		VALUES ('Rejs 2019', 2019, 120) RETURNING id`), &standaloneVoyage)

	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var linkedCruise int64
	mustScan(t, conn.QueryRow(`SELECT cruise_id FROM trips WHERE id = $1`, linkedTrip), &linkedCruise)
	if linkedCruise != eventID {
		t.Errorf("linked trip moved to cruise %d, want %d", linkedCruise, eventID)
	}

	var name, startPort, currency string
	mustScan(t, conn.QueryRow(`SELECT c.name, c.start_port, c.currency FROM trips t
		JOIN cruises c ON c.id = t.cruise_id WHERE t.id = $1`, standaloneTrip), &name, &startPort, &currency)
	if name != "Weekend" || startPort != "Gdynia" || currency != "EUR" {
		t.Errorf("standalone trip wrapped in cruise (%q, %q, %q), want (Weekend, Gdynia, EUR)", name, startPort, currency)
	}
	var member, nonMember, fund, deposit float64
	mustScan(t, conn.QueryRow(`SELECT c.cost_per_person, c.cost_per_person_non_member, c.yacht_fund, c.deposit
		FROM trips t JOIN cruises c ON c.id = t.cruise_id WHERE t.id = $1`, standaloneTrip), &member, &nonMember, &fund, &deposit)
	if member != 760 || nonMember != 800 || fund != 150 || deposit != 100 {
		t.Errorf("wrapped cruise prices = (%v, %v, %v, %v), want (760, 800, 150, 100)", member, nonMember, fund, deposit)
	}
	mustScan(t, conn.QueryRow(`SELECT c.name FROM voyages v
		JOIN cruises c ON c.id = v.cruise_id WHERE v.id = $1`, standaloneVoyage), &name)
	if name != "Rejs 2019" {
		t.Errorf("standalone voyage wrapped in cruise %q, want Rejs 2019", name)
	}

	var cruises int
	mustScan(t, conn.QueryRow(`SELECT COUNT(*) FROM cruises`), &cruises)
	if cruises != 3 {
		t.Errorf("cruises = %d, want 3 (event plus one per standalone row)", cruises)
	}

	assertPgError(t, "trip without cruise", "23502",
		execErr(conn.Exec(`INSERT INTO trips (name) VALUES ('Orphan')`)))
	assertPgError(t, "voyage without cruise", "23502",
		execErr(conn.Exec(`INSERT INTO voyages (name) VALUES ('Orphan')`)))
	assertPgError(t, "delete cruise with legs", "23503",
		execErr(conn.Exec(`DELETE FROM cruises WHERE id = $1`, eventID)))
}

type scanner interface{ Scan(dest ...any) error }

// applyMigrationsBefore applies and records every embedded migration that
// sorts before stop, leaving the database one step short of it.
func applyMigrationsBefore(t *testing.T, conn *sql.DB, stop string) {
	t.Helper()
	if _, err := conn.Exec(`CREATE TABLE schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, name := range embeddedMigrations(t) {
		if name >= stop {
			break
		}
		body, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := conn.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		if _, err := conn.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			t.Fatalf("record %s: %v", name, err)
		}
	}
}

func mustScan(t *testing.T, row scanner, dest ...any) {
	t.Helper()
	if err := row.Scan(dest...); err != nil {
		t.Fatalf("scan: %v", err)
	}
}

func execErr(_ any, err error) error { return err }

func assertPgError(t *testing.T, what, code string, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("%s: got %v, want SQLSTATE %s", what, err, code)
	}
}
