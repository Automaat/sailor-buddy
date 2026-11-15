package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// standaloneTripBody is a trip without a cruise, with every field the
// wrapper cruise copies.
func standaloneTripBody() map[string]any {
	return map[string]any{
		"name":                       "Etap Bałtyk",
		"embark_date":                "2027-06-01",
		"disembark_date":             "2027-06-08",
		"countries":                  "PL, SE",
		"start_port":                 "Gdynia",
		"end_port":                   "Visby",
		"description":                "Tydzień na Bałtyku",
		"max_crew":                   7,
		"cost_per_person":            1500,
		"currency":                   "EUR",
		"cost_per_person_non_member": 1700,
		"yacht_fund":                 300,
		"deposit":                    200,
	}
}

func TestTripCreate_StandaloneGetsWrapperCruise(t *testing.T) {
	var cruise sqlcdb.CreateCruiseParams
	var trip sqlcdb.CreateTripParams
	inTx := false
	m := &mockQuerier{
		getCruiseFn: func(context.Context, int64) (sqlcdb.Cruise, error) {
			t.Fatal("a standalone trip has no cruise to verify")
			return sqlcdb.Cruise{}, nil
		},
		createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
			if !inTx {
				t.Fatal("wrapper cruise created outside the transaction")
			}
			cruise = arg
			return sqlcdb.Cruise{ID: 42, Name: arg.Name}, nil
		},
		createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			if !inTx {
				t.Fatal("trip created outside the transaction")
			}
			trip = arg
			return sqlcdb.Trip{ID: 7, CruiseID: arg.CruiseID, Name: arg.Name, Currency: arg.Currency}, nil
		},
	}
	runTx := func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		inTx = true
		defer func() { inTx = false }()
		return fn(m)
	}
	_, api := humatest.New(t)
	registerTripRoutes(api, &TripHandler{q: m, runTx: runTx})

	resp := api.PostCtx(userCtx(context.Background()), "/trips", standaloneTripBody())
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var got dto.Trip
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.CruiseID != 42 || trip.CruiseID != 42 {
		t.Fatalf("trip cruise = %d (stored %d), want wrapper 42", got.CruiseID, trip.CruiseID)
	}
	if cruise.SeriesID.Valid || cruise.Edition.Valid {
		t.Fatalf("wrapper cruise has a series: %+v", cruise)
	}
	if cruise.Kind != "sea" || cruise.Name != "Etap Bałtyk" || cruise.CreatedBy.Int64 != 1 {
		t.Fatalf("wrapper kind/name/creator = %q/%q/%d", cruise.Kind, cruise.Name, cruise.CreatedBy.Int64)
	}
	copied := map[string][2]any{
		"embark_date":                {cruise.EmbarkDate.String, "2027-06-01"},
		"disembark_date":             {cruise.DisembarkDate.String, "2027-06-08"},
		"countries":                  {cruise.Countries.String, "PL, SE"},
		"start_port":                 {cruise.StartPort.String, "Gdynia"},
		"end_port":                   {cruise.EndPort.String, "Visby"},
		"description":                {cruise.Description.String, "Tydzień na Bałtyku"},
		"max_crew":                   {cruise.MaxCrew.Int64, int64(7)},
		"cost_per_person":            {cruise.CostPerPerson.Float64, 1500.0},
		"currency":                   {cruise.Currency, "EUR"},
		"cost_per_person_non_member": {cruise.CostPerPersonNonMember.Float64, 1700.0},
		"yacht_fund":                 {cruise.YachtFund.Float64, 300.0},
		"deposit":                    {cruise.Deposit.Float64, 200.0},
	}
	for field, pair := range copied {
		if pair[0] != pair[1] {
			t.Errorf("wrapper %s = %v, want %v", field, pair[0], pair[1])
		}
	}
	if string(cruise.Included) != "[]" || string(cruise.NotIncluded) != "[]" {
		t.Errorf("wrapper price lists = %s / %s, want empty arrays", cruise.Included, cruise.NotIncluded)
	}
}

func TestTripCreate_StandaloneRollsBackWhenTripFails(t *testing.T) {
	yachtFK := &pgconn.PgError{Code: "23503", ConstraintName: "trips_yacht_id_fkey"}
	tests := []struct {
		name    string
		tripErr error
		want    int
	}{
		{name: "trip insert fails", tripErr: errors.New("db down"), want: http.StatusInternalServerError},
		{name: "yacht deleted meanwhile", tripErr: yachtFK, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
					return sqlcdb.Cruise{ID: 42}, nil
				},
				createTripFn: func(context.Context, sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{}, tt.tripErr
				},
			}
			var txErr error
			runTx := func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
				txErr = fn(m)
				return txErr
			}
			_, api := humatest.New(t)
			registerTripRoutes(api, &TripHandler{q: m, runTx: runTx})

			body := standaloneTripBody()
			body["yacht_id"] = 3
			resp := api.PostCtx(userCtx(context.Background()), "/trips", body)
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if !errors.Is(txErr, tt.tripErr) {
				t.Fatalf("transaction returned %v, want the trip error so it rolls back", txErr)
			}
		})
	}

	t.Run("wrapper cruise insert fails", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("db down")
			},
		}
		resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", standaloneTripBody())
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body=%s", resp.Code, resp.Body)
		}
	})
}

func TestTripCreate_WithCruiseSkipsWrapper(t *testing.T) {
	var stored int64
	m := &mockQuerier{
		createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
			stored = arg.CruiseID
			return sqlcdb.Trip{ID: 7, CruiseID: arg.CruiseID, Name: arg.Name}, nil
		},
	}
	body := standaloneTripBody()
	body["cruise_id"] = 13
	resp := tripTestAPI(t, m).PostCtx(userCtx(context.Background()), "/trips", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if stored != 13 {
		t.Fatalf("trip cruise = %d, want 13", stored)
	}
}

// TestTripCreate_StandaloneRollbackAgainstDatabase proves the wrapper cruise
// goes away with a trip insert the database refuses (unknown yacht).
func TestTripCreate_StandaloneRollbackAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	ctx := context.Background()
	admin, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "admin@example.com", Name: "Admin"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, api := humatest.New(t)
	RegisterTripRoutes(api, q, conn)
	count := func() int {
		var n int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM cruises").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	body := standaloneTripBody()
	body["yacht_id"] = 999999
	resp := api.PostCtx(callerCtx(admin.ID, "admin"), "/trips", body)
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", resp.Code, resp.Body)
	}
	if n := count(); n != 0 {
		t.Fatalf("cruises after a failed standalone trip = %d, want 0", n)
	}

	resp = api.PostCtx(callerCtx(admin.ID, "admin"), "/trips", standaloneTripBody())
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
	var trip dto.Trip
	if err := json.Unmarshal(resp.Body.Bytes(), &trip); err != nil {
		t.Fatal(err)
	}
	cruise, err := q.GetCruise(ctx, trip.CruiseID)
	if err != nil {
		t.Fatalf("wrapper cruise: %v", err)
	}
	if cruise.SeriesID.Valid || cruise.Name != "Etap Bałtyk" || cruise.Currency != "EUR" {
		t.Fatalf("wrapper cruise = %+v", cruise)
	}
}
