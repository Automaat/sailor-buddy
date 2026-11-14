package db

import (
	"context"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestMigrate_BackfillsOnePersonPerAccount(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, "024_people.sql")

	var named, unnamed, linked int64
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name, phone, patent_number)
		VALUES ('ola@example.dev', ' Ola Nowak ', '+48 600 000 001', 'JS-1') RETURNING id`), &named)
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('anon@example.dev', '') RETURNING id`), &unnamed)
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('jan@example.dev', 'Jan') RETURNING id`), &linked)
	if _, err := conn.Exec(`INSERT INTO crew_members (user_id, full_name) VALUES ($1, 'Jan Kowalski')`, linked); err != nil {
		t.Fatalf("seed linked person: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO crew_members (full_name, email) VALUES ('Ola (old record)', 'OLA@example.dev')`); err != nil {
		t.Fatalf("seed unlinked person: %v", err)
	}

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	type person struct {
		name, email, phone, patent string
		createdBy                  bool
	}
	get := func(user int64) person {
		t.Helper()
		var p person
		mustScan(t, conn.QueryRow(`SELECT full_name, COALESCE(email, ''), COALESCE(phone, ''),
			COALESCE(patent_number, ''), created_by IS NOT NULL FROM crew_members WHERE user_id = $1`, user),
			&p.name, &p.email, &p.phone, &p.patent, &p.createdBy)
		return p
	}
	if got, want := get(named), (person{"Ola Nowak", "ola@example.dev", "+48 600 000 001", "JS-1", false}); got != want {
		t.Errorf("named account person = %+v, want %+v", got, want)
	}
	if got := get(unnamed); got.name != "anon@example.dev" {
		t.Errorf("unnamed account person name = %q, want the e-mail", got.name)
	}
	if got := get(linked); got.name != "Jan Kowalski" {
		t.Errorf("linked account person = %q, want the existing record kept", got.name)
	}

	var total, accounts, unlinkedOla int64
	mustScan(t, conn.QueryRow(`SELECT COUNT(*), COUNT(user_id),
		COUNT(*) FILTER (WHERE full_name = 'Ola (old record)' AND user_id IS NULL) FROM crew_members`),
		&total, &accounts, &unlinkedOla)
	if total != 4 || accounts != 3 || unlinkedOla != 1 {
		t.Errorf("people = %d (%d with account, old Ola unlinked=%d), want 4 (3, 1): no link by e-mail", total, accounts, unlinkedOla)
	}
}

func TestMigrate_PeopleRefusesDuplicateLinks(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, "024_people.sql")

	var user int64
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('jan@example.dev', 'Jan') RETURNING id`), &user)
	if _, err := conn.Exec(`INSERT INTO crew_members (user_id, full_name) VALUES ($1, 'Jan A'), ($1, 'Jan B')`, user); err != nil {
		t.Fatalf("seed duplicate links: %v", err)
	}

	err := Migrate(t.Context(), conn)
	if err == nil || !strings.Contains(err.Error(), "linked to more than one crew member") {
		t.Fatalf("migrate = %v, want the duplicate-link refusal", err)
	}
	var people int64
	mustScan(t, conn.QueryRow(`SELECT COUNT(*) FROM crew_members`), &people)
	if people != 2 {
		t.Errorf("people after refused migration = %d, want the 2 untouched rows", people)
	}
}

func TestPeople_OneAccountOnePerson(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "ewa@example.dev", Name: "Ewa"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	for range 2 {
		if err := q.EnsureUserPerson(ctx, user.ID); err != nil {
			t.Fatalf("ensure person: %v", err)
		}
	}
	userID := types.NullInt64{Int64: user.ID, Valid: true}
	person, err := q.GetCrewMemberByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	if person.FullName != "Ewa" || person.Email.String != "ewa@example.dev" {
		t.Errorf("person = (%q, %q), want (Ewa, ewa@example.dev)", person.FullName, person.Email.String)
	}

	_, err = q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{FullName: "Ewa again", UserID: userID})
	assertPgError(t, "second person for one account", "23505", err)

	other, err := q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{FullName: "Someone"})
	if err != nil {
		t.Fatalf("create unlinked person: %v", err)
	}
	_, err = conn.Exec(`UPDATE crew_members SET user_id = $1 WHERE id = $2`, user.ID, other.ID)
	assertPgError(t, "link a second person to the account", "23505", err)

	if _, err := q.UpdateUserProfile(ctx, sqlcdb.UpdateUserProfileParams{
		ID: user.ID, FirstName: "Ewa", LastName: "Wiśniewska",
		Phone:        types.NullString{String: "+48 600 000 002", Valid: true},
		PatentType:   types.NullString{String: "jachtowy_sternik_morski", Valid: true},
		PatentNumber: types.NullString{String: "JSM-7", Valid: true},
	}); err != nil {
		t.Fatalf("update profile: %v", err)
	}
	person, err = q.GetCrewMemberByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("get person after profile: %v", err)
	}
	if person.FullName != "Ewa Wiśniewska" || person.Phone.String != "+48 600 000 002" || person.PatentNumber.String != "JSM-7" {
		t.Errorf("person after profile = (%q, %q, %q), want the profile's name, phone and patent",
			person.FullName, person.Phone.String, person.PatentNumber.String)
	}
	unchanged, err := q.GetCrewMember(ctx, other.ID)
	if err != nil {
		t.Fatalf("get other person: %v", err)
	}
	if unchanged.FullName != "Someone" {
		t.Errorf("profile edit changed another person to %q", unchanged.FullName)
	}
}
