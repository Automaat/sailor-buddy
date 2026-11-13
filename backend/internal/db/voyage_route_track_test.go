package db

import (
	"context"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestMigrate_VoyageRouteKeepsExistingPorts(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, "025_voyage_route_track.sql")

	var cruiseID, voyageID int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ('event') RETURNING id`), &cruiseID)
	mustScan(t, conn.QueryRow(`INSERT INTO voyages (cruise_id, name, image_route_url, description)
		VALUES ($1, 'leg', '/uploads/route.png', 'opis') RETURNING id`, cruiseID), &voyageID)
	if _, err := conn.Exec(`INSERT INTO voyage_ports (voyage_id, name, latitude, longitude, position)
		VALUES ($1, 'Port A', 54.5, 18.5, 0)`, voyageID); err != nil {
		t.Fatalf("insert port: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ports, err := sqlcdb.New(conn).ListVoyagePorts(context.Background(), voyageID)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	if len(ports) != 1 || ports[0].Latitude.Float64 != 54.5 || ports[0].Longitude.Float64 != 18.5 || ports[0].Anchorage {
		t.Fatalf("existing port changed: %+v", ports)
	}
	v, err := sqlcdb.New(conn).GetVoyage(context.Background(), voyageID)
	if err != nil {
		t.Fatalf("get voyage: %v", err)
	}
	if v.ImageRouteUrl.String != "/uploads/route.png" || v.Description.String != "opis" || v.TrackUrl.Valid || v.VoyageCard.Valid {
		t.Fatalf("existing voyage changed: %+v", v)
	}
}

func TestVoyageRoute_NameOnlyPortsAndRecordFields(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	var cruiseID, voyageID, yachtID int64
	mustScan(t, conn.QueryRow(`INSERT INTO cruises (name) VALUES ('event') RETURNING id`), &cruiseID)
	mustScan(t, conn.QueryRow(`INSERT INTO voyages (cruise_id, name) VALUES ($1, 'leg') RETURNING id`, cruiseID), &voyageID)
	mustScan(t, conn.QueryRow(`INSERT INTO yachts (name, yacht_type) VALUES ('Yacht A', 'Type A') RETURNING id`), &yachtID)

	for i, p := range []sqlcdb.CreateVoyagePortParams{
		{Name: "Port A", Latitude: types.NullFloat64{Float64: 54.5, Valid: true}, Longitude: types.NullFloat64{Float64: 18.5, Valid: true}},
		{Name: "Bay B", Anchorage: true},
		{Name: "Port C"},
	} {
		p.VoyageID = voyageID
		p.Position = int64(i)
		if _, err := q.CreateVoyagePort(ctx, p); err != nil {
			t.Fatalf("create port %q: %v", p.Name, err)
		}
	}
	ports, err := q.ListVoyagePorts(ctx, voyageID)
	if err != nil {
		t.Fatalf("list ports: %v", err)
	}
	if len(ports) != 3 || ports[0].Name != "Port A" || ports[1].Name != "Bay B" || ports[2].Name != "Port C" {
		t.Fatalf("ports out of order: %+v", ports)
	}
	if ports[1].Latitude.Valid || !ports[1].Anchorage || ports[2].Anchorage {
		t.Fatalf("unexpected name-only ports: %+v", ports[1:])
	}

	if _, err := conn.Exec(`INSERT INTO voyage_ports (voyage_id, name, latitude) VALUES ($1, 'half', 1)`, voyageID); err == nil {
		t.Fatal("a port with only a latitude was accepted")
	}

	if err := q.UpdateVoyageLog(ctx, sqlcdb.UpdateVoyageLogParams{
		ID:         voyageID,
		YachtID:    types.NullInt64{Int64: yachtID, Valid: true},
		TrackUrl:   types.NullString{String: "https://example.com/t", Valid: true},
		VoyageCard: types.NullString{String: "w klubie", Valid: true},
	}); err != nil {
		t.Fatalf("update log: %v", err)
	}
	v, err := q.GetVoyage(ctx, voyageID)
	if err != nil {
		t.Fatalf("get voyage: %v", err)
	}
	if v.YachtID.Int64 != yachtID || v.TrackUrl.String != "https://example.com/t" || v.VoyageCard.String != "w klubie" {
		t.Fatalf("log fields not stored: %+v", v)
	}
}
