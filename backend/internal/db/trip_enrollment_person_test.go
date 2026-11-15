package db

import (
	"database/sql"
	"strings"
	"testing"
)

const tripEnrollmentPersonMigration = "034_trip_enrollment_person.sql"

// tripEnrollmentFixture seeds, on the schema before migration 034, a leg
// and two accounts with their people, and signs both accounts up.
func tripEnrollmentFixture(t *testing.T) (conn *sql.DB, trip, annaPerson, olekPerson int64, query func(string, ...any) int64) {
	t.Helper()
	conn = testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, tripEnrollmentPersonMigration)
	query = func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(q, args...), &id)
		return id
	}
	cruise := query(`INSERT INTO cruises (name) VALUES ('Bałtyk 2026') RETURNING id`)
	trip = query(`INSERT INTO trips (cruise_id, name) VALUES ($1, 'Etap 1') RETURNING id`, cruise)
	anna := query(`INSERT INTO users (email, name) VALUES ('anna@example.dev', 'Anna') RETURNING id`)
	olek := query(`INSERT INTO users (email, name) VALUES ('olek@example.dev', 'Olek') RETURNING id`)
	annaPerson = query(`INSERT INTO crew_members (user_id, full_name) VALUES ($1, 'Anna Nowak') RETURNING id`, anna)
	olekPerson = query(`INSERT INTO crew_members (user_id, full_name) VALUES ($1, 'Olek Kowal') RETURNING id`, olek)
	query(`INSERT INTO trip_enrollments (trip_id, user_id, status, payment_status, note)
		VALUES ($1, $2, 'accepted', 'deposit', 'koja dziobowa') RETURNING id`, trip, anna)
	query(`INSERT INTO trip_enrollments (trip_id, user_id) VALUES ($1, $2) RETURNING id`, trip, olek)
	return conn, trip, annaPerson, olekPerson, query
}

// TestMigrate_TripEnrollmentPerson checks migration 034: each sign-up moves
// to its account's person with its status, payment and note, the user_id
// column goes, a person signs up once per leg, a person without an account
// can sign up, and deleting a person removes its sign-ups.
func TestMigrate_TripEnrollmentPerson(t *testing.T) {
	conn, trip, annaPerson, olekPerson, query := tripEnrollmentFixture(t)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	assertColumnAbsent(t, conn, "trip_enrollments", "user_id")
	type signUp struct {
		person                int64
		status, payment, note string
	}
	rows, err := conn.Query(`SELECT crew_member_id, status, payment_status, COALESCE(note, '')
		FROM trip_enrollments WHERE trip_id = $1 ORDER BY crew_member_id`, trip)
	if err != nil {
		t.Fatalf("list sign-ups: %v", err)
	}
	var got []signUp
	for rows.Next() {
		var s signUp
		if err := rows.Scan(&s.person, &s.status, &s.payment, &s.note); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, s)
	}
	_ = rows.Close()
	want := []signUp{
		{annaPerson, "accepted", "deposit", "koja dziobowa"},
		{olekPerson, "pending", "unpaid", ""},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sign-ups = %+v, want %+v", got, want)
	}

	assertPgError(t, "second sign-up of a person", "23505",
		execErr(conn.Exec(`INSERT INTO trip_enrollments (trip_id, crew_member_id) VALUES ($1, $2)`, trip, annaPerson)))
	assertPgError(t, "sign-up without a person", "23502",
		execErr(conn.Exec(`INSERT INTO trip_enrollments (trip_id) VALUES ($1)`, trip)))

	noAccount := query(`INSERT INTO crew_members (full_name) VALUES ('Ewa bez konta') RETURNING id`)
	query(`INSERT INTO trip_enrollments (trip_id, crew_member_id) VALUES ($1, $2) RETURNING id`, trip, noAccount)
	if _, err := conn.Exec(`DELETE FROM crew_members WHERE id = $1`, noAccount); err != nil {
		t.Fatalf("delete person: %v", err)
	}
	if n := query(`SELECT COUNT(*) FROM trip_enrollments WHERE crew_member_id = $1`, noAccount); n != 0 {
		t.Errorf("deleted person keeps %d sign-ups, want 0", n)
	}
	assertIndexExists(t, conn, "idx_trip_enrollments_crew_member_id")
}

// TestMigrate_TripEnrollmentPersonRefusesUnmappedSignUp checks migration 034
// stops, rather than drop a sign-up, when the account has no person.
func TestMigrate_TripEnrollmentPersonRefusesUnmappedSignUp(t *testing.T) {
	conn, trip, _, _, query := tripEnrollmentFixture(t)
	lost := query(`INSERT INTO users (email, name) VALUES ('bez-osoby@example.dev', 'X') RETURNING id`)
	query(`INSERT INTO trip_enrollments (trip_id, user_id) VALUES ($1, $2) RETURNING id`, trip, lost)

	err := Migrate(t.Context(), conn)
	if err == nil || !strings.Contains(err.Error(), "has no person") {
		t.Fatalf("migrate = %v, want the unmapped sign-up refused", err)
	}
	if n := query(`SELECT COUNT(*) FROM trip_enrollments WHERE user_id = $1`, lost); n != 1 {
		t.Errorf("unmapped sign-ups = %d, want 1 left untouched", n)
	}
}
