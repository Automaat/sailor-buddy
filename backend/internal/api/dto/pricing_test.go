package dto

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestPriceList(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want []string
	}{
		{"nil column", nil, []string{}},
		{"empty array", json.RawMessage(`[]`), []string{}},
		{"json null", json.RawMessage(`null`), []string{}},
		{"items", json.RawMessage(`["jacht","paliwo"]`), []string{"jacht", "paliwo"}},
		{"not an array", json.RawMessage(`{"a":1}`), []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringList(tc.raw)
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("stringList(%s) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEnrollFromRow_Prices(t *testing.T) {
	price := func(f float64) types.NullFloat64 { return types.NullFloat64{Float64: f, Valid: true} }

	c := EnrollCruiseFromRow(sqlcdb.GetCruiseByEnrollTokenRow{
		ID: 1, Name: "SSS", Currency: "EUR",
		CostPerPerson: price(760), CostPerPersonNonMember: price(800),
		YachtFund: price(150), Deposit: price(50),
		Included: json.RawMessage(`["jacht"]`), NotIncluded: json.RawMessage(`["dojazd"]`),
	}, false)
	if c.Currency != "EUR" || *c.CostPerPerson != 760 || *c.CostPerPersonNonMember != 800 ||
		*c.YachtFund != 150 || *c.Deposit != 50 ||
		!reflect.DeepEqual(c.Included, []string{"jacht"}) || !reflect.DeepEqual(c.NotIncluded, []string{"dojazd"}) {
		t.Fatalf("cruise = %+v", c)
	}

	tr := EnrollTripFromRow(sqlcdb.GetTripByEnrollTokenRow{
		ID: 2, Name: "WRM", Currency: "PLN", CostPerPerson: price(820), CostPerPersonNonMember: price(870),
	})
	if tr.Currency != "PLN" || *tr.CostPerPerson != 820 || *tr.CostPerPersonNonMember != 870 ||
		tr.YachtFund != nil || tr.Deposit != nil {
		t.Fatalf("trip = %+v", tr)
	}
}
