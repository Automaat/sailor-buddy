package handlers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// TestCreateVoyageFromTrip_CopiesPorts checks that the voyage of a completed
// trip gets the trip's start and end ports with their coordinates.
func TestCreateVoyageFromTrip_CopiesPorts(t *testing.T) {
	trip := sqlcdb.Trip{
		ID: 4, CruiseID: 2, Name: "Etap",
		StartPort:          types.NullString{String: "Hel", Valid: true},
		StartPortLatitude:  types.NullFloat64{Float64: 54.6, Valid: true},
		StartPortLongitude: types.NullFloat64{Float64: 18.8, Valid: true},
		EndPort:            types.NullString{String: "Visby", Valid: true},
	}
	var got sqlcdb.CreateVoyageParams
	m := &mockQuerier{createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
		got = arg
		return sqlcdb.Voyage{ID: 9}, nil
	}}
	if _, err := createVoyageFromTrip(context.Background(), m, trip, dto.CompleteTripBody{}); err != nil {
		t.Fatal(err)
	}
	gotStart := storedPort{got.StartPort, got.StartPortLatitude, got.StartPortLongitude}
	gotEnd := storedPort{got.EndPort, got.EndPortLatitude, got.EndPortLongitude}
	if gotStart != located("Hel", 54.6, 18.8) || gotEnd != named("Visby") {
		t.Fatalf("voyage ports = %+v / %+v", gotStart, gotEnd)
	}
}

// TestCompleteTrip_PortCoordinatesDB completes a trip in the database and
// reads the voyage back: its start and end ports keep the trip's names and
// coordinates, and a port without coordinates stays without them.
func TestCompleteTrip_PortCoordinatesDB(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Bałtyk", Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "Etap", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
		StartPort:          types.NullString{String: "Hel", Valid: true},
		StartPortLatitude:  types.NullFloat64{Float64: 54.6, Valid: true},
		StartPortLongitude: types.NullFloat64{Float64: 18.8, Valid: true},
		EndPort:            types.NullString{String: "Visby", Valid: true},
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	voyage, err := completeTripTx(ctx, conn, trip.ID, dto.CompleteTripBody{})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	stored, err := q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: voyage.ID, IncludeHidden: true})
	if err != nil {
		t.Fatalf("get voyage: %v", err)
	}
	got := dto.VoyageFromDB(stored)
	if got.StartPort == nil || got.StartPort.Name != "Hel" || got.StartPort.Latitude == nil ||
		*got.StartPort.Latitude != 54.6 || got.StartPort.Longitude == nil || *got.StartPort.Longitude != 18.8 {
		t.Fatalf("voyage start port = %+v, want Hel at 54.6, 18.8", got.StartPort)
	}
	if got.EndPort == nil || *got.EndPort != (dto.Port{Name: "Visby"}) {
		t.Fatalf("voyage end port = %+v, want Visby without coordinates", got.EndPort)
	}
}
