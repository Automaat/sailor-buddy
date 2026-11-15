package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// personContacts reads the crew person through GET /crew/{id} and through
// the people list, and returns the emergency contact name each one shows.
func personContacts(t *testing.T, api humatest.TestAPI, caller context.Context, personID int64) (detail, list *string) {
	t.Helper()
	resp := api.GetCtx(caller, fmt.Sprintf("/crew/%d", personID))
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /crew/%d = %d; body=%s", personID, resp.Code, resp.Body)
	}
	var person dto.CrewMember
	if err := json.Unmarshal(resp.Body.Bytes(), &person); err != nil {
		t.Fatalf("decode person: %v", err)
	}
	if person.Phone == nil || person.Email == nil {
		t.Fatalf("person phone/e-mail = (%v, %v), want both for every caller", person.Phone, person.Email)
	}
	resp = api.GetCtx(caller, "/crew?q=Jan%20Nowak")
	if resp.Code != http.StatusOK {
		t.Fatalf("GET /crew = %d; body=%s", resp.Code, resp.Body)
	}
	var page dto.Page[dto.CrewMember]
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != personID {
		t.Fatalf("decode people: %v; body=%s", err, resp.Body)
	}
	return person.EmergencyContactName, page.Items[0].EmergencyContactName
}

// TestPeople_ContactsFollowCaptainChange reassigns the captain of the
// person's leg and reads the person as each caller, by detail and in the
// list: the emergency contact comes to admins and to current captains of a
// leg or voyage the person is on, never to the former captain or a member.
func TestPeople_ContactsFollowCaptainChange(t *testing.T) {
	d := seedCaptainLegDB(t)
	ctx := context.Background()
	id := d.leg(t, d.captainPerson, "2026-11-14", true)
	if _, err := d.conn.ExecContext(ctx,
		"UPDATE trips SET captain_person_id = $1 WHERE id = $2", d.otherCaptainPerson, id); err != nil {
		t.Fatalf("reassign captain: %v", err)
	}
	voyageCaptain, err := d.q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "rejs@example.dev", Name: "rejs"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	voyage, err := d.q.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		CruiseID: d.cruiseID, Name: "Rejs",
		CaptainPersonID: types.NullInt64{Int64: ensurePerson(t, d.conn, voyageCaptain.ID), Valid: true},
	})
	if err != nil {
		t.Fatalf("create voyage: %v", err)
	}
	if _, err := d.q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID: types.NullInt64{Int64: voyage.ID, Valid: true}, CrewMemberID: d.crewPerson, Role: "zalogant",
	}); err != nil {
		t.Fatalf("add voyage crew: %v", err)
	}
	api := d.api(t, time.Now())
	for _, tt := range []struct {
		name     string
		caller   context.Context
		contacts bool
	}{
		{name: "admin", caller: callerCtx(1, "admin"), contacts: true},
		{name: "former leg captain", caller: callerCtx(d.captainUser, "member"), contacts: false},
		{name: "new leg captain", caller: callerCtx(d.otherCaptainUser, "member"), contacts: true},
		{name: "voyage captain", caller: callerCtx(voyageCaptain.ID, "member"), contacts: true},
		{name: "member", caller: callerCtx(d.memberUser, "member"), contacts: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			detail, list := personContacts(t, api, tt.caller, d.crewPerson)
			for where, got := range map[string]*string{"detail": detail, "list": list} {
				if tt.contacts && (got == nil || *got != "Ewa Nowak") || !tt.contacts && got != nil {
					t.Errorf("%s emergency contact = %v, want present = %v", where, got, tt.contacts)
				}
			}
		})
	}
}

// interleaveQuerier runs between once, right after the handler's first
// person read returns, so a change it commits lands between that read and
// anything the handler queries after it.
type interleaveQuerier struct {
	sqlcdb.Querier
	once    sync.Once
	between func()
}

func (q *interleaveQuerier) GetCrewMemberForCaller(ctx context.Context, arg sqlcdb.GetCrewMemberForCallerParams) (sqlcdb.GetCrewMemberForCallerRow, error) {
	row, err := q.Querier.GetCrewMemberForCaller(ctx, arg)
	q.once.Do(q.between)
	return row, err
}

func (q *interleaveQuerier) ListCrewMembers(ctx context.Context, arg sqlcdb.ListCrewMembersParams) ([]sqlcdb.ListCrewMembersRow, error) {
	rows, err := q.Querier.ListCrewMembers(ctx, arg)
	q.once.Do(q.between)
	return rows, err
}

// TestPeople_ContactsSnapshot commits a new emergency contact and makes the
// caller the person's leg captain right after the handler read the person.
// The caller was never captain while the old contact was current, so the
// response must not carry it; a read after the commit shows the new one.
func TestPeople_ContactsSnapshot(t *testing.T) {
	for _, path := range []string{"detail", "list"} {
		t.Run(path, func(t *testing.T) {
			d := seedCaptainLegDB(t)
			ctx := context.Background()
			id := d.leg(t, d.captainPerson, "2026-11-14", true)
			memberPerson := ensurePerson(t, d.conn, d.memberUser)
			q := &interleaveQuerier{Querier: d.q, between: func() {
				tx, err := d.conn.BeginTx(ctx, nil)
				if err != nil {
					t.Errorf("begin: %v", err)
					return
				}
				defer func() { _ = tx.Rollback() }()
				if _, err := tx.ExecContext(ctx, "UPDATE crew_members SET emergency_contact_name = 'Nowy kontakt' WHERE id = $1", d.crewPerson); err != nil {
					t.Errorf("change contact: %v", err)
					return
				}
				if _, err := tx.ExecContext(ctx, "UPDATE trips SET captain_person_id = $1 WHERE id = $2", memberPerson, id); err != nil {
					t.Errorf("assign captain: %v", err)
					return
				}
				if err := tx.Commit(); err != nil {
					t.Errorf("commit: %v", err)
				}
			}}
			_, api := humatest.New(t)
			registerCrewRoutes(api, NewCrewHandler(q, d.conn, nil))
			caller := callerCtx(d.memberUser, "member")

			var raced *string
			if path == "detail" {
				resp := api.GetCtx(caller, fmt.Sprintf("/crew/%d", d.crewPerson))
				var person dto.CrewMember
				if err := json.Unmarshal(resp.Body.Bytes(), &person); err != nil || resp.Code != http.StatusOK {
					t.Fatalf("GET /crew/%d = %d (%v); body=%s", d.crewPerson, resp.Code, err, resp.Body)
				}
				raced = person.EmergencyContactName
			} else {
				resp := api.GetCtx(caller, "/crew?q=Jan%20Nowak")
				var page dto.Page[dto.CrewMember]
				if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil || resp.Code != http.StatusOK || len(page.Items) != 1 {
					t.Fatalf("GET /crew = %d (%v); body=%s", resp.Code, err, resp.Body)
				}
				raced = page.Items[0].EmergencyContactName
			}
			if raced != nil {
				t.Fatalf("emergency contact read before the captain change = %q, want none", *raced)
			}

			detail, list := personContacts(t, api, caller, d.crewPerson)
			if detail == nil || *detail != "Nowy kontakt" || list == nil || *list != "Nowy kontakt" {
				t.Fatalf("emergency contact after the change = (%v, %v), want the new one", detail, list)
			}
		})
	}
}
