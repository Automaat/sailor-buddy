package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// TestCruisePricing_RoundTrip stores a cruise with its price lists and reads
// it back through the enroll-token query, stores a two-tier trip, then checks
// the CHECK constraints behind the API's 422s. Prices and currency are on the
// trip; the cruise keeps only what the price covers.
func TestCruisePricing_RoundTrip(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)
	price := func(f float64) types.NullFloat64 { return types.NullFloat64{Float64: f, Valid: true} }

	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name:        "SSS 2025",
		Included:    json.RawMessage(`["jacht","wyżywienie \"pełne\""]`),
		NotIncluded: json.RawMessage(`[]`),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	if err := q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{EnrollToken: text("tok"), ID: cruise.ID}); err != nil {
		t.Fatalf("set token: %v", err)
	}
	if _, err := q.PublishCruise(ctx, cruise.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	row, err := q.GetCruiseByEnrollToken(ctx, text("tok"))
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	var included []string
	if err := json.Unmarshal(row.Included, &included); err != nil {
		t.Fatalf("decode included %s: %v", row.Included, err)
	}
	if len(included) != 2 || included[1] != `wyżywienie "pełne"` || string(row.NotIncluded) != "[]" {
		t.Fatalf("row = %+v, included = %v", row, included)
	}

	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "WRM", Status: sqlcdb.TripStatusPlanned, Currency: "EUR",
		CostPerPerson: price(760), CostPerPersonNonMember: price(800),
		YachtFund: price(150), Deposit: price(100),
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if trip.Currency != "EUR" || trip.CostPerPerson.Float64 != 760 || trip.CostPerPersonNonMember.Float64 != 800 ||
		trip.YachtFund.Float64 != 150 || trip.Deposit.Float64 != 100 {
		t.Fatalf("trip = %+v", trip)
	}

	badCruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Bad", Included: json.RawMessage(`{"a":1}`), NotIncluded: json.RawMessage(`[]`),
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "cruises_included_check" {
		t.Errorf("included: got %+v, err = %v, want CHECK violation on cruises_included_check", badCruise, err)
	}

	badTrips := []struct {
		name       string
		params     sqlcdb.CreateTripParams
		constraint string
	}{
		{"currency", sqlcdb.CreateTripParams{Currency: "USD"}, "trips_currency_check"},
		{"deposit", sqlcdb.CreateTripParams{Currency: "PLN", Deposit: price(-1)}, "trips_deposit_check"},
	}
	for _, tc := range badTrips {
		p := tc.params
		p.CruiseID, p.Name, p.Status = cruise.ID, "Bad", sqlcdb.TripStatusPlanned
		_, err := q.CreateTrip(ctx, p)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != tc.constraint {
			t.Errorf("%s: err = %v, want CHECK violation on %s", tc.name, err, tc.constraint)
		}
	}
}
