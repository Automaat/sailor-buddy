package db

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

const portSearchMigration = "036_port_search.sql"

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

	var volatility string
	mustScan(t, conn.QueryRow(`SELECT provolatile FROM pg_proc WHERE proname = 'port_search_norm'`), &volatility)
	if volatility != "i" {
		t.Fatalf("port_search_norm volatility = %q, want immutable (i)", volatility)
	}
	var folded string
	mustScan(t, conn.QueryRow(`SELECT port_search_norm('  Świnoujście ')`), &folded)
	if folded != "swinoujscie" {
		t.Fatalf("port_search_norm = %q, want swinoujscie", folded)
	}
	var indexed bool
	mustScan(t, conn.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_indexes
		WHERE tablename = 'voyage_ports' AND indexname = 'voyage_ports_name_trgm_idx')`), &indexed)
	if !indexed {
		t.Fatal("trigram index missing")
	}

	got, err := sqlcdb.New(conn).SuggestPorts(t.Context(), sqlcdb.SuggestPortsParams{
		Query: "swinou", IncludeHidden: false, RowLimit: 8,
	})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Świnoujście" {
		t.Fatalf("existing port not suggested: %+v", got)
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
			{"Hel", true, 54.60, 3},
			{"Helsinki", true, 60.17, 1},
		}},
		{"name-only port", "zatoka", false, 8, []row{{"Zatoka Pucka", false, 0, 1}}},
		{"limit", "hel", false, 1, []row{{"Hel", true, 54.60, 3}}},
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
