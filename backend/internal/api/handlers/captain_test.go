package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

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

func TestVoyageLog_Authorization(t *testing.T) {
	const captainID = 7
	tests := []struct {
		name   string
		ctx    context.Context
		voyage error
		want   int
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), want: http.StatusNoContent},
		{name: "assigned captain", ctx: callerCtx(captainID, "member"), want: http.StatusNoContent},
		{name: "other member", ctx: callerCtx(8, "member"), want: http.StatusForbidden},
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
					return sqlcdb.Voyage{ID: id, CaptainUserID: types.NullInt64{Int64: captainID, Valid: true}}, nil
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

func TestVoyagePorts_AddByAssignedCaptain(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
			return sqlcdb.Voyage{ID: id, CaptainUserID: types.NullInt64{Int64: 7, Valid: true}}, nil
		},
		createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
			return sqlcdb.VoyagePort{ID: 1, VoyageID: arg.VoyageID, Name: arg.Name}, nil
		},
	}
	resp := voyagePortTestAPI(t, m).PostCtx(callerCtx(7, "member"), "/voyages/5/ports",
		map[string]any{"name": "Hel", "latitude": 54.6, "longitude": 18.8})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", resp.Code, resp.Body)
	}
}

func TestTripCreate_CaptainAssignment(t *testing.T) {
	noPatent := captainUser(7)
	noPatent.PatentNumber = types.NullString{}
	notCaptain := captainUser(7)
	notCaptain.IsCaptain = false

	tests := []struct {
		name     string
		user     sqlcdb.User
		userErr  error
		want     int
		wantName string
	}{
		{name: "eligible captain", user: captainUser(7), want: http.StatusCreated, wantName: "Kpt. Anna"},
		{name: "not in captain group", user: notCaptain, want: http.StatusUnprocessableEntity},
		{name: "no patent number", user: noPatent, want: http.StatusUnprocessableEntity},
		{name: "unknown user", userErr: sql.ErrNoRows, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.CreateTripParams
			m := &mockQuerier{
				getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) { return tt.user, tt.userErr },
				createTripFn: func(_ context.Context, arg sqlcdb.CreateTripParams) (sqlcdb.Trip, error) {
					got = &arg
					return sqlcdb.Trip{ID: 1, Name: arg.Name, Status: sqlcdb.TripStatusPlanned}, nil
				},
			}
			resp := tripTestAPI(t, m).PostCtx(callerCtx(1, "admin"), "/trips", map[string]any{
				"cruise_id": 1, "name": "Navigare", "captain_user_id": 7, "captain_name": "ignored",
			})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.want != http.StatusCreated {
				if got != nil {
					t.Fatal("trip created despite rejection")
				}
				return
			}
			if got.CaptainUserID.Int64 != 7 || got.CaptainName.String != tt.wantName {
				t.Fatalf("captain = (%v, %q), want (7, %q)", got.CaptainUserID, got.CaptainName.String, tt.wantName)
			}
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

func TestTripUpdate_UnchangedCaptainSkipsEligibility(t *testing.T) {
	leftCaptainGroup := captainUser(7)
	leftCaptainGroup.IsCaptain = false
	tests := []struct {
		name    string
		current types.NullInt64
		want    int
	}{
		{name: "same captain kept", current: types.NullInt64{Int64: 7, Valid: true}, want: http.StatusNoContent},
		{name: "newly assigned", current: types.NullInt64{}, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{ID: id, CruiseID: 1, CaptainUserID: tt.current}, nil
				},
				getUserByIDFn: func(context.Context, int64) (sqlcdb.User, error) { return leftCaptainGroup, nil },
				updateTripFn:  func(context.Context, sqlcdb.UpdateTripParams) error { return nil },
			}
			resp := tripTestAPI(t, m).PutCtx(callerCtx(1, "admin"), "/trips/3",
				map[string]any{"cruise_id": 1, "name": "Navigare", "captain_user_id": 7})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}
