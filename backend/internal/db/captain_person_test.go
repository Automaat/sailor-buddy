package db

import (
	"database/sql"
	"testing"
)

const captainPersonMigration = "026_captain_person.sql"

func TestMigrate_CaptainBecomesPerson(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, captainPersonMigration)

	var cruise, withPerson, withoutPerson, person int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ('Bałtyk') RETURNING id`), &cruise)
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('anna@example.dev', 'Anna') RETURNING id`), &withPerson)
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('piotr@example.dev', 'Piotr') RETURNING id`), &withoutPerson)
	mustScan(t, conn.QueryRow(`INSERT INTO crew_members (user_id, full_name) VALUES ($1, 'Anna Nowak') RETURNING id`, withPerson), &person)

	insert := func(table string, captain any, name string) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO `+table+` (name, cruise_id, captain_user_id, captain_name)
			VALUES ('leg', $1, $2, $3) RETURNING id`, cruise, captain, name), &id)
		return id
	}
	trip := insert("trips", withPerson, "Kpt. Anna")
	voyage := insert("voyages", withPerson, "Kpt. Anna")
	orphanVoyage := insert("voyages", withoutPerson, "Kpt. Piotr")
	freeTextTrip := insert("trips", nil, "Kpt. Spoza Klubu")

	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	captain := func(table string, id int64) (sql.NullInt64, string) {
		t.Helper()
		var p sql.NullInt64
		var name string
		mustScan(t, conn.QueryRow(`SELECT captain_person_id, captain_name FROM `+table+` WHERE id = $1`, id), &p, &name)
		return p, name
	}
	tests := []struct {
		what, table string
		id          int64
		wantPerson  sql.NullInt64
		wantName    string
	}{
		{"trip of an account with a person", "trips", trip, sql.NullInt64{Int64: person, Valid: true}, "Kpt. Anna"},
		{"voyage of an account with a person", "voyages", voyage, sql.NullInt64{Int64: person, Valid: true}, "Kpt. Anna"},
		{"voyage of an account without a person", "voyages", orphanVoyage, sql.NullInt64{}, "Kpt. Piotr"},
		{"free-text trip", "trips", freeTextTrip, sql.NullInt64{}, "Kpt. Spoza Klubu"},
	}
	for _, tt := range tests {
		if p, name := captain(tt.table, tt.id); p != tt.wantPerson || name != tt.wantName {
			t.Errorf("%s: captain = (%v, %q), want (%v, %q)", tt.what, p, name, tt.wantPerson, tt.wantName)
		}
	}

	var userColumns int64
	mustScan(t, conn.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name IN ('trips', 'voyages') AND column_name = 'captain_user_id'`), &userColumns)
	if userColumns != 0 {
		t.Errorf("captain_user_id columns left = %d, want 0", userColumns)
	}
	assertIndexExists(t, conn, "idx_trips_captain_person_id")
	assertIndexExists(t, conn, "idx_voyages_captain_person_id")

	var unlinked int64
	mustScan(t, conn.QueryRow(`INSERT INTO crew_members (full_name) VALUES ('Kpt. Bez Konta') RETURNING id`), &unlinked)
	if _, err := conn.Exec(`UPDATE voyages SET captain_person_id = $1 WHERE id = $2`, unlinked, orphanVoyage); err != nil {
		t.Fatalf("assign person without an account: %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM crew_members WHERE id = $1`, unlinked); err != nil {
		t.Fatalf("delete captain person: %v", err)
	}
	if p, name := captain("voyages", orphanVoyage); p.Valid || name != "Kpt. Piotr" {
		t.Errorf("after deleting the captain person = (%v, %q), want (NULL, the kept name)", p, name)
	}
}
