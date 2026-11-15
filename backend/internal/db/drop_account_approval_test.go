package db

import "testing"

const dropAccountApprovalMigration = "035_drop_account_approval.sql"

// TestMigrate_DropsAccountApproval checks migration 035: users.access_approved
// is gone, and every account keeps its role, membership and suspension, so a
// suspended account stays suspended and a pending one is now just an account.
func TestMigrate_DropsAccountApproval(t *testing.T) {
	conn := testDB(t)
	resetSchema(t, conn)
	applyMigrationsBefore(t, conn, dropAccountApprovalMigration)

	type account struct {
		email, role, membership string
		approved, suspended     bool
	}
	accounts := []account{
		{"admin@example.dev", "admin", "czlonek", true, false},
		{"member@example.dev", "member", "sympatyk", true, false},
		{"pending@example.dev", "member", "gosc", false, false},
		{"suspended@example.dev", "member", "czlonek", true, true},
		{"pending-suspended@example.dev", "member", "gosc", false, true},
	}
	ids := make([]int64, len(accounts))
	for i, a := range accounts {
		mustScan(t, conn.QueryRow(`INSERT INTO users (email, name, role, membership, access_approved, suspended)
			VALUES ($1, $1, $2, $3, $4, $5) RETURNING id`,
			a.email, a.role, a.membership, a.approved, a.suspended), &ids[i])
	}

	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	assertColumnAbsent(t, conn, "users", "access_approved")
	var count int
	mustScan(t, conn.QueryRow(`SELECT count(*) FROM users`), &count)
	if count != len(accounts) {
		t.Fatalf("users = %d, want %d", count, len(accounts))
	}
	for i, want := range accounts {
		var got account
		mustScan(t, conn.QueryRow(`SELECT email, role, membership, suspended FROM users WHERE id = $1`, ids[i]),
			&got.email, &got.role, &got.membership, &got.suspended)
		want.approved = false
		if got != want {
			t.Errorf("user %d = %+v, want %+v", ids[i], got, want)
		}
	}

	var suspended bool
	mustScan(t, conn.QueryRow(`INSERT INTO users (email, name) VALUES ('new@example.dev', 'New') RETURNING suspended`), &suspended)
	if suspended {
		t.Error("new account starts suspended")
	}
}
