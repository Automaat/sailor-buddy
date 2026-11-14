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

// TestCruisePricing_RoundTrip stores a two-tier EUR cruise with its price
// lists and reads it back through the enroll-token query, then checks the
// CHECK constraints behind the API's 422s.
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
		Name: "SSS 2025", Kind: "sea", Currency: "EUR",
		CostPerPerson: price(760), CostPerPersonNonMember: price(800),
		YachtFund: price(150), Deposit: price(100),
		Included:    json.RawMessage(`["jacht","wyżywienie \"pełne\""]`),
		NotIncluded: json.RawMessage(`[]`),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	if err := q.SetCruiseEnrollToken(ctx, sqlcdb.SetCruiseEnrollTokenParams{EnrollToken: text("tok"), ID: cruise.ID}); err != nil {
		t.Fatalf("set token: %v", err)
	}
	row, err := q.GetCruiseByEnrollToken(ctx, text("tok"))
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	var included []string
	if err := json.Unmarshal(row.Included, &included); err != nil {
		t.Fatalf("decode included %s: %v", row.Included, err)
	}
	if row.Currency != "EUR" || row.CostPerPersonNonMember.Float64 != 800 || row.YachtFund.Float64 != 150 ||
		row.Deposit.Float64 != 100 || len(included) != 2 || included[1] != `wyżywienie "pełne"` {
		t.Fatalf("row = %+v, included = %v", row, included)
	}

	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "WRM", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
		CostPerPerson: price(820), CostPerPersonNonMember: price(870),
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if trip.Currency != "PLN" || trip.CostPerPersonNonMember.Float64 != 870 {
		t.Fatalf("trip = %+v", trip)
	}

	bad := []struct {
		name       string
		params     sqlcdb.CreateCruiseParams
		constraint string
	}{
		{"currency", sqlcdb.CreateCruiseParams{Currency: "USD"}, "cruises_currency_check"},
		{"deposit", sqlcdb.CreateCruiseParams{Currency: "PLN", Deposit: price(-1)}, "cruises_deposit_check"},
		{"included", sqlcdb.CreateCruiseParams{Currency: "PLN", Included: json.RawMessage(`{"a":1}`)}, "cruises_included_check"},
	}
	for _, tc := range bad {
		p := tc.params
		p.Name, p.Kind = "Bad", "sea"
		if p.Included == nil {
			p.Included = json.RawMessage(`[]`)
		}
		p.NotIncluded = json.RawMessage(`[]`)
		_, err := q.CreateCruise(ctx, p)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != tc.constraint {
			t.Errorf("%s: err = %v, want CHECK violation on %s", tc.name, err, tc.constraint)
		}
	}
}
