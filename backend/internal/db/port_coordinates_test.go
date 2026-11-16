package db

import (
	"reflect"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

const portCoordinatesMigration = "039_port_coordinates.sql"

// TestMigrate_PortCoordinates checks that migration 039 keeps the start and
// end ports already stored, without coordinates, and that the tables then
// take a located port but refuse half a pair or coordinates on no port.
func TestMigrate_PortCoordinates(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, portCoordinatesMigration)

	cruiseID := insertPortSearchCruise(t, conn, true)
	ids := map[string]int64{}
	for _, table := range []string{"trips", "voyages"} {
		var id int64
		mustScan(t, conn.QueryRow(`INSERT INTO `+table+` (cruise_id, name, start_port, end_port)
			VALUES ($1, 'leg', 'Hel', NULL) RETURNING id`, cruiseID), &id)
		ids[table] = id
	}
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for table, id := range ids {
		var start, end types.NullString
		var coords [4]types.NullFloat64
		mustScan(t, conn.QueryRow(`SELECT start_port, end_port, start_port_latitude, start_port_longitude,
			end_port_latitude, end_port_longitude FROM `+table+` WHERE id = $1`, id),
			&start, &end, &coords[0], &coords[1], &coords[2], &coords[3])
		if start != text("Hel") || end.Valid || coords != [4]types.NullFloat64{} {
			t.Errorf("%s %d after migration = %+v / %+v / %+v", table, id, start, end, coords)
		}

		writes := []struct {
			name, set string
			ok        bool
		}{
			{"located start", "start_port_latitude = 54.6, start_port_longitude = 18.8", true},
			{"located end", "end_port = 'Visby', end_port_latitude = 57.6, end_port_longitude = 18.3", true},
			{"start latitude only", "start_port_latitude = 54.6, start_port_longitude = NULL", false},
			{"end longitude only", "end_port = 'Visby', end_port_latitude = NULL, end_port_longitude = 18.3", false},
			{"coordinates on no start port", "start_port = NULL, start_port_latitude = 54.6, start_port_longitude = 18.8", false},
			{"coordinates on no end port", "end_port = NULL, end_port_latitude = 57.6, end_port_longitude = 18.3", false},
		}
		for _, w := range writes {
			_, err := conn.Exec(`UPDATE `+table+` SET `+w.set+` WHERE id = $1`, id)
			if (err == nil) != w.ok {
				t.Errorf("%s %s: err = %v, want accepted %v", table, w.name, err, w.ok)
			}
		}
	}
}

// TestSuggestPorts_StartEndCoordinates checks that a start or end port
// entered with coordinates suggests them, and that a name entered in
// several places takes the coordinates of its latest located entry,
// whichever source it is in. Rows are inserted oldest first:
//   - Gdynia: a visited port, a trip end port, then a voyage start port,
//     each with other coordinates;
//   - Ustka: a located trip start port, then a name-only visited port;
//   - Łeba: a located voyage end port, then a located visited port.
func TestSuggestPorts_StartEndCoordinates(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cruiseID := insertPortSearchCruise(t, conn, true)
	voyageID := insertPortSearchVoyage(t, conn, cruiseID, true)

	insertLocatedStart := func(table, start string, lat, lon float64) {
		t.Helper()
		if _, err := conn.Exec(`INSERT INTO `+table+` (cruise_id, name, published_at,
				start_port, start_port_latitude, start_port_longitude)
			VALUES ($1, 'leg', now(), $2, $3, $4)`, cruiseID, start, lat, lon); err != nil {
			t.Fatalf("insert %s: %v", table, err)
		}
	}
	insertLocatedEnd := func(table, end string, lat, lon float64) {
		t.Helper()
		if _, err := conn.Exec(`INSERT INTO `+table+` (cruise_id, name, published_at,
				end_port, end_port_latitude, end_port_longitude)
			VALUES ($1, 'leg', now(), $2, $3, $4)`, cruiseID, end, lat, lon); err != nil {
			t.Fatalf("insert %s: %v", table, err)
		}
	}

	insertLocatedStart("trips", "Kołobrzeg", 54.18, 15.58)
	insertLocatedEnd("voyages", "Rønne", 55.10, 14.70)
	insertPortSearchPort(t, conn, voyageID, "Gdynia", 54.50, 18.50)
	insertLocatedEnd("trips", "Gdynia", 54.51, 18.51)
	insertLocatedStart("voyages", "Gdynia", 54.52, 18.52)
	insertLocatedStart("trips", "Ustka", 54.58, 16.86)
	insertPortSearchNameOnly(t, conn, voyageID, "Ustka")
	insertLocatedEnd("voyages", "Łeba", 54.76, 17.55)
	insertPortSearchPort(t, conn, voyageID, "Łeba", 54.77, 17.56)

	type row struct {
		name     string
		lat, lon float64
		uses     int64
	}
	tests := []struct {
		name, query string
		want        []row
	}{
		{"trip start port", "kolobrz", []row{{"Kołobrzeg", 54.18, 15.58, 1}}},
		{"voyage end port", "ronne", []row{{"Rønne", 55.10, 14.70, 1}}},
		{"latest of three sources is a voyage start port", "gdyn", []row{{"Gdynia", 54.52, 18.52, 3}}},
		{"name-only visited port keeps a start port's coordinates", "ustk", []row{{"Ustka", 54.58, 16.86, 2}}},
		{"later visited port wins over an end port", "leba", []row{{"Łeba", 54.77, 17.56, 2}}},
	}
	q := sqlcdb.New(conn)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := q.SuggestPorts(t.Context(), sqlcdb.SuggestPortsParams{Query: tt.query, RowLimit: 8})
			if err != nil {
				t.Fatalf("suggest: %v", err)
			}
			var rows []row
			for _, g := range got {
				if !g.Latitude.Valid || !g.Longitude.Valid {
					t.Fatalf("suggestion without coordinates: %+v", g)
				}
				rows = append(rows, row{g.Name, g.Latitude.Float64, g.Longitude.Float64, g.Uses})
			}
			if !reflect.DeepEqual(rows, tt.want) {
				t.Fatalf("suggest %q = %+v, want %+v", tt.query, rows, tt.want)
			}
		})
	}
}
