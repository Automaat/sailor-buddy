package db

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

const portSearchMigration = "036_port_search.sql"

// portSearchIndexes are the trigram indexes behind the port suggestions,
// one per searched column.
var portSearchIndexes = []string{
	"voyage_ports_name_trgm_idx",
	"trips_start_port_trgm_idx",
	"trips_end_port_trgm_idx",
	"voyages_start_port_trgm_idx",
	"voyages_end_port_trgm_idx",
}

// TestMigrate_PortSearch checks migration 036 on a database that already
// holds ports: it installs both extensions, the IMMUTABLE normalizer and the
// trigram index, and the existing ports become searchable.
func TestMigrate_PortSearch(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, portSearchMigration)

	cruiseID := insertPortSearchCruise(t, conn, true)
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)
	insertPortSearchPort(t, conn, voyageID, "Świnoujście", 53.91, 14.25)

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	assertPortSearch(t, conn, "public")
}

// TestMigrate_PortSearchExtensionsSchema runs every migration on a fresh
// database where both extensions were installed in their own schema, off
// search_path, beforehand, which CREATE EXTENSION IF NOT EXISTS keeps. The
// suggestions then run with the default search_path.
func TestMigrate_PortSearchExtensionsSchema(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	dropExtensionsSchema := func() {
		if _, err := conn.Exec(`DROP SCHEMA IF EXISTS extensions CASCADE`); err != nil {
			t.Fatalf("drop extensions schema: %v", err)
		}
	}
	dropExtensionsSchema()
	t.Cleanup(dropExtensionsSchema)
	if _, err := conn.Exec(`CREATE SCHEMA extensions;
		CREATE EXTENSION pg_trgm SCHEMA extensions;
		CREATE EXTENSION unaccent SCHEMA extensions;`); err != nil {
		t.Fatalf("install extensions in extensions: %v", err)
	}

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var searchPath string
	mustScan(t, conn.QueryRow(`SHOW search_path`), &searchPath)
	if strings.Contains(searchPath, "extensions") {
		t.Fatalf("search_path = %q, want it without the extensions schema", searchPath)
	}

	cruiseID := insertPortSearchCruise(t, conn, true)
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)
	insertPortSearchPort(t, conn, voyageID, "Świnoujście", 53.91, 14.25)
	assertPortSearch(t, conn, "extensions")
}

// TestMigrate_PortSearchUpgrade checks a database that applied 036 as first
// released, with unaccent hard-coded in public: later migrations apply on
// top of it and port search keeps working.
func TestMigrate_PortSearchUpgrade(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, portSearchMigration)
	if _, err := conn.Exec(`
		CREATE EXTENSION IF NOT EXISTS pg_trgm;
		CREATE EXTENSION IF NOT EXISTS unaccent;
		CREATE FUNCTION port_search_norm(name TEXT) RETURNS TEXT
			LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
			AS $$ SELECT lower(public.unaccent('public.unaccent'::regdictionary, btrim($1))) $$;
		CREATE INDEX voyage_ports_name_trgm_idx
			ON voyage_ports USING gin (port_search_norm(name) gin_trgm_ops);`); err != nil {
		t.Fatalf("apply released 036: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", portSearchMigration); err != nil {
		t.Fatalf("record 036: %v", err)
	}

	cruiseID := insertPortSearchCruise(t, conn, true)
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)
	insertPortSearchPort(t, conn, voyageID, "Świnoujście", 53.91, 14.25)

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	assertPortSearch(t, conn, "public")
}

// assertPortSearch checks the port search objects of migrations 036 and 037
// against both extensions installed in schema, and that a stored
// "Świnoujście" is suggested for queries typed without diacritics.
func assertPortSearch(t *testing.T, conn *sql.DB, schema string) {
	t.Helper()
	var exts []string
	rows, err := conn.Query(`SELECT extname FROM pg_extension WHERE extname IN ('pg_trgm', 'unaccent') ORDER BY extname`)
	if err != nil {
		t.Fatalf("list extensions: %v", err)
	}
	for rows.Next() {
		var name string
		mustScan(t, rows, &name)
		exts = append(exts, name)
	}
	_ = rows.Close()
	if !reflect.DeepEqual(exts, []string{"pg_trgm", "unaccent"}) {
		t.Fatalf("extensions = %v, want [pg_trgm unaccent]", exts)
	}
	for _, ext := range exts {
		var extSchema string
		mustScan(t, conn.QueryRow(`SELECT extnamespace::regnamespace::TEXT FROM pg_extension
			WHERE extname = $1`, ext), &extSchema)
		if extSchema != schema {
			t.Fatalf("%s schema = %q, want %q", ext, extSchema, schema)
		}
	}

	var volatility, src string
	mustScan(t, conn.QueryRow(`SELECT provolatile, prosrc FROM pg_proc WHERE proname = 'port_search_norm'`),
		&volatility, &src)
	if volatility != "i" {
		t.Fatalf("port_search_norm volatility = %q, want immutable (i)", volatility)
	}
	wantSrc := " SELECT lower(" + schema + ".unaccent('" + schema + ".unaccent'::regdictionary, btrim($1))) "
	if src != wantSrc {
		t.Fatalf("port_search_norm body = %q, want %q", src, wantSrc)
	}
	var folded string
	mustScan(t, conn.QueryRow(`SELECT port_search_norm('  Świnoujście ')`), &folded)
	if folded != "swinoujscie" {
		t.Fatalf("port_search_norm = %q, want swinoujscie", folded)
	}
	for _, index := range portSearchIndexes {
		var opclass string
		mustScan(t, conn.QueryRow(`SELECT opc.opcnamespace::regnamespace || '.' || opc.opcname
			FROM pg_index i JOIN pg_opclass opc ON opc.oid = i.indclass[0]
			WHERE i.indexrelid = $1::regclass`, index), &opclass)
		if opclass != schema+".gin_trgm_ops" {
			t.Fatalf("index %s operator class = %q, want %s.gin_trgm_ops", index, opclass, schema)
		}
	}
	var matchSrc string
	mustScan(t, conn.QueryRow(`SELECT prosrc FROM pg_proc WHERE proname = 'port_search_matches'`), &matchSrc)
	if !strings.Contains(matchSrc, "OPERATOR("+schema+".%)") || !strings.Contains(matchSrc, "OPERATOR("+schema+".<%)") {
		t.Fatalf("port_search_matches body = %q, want operators of schema %s", matchSrc, schema)
	}

	for _, query := range []string{"swinou", "Swinoujscie"} {
		got, err := sqlcdb.New(conn).SuggestPorts(t.Context(), sqlcdb.SuggestPortsParams{
			Query: query, IncludeHidden: false, RowLimit: 8,
		})
		if err != nil {
			t.Fatalf("suggest %q: %v", query, err)
		}
		if len(got) != 1 || got[0].Name != "Świnoujście" {
			t.Fatalf("suggest %q = %+v, want Świnoujście", query, got)
		}
	}
}

func TestSuggestPorts(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	publishedCruise := insertPortSearchCruise(t, conn, true)
	hiddenCruise := insertPortSearchCruise(t, conn, false)
	visible := insertPortSearchVoyage(t, conn, publishedCruise, true)
	visible2 := insertPortSearchVoyage(t, conn, publishedCruise, true)
	hiddenVoyage := insertPortSearchVoyage(t, conn, publishedCruise, false)
	hiddenByCruise := insertPortSearchVoyage(t, conn, hiddenCruise, true)

	insertPortSearchPort(t, conn, visible, "Świnoujście", 53.91, 14.25)
	insertPortSearchPort(t, conn, visible2, "Swinoujscie", 53.92, 14.26)
	insertPortSearchPort(t, conn, visible2, "Świnoujście", 53.93, 14.27)
	insertPortSearchPort(t, conn, visible, "Jastarnia", 54.70, 18.68)
	insertPortSearchPort(t, conn, visible, "Svolvær", 68.23, 14.57)
	insertPortSearchPort(t, conn, visible, "Hel", 54.61, 18.80)
	insertPortSearchPort(t, conn, visible2, "Hel", 54.60, 18.79)
	insertPortSearchNameOnly(t, conn, visible2, "Hel")
	insertPortSearchPort(t, conn, visible, "Helsinki", 60.17, 24.94)
	insertPortSearchNameOnly(t, conn, visible, "Zatoka Pucka")
	insertPortSearchPort(t, conn, hiddenVoyage, "Władysławowo", 54.79, 18.41)
	insertPortSearchPort(t, conn, hiddenByCruise, "Svendborg", 55.06, 10.61)

	// Start and end ports of trips and voyages, entered without coordinates.
	insertPortSearchLeg(t, conn, "trips", publishedCruise, true, "Kołobrzeg", "Hel")
	insertPortSearchLeg(t, conn, "trips", publishedCruise, true, "", "Stara Ustka")
	insertPortSearchLeg(t, conn, "voyages", publishedCruise, true, "Ustka Port Jachtowy Wschodni", "Łeba")
	insertPortSearchLeg(t, conn, "trips", publishedCruise, false, "Rowy", "")
	insertPortSearchLeg(t, conn, "trips", hiddenCruise, true, "", "Dziwnów")
	insertPortSearchLeg(t, conn, "voyages", publishedCruise, false, "Mielno", "")
	insertPortSearchLeg(t, conn, "voyages", hiddenCruise, true, "", "Darłowo")

	type row struct {
		name   string
		coords bool
		lat    float64
		uses   int64
	}
	tests := []struct {
		name          string
		query         string
		includeHidden bool
		limit         int32
		want          []row
	}{
		{"prefix without diacritics", "swinou", false, 8, []row{{"Świnoujście", true, 53.93, 3}}},
		{"prefix in upper case", "JASTARN", false, 8, []row{{"Jastarnia", true, 54.70, 1}}},
		{"typo", "Svolvar", false, 8, []row{{"Svolvær", true, 68.23, 1}}},
		{"exact before longer prefix match", "hel", false, 8, []row{
			{"Hel", true, 54.60, 4},
			{"Helsinki", true, 60.17, 1},
		}},
		{"name-only port", "zatoka", false, 8, []row{{"Zatoka Pucka", false, 0, 1}}},
		{"limit", "hel", false, 1, []row{{"Hel", true, 54.60, 4}}},
		{"trip start port only", "KOLOBRZ", false, 8, []row{{"Kołobrzeg", false, 0, 1}}},
		{"trip start port with accents", "kołob", false, 8, []row{{"Kołobrzeg", false, 0, 1}}},
		{"two-character query", "ko", false, 8, []row{{"Kołobrzeg", false, 0, 1}}},
		{"voyage end port", "leba", false, 8, []row{{"Łeba", false, 0, 1}}},
		{"prefix before a more similar name", "ustka", false, 8, []row{
			{"Ustka Port Jachtowy Wschodni", false, 0, 1},
			{"Stara Ustka", false, 0, 1},
		}},
		{"hidden trip not suggested to members", "rowy", false, 8, nil},
		{"hidden trip suggested with include_hidden", "rowy", true, 8, []row{{"Rowy", false, 0, 1}}},
		{"trip of a hidden cruise not suggested to members", "dziwn", false, 8, nil},
		{"trip of a hidden cruise suggested with include_hidden", "dziwn", true, 8, []row{{"Dziwnów", false, 0, 1}}},
		{"hidden voyage start port not suggested to members", "mieln", false, 8, nil},
		{"hidden voyage start port suggested with include_hidden", "mieln", true, 8, []row{{"Mielno", false, 0, 1}}},
		{"voyage of a hidden cruise not suggested to members", "darlo", false, 8, nil},
		{"voyage of a hidden cruise suggested with include_hidden", "darlo", true, 8, []row{{"Darłowo", false, 0, 1}}},
		{"LIKE wildcard matches literally", "k%", false, 8, nil},
		{"LIKE single-character wildcard matches literally", "k_", false, 8, nil},
		{"hidden voyage not suggested to members", "wlady", false, 8, nil},
		{"hidden voyage suggested with include_hidden", "wlady", true, 8, []row{{"Władysławowo", true, 54.79, 1}}},
		{"hidden cruise not suggested to members", "svend", false, 8, nil},
		{"hidden cruise suggested with include_hidden", "svend", true, 8, []row{{"Svendborg", true, 55.06, 1}}},
		{"no match", "Malaga", false, 8, nil},
	}
	q := sqlcdb.New(conn)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := q.SuggestPorts(t.Context(), sqlcdb.SuggestPortsParams{
				Query: tt.query, IncludeHidden: tt.includeHidden, RowLimit: tt.limit,
			})
			if err != nil {
				t.Fatalf("suggest: %v", err)
			}
			var rows []row
			for _, g := range got {
				if g.Latitude.Valid != g.Longitude.Valid {
					t.Fatalf("unpaired coordinates: %+v", g)
				}
				rows = append(rows, row{g.Name, g.Latitude.Valid, g.Latitude.Float64, g.Uses})
			}
			if !reflect.DeepEqual(rows, tt.want) {
				t.Fatalf("suggest %q = %+v, want %+v", tt.query, rows, tt.want)
			}
		})
	}
}

func insertPortSearchCruise(t *testing.T, conn *sql.DB, published bool) int64 {
	t.Helper()
	var id int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name, published_at)
		VALUES ('event', CASE WHEN $1::BOOLEAN THEN CURRENT_TIMESTAMP END) RETURNING id`, published), &id)
	return id
}

func insertPortSearchVoyage(t *testing.T, conn *sql.DB, cruiseID int64, published bool) int64 {
	t.Helper()
	var id int64
	mustScan(t, conn.QueryRow(`INSERT INTO voyages (cruise_id, name, published_at)
		VALUES ($1, 'leg', CASE WHEN $2::BOOLEAN THEN CURRENT_TIMESTAMP END) RETURNING id`, cruiseID, published), &id)
	return id
}

// insertPortSearchLeg inserts a trip or a voyage (table) with the given
// start and end ports; an empty port is NULL.
func insertPortSearchLeg(t *testing.T, conn *sql.DB, table string, cruiseID int64, published bool, start, end string) {
	t.Helper()
	query := `INSERT INTO trips (cruise_id, name, start_port, end_port, published_at)
		VALUES ($1, 'leg', NULLIF($2, ''), NULLIF($3, ''), CASE WHEN $4::BOOLEAN THEN CURRENT_TIMESTAMP END)`
	if table == "voyages" {
		query = strings.Replace(query, "trips", "voyages", 1)
	}
	if _, err := conn.Exec(query, cruiseID, start, end, published); err != nil {
		t.Fatalf("insert %s %q-%q: %v", table, start, end, err)
	}
}

func insertPortSearchPort(t *testing.T, conn *sql.DB, voyageID int64, name string, lat, lon float64) {
	t.Helper()
	if _, err := conn.Exec(`INSERT INTO voyage_ports (voyage_id, name, latitude, longitude)
		VALUES ($1, $2, $3, $4)`, voyageID, name, lat, lon); err != nil {
		t.Fatalf("insert port %q: %v", name, err)
	}
}

func insertPortSearchNameOnly(t *testing.T, conn *sql.DB, voyageID int64, name string) {
	t.Helper()
	if _, err := conn.Exec(`INSERT INTO voyage_ports (voyage_id, name) VALUES ($1, $2)`, voyageID, name); err != nil {
		t.Fatalf("insert port %q: %v", name, err)
	}
}
