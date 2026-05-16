package dto

import (
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
