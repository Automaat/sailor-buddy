package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func dashboardTestAPI(t *testing.T, q sqlcdb.Querier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	RegisterDashboardRoutes(api, q)
	return api
}

func getDashboard(t *testing.T, api humatest.TestAPI, ctx context.Context) dto.Dashboard {
	t.Helper()
	resp := api.GetCtx(ctx, "/dashboard")
	if resp.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var out dto.Dashboard
	if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func clubMock(m *mockQuerier) *mockQuerier {
	m.getDashboardFn = func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
		return sqlcdb.GetDashboardStatsRow{VoyageCount: 3, TotalMiles: 420}, nil
	}
	m.getVoyagesByYearFn = func(context.Context) ([]sqlcdb.GetVoyagesByYearRow, error) {
		return []sqlcdb.GetVoyagesByYearRow{{VoyageCount: 3, TotalMiles: 420}}, nil
	}
	return m
}

func TestDashboardHandler_Get(t *testing.T) {
	year := func(y int64) types.NullInt64 { return types.NullInt64{Int64: y, Valid: true} }
	rows := []sqlcdb.ListUserSeaTimeVoyagesRow{
		{ID: 2, Name: "Bornholm", Year: year(2025), AsCaptain: false, Miles: 120, HoursTotal: 30, HoursSail: 20, HoursEngine: 10, Days: 4},
		{ID: 1, Name: "Gotland", Year: year(2024), AsCaptain: true, Miles: 200, HoursTotal: 50, HoursSail: 40, HoursEngine: 10, Days: 6},
		{ID: 3, Name: "Zatoka", Year: year(2024), AsCaptain: false, Miles: 10, HoursTotal: 3, HoursSail: 3, Days: 1},
	}

	t.Run("member gets own totals split by role and no club section", func(t *testing.T) {
		var gotUser int64
		m := &mockQuerier{listUserSeaTimeFn: func(_ context.Context, userID int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) {
			gotUser = userID
			return rows, nil
		}}
		d := getDashboard(t, dashboardTestAPI(t, m), callerCtx(7, "member"))
		if gotUser != 7 {
			t.Errorf("queried user %d, want caller 7", gotUser)
		}
		p := d.Personal
		if p.Total.VoyageCount != 3 || p.Total.TotalMiles != 330 || p.Total.TotalDays != 11 ||
			p.Total.TotalHours != 83 || p.Total.TotalHoursSail != 63 || p.Total.TotalHoursEngine != 20 {
			t.Errorf("total = %+v", p.Total)
		}
		if p.AsCaptain.VoyageCount != 1 || p.AsCaptain.TotalMiles != 200 {
			t.Errorf("as_captain = %+v", p.AsCaptain)
		}
		if p.AsCrew.VoyageCount != 2 || p.AsCrew.TotalMiles != 130 {
			t.Errorf("as_crew = %+v", p.AsCrew)
		}
		if len(p.ByYear) != 2 || *p.ByYear[0].Year != 2024 || p.ByYear[0].VoyageCount != 2 ||
			p.ByYear[0].TotalMiles != 210 || *p.ByYear[1].Year != 2025 {
			t.Errorf("by_year = %+v", p.ByYear)
		}
		if len(p.Voyages) != 3 || p.Voyages[0].ID != 2 || p.Voyages[0].Role != "crew" || p.Voyages[1].Role != "captain" {
			t.Errorf("voyages = %+v", p.Voyages)
		}
		if d.Club != nil {
			t.Errorf("member got club section %+v", d.Club)
		}
	})

	t.Run("user with no voyages gets zero totals and empty lists", func(t *testing.T) {
		m := &mockQuerier{listUserSeaTimeFn: func(context.Context, int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) {
			return nil, nil
		}}
		resp := dashboardTestAPI(t, m).GetCtx(callerCtx(7, "member"), "/dashboard")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d; body=%s", resp.Code, resp.Body)
		}
		var raw map[string]map[string]json.RawMessage
		if err := json.Unmarshal(resp.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, k := range []string{"by_year", "voyages"} {
			if string(raw["personal"][k]) != "[]" {
				t.Errorf("personal.%s = %s, want []", k, raw["personal"][k])
			}
		}
		if _, ok := raw["club"]; ok {
			t.Error("member response has club key")
		}
	})

	t.Run("admin also gets club totals", func(t *testing.T) {
		m := clubMock(&mockQuerier{listUserSeaTimeFn: func(context.Context, int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) {
			return rows[:1], nil
		}})
		d := getDashboard(t, dashboardTestAPI(t, m), callerCtx(1, "admin"))
		if d.Personal.Total.VoyageCount != 1 {
			t.Errorf("personal total = %+v", d.Personal.Total)
		}
		if d.Club == nil || d.Club.Total.VoyageCount != 3 || d.Club.Total.TotalMiles != 420 || len(d.Club.ByYear) != 1 {
			t.Errorf("club = %+v", d.Club)
		}
	})

	errCases := []struct {
		name string
		m    *mockQuerier
	}{
		{"personal db error", &mockQuerier{listUserSeaTimeFn: func(context.Context, int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) {
			return nil, errors.New("fail")
		}}},
		{"stats db error", &mockQuerier{
			listUserSeaTimeFn: func(context.Context, int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) { return nil, nil },
			getDashboardFn: func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, errors.New("fail")
			},
		}},
		{"yearly db error", &mockQuerier{
			listUserSeaTimeFn: func(context.Context, int64) ([]sqlcdb.ListUserSeaTimeVoyagesRow, error) { return nil, nil },
			getDashboardFn: func(context.Context) (sqlcdb.GetDashboardStatsRow, error) {
				return sqlcdb.GetDashboardStatsRow{}, nil
			},
			getVoyagesByYearFn: func(context.Context) ([]sqlcdb.GetVoyagesByYearRow, error) {
				return nil, errors.New("fail")
			},
		}},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := dashboardTestAPI(t, tc.m).GetCtx(callerCtx(1, "admin"), "/dashboard")
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf("got %d, want 500", resp.Code)
			}
		})
	}

	t.Run("unauthenticated", func(t *testing.T) {
		resp := dashboardTestAPI(t, &mockQuerier{}).GetCtx(context.Background(), "/dashboard")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.Code)
		}
	})
}

// TestDashboardSeaTime_DB checks the sea-time query against a real schema:
// voyages captained by the user's person and linked-crew voyages count, an
// overlap counts once, and unlinked crew records and captain persons do not
// count.
func TestDashboardSeaTime_DB(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	mkUser := func(email string) int64 {
		u, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: email, Name: email})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		return u.ID
	}
	alice, bob := mkUser("alice@example.com"), mkUser("bob@example.com")
	nullID := func(id int64) types.NullInt64 { return types.NullInt64{Int64: id, Valid: true} }

	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Baltic", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	yacht, err := q.CreateYacht(ctx, sqlcdb.CreateYachtParams{Name: "Zjawa"})
	if err != nil {
		t.Fatalf("create yacht: %v", err)
	}
	mkCrew := func(name string, user types.NullInt64) int64 {
		c, err := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{FullName: name, UserID: user})
		if err != nil {
			t.Fatalf("create crew member: %v", err)
		}
		return c.ID
	}
	aliceCrew := mkCrew("Alice", nullID(alice))
	strangerCrew := mkCrew("Alice (no account)", types.NullInt64{})

	mkVoyage := func(name string, year int64, miles float64, captain types.NullInt64) int64 {
		v, err := q.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
			CruiseID: cruise.ID, Name: name, Year: nullID(year), YachtID: nullID(yacht.ID),
			Miles: miles, HoursTotal: miles / 5, HoursSail: miles / 10, HoursEngine: miles / 10, Days: 2,
			CaptainPersonID: captain,
		})
		if err != nil {
			t.Fatalf("create voyage: %v", err)
		}
		return v.ID
	}
	captained := mkVoyage("Captained and crewed", 2024, 100, nullID(aliceCrew))
	crewed := mkVoyage("Crewed", 2025, 50, types.NullInt64{})
	mkVoyage("Unlinked crew", 2025, 400, types.NullInt64{})
	mkVoyage("Captained by a person without an account", 2025, 30, nullID(strangerCrew))

	assign := func(voyage, crew int64) {
		if _, err := q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
			VoyageID: nullID(voyage), CrewMemberID: crew, Role: "zalogant",
		}); err != nil {
			t.Fatalf("assign crew: %v", err)
		}
	}
	assign(captained, aliceCrew)
	assign(crewed, aliceCrew)
	assign(captained, strangerCrew)

	api := dashboardTestAPI(t, q)

	d := getDashboard(t, api, callerCtx(alice, "member"))
	p := d.Personal
	if p.Total.VoyageCount != 2 || p.Total.TotalMiles != 150 || p.Total.TotalDays != 4 {
		t.Errorf("alice total = %+v, want 2 voyages / 150 Mm / 4 days", p.Total)
	}
	if p.AsCaptain.VoyageCount != 1 || p.AsCaptain.TotalMiles != 100 {
		t.Errorf("alice as_captain = %+v", p.AsCaptain)
	}
	if p.AsCrew.VoyageCount != 1 || p.AsCrew.TotalMiles != 50 {
		t.Errorf("alice as_crew = %+v", p.AsCrew)
	}
	if len(p.Voyages) != 2 || p.Voyages[0].ID != crewed || p.Voyages[0].Role != "crew" ||
		p.Voyages[1].ID != captained || p.Voyages[1].Role != "captain" {
		t.Errorf("alice voyages = %+v", p.Voyages)
	}
	if p.Voyages[0].YachtName == nil || *p.Voyages[0].YachtName != "Zjawa" {
		t.Errorf("yacht name = %v, want Zjawa", p.Voyages[0].YachtName)
	}
	if d.Club != nil {
		t.Error("member got club section")
	}

	d = getDashboard(t, api, callerCtx(bob, "admin"))
	if d.Personal.Total.VoyageCount != 0 || len(d.Personal.Voyages) != 0 {
		t.Errorf("bob personal = %+v, want empty", d.Personal)
	}
	if d.Club == nil || d.Club.Total.VoyageCount != 4 || d.Club.Total.TotalMiles != 580 {
		t.Errorf("admin club = %+v, want 4 voyages / 580 Mm", d.Club)
	}
}
