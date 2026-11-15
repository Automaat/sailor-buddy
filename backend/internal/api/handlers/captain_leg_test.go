package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Leg 9 is captained by person 70 (account recordCaptainID). Person 71
// (account otherCaptainID) captains another leg; person 72 has no account.
const (
	otherCaptainID       = 9
	otherCaptainPersonID = 71
	unlinkedPersonID     = 72
)

func legPeople(_ context.Context, id int64) (sqlcdb.CrewMember, error) {
	switch id {
	case recordCaptainPersonID:
		return sqlcdb.CrewMember{ID: id, UserID: types.NullInt64{Int64: recordCaptainID, Valid: true}}, nil
	case otherCaptainPersonID:
		return sqlcdb.CrewMember{ID: id, UserID: types.NullInt64{Int64: otherCaptainID, Valid: true}}, nil
	case unlinkedPersonID:
		return sqlcdb.CrewMember{ID: id}, nil
	}
	return sqlcdb.CrewMember{}, sql.ErrNoRows
}

func legPerson(id int64) types.NullInt64 { return types.NullInt64{Int64: id, Valid: true} }

func legDate(s string) types.NullString { return types.NullString{String: s, Valid: s != ""} }

func TestLegEnded(t *testing.T) {
	tests := []struct {
		name      string
		disembark types.NullString
		want      bool
	}{
		{name: "no disembark date", disembark: types.NullString{}, want: false},
		{name: "blank disembark date", disembark: types.NullString{Valid: true}, want: false},
		{name: "malformed date", disembark: legDate("14.11.2026"), want: false},
		{name: "ended yesterday", disembark: legDate("2026-11-14"), want: true},
		{name: "ends today", disembark: legDate("2026-11-15"), want: false},
		{name: "ends tomorrow", disembark: legDate("2026-11-16"), want: false},
		{name: "date with a time part", disembark: legDate("2026-11-14T10:00:00Z"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := legEnded(tt.disembark, "2026-11-15"); got != tt.want {
				t.Fatalf("legEnded(%q) = %v, want %v", tt.disembark.String, got, tt.want)
			}
		})
	}
}

// TestClubToday_WarsawMidnight pins the day the leg-ended rule compares
// against: Warsaw is UTC+1 in November, so the club date turns at 23:00 UTC.
func TestClubToday_WarsawMidnight(t *testing.T) {
	tests := []struct {
		now  time.Time
		want string
	}{
		{now: time.Date(2026, 11, 14, 22, 59, 0, 0, time.UTC), want: "2026-11-14"},
		{now: time.Date(2026, 11, 14, 23, 0, 0, 0, time.UTC), want: "2026-11-15"},
		{now: time.Date(2026, 7, 14, 21, 59, 0, 0, time.UTC), want: "2026-07-14"},
		{now: time.Date(2026, 7, 14, 22, 0, 0, 0, time.UTC), want: "2026-07-15"},
	}
	for _, tt := range tests {
		if got := clubTodayAt(func() time.Time { return tt.now }); got != tt.want {
			t.Errorf("clubTodayAt(%s) = %s, want %s", tt.now, got, tt.want)
		}
	}
}

func tripTestAPIAt(t *testing.T, m *mockQuerier, now time.Time) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerTripRoutes(api, &TripHandler{q: m, runTx: directTxRunner(m), now: func() time.Time { return now }})
	return api
}

// TestTripComplete_CaptainRefused covers the refusals decided before the
// completion transaction; the handler has no database here, so a request
// that got past them would panic.
func TestTripComplete_CaptainRefused(t *testing.T) {
	noon := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		caller    int64
		captain   types.NullInt64
		disembark string
		now       time.Time
		missing   bool
		want      int
	}{
		{name: "other member", caller: 8, captain: legPerson(recordCaptainPersonID), disembark: "2026-11-01", now: noon, want: http.StatusForbidden},
		{name: "captain of another leg", caller: otherCaptainID, captain: legPerson(recordCaptainPersonID), disembark: "2026-11-01", now: noon, want: http.StatusForbidden},
		{name: "leg without captain", caller: recordCaptainID, disembark: "2026-11-01", now: noon, want: http.StatusForbidden},
		{name: "captain person without account", caller: recordCaptainID, captain: legPerson(unlinkedPersonID), disembark: "2026-11-01", now: noon, want: http.StatusForbidden},
		{name: "captain on the last day", caller: recordCaptainID, captain: legPerson(recordCaptainPersonID), disembark: "2026-11-15", now: noon, want: http.StatusForbidden},
		{name: "captain before the leg ends", caller: recordCaptainID, captain: legPerson(recordCaptainPersonID), disembark: "2026-11-20", now: noon, want: http.StatusForbidden},
		{name: "captain without disembark date", caller: recordCaptainID, captain: legPerson(recordCaptainPersonID), now: noon, want: http.StatusForbidden},
		{
			name: "captain before Warsaw midnight", caller: recordCaptainID, captain: legPerson(recordCaptainPersonID),
			disembark: "2026-11-14", now: time.Date(2026, 11, 14, 22, 30, 0, 0, time.UTC), want: http.StatusForbidden,
		},
		{name: "missing trip", caller: recordCaptainID, missing: true, now: noon, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					if tt.missing {
						return sqlcdb.Trip{}, sql.ErrNoRows
					}
					return sqlcdb.Trip{
						ID: id, Status: sqlcdb.TripStatusPlanned, CaptainPersonID: tt.captain,
						DisembarkDate: legDate(tt.disembark),
					}, nil
				},
				getCrewMemberFn: legPeople,
			}
			resp := tripTestAPIAt(t, m, tt.now).PostCtx(callerCtx(tt.caller, "member"), "/trips/9/complete", map[string]any{})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
		})
	}
}

// TestTripCrew_Contacts checks the handler asks the crew query as the
// caller (admin flag and account) and keeps the phone and emergency contact
// only on rows the query marks visible; the rest of the list stays open.
// TestTripCrew_ContactsDB checks the query's captain rule.
func TestTripCrew_Contacts(t *testing.T) {
	row := sqlcdb.ListTripCrewAssignmentsRow{
		ID: 1, TripID: legPerson(9), CrewMemberID: 2, Role: "zalogant", FullName: "Jan Nowak",
		Email:                 types.NullString{String: "jan@example.dev", Valid: true},
		Phone:                 types.NullString{String: "+48 600 100 200", Valid: true},
		EmergencyContactName:  types.NullString{String: "Ewa Nowak", Valid: true},
		EmergencyContactPhone: types.NullString{String: "+48 600 300 400", Valid: true},
	}
	tests := []struct {
		name      string
		ctx       context.Context
		visible   bool
		wantAdmin bool
		wantUser  int64
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), visible: true, wantAdmin: true, wantUser: 1},
		{name: "member the query allows", ctx: callerCtx(recordCaptainID, "member"), visible: true, wantUser: recordCaptainID},
		{name: "member the query refuses", ctx: callerCtx(8, "member"), visible: false, wantUser: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sqlcdb.ListTripCrewAssignmentsParams
			m := &mockQuerier{
				getTripFn: func(_ context.Context, id int64) (sqlcdb.Trip, error) {
					return sqlcdb.Trip{ID: id, CaptainPersonID: legPerson(recordCaptainPersonID)}, nil
				},
				listTripCrewFn: func(_ context.Context, arg sqlcdb.ListTripCrewAssignmentsParams) ([]sqlcdb.ListTripCrewAssignmentsRow, error) {
					got = arg
					r := row
					r.ContactsVisible = tt.visible
					return []sqlcdb.ListTripCrewAssignmentsRow{r}, nil
				},
			}
			resp := crewTestAPI(t, m).GetCtx(tt.ctx, "/trips/9/crew")
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
			}
			if got.TripID != 9 || got.IsAdmin != tt.wantAdmin || !got.CallerUserID.Valid || got.CallerUserID.Int64 != tt.wantUser {
				t.Fatalf("query params = %+v, want trip 9, admin %v, caller %d", got, tt.wantAdmin, tt.wantUser)
			}
			var crew []map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &crew); err != nil || len(crew) != 1 {
				t.Fatalf("decode crew: %v; body=%s", err, resp.Body)
			}
			entry := crew[0]
			if entry["full_name"] != "Jan Nowak" || entry["role"] != "zalogant" || entry["email"] != "jan@example.dev" {
				t.Fatalf("crew entry = %v, want name, role and e-mail for everyone", entry)
			}
			for _, field := range []string{"phone", "emergency_contact_name", "emergency_contact_phone"} {
				if _, ok := entry[field]; ok != tt.visible {
					t.Errorf("%s present = %v, want %v", field, ok, tt.visible)
				}
			}
		})
	}
}

// TestVoyageCaptain_OpinionsAndPorts covers the voyage captain's writes:
// opinions and ports of their own voyage, nobody else's.
func TestVoyageCaptain_OpinionsAndPorts(t *testing.T) {
	callers := []struct {
		name    string
		ctx     context.Context
		allowed bool
	}{
		{name: "admin", ctx: callerCtx(1, "admin"), allowed: true},
		{name: "voyage captain", ctx: callerCtx(recordCaptainID, "member"), allowed: true},
		{name: "other member", ctx: callerCtx(8, "member"), allowed: false},
		{name: "captain of another voyage", ctx: callerCtx(otherCaptainID, "member"), allowed: false},
	}
	assignment := sqlcdb.GetVoyageCrewAssignmentByMemberRow{ID: 1, CrewMemberID: 4, Role: "zalogant", FullName: "Jan Nowak"}
	for _, c := range callers {
		t.Run(c.name, func(t *testing.T) {
			var wrote []string
			m := &mockQuerier{
				getVoyageFn:     captainVoyage,
				getCrewMemberFn: legPeople,
				getVoyageCrewByMemberFn: func(context.Context, sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
					return assignment, nil
				},
				getOpinionByMemberFn: func(context.Context, sqlcdb.GetVoyageOpinionByMemberParams) (sqlcdb.VoyageOpinion, error) {
					return sqlcdb.VoyageOpinion{}, sql.ErrNoRows
				},
				upsertVoyageOpinionFn: func(_ context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
					wrote = append(wrote, "generate")
					return sqlcdb.VoyageOpinion{ID: 7, VoyageID: arg.VoyageID, CrewMemberID: arg.CrewMemberID, FileFormat: arg.FileFormat}, nil
				},
				getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
					return sqlcdb.VoyageOpinion{ID: id, VoyageID: 5, CrewMemberID: 4}, nil
				},
				deleteVoyageOpinionFn: func(context.Context, int64) error {
					wrote = append(wrote, "delete opinion")
					return nil
				},
				createVoyagePortFn: func(_ context.Context, arg sqlcdb.CreateVoyagePortParams) (sqlcdb.VoyagePort, error) {
					wrote = append(wrote, "add port")
					return sqlcdb.VoyagePort{ID: 3, VoyageID: arg.VoyageID, Name: arg.Name}, nil
				},
				deleteVoyagePortFn: func(context.Context, sqlcdb.DeleteVoyagePortParams) error {
					wrote = append(wrote, "remove port")
					return nil
				},
			}
			opinions := opinionTestAPI(t, m)
			ports := voyagePortTestAPI(t, m)
			requests := []struct {
				name string
				send func() *httptest.ResponseRecorder
				ok   int
			}{
				{name: "generate opinion", ok: http.StatusCreated, send: func() *httptest.ResponseRecorder {
					return opinions.PostCtx(c.ctx, "/voyages/5/opinions", map[string]any{"crew_member_id": 4, "format": "docx"})
				}},
				{name: "delete opinion", ok: http.StatusNoContent, send: func() *httptest.ResponseRecorder {
					return opinions.DeleteCtx(c.ctx, "/voyages/5/opinions/7")
				}},
				{name: "add port", ok: http.StatusCreated, send: func() *httptest.ResponseRecorder {
					return ports.PostCtx(c.ctx, "/voyages/5/ports", map[string]any{"name": "Hel"})
				}},
				{name: "remove port", ok: http.StatusNoContent, send: func() *httptest.ResponseRecorder {
					return ports.DeleteCtx(c.ctx, "/voyages/5/ports/3")
				}},
			}
			for _, r := range requests {
				want := http.StatusForbidden
				if c.allowed {
					want = r.ok
				}
				if resp := r.send(); resp.Code != want {
					t.Errorf("%s: status = %d, want %d; body=%s", r.name, resp.Code, want, resp.Body)
				}
			}
			if c.allowed && len(wrote) != len(requests) || !c.allowed && len(wrote) != 0 {
				t.Fatalf("writes = %v", wrote)
			}
		})
	}
}
