package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// captainLegDB is a test database with the account of a leg captain, the
// account of another leg's captain, a plain member and a crew member with
// contacts. Each test seeds its own legs on it.
type captainLegDB struct {
	conn               *sql.DB
	q                  *sqlcdb.Queries
	cruiseID           int64
	captainUser        int64
	captainPerson      int64
	otherCaptainUser   int64
	otherCaptainPerson int64
	memberUser         int64
	crewPerson         int64
}

func seedCaptainLegDB(t *testing.T) captainLegDB {
	t.Helper()
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	user := func(email string) int64 {
		u, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: email, Name: email})
		if err != nil {
			t.Fatalf("create user %s: %v", email, err)
		}
		return u.ID
	}
	d := captainLegDB{conn: conn, q: q}
	d.captainUser = user("kapitan@example.dev")
	d.captainPerson = ensurePerson(t, conn, d.captainUser)
	d.otherCaptainUser = user("inny@example.dev")
	d.otherCaptainPerson = ensurePerson(t, conn, d.otherCaptainUser)
	d.memberUser = user("czlonek@example.dev")
	ensurePerson(t, conn, d.memberUser)
	crew, err := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{
		FullName:              "Jan Nowak",
		Email:                 types.NullString{String: "jan@example.dev", Valid: true},
		Phone:                 types.NullString{String: "+48 600 100 200", Valid: true},
		EmergencyContactName:  types.NullString{String: "Ewa Nowak", Valid: true},
		EmergencyContactPhone: types.NullString{String: "+48 600 300 400", Valid: true},
		OtherNames:            json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create crew member: %v", err)
	}
	d.crewPerson = crew.ID
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "Bałtyk", Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	if _, err := q.PublishCruise(ctx, cruise.ID); err != nil {
		t.Fatalf("publish cruise: %v", err)
	}
	d.cruiseID = cruise.ID
	return d
}

// leg seeds a planned leg captained by captainPerson with the crew member on
// it; published false leaves it a hidden draft.
func (d captainLegDB) leg(t *testing.T, captainPerson int64, disembark string, published bool) int64 {
	t.Helper()
	ctx := context.Background()
	trip, err := d.q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: d.cruiseID, Name: "Etap", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
		CaptainPersonID: types.NullInt64{Int64: captainPerson, Valid: true},
		EmbarkDate:      types.NullString{String: "2026-11-07", Valid: disembark != ""},
		DisembarkDate:   types.NullString{String: disembark, Valid: disembark != ""},
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	if published {
		if _, err := d.q.PublishTrip(ctx, trip.ID); err != nil {
			t.Fatalf("publish trip: %v", err)
		}
	}
	if err := d.q.AddAcceptedTripCrewAssignment(ctx, sqlcdb.AddAcceptedTripCrewAssignmentParams{
		TripID: trip.ID, CrewMemberID: d.crewPerson,
	}); err != nil {
		t.Fatalf("add crew: %v", err)
	}
	return trip.ID
}

func (d captainLegDB) api(t *testing.T, now time.Time) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	registerTripRoutes(api, &TripHandler{q: d.q, db: d.conn, runTx: sqlTxRunner(d.conn), now: func() time.Time { return now }})
	registerCrewRoutes(api, NewCrewHandler(d.q, d.conn, nil))
	return api
}

func (d captainLegDB) tripStatus(t *testing.T, id int64) string {
	t.Helper()
	trip, err := d.q.GetTrip(context.Background(), sqlcdb.GetTripParams{ID: id, IncludeHidden: true})
	if errors.Is(err, sql.ErrNoRows) {
		return "gone"
	}
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	return string(trip.Status)
}

// TestCompleteTrip_CaptainDateRule runs leg completion against the database
// for each caller and the date rule. Warsaw is UTC+1 in November, so
// lastEvening is 23:30 on the disembark day there and nextMorning 00:30 on
// the day after.
func TestCompleteTrip_CaptainDateRule(t *testing.T) {
	d := seedCaptainLegDB(t)
	lastEvening := time.Date(2026, 11, 14, 22, 30, 0, 0, time.UTC)
	nextMorning := time.Date(2026, 11, 14, 23, 30, 0, 0, time.UTC)
	captain := callerCtx(d.captainUser, "member")
	tests := []struct {
		name      string
		caller    context.Context
		disembark string
		published bool
		now       time.Time
		want      int
	}{
		{name: "captain after the end date", caller: captain, disembark: "2026-11-14", published: true, now: nextMorning, want: http.StatusCreated},
		{name: "captain on the end date", caller: captain, disembark: "2026-11-14", published: true, now: lastEvening, want: http.StatusForbidden},
		{name: "captain without end date", caller: captain, published: true, now: nextMorning, want: http.StatusForbidden},
		{name: "other member", caller: callerCtx(d.memberUser, "member"), disembark: "2026-11-01", published: true, now: nextMorning, want: http.StatusForbidden},
		{name: "captain of another leg", caller: callerCtx(d.otherCaptainUser, "member"), disembark: "2026-11-01", published: true, now: nextMorning, want: http.StatusForbidden},
		{name: "member on a hidden leg", caller: callerCtx(d.memberUser, "member"), disembark: "2026-11-01", now: nextMorning, want: http.StatusNotFound},
		{name: "captain of a hidden leg", caller: captain, disembark: "2026-11-01", now: nextMorning, want: http.StatusCreated},
		{name: "admin before the end date", caller: callerCtx(1, "admin"), disembark: "2026-12-01", published: true, now: nextMorning, want: http.StatusCreated},
		{name: "admin without end date", caller: callerCtx(1, "admin"), published: true, now: nextMorning, want: http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := d.leg(t, d.captainPerson, tt.disembark, tt.published)
			resp := d.api(t, tt.now).PostCtx(tt.caller, fmt.Sprintf("/trips/%d/complete", id), map[string]any{"miles": 120})
			if resp.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			wantStatus := "planned"
			if tt.want == http.StatusCreated {
				wantStatus = "gone"
				var voyage dto.Voyage
				if err := json.Unmarshal(resp.Body.Bytes(), &voyage); err != nil {
					t.Fatalf("decode voyage: %v", err)
				}
				if voyage.CaptainPersonID == nil || *voyage.CaptainPersonID != d.captainPerson || voyage.Miles != 120 {
					t.Fatalf("voyage captain = %v, miles = %v; want %d, 120", voyage.CaptainPersonID, voyage.Miles, d.captainPerson)
				}
			}
			if got := d.tripStatus(t, id); got != wantStatus {
				t.Fatalf("trip after the request = %s, want %s", got, wantStatus)
			}
		})
	}
}

// TestCompleteTrip_CaptainCheckedInTransaction runs the transaction with the
// captain rule for a caller the leg's locked captain is not: it refuses
// before any write, whatever a check before the transaction saw.
func TestCompleteTrip_CaptainCheckedInTransaction(t *testing.T) {
	d := seedCaptainLegDB(t)
	id := d.leg(t, d.otherCaptainPerson, "2026-11-01", true)
	h := &TripHandler{now: func() time.Time { return time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC) }}
	ctx := callerCtx(d.captainUser, "member")

	_, err := completeTripTxAs(ctx, d.conn, nil, h.authorizeCaptainCompletion, id, dto.CompleteTripBody{})
	status, ok := errors.AsType[huma.StatusError](err)
	if !ok || status.GetStatus() != http.StatusForbidden {
		t.Fatalf("err = %v, want 403", err)
	}
	if got := d.tripStatus(t, id); got != "planned" {
		t.Fatalf("trip after the refusal = %s, want planned", got)
	}
}

// TestTripCrew_ContactsDB reads a leg's crew from the database as each
// caller: the phone and emergency contact come only to admins and the leg's
// captain, not to the captain of another leg the same person sails.
func TestTripCrew_ContactsDB(t *testing.T) {
	d := seedCaptainLegDB(t)
	id := d.leg(t, d.captainPerson, "2026-11-14", true)
	d.leg(t, d.otherCaptainPerson, "2026-11-14", true)
	api := d.api(t, time.Now())
	tests := []struct {
		name     string
		caller   context.Context
		contacts bool
	}{
		{name: "admin", caller: callerCtx(1, "admin"), contacts: true},
		{name: "leg captain", caller: callerCtx(d.captainUser, "member"), contacts: true},
		{name: "other member", caller: callerCtx(d.memberUser, "member"), contacts: false},
		{name: "captain of another leg with the same crew", caller: callerCtx(d.otherCaptainUser, "member"), contacts: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := api.GetCtx(tt.caller, fmt.Sprintf("/trips/%d/crew", id))
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", resp.Code, resp.Body)
			}
			var crew []dto.CrewAssignment
			if err := json.Unmarshal(resp.Body.Bytes(), &crew); err != nil || len(crew) != 1 {
				t.Fatalf("decode crew: %v; body=%s", err, resp.Body)
			}
			c := crew[0]
			if c.FullName != "Jan Nowak" || c.Role != "zalogant" || c.Email == nil {
				t.Fatalf("crew entry = %+v, want name, role and e-mail", c)
			}
			present := c.Phone != nil && *c.Phone == "+48 600 100 200" &&
				c.EmergencyContactName != nil && *c.EmergencyContactName == "Ewa Nowak" &&
				c.EmergencyContactPhone != nil && *c.EmergencyContactPhone == "+48 600 300 400"
			absent := c.Phone == nil && c.EmergencyContactName == nil && c.EmergencyContactPhone == nil
			if tt.contacts && !present || !tt.contacts && !absent {
				t.Fatalf("contacts = (%v, %v, %v), want present = %v", c.Phone, c.EmergencyContactName, c.EmergencyContactPhone, tt.contacts)
			}
		})
	}
}
