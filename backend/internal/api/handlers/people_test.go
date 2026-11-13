package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestCrewHandler_ListPassesSearch(t *testing.T) {
	cases := map[string]types.NullString{
		"/crew":               {},
		"/crew?q=%20%20":      {},
		"/crew?q=%20Kowal%20": {String: "Kowal", Valid: true},
		"/crew?q=ola%40x.dev": {String: "ola@x.dev", Valid: true},
		"/crew?q=Wi%C5%9Bnia": {String: "Wiśnia", Valid: true},
	}
	for path, want := range cases {
		var listed, counted types.NullString
		m := &mockQuerier{
			listCrewMembersFn: func(_ context.Context, arg sqlcdb.ListCrewMembersParams) ([]sqlcdb.CrewMember, error) {
				listed = arg.Search
				return nil, nil
			},
			countCrewMembersFn: func(_ context.Context, search types.NullString) (int64, error) {
				counted = search
				return 0, nil
			},
		}
		resp := crewTestAPI(t, m).GetCtx(memberCtx(2), path)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: got %d; body=%s", path, resp.Code, resp.Body)
		}
		if listed != want || counted != want {
			t.Errorf("%s: search = (%+v, %+v), want %+v", path, listed, counted, want)
		}
	}
}

func TestCrewHandler_OtherNamesAreCleaned(t *testing.T) {
	var stored json.RawMessage
	m := &mockQuerier{
		createCrewMemberFn: func(_ context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
			stored = arg.OtherNames
			return sqlcdb.CrewMember{ID: 1, FullName: arg.FullName, OtherNames: arg.OtherNames}, nil
		},
	}
	resp := crewTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/crew", map[string]any{
		"full_name":   "Anna Nowak",
		"other_names": []string{" Anna Kowalska ", "", "anna kowalska", "Ania"},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("got %d; body=%s", resp.Code, resp.Body)
	}
	if string(stored) != `["Anna Kowalska","Ania"]` {
		t.Errorf("stored other names = %s", stored)
	}
	var got dto.CrewMember
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !slices.Equal(got.OtherNames, []string{"Anna Kowalska", "Ania"}) || got.HasAccount {
		t.Errorf("response = %+v", got)
	}

	resp = crewTestAPI(t, &mockQuerier{}).PutCtx(callerCtx(1, "admin"), "/crew/1", map[string]any{
		"full_name": "Anna", "other_names": []string{strings.Repeat("x", dto.MaxOtherNameLen+1)},
	})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Errorf("too long other name: got %d, want 422", resp.Code)
	}
}

func TestCrewHandler_DeleteKeepsAccountPerson(t *testing.T) {
	cases := []struct {
		name    string
		deleted int64
		getErr  error
		want    int
	}{
		{"person without account", 1, nil, http.StatusNoContent},
		{"person linked to an account", 0, nil, http.StatusConflict},
		{"missing person", 0, sql.ErrNoRows, http.StatusNoContent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &mockQuerier{
				deleteCrewMemberFn: func(context.Context, int64) (int64, error) { return c.deleted, nil },
				getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) {
					return sqlcdb.CrewMember{ID: 7, UserID: types.NullInt64{Int64: 3, Valid: true}}, c.getErr
				},
			}
			resp := crewTestAPI(t, m).DeleteCtx(callerCtx(1, "admin"), "/crew/7")
			if resp.Code != c.want {
				t.Errorf("got %d, want %d; body=%s", resp.Code, c.want, resp.Body)
			}
		})
	}
}

// TestPeople_DB walks the people list against a real schema: an account's
// person comes from sign-in and follows profile edits, search matches a
// name, another name or an e-mail, and the account person cannot be deleted.
func TestPeople_DB(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "ewa@example.dev", Name: ""})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := q.EnsureUserPerson(ctx, user.ID); err != nil {
		t.Fatalf("ensure person: %v", err)
	}

	_, api := humatest.New(t)
	RegisterCrewRoutes(api, q, conn)
	RegisterAuthRoutes(api, q, conn)
	admin := callerCtx(user.ID, "admin")

	resp := api.PutCtx(memberCtx(user.ID), "/auth/me", map[string]any{
		"first_name": "Ewa", "last_name": "Wiśniewska", "phone": "+48 600 000 003",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update profile: got %d; body=%s", resp.Code, resp.Body)
	}

	resp = api.PostCtx(admin, "/crew", map[string]any{
		"full_name": "Zofia Nowak", "email": "zosia@example.dev", "other_names": []string{"Zofia Łęcka"},
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create person: got %d; body=%s", resp.Code, resp.Body)
	}
	var zofia dto.CrewMember
	if err := json.Unmarshal(resp.Body.Bytes(), &zofia); err != nil {
		t.Fatalf("decode: %v", err)
	}

	search := func(q string) []string {
		t.Helper()
		resp := api.GetCtx(memberCtx(user.ID), "/crew?q="+q)
		if resp.Code != http.StatusOK {
			t.Fatalf("search %q: got %d; body=%s", q, resp.Code, resp.Body)
		}
		var page dto.Page[dto.CrewMember]
		if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		names := make([]string, len(page.Items))
		for i, p := range page.Items {
			names[i] = p.FullName
			if p.HasAccount != (p.UserID != nil) {
				t.Errorf("%s: has_account=%v with user_id=%v", p.FullName, p.HasAccount, p.UserID)
			}
		}
		if page.Total != int64(len(names)) {
			t.Errorf("search %q: total %d for %d items", q, page.Total, len(names))
		}
		return names
	}
	for q, want := range map[string][]string{
		"":                {"Ewa Wiśniewska", "Zofia Nowak"},
		"wiśniew":         {"Ewa Wiśniewska"},
		"%C5%81%C4%98CKA": {"Zofia Nowak"},
		"zosia%40":        {"Zofia Nowak"},
		"EWA%40EXAMPLE":   {"Ewa Wiśniewska"},
		"%25":             {},
	} {
		if got := search(q); !slices.Equal(got, want) {
			t.Errorf("search %q = %v, want %v", q, got, want)
		}
	}

	person, err := q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: user.ID, Valid: true})
	if err != nil {
		t.Fatalf("account person: %v", err)
	}
	if person.Phone.String != "+48 600 000 003" {
		t.Errorf("account person phone = %q, want the profile phone", person.Phone.String)
	}
	if resp := api.DeleteCtx(admin, "/crew/"+strconv.FormatInt(person.ID, 10)); resp.Code != http.StatusConflict {
		t.Errorf("delete account person: got %d, want 409", resp.Code)
	}
	if resp := api.DeleteCtx(admin, "/crew/"+strconv.FormatInt(zofia.ID, 10)); resp.Code != http.StatusNoContent {
		t.Errorf("delete person without account: got %d, want 204", resp.Code)
	}
	if got := search(""); !slices.Equal(got, []string{"Ewa Wiśniewska"}) {
		t.Errorf("people after delete = %v", got)
	}
}

// TestDashboardSeaTime_AccountPerson checks that a voyage crewed through the
// person created at sign-in counts as the account's sea time.
func TestDashboardSeaTime_AccountPerson(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "olek@example.dev", Name: "Olek"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := q.EnsureUserPerson(ctx, user.ID); err != nil {
		t.Fatalf("ensure person: %v", err)
	}
	person, err := q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: user.ID, Valid: true})
	if err != nil {
		t.Fatalf("account person: %v", err)
	}
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Bałtyk", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	voyage, err := q.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		CruiseID: cruise.ID, Name: "Gdynia–Bornholm", Year: types.NullInt64{Int64: 2026, Valid: true},
		Miles: 120, HoursTotal: 30, Days: 3,
	})
	if err != nil {
		t.Fatalf("create voyage: %v", err)
	}
	if _, err := q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID: types.NullInt64{Int64: voyage.ID, Valid: true}, CrewMemberID: person.ID, Role: "zalogant",
	}); err != nil {
		t.Fatalf("assign crew: %v", err)
	}

	d := getDashboard(t, dashboardTestAPI(t, q), memberCtx(user.ID))
	if d.Personal.AsCrew.VoyageCount != 1 || d.Personal.Total.TotalMiles != 120 {
		t.Errorf("personal sea time = %+v, want the crewed voyage", d.Personal)
	}
}
