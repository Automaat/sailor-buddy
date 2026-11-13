package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func validFloat(f float64) types.NullFloat64 { return types.NullFloat64{Float64: f, Valid: true} }

func floatEq(got types.NullFloat64, want float64) bool { return got.Valid && got.Float64 == want }

// cruiseFromCreate echoes the stored params back as the created row, the way
// the database's RETURNING * would.
func cruiseFromCreate(arg sqlcdb.CreateCruiseParams) sqlcdb.Cruise {
	return sqlcdb.Cruise{
		ID: 1, Name: arg.Name, Kind: arg.Kind, CostPerPerson: arg.CostPerPerson,
		Currency: arg.Currency, CostPerPersonNonMember: arg.CostPerPersonNonMember,
		YachtFund: arg.YachtFund, Deposit: arg.Deposit,
		Included: arg.Included, NotIncluded: arg.NotIncluded,
	}
}

func TestCruisePricing_CreateStoresAndReturnsPrices(t *testing.T) {
	var got sqlcdb.CreateCruiseParams
	m := &mockQuerier{
		createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
			got = arg
			return cruiseFromCreate(arg), nil
		},
	}
	resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", map[string]any{
		"name":                       "SSS 2025",
		"currency":                   "EUR",
		"cost_per_person":            760,
		"cost_per_person_non_member": 800,
		"yacht_fund":                 150,
		"deposit":                    100,
		"included":                   []string{" jacht ", "", "skipper"},
		"not_included":               []string{"dojazd"},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if got.Currency != "EUR" || !floatEq(got.CostPerPerson, 760) || !floatEq(got.CostPerPersonNonMember, 800) ||
		!floatEq(got.YachtFund, 150) || !floatEq(got.Deposit, 100) {
		t.Fatalf("params = %+v", got)
	}
	if string(got.Included) != `["jacht","skipper"]` || string(got.NotIncluded) != `["dojazd"]` {
		t.Fatalf("lists = %s / %s, want trimmed and blank-free", got.Included, got.NotIncluded)
	}
	var body dto.Cruise
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Currency != "EUR" || body.CostPerPersonNonMember == nil || *body.CostPerPersonNonMember != 800 ||
		body.YachtFund == nil || *body.YachtFund != 150 || body.Deposit == nil || *body.Deposit != 100 {
		t.Fatalf("response = %+v", body)
	}
	if strings.Join(body.Included, "|") != "jacht|skipper" || strings.Join(body.NotIncluded, "|") != "dojazd" {
		t.Fatalf("response lists = %v / %v", body.Included, body.NotIncluded)
	}
}

func TestCruisePricing_DefaultsToPLNAndEmptyLists(t *testing.T) {
	var got sqlcdb.CreateCruiseParams
	m := &mockQuerier{
		createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
			got = arg
			return cruiseFromCreate(arg), nil
		},
	}
	resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", map[string]any{"name": "WRM"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if got.Currency != "PLN" || string(got.Included) != "[]" || string(got.NotIncluded) != "[]" {
		t.Fatalf("params = currency %q, lists %s / %s", got.Currency, got.Included, got.NotIncluded)
	}
	if !strings.Contains(resp.Body.String(), `"included":[]`) || !strings.Contains(resp.Body.String(), `"not_included":[]`) {
		t.Fatalf("lists should serialize as []: %s", resp.Body)
	}
}

func TestCruisePricing_GetLegacyRowHasEmptyLists(t *testing.T) {
	m := &mockQuerier{
		getCruiseFn: func(_ context.Context, id int64) (sqlcdb.Cruise, error) {
			return sqlcdb.Cruise{ID: id, Name: "Old", Currency: "PLN", CostPerPerson: validFloat(870)}, nil
		},
	}
	resp := cruiseTestAPI(t, m).GetCtx(userCtx(context.Background()), "/cruises/1")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	for _, want := range []string{`"currency":"PLN"`, `"cost_per_person":870`, `"included":[]`, `"not_included":[]`} {
		if !strings.Contains(resp.Body.String(), want) {
			t.Fatalf("body missing %s: %s", want, resp.Body)
		}
	}
}

func TestCruisePricing_UpdatePassesPrices(t *testing.T) {
	var got sqlcdb.UpdateCruiseParams
	m := &mockQuerier{
		updateCruiseFn: func(_ context.Context, arg sqlcdb.UpdateCruiseParams) error {
			got = arg
			return nil
		},
	}
	resp := cruiseTestAPI(t, m).PutCtx(userCtx(context.Background()), "/cruises/3", map[string]any{
		"name":                       "NNE",
		"cost_per_person":            1000,
		"cost_per_person_non_member": 1250,
		"included":                   []string{"wyżywienie"},
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if got.ID != 3 || got.Currency != "PLN" || !floatEq(got.CostPerPersonNonMember, 1250) ||
		got.YachtFund.Valid || got.Deposit.Valid || string(got.Included) != `["wyżywienie"]` || string(got.NotIncluded) != "[]" {
		t.Fatalf("params = %+v", got)
	}
}

func TestCruisePricing_Rejects(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"unknown currency", map[string]any{"currency": "USD"}},
		{"lowercase currency", map[string]any{"currency": "pln"}},
		{"negative member price", map[string]any{"cost_per_person": -1}},
		{"negative non-member price", map[string]any{"cost_per_person_non_member": -0.01}},
		{"negative yacht fund", map[string]any{"yacht_fund": -150}},
		{"negative deposit", map[string]any{"deposit": -1}},
		{"overlong included entry", map[string]any{"included": []string{strings.Repeat("ą", dto.MaxPriceListItemLen+1)}}},
		{"overlong not included entry", map[string]any{"not_included": []string{strings.Repeat("x", dto.MaxPriceListItemLen+1)}}},
		{"too many included entries", map[string]any{"included": make([]string, 31)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.body["name"] = "X"
			for _, method := range []string{http.MethodPost, http.MethodPut} {
				api := cruiseTestAPI(t, &mockQuerier{})
				path := "/cruises"
				if method == http.MethodPut {
					path = "/cruises/1"
				}
				resp := api.DoCtx(userCtx(context.Background()), method, path, tc.body)
				if resp.Code != http.StatusUnprocessableEntity {
					t.Fatalf("%s: status = %d, want 422; body=%s", method, resp.Code, resp.Body)
				}
			}
		})
	}
}

func TestCruisePricing_ListItemAtLimitAccepted(t *testing.T) {
	var got sqlcdb.CreateCruiseParams
	m := &mockQuerier{
		createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
			got = arg
			return cruiseFromCreate(arg), nil
		},
	}
	item := strings.Repeat("ą", dto.MaxPriceListItemLen)
	resp := cruiseTestAPI(t, m).PostCtx(userCtx(context.Background()), "/cruises", map[string]any{
		"name": "X", "included": []string{item},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var stored []string
	if err := json.Unmarshal(got.Included, &stored); err != nil || len(stored) != 1 || stored[0] != item {
		t.Fatalf("stored = %v (%v)", stored, err)
	}
}

func TestTripPricing_CreateStoresAndReturnsPrices(t *testing.T) {
	var got sqlcdb.CreateTripParams
	m := &mockQuerier{
		createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			got = arg
			return sqlcdb.Trip{
				ID: 9, Name: arg.Name, Status: arg.Status, CostPerPerson: arg.CostPerPerson,
				Currency: arg.Currency, CostPerPersonNonMember: arg.CostPerPersonNonMember,
				YachtFund: arg.YachtFund, Deposit: arg.Deposit,
			}, nil
		},
	}
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", map[string]any{
		"cruise_id":                  1,
		"name":                       "WRM 2025",
		"currency":                   "PLN",
		"cost_per_person":            820,
		"cost_per_person_non_member": 870,
		"yacht_fund":                 300,
		"deposit":                    200,
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if got.Currency != "PLN" || !floatEq(got.CostPerPerson, 820) || !floatEq(got.CostPerPersonNonMember, 870) ||
		!floatEq(got.YachtFund, 300) || !floatEq(got.Deposit, 200) {
		t.Fatalf("params = %+v", got)
	}
	var body dto.Trip
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Currency != "PLN" || body.CostPerPersonNonMember == nil || *body.CostPerPersonNonMember != 870 ||
		body.YachtFund == nil || *body.YachtFund != 300 || body.Deposit == nil || *body.Deposit != 200 {
		t.Fatalf("response = %+v", body)
	}
}

func TestTripPricing_UpdatePassesPricesAndDefaultsCurrency(t *testing.T) {
	var got sqlcdb.UpdateTripParams
	m := &mockQuerier{
		getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
		updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
			got = arg
			return nil
		},
	}
	resp := tripTestAPI(t, m).PutCtx(userCtx(context.Background()), "/trips/4", map[string]any{
		"cruise_id": 1, "name": "NNE leg", "cost_per_person_non_member": 250, "deposit": 0,
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if got.ID != 4 || got.Currency != "PLN" || !floatEq(got.CostPerPersonNonMember, 250) ||
		!floatEq(got.Deposit, 0) || got.YachtFund.Valid {
		t.Fatalf("params = %+v", got)
	}
}

func TestTripPricing_Rejects(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{"unknown currency", map[string]any{"currency": "GBP"}},
		{"negative member price", map[string]any{"cost_per_person": -5}},
		{"negative non-member price", map[string]any{"cost_per_person_non_member": -5}},
		{"negative yacht fund", map[string]any{"yacht_fund": -5}},
		{"negative deposit", map[string]any{"deposit": -5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.body["name"] = "X"
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) { return sqlcdb.Trip{ID: id}, nil },
			}
			for _, method := range []string{http.MethodPost, http.MethodPut} {
				path := "/trips"
				if method == http.MethodPut {
					path = "/trips/1"
				}
				resp := tripTestAPI(t, m).DoCtx(userCtx(context.Background()), method, path, tc.body)
				if resp.Code != http.StatusUnprocessableEntity {
					t.Fatalf("%s: status = %d, want 422; body=%s", method, resp.Code, resp.Body)
				}
			}
		})
	}
}
