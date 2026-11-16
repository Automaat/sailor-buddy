package db

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

const countryCodesMigration = "038_country_codes.sql"

// countryCodesFixture applies the migrations before 038 and returns an
// insert helper that answers the new row's id.
func countryCodesFixture(t *testing.T) (conn *sql.DB, insert func(string, ...any) int64) {
	t.Helper()
	conn = testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, countryCodesMigration)
	insert = func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		mustScan(t, conn.QueryRow(q, args...), &id)
		return id
	}
	return conn, insert
}

func countriesOf(t *testing.T, conn *sql.DB, table string, id int64) []string {
	t.Helper()
	var got types.TextArray
	mustScan(t, conn.QueryRow(`SELECT countries FROM `+table+` WHERE id = $1`, id), &got)
	return got
}

// TestMigrate_CountryCodes checks migration 038 turns the free-text
// countries of editions, legs and voyages into lists of codes: production's
// "Norwegia" and "dania", lists split on separators, Polish and English
// names with any case and accents, names that contain " i " and empty
// values.
func TestMigrate_CountryCodes(t *testing.T) {
	conn, insert := countryCodesFixture(t)
	cruise := func(countries any) int64 {
		return insert(`INSERT INTO cruises (name, countries) VALUES ('Edycja', $1) RETURNING id`, countries)
	}
	norway := cruise("Norwegia")
	nordic := cruise("Norwegia, Szwecja")
	cruiseNone := cruise(nil)
	cruiseBlank := cruise("  ")

	trip := func(countries any) int64 {
		return insert(`INSERT INTO trips (cruise_id, name, countries) VALUES ($1, 'Etap', $2) RETURNING id`, norway, countries)
	}
	tripNorway := trip("Norwegia")
	balkans := trip("Bośnia i Hercegowina; CHORWACJA")
	adriatic := trip("Croatia and Montenegro")
	codes := trip("pl / se / PL")
	poland := trip("polska")

	voyage := func(countries any) int64 {
		return insert(`INSERT INTO voyages (cruise_id, name, countries) VALUES ($1, 'Rejs', $2) RETURNING id`, norway, countries)
	}
	denmark := voyage("dania")
	aegean := voyage("Grecja i Turcja")
	trinidad := voyage("Trinidad & Tobago")
	voyageNone := voyage(nil)

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	tests := []struct {
		table string
		id    int64
		want  []string
	}{
		{"cruises", norway, []string{"NO"}},
		{"cruises", nordic, []string{"NO", "SE"}},
		{"cruises", cruiseNone, []string{}},
		{"cruises", cruiseBlank, []string{}},
		{"trips", tripNorway, []string{"NO"}},
		{"trips", balkans, []string{"BA", "HR"}},
		{"trips", adriatic, []string{"HR", "ME"}},
		{"trips", codes, []string{"PL", "SE"}},
		{"trips", poland, []string{"PL"}},
		{"voyages", denmark, []string{"DK"}},
		{"voyages", aegean, []string{"GR", "TR"}},
		{"voyages", trinidad, []string{"TT"}},
		{"voyages", voyageNone, []string{}},
	}
	for _, tt := range tests {
		if got := countriesOf(t, conn, tt.table, tt.id); !slices.Equal(got, tt.want) {
			t.Errorf("%s %d countries = %q, want %q", tt.table, tt.id, got, tt.want)
		}
	}

	assertTableAbsent(t, conn, "country_names_038")
	for _, table := range []string{"cruises", "trips", "voyages"} {
		assertConstraintExists(t, conn, table+"_countries_codes")
	}
	var helpers int64
	mustScan(t, conn.QueryRow(`SELECT COUNT(*) FROM pg_proc WHERE proname LIKE 'country_%_038'`), &helpers)
	if helpers != 0 {
		t.Errorf("migration left %d helper functions", helpers)
	}
}

// TestMigrate_CountryCodesColumns checks the columns after migration 038:
// a new row gets an empty list, NULL and malformed codes are refused, and
// the sqlc queries store and read a list.
func TestMigrate_CountryCodesColumns(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var cruise int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ('Edycja') RETURNING id`), &cruise)
	if got := countriesOf(t, conn, "cruises", cruise); len(got) != 0 {
		t.Errorf("new cruise countries = %q, want empty", got)
	}

	for _, bad := range []string{`NULL`, `ARRAY['no']`, `ARRAY['NOR']`, `ARRAY['NO', NULL]`, `ARRAY[['NO'], ['DK']]`} {
		_, err := conn.Exec(`UPDATE cruises SET countries = `+bad+` WHERE id = $1`, cruise)
		want := "23514"
		if bad == "NULL" {
			want = "23502"
		}
		assertPgError(t, "countries = "+bad, want, err)
	}

	q := sqlcdb.New(conn)
	trip, err := q.CreateTrip(t.Context(), sqlcdb.CreateTripParams{
		CruiseID: cruise, Name: "Etap", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
		Countries: types.TextArray{"HR", "ME"},
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if !slices.Equal(trip.Countries, types.TextArray{"HR", "ME"}) {
		t.Errorf("created trip countries = %q, want [HR ME]", trip.Countries)
	}
	got, err := q.GetTrip(t.Context(), sqlcdb.GetTripParams{ID: trip.ID, IncludeHidden: true})
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	if !slices.Equal(got.Countries, types.TextArray{"HR", "ME"}) {
		t.Errorf("read trip countries = %q, want [HR ME]", got.Countries)
	}

	bare, err := q.CreateTrip(t.Context(), sqlcdb.CreateTripParams{
		CruiseID: cruise, Name: "Bez krajów", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create trip without countries: %v", err)
	}
	if bare.Countries == nil || len(bare.Countries) != 0 {
		t.Errorf("trip without countries = %#v, want an empty, non-nil list", bare.Countries)
	}
}

// TestMigrate_CountryCodesRefusesUnknownName checks migration 038 stops,
// naming the row and the value, rather than drop a country it cannot map.
func TestMigrate_CountryCodesRefusesUnknownName(t *testing.T) {
	conn, insert := countryCodesFixture(t)
	cruise := insert(`INSERT INTO cruises (name, countries) VALUES ('Edycja', 'Norwegia') RETURNING id`)
	voyage := insert(`INSERT INTO voyages (cruise_id, name, countries) VALUES ($1, 'Rejs', 'Dania, Atlantyda') RETURNING id`, cruise)

	err := Migrate(t.Context(), conn)
	if err == nil || !strings.Contains(err.Error(), `"Atlantyda" is not a known country`) ||
		!strings.Contains(err.Error(), "voyage ") {
		t.Fatalf("migrate = %v, want the unknown country refused", err)
	}
	var kept string
	mustScan(t, conn.QueryRow(`SELECT countries FROM voyages WHERE id = $1`, voyage), &kept)
	if kept != "Dania, Atlantyda" {
		t.Errorf("voyage countries = %q, want the text left untouched", kept)
	}
}
