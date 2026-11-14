package middleware

import (
	"sync"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
)

func TestBootstrapAdminRequiresRegisteredUID(t *testing.T) {
	conn := freshDB(t)
	for _, uid := range []string{"", "unknown", " unknown "} {
		if err := auth.BootstrapAdmin(t.Context(), conn, uid); err == nil {
			t.Fatalf("accepted UID %q", uid)
		}
	}
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("created %d accounts", n)
	}
}

func TestBootstrapAdminIsExplicitAndIdempotent(t *testing.T) {
	conn := freshDB(t)
	if _, err := conn.Exec(`INSERT INTO users(email,name,firebase_uid) VALUES ('chosen@example.dev','Chosen','chosen'),('other@example.dev','Other','other')`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := auth.BootstrapAdmin(t.Context(), conn, "chosen"); err != nil {
			t.Fatal(err)
		}
	}
	if err := auth.BootstrapAdmin(t.Context(), conn, "other"); err == nil {
		t.Fatal("added a second admin")
	}
	var chosen, other string
	if err := conn.QueryRow("SELECT role FROM users WHERE firebase_uid='chosen'").Scan(&chosen); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("SELECT role FROM users WHERE firebase_uid='other'").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if chosen != "admin" || other != "member" {
		t.Fatalf("roles: chosen=%s other=%s", chosen, other)
	}
}

func TestBootstrapAdminConcurrentTargetsPromoteOnlyOne(t *testing.T) {
	conn := freshDB(t)
	if _, err := conn.Exec(`INSERT INTO users(email,name,firebase_uid) VALUES ('a@example.dev','A','a'),('b@example.dev','B','b')`); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, uid := range []string{"a", "b"} {
		wg.Go(func() { <-start; results <- auth.BootstrapAdmin(t.Context(), conn, uid) })
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful bootstraps=%d", successes)
	}
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM users WHERE role='admin'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("admins=%d", n)
	}
}

func TestAuthPreservesPrivatelyProvisionedAdmin(t *testing.T) {
	conn := freshDB(t)
	verifier := fakeVerifier{"chosen": {UID: "chosen", Claims: map[string]any{"email": "chosen@example.dev"}}, "other": {UID: "other", Claims: map[string]any{"email": "other@example.dev"}}}
	signIn := authProbe(t, conn, verifier)
	if got := signIn("chosen").Role; got != "member" {
		t.Fatalf("first sign-in role=%s", got)
	}
	if err := auth.BootstrapAdmin(t.Context(), conn, "chosen"); err != nil {
		t.Fatal(err)
	}
	if got := signIn("chosen").Role; got != "admin" {
		t.Fatalf("bootstrap role=%s", got)
	}
	if got := signIn("other").Role; got != "member" {
		t.Fatalf("other role=%s", got)
	}
	if _, err := conn.Exec("UPDATE users SET role='member'"); err != nil {
		t.Fatal(err)
	}
	if got := signIn("chosen").Role; got != "member" {
		t.Fatalf("role after demotion=%s", got)
	}
}
