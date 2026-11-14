package db

import (
	"context"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestMigrate_SplitsUserNames(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, "021_user_onboarding.sql")

	for _, name := range []string{"Jan Maria Kowalski", "  Anna  Nowak ", "Cher"} {
		if _, err := conn.Exec(`INSERT INTO users (email, name) VALUES ($1, $2)`, name+"@example.dev", name); err != nil {
			t.Fatalf("insert %q: %v", name, err)
		}
	}
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	want := map[string][2]string{
		"Jan Maria Kowalski": {"Jan", "Maria Kowalski"},
		"  Anna  Nowak ":     {"Anna", "Nowak"},
		"Cher":               {"Cher", ""},
	}
	for name, parts := range want {
		var first, last string
		var onboarded bool
		mustScan(t, conn.QueryRow(`SELECT first_name, last_name, onboarded_at IS NOT NULL
			FROM users WHERE name = $1`, name), &first, &last, &onboarded)
		if first != parts[0] || last != parts[1] || onboarded {
			t.Errorf("%q split to (%q, %q, onboarded=%v), want (%q, %q, false)",
				name, first, last, onboarded, parts[0], parts[1])
		}
	}
}

func TestUserProfile_OnboardingKeepsName(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	q := sqlcdb.New(conn)

	uid := types.NullString{String: "fb-1", Valid: true}
	user, err := q.UpsertUserByFirebaseUID(ctx, sqlcdb.UpsertUserByFirebaseUIDParams{
		Email: "jan@example.dev", Name: "jan k", FirebaseUid: uid,
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if user.Membership != "gosc" || user.OnboardedAt.Valid {
		t.Fatalf("new user = (%q, onboarded=%v), want (gosc, false)", user.Membership, user.OnboardedAt.Valid)
	}

	user, err = q.UpdateUserProfile(ctx, sqlcdb.UpdateUserProfileParams{
		ID: user.ID, FirstName: "Jan", LastName: "Kowalski",
		Phone: types.NullString{String: "+48 600 100 200", Valid: true},
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if user.Name != "Jan Kowalski" || !user.OnboardedAt.Valid {
		t.Fatalf("after onboarding = (%q, onboarded=%v), want (Jan Kowalski, true)", user.Name, user.OnboardedAt.Valid)
	}

	user, err = q.UpsertUserByFirebaseUID(ctx, sqlcdb.UpsertUserByFirebaseUIDParams{
		Email: "jan@example.dev", Name: "jan k", FirebaseUid: uid,
	})
	if err != nil {
		t.Fatalf("re-provision: %v", err)
	}
	if user.Name != "Jan Kowalski" {
		t.Fatalf("sign-in overwrote onboarded name with %q", user.Name)
	}

	user, err = q.LinkFirebaseUIDByEmail(ctx, sqlcdb.LinkFirebaseUIDByEmailParams{
		FirebaseUid: uid, NewName: "jan k", Email: "jan@example.dev",
	})
	if err != nil {
		t.Fatalf("link by email: %v", err)
	}
	if user.Name != "Jan Kowalski" {
		t.Fatalf("link by email overwrote onboarded name with %q", user.Name)
	}
}
