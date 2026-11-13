package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// callerCtx injects an authenticated caller with the given ID and role.
func callerCtx(userID int64, role string) context.Context {
	return context.WithValue(context.Background(), middleware.UserCtxKey, &auth.Claims{
		UserID: userID, Email: "c@example.com", Name: "Caller", Role: role,
	})
}

func captainUser(id int64) sqlcdb.User {
	return sqlcdb.User{
		ID: id, Name: "Kpt. Anna", Role: "member", IsCaptain: true, Membership: "czlonek",
		PatentNumber: types.NullString{String: "JSM-1", Valid: true},
	}
}

// captainPerson is the person (crew record) 70, linked to account userID
// when it is non-zero.
func captainPerson(userID int64) sqlcdb.CrewMember {
	p := sqlcdb.CrewMember{ID: 70, FullName: "Kpt. Anna"}
	if userID != 0 {
		p.UserID = types.NullInt64{Int64: userID, Valid: true}
	}
	return p
}

func TestVoyageLog_Authorization(t *testing.T) {
	const captainID = 7
	linked := captainPerson(captainID)
	tests := []struct {
		name    string
		ctx     context.Context
		voyage  error
		captain *sqlcdb.CrewMember
		want    int
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), captain: &linked, want: http.StatusNoContent},
		{name: "captain person linked to the caller", ctx: callerCtx(captainID, "member"), captain: &linked, want: http.StatusNoContent},
		{name: "other member", ctx: callerCtx(8, "member"), captain: &linked, want: http.StatusForbidden},
		{name: "captain person without an account", ctx: callerCtx(captainID, "member"), captain: new(captainPerson(0)), want: http.StatusForbidden},
		{name: "no captain", ctx: callerCtx(captainID, "member"), want: http.StatusForbidden},
		{name: "missing voyage", ctx: callerCtx(1, "admin"), voyage: sql.ErrNoRows, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.UpdateVoyageLogParams
			m := &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
					if tt.voyage != nil {
						return sqlcdb.Voyage{}, tt.voyage
					}
					v := sqlcdb.Voyage{ID: id}
					if tt.captain != nil {
						v.CaptainPersonID = types.NullInt64{Int64: tt.captain.ID, Valid: true}
					}
					return v, nil
				},
				getCrewMemberFn: func(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
					if tt.captain == nil || id != tt.captain.ID {
						return sqlcdb.CrewMember{}, sql.ErrNoRows
					}
					return *tt.captain, nil
				},
				updateVoyageLogFn: func(_ context.Context, arg sqlcdb.UpdateVoyageLogParams) error {
					got = &arg
					return nil
				},
			}
			resp := voyageTestAPI(t, m).PutCtx(tt.ctx, "/voyages/5/log", map[string]any{
				"hours_total": 40.5, "miles": 120, "days": 7, "captain_notes": "Wiało 6B pod Helem",
			})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusNoContent {
				if got != nil {
					t.Fatal("log written despite rejection")
				}
				return
			}
			if got == nil || got.ID != 5 || got.Miles != 120 || got.CaptainNotes.String != "Wiało 6B pod Helem" {
				t.Fatalf("unexpected log params: %+v", got)
			}
		})
	}
}

func TestVoyagePorts_CaptainRights(t *testing.T) {
	tests := []struct {
		name    string
		captain sqlcdb.CrewMember
		want    int
	}{
		{name: "captain person linked to the caller", captain: captainPerson(7), want: http.StatusCreated},
		{name: "captain person without an account", captain: captainPerson(0), want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := false
			m := &mockQuerier{
				getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
					return sqlcdb.Voyage{ID: id, CaptainPersonID: types.NullInt64{Int64: 70, Valid: true}}, nil
				},
				getCrewMemberFn: func(context.Context, int64) (sqlcdb.CrewMember, error) { return tt.captain, nil },
				createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
					created = true
					return sqlcdb.VoyagePort{ID: 1, VoyageID: arg.VoyageID, Name: arg.Name}, nil
				},
			}
			resp := voyagePortTestAPI(t, m).PostCtx(callerCtx(7, "member"), "/voyages/5/ports",
				map[string]any{"name": "Hel", "latitude": 54.6, "longitude": 18.8})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if created != (tt.want == http.StatusCreated) {
				t.Fatalf("port created = %v, want %v", created, tt.want == http.StatusCreated)
			}
		})
	}
}

// captainCases covers the captain fields of a leg or voyage body: any person
// may be chosen and their name is snapshotted; free text is kept without one.
var captainCases = []struct {
	name       string
	body       map[string]any
	personErr  error
	want       int
	wantPerson types.NullInt64
	wantName   string
}{
	{
		name: "any person, name snapshotted", body: map[string]any{"captain_person_id": 70, "captain_name": "ignored"},
		want: http.StatusOK, wantPerson: types.NullInt64{Int64: 70, Valid: true}, wantName: "Kpt. Anna",
	},
	{name: "unknown person", body: map[string]any{"captain_person_id": 70}, personErr: sql.ErrNoRows, want: http.StatusUnprocessableEntity},
	{name: "free-text captain", body: map[string]any{"captain_name": "Kpt. Spoza"}, want: http.StatusOK, wantName: "Kpt. Spoza"},
}

func captainBody(extra map[string]any) map[string]any {
	body := map[string]any{"cruise_id": 1, "name": "Navigare"}
	maps.Copy(body, extra)
	return body
}

func captainLookup(err error) func(context.Context, int64) (sqlcdb.CrewMember, error) {
	return func(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
		if err != nil {
			return sqlcdb.CrewMember{}, err
		}
		p := captainPerson(0)
		p.ID = id
		return p, nil
	}
}

func checkCaptain(t *testing.T, code, want int, written bool, gotPerson types.NullInt64, gotName types.NullString, wantPerson types.NullInt64, wantName string) {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d", code, want)
	}
	if want >= 300 {
		if written {
			t.Fatal("written despite rejection")
		}
		return
	}
	if gotPerson != wantPerson || gotName.String != wantName {
		t.Fatalf("captain = (%v, %q), want (%v, %q)", gotPerson, gotName.String, wantPerson, wantName)
	}
}

func TestTripCaptainPerson(t *testing.T) {
	for _, tt := range captainCases {
		t.Run("create "+tt.name, func(t *testing.T) {
			var got *sqlcdb.CreateTripParams
			m := &mockQuerier{
				getCrewMemberFn: captainLookup(tt.personErr),
				createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					got = &arg
					return sqlcdb.Trip{ID: 1, Name: arg.Name, Status: sqlcdb.TripStatusPlanned}, nil
				},
			}
			resp := tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", captainBody(tt.body))
			want := tt.want
			if want == http.StatusOK {
				want = http.StatusCreated
			}
			var p sqlcdb.CreateTripParams
			if got != nil {
				p = *got
			}
			checkCaptain(t, resp.Code, want, got != nil, p.CaptainPersonID, p.CaptainName, tt.wantPerson, tt.wantName)
		})
		t.Run("update "+tt.name, func(t *testing.T) {
			var got *sqlcdb.UpdateTripParams
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{ID: id, CruiseID: 1}, nil
				},
				getCrewMemberFn: captainLookup(tt.personErr),
				updateTripFn: func(_ context.Context, arg sqlcdb.UpdateTripParams) error {
					got = &arg
					return nil
				},
			}
			resp := tripTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/trips/3", captainBody(tt.body))
			want := tt.want
			if want == http.StatusOK {
				want = http.StatusNoContent
			}
			var p sqlcdb.UpdateTripParams
			if got != nil {
				p = *got
			}
			checkCaptain(t, resp.Code, want, got != nil, p.CaptainPersonID, p.CaptainName, tt.wantPerson, tt.wantName)
		})
	}
}

func TestVoyageCaptainPerson(t *testing.T) {
	for _, tt := range captainCases {
		t.Run("create "+tt.name, func(t *testing.T) {
			var got *sqlcdb.CreateVoyageParams
			m := &mockQuerier{
				getCrewMemberFn: captainLookup(tt.personErr),
				createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
					got = &arg
					return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
				},
			}
			resp := voyageTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/voyages", captainBody(tt.body))
			want := tt.want
			if want == http.StatusOK {
				want = http.StatusCreated
			}
			var p sqlcdb.CreateVoyageParams
			if got != nil {
				p = *got
			}
			checkCaptain(t, resp.Code, want, got != nil, p.CaptainPersonID, p.CaptainName, tt.wantPerson, tt.wantName)
		})
		t.Run("update "+tt.name, func(t *testing.T) {
			var got *sqlcdb.UpdateVoyageParams
			m := &mockQuerier{
				getVoyageFn:     func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
				getCrewMemberFn: captainLookup(tt.personErr),
				updateVoyageFn: func(_ context.Context, arg sqlcdb.UpdateVoyageParams) error {
					got = &arg
					return nil
				},
			}
			resp := voyageTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/voyages/3", captainBody(tt.body))
			want := tt.want
			if want == http.StatusOK {
				want = http.StatusNoContent
			}
			var p sqlcdb.UpdateVoyageParams
			if got != nil {
				p = *got
			}
			checkCaptain(t, resp.Code, want, got != nil, p.CaptainPersonID, p.CaptainName, tt.wantPerson, tt.wantName)
		})
	}
}

func TestMembers_UpdateCaptain(t *testing.T) {
	noPatent := captainUser(7)
	noPatent.IsCaptain = false
	noPatent.PatentNumber = types.NullString{}
	sympatyk := captainUser(7)
	sympatyk.IsCaptain = false
	sympatyk.Membership = "sympatyk"

	tests := []struct {
		name   string
		ctx    context.Context
		target sqlcdb.User
		body   bool
		want   int
	}{
		{name: "admin grants eligible member", ctx: callerCtx(1, "admin"), target: captainUser(7), body: true, want: http.StatusNoContent},
		{name: "admin revokes without checks", ctx: callerCtx(1, "admin"), target: noPatent, body: false, want: http.StatusNoContent},
		{name: "missing patent number", ctx: callerCtx(1, "admin"), target: noPatent, body: true, want: http.StatusUnprocessableEntity},
		{name: "not a club member", ctx: callerCtx(1, "admin"), target: sympatyk, body: true, want: http.StatusUnprocessableEntity},
		{name: "member cannot grant", ctx: callerCtx(2, "member"), target: captainUser(7), body: true, want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			written := false
			m := &mockQuerier{
				getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) { return tt.target, nil },
				updateUserCaptainFn: func(_ context.Context, arg sqlcdb.UpdateUserCaptainParams) error {
					written = arg.ID == 7 && arg.IsCaptain == tt.body
					return nil
				},
			}
			resp := membersTestAPI(t, m).PutCtx(tt.ctx, "/members/7/captain", map[string]any{"is_captain": tt.body})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if written != (tt.want == http.StatusNoContent) {
				t.Fatalf("written = %v, want %v", written, tt.want == http.StatusNoContent)
			}
		})
	}
}

func TestMembers_UpdateMembership(t *testing.T) {
	plain := captainUser(7)
	plain.IsCaptain = false

	tests := []struct {
		name       string
		target     sqlcdb.User
		membership string
		want       int
	}{
		{name: "member to sympatyk", target: plain, membership: "sympatyk", want: http.StatusNoContent},
		{name: "captain must stay czlonek", target: captainUser(7), membership: "gosc", want: http.StatusUnprocessableEntity},
		{name: "captain stays czlonek", target: captainUser(7), membership: "czlonek", want: http.StatusNoContent},
		{name: "unknown status", target: plain, membership: "vip", want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getUserByIDFn:          func(context.Context, int64) (sqlcdb.User, error) { return tt.target, nil },
				updateUserMembershipFn: func(context.Context, sqlcdb.UpdateUserMembershipParams) error { return nil },
			}
			resp := membersTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/members/7/membership",
				map[string]any{"membership": tt.membership})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}

func TestCompleteTrip_KeepsCaptainPerson(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)

	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Bałtyk", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	person, err := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{FullName: "Kpt. Bez Konta"})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	captain := types.NullInt64{Int64: person.ID, Valid: true}
	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "Leg 1", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
		CaptainPersonID: captain, CaptainName: types.NullString{String: person.FullName, Valid: true},
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}

	voyage, err := completeTripTx(ctx, conn, trip.ID, dto.CompleteTripBody{})
	if err != nil {
		t.Fatalf("complete trip: %v", err)
	}
	if voyage.CaptainPersonID != captain || voyage.CaptainName.String != "Kpt. Bez Konta" {
		t.Fatalf("voyage captain = (%v, %q), want (%v, Kpt. Bez Konta)", voyage.CaptainPersonID, voyage.CaptainName.String, captain)
	}
}
