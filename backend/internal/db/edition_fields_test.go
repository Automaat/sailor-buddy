package db

import "testing"

const editionFieldsMigration = "033_edition_fields.sql"

// TestMigrate_EditionFields checks migration 033: a series becomes inland
// when any of its editions was inland, other series and new ones default to
// sea, and the cruise columns that moved to the legs (or were dropped) are
// gone while the edition-wide ones keep their data.
func TestMigrate_EditionFields(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, editionFieldsMigration)

	series := func(name string) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO cruise_series (name) VALUES ($1) RETURNING id`, name), &id)
		return id
	}
	edition := func(seriesID any, name, kind string) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO cruises (name, series_id, edition, kind, currency,
			embark_date, start_port, max_crew, cost_per_person, group_chat_url, travel_notes)
			VALUES ($2, $1, '2025', $3, 'EUR', '2025-05-01', 'Gdynia', 8, 900,
				'https://chat.example/crew', 'Pociąg do Gdyni') RETURNING id`,
			seriesID, name, kind), &id)
		return id
	}
	inland := series("Mazury")
	edition(inland, "Mazury 2025", "inland")
	mixed := series("Mixed")
	edition(mixed, "Mixed sea", "sea")
	edition(mixed, "Mixed inland", "inland")
	sea := series("Bałtyk")
	kept := edition(sea, "Bałtyk 2025", "sea")
	empty := series("Planned")
	edition(nil, "Standalone inland", "inland")

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tests := []struct {
		what string
		id   int64
		want string
	}{
		{"series with an inland edition", inland, "inland"},
		{"series with sea and inland editions", mixed, "inland"},
		{"series with sea editions", sea, "sea"},
		{"series without editions", empty, "sea"},
	}
	for _, tt := range tests {
		var kind string
		mustScan(t, conn.QueryRow(`SELECT kind FROM cruise_series WHERE id = $1`, tt.id), &kind)
		if kind != tt.want {
			t.Errorf("%s: kind = %q, want %q", tt.what, kind, tt.want)
		}
	}

	var kind string
	mustScan(t, conn.QueryRow(`INSERT INTO cruise_series (name) VALUES ('New') RETURNING kind`), &kind)
	if kind != "sea" {
		t.Errorf("new series kind = %q, want sea", kind)
	}
	assertPgError(t, "unknown series kind", "23514",
		execErr(conn.Exec(`INSERT INTO cruise_series (name, kind) VALUES ('Bad', 'lake')`)))

	for _, column := range []string{
		"kind", "embark_date", "disembark_date", "start_port", "end_port", "max_crew",
		"cost_per_person", "cost_per_person_non_member", "currency", "yacht_fund", "deposit",
		"group_chat_url",
	} {
		assertColumnAbsent(t, conn, "cruises", column)
	}

	var name, notes string
	mustScan(t, conn.QueryRow(`SELECT name, travel_notes FROM cruises WHERE id = $1`, kept), &name, &notes)
	if name != "Bałtyk 2025" || notes != "Pociąg do Gdyni" {
		t.Errorf("kept cruise = (%q, %q), want (Bałtyk 2025, Pociąg do Gdyni)", name, notes)
	}
}
