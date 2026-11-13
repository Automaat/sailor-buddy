package dto

import (
	"reflect"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCruiseEnrollmentsFromDB(t *testing.T) {
	rows := []sqlcdb.ListCruiseEnrollmentsRow{
		{
			ID:        1,
			CruiseID:  7,
			UserID:    3,
			TripID:    types.NullInt64{Int64: 9, Valid: true},
			Status:    "accepted",
			UserName:  "Anna",
			UserEmail: "anna@example.com",
			TripName:  types.NullString{String: "Rejs Bałtyk", Valid: true},
		},
		{
			ID:        2,
			CruiseID:  7,
			UserID:    4,
			Status:    "pending",
			UserName:  "Bo",
			UserEmail: "bo@example.com",
		},
	}

	out := CruiseEnrollmentsFromDB(rows)
	if len(out) != 2 {
		t.Fatalf("want 2 enrollments, got %d", len(out))
	}

	assigned := out[0]
	if assigned.TripName == nil || *assigned.TripName != "Rejs Bałtyk" {
		t.Fatalf("want trip_name %q, got %v", "Rejs Bałtyk", assigned.TripName)
	}
	if assigned.CruiseID == nil || *assigned.CruiseID != 7 {
		t.Fatalf("want cruise_id 7, got %v", assigned.CruiseID)
	}
	if assigned.TripID == nil || *assigned.TripID != 9 {
		t.Fatalf("want trip_id 9, got %v", assigned.TripID)
	}

	unassigned := out[1]
	if unassigned.TripName != nil {
		t.Fatalf("want nil trip_name when the row is unassigned, got %v", *unassigned.TripName)
	}
	if unassigned.TripID != nil {
		t.Fatalf("want nil trip_id when the row is unassigned, got %v", *unassigned.TripID)
	}
}

func TestEnrollLegsFromDB(t *testing.T) {
	rows := []sqlcdb.Trip{
		{
			ID:                     1,
			Name:                   "Bodø – Tromsø",
			Status:                 sqlcdb.TripStatusCancelled,
			EmbarkDate:             types.NullString{String: "2025-07-01", Valid: true},
			DisembarkDate:          types.NullString{String: "2025-07-10", Valid: true},
			Countries:              types.NullString{String: "Norwegia", Valid: true},
			StartPort:              types.NullString{String: "Bodø", Valid: true},
			EndPort:                types.NullString{String: "Tromsø", Valid: true},
			Description:            types.NullString{String: "Etap 1", Valid: true},
			MaxCrew:                types.NullInt64{Int64: 8, Valid: true},
			CaptainName:            types.NullString{String: "Jan", Valid: true},
			ImagePhotoUrl:          types.NullString{String: "/uploads/a.jpg", Valid: true},
			CostPerPerson:          types.NullFloat64{Float64: 2500, Valid: true},
			Currency:               "EUR",
			CostPerPersonNonMember: types.NullFloat64{Float64: 3000, Valid: true},
			YachtFund:              types.NullFloat64{Float64: 500, Valid: true},
			Deposit:                types.NullFloat64{Float64: 200, Valid: true},
			CreatedBy:              types.NullInt64{Int64: 42, Valid: true},
			CostTotal:              types.NullFloat64{Float64: 20000, Valid: true},
			EnrollToken:            types.NullString{String: "leg-secret", Valid: true},
		},
		{ID: 2, Name: "Pusty", Status: sqlcdb.TripStatusPlanned, Currency: "PLN"},
	}

	out := EnrollLegsFromDB(rows)
	if len(out) != 2 {
		t.Fatalf("want 2 legs, got %d", len(out))
	}
	want := EnrollLeg{
		ID:                     1,
		Name:                   "Bodø – Tromsø",
		EmbarkDate:             new("2025-07-01"),
		DisembarkDate:          new("2025-07-10"),
		Countries:              new("Norwegia"),
		StartPort:              new("Bodø"),
		EndPort:                new("Tromsø"),
		Description:            new("Etap 1"),
		MaxCrew:                new(int64(8)),
		CaptainName:            new("Jan"),
		ImagePhotoUrl:          new("/uploads/a.jpg"),
		CostPerPerson:          new(2500.0),
		Currency:               "EUR",
		CostPerPersonNonMember: new(3000.0),
		YachtFund:              new(500.0),
		Deposit:                new(200.0),
		Status:                 "cancelled",
	}
	if !reflect.DeepEqual(out[0], want) {
		t.Fatalf("leg 1 = %+v, want %+v", out[0], want)
	}
	empty := EnrollLeg{ID: 2, Name: "Pusty", Currency: "PLN", Status: "planned"}
	if !reflect.DeepEqual(out[1], empty) {
		t.Fatalf("leg 2 = %+v, want %+v", out[1], empty)
	}
	if got := EnrollLegsFromDB(nil); len(got) != 0 {
		t.Fatalf("nil rows: want no legs, got %d", len(got))
	}
}
