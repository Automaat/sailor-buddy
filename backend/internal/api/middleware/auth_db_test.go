package middleware

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// freshDB creates a migrated database of its own on the server named by
// SAILOR_TEST_DATABASE_URL and drops it after the test.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SAILOR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAILOR_TEST_DATABASE_URL not set; skipping database test")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random db name: %v", err)
	}
	name := "middleware_" + hex.EncodeToString(b)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.Path = "/" + name
	conn, err := db.Open(u.String())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

// fakeVerifier accepts any token naming a key of its map.
type fakeVerifier map[string]*fbauth.Token

func (f fakeVerifier) VerifyIDToken(_ context.Context, token string) (*fbauth.Token, error) {
	if tok, ok := f[token]; ok {
		return tok, nil
	}
	return nil, errors.New("invalid token")
}

func TestAuth_FirstSignInCreatesOnePerson(t *testing.T) {
	conn := freshDB(t)
	q := sqlcdb.New(conn)
	verifier := fakeVerifier{
		"tok-ola": {UID: "fb-ola", Claims: map[string]any{"email": "ola@example.dev", "name": "Ola Nowak"}},
		"tok-new": {UID: "fb-new", Claims: map[string]any{"email": "new@example.dev"}},
	}
	var seen *int64
	handler := Auth(verifier, q, conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetUser(r.Context()).UserID
		seen = &id
		w.WriteHeader(http.StatusNoContent)
	}))
	signIn := func(token string) int64 {
		t.Helper()
		seen = nil
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent || seen == nil {
			t.Fatalf("sign in %s: got %d; body=%s", token, rec.Code, rec.Body)
		}
		return *seen
	}

	ola := signIn("tok-ola")
	signIn("tok-ola")
	newcomer := signIn("tok-new")

	for user, want := range map[int64]string{ola: "Ola Nowak", newcomer: "new@example.dev"} {
		person, err := q.GetCrewMemberByUserID(context.Background(), types.NullInt64{Int64: user, Valid: true})
		if err != nil {
			t.Fatalf("person of user %d: %v", user, err)
		}
		if person.FullName != want {
			t.Errorf("person of user %d = %q, want %q", user, person.FullName, want)
		}
	}
	var people int64
	if err := conn.QueryRow(`SELECT COUNT(*) FROM crew_members`).Scan(&people); err != nil {
		t.Fatalf("count people: %v", err)
	}
	if people != 2 {
		t.Errorf("people after three sign-ins of two accounts = %d, want 2", people)
	}
}

// waitForLockWaiters blocks until at least n sessions of the test database
// are waiting on a lock.
func waitForLockWaiters(t *testing.T, conn *sql.DB, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := conn.QueryRow(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("count lock waiters: %v", err)
		}
		if waiting >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d lock waiters, have %d", n, waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestAuth_ConcurrentFirstSignInsPromoteOneAdmin replays two fresh accounts
// signing in at once while the club has no admin. Both pass the unlocked
// admin count; the test holds the first-admin lock until both wait on it,
// then exactly one of them must become admin.
func TestAuth_ConcurrentFirstSignInsPromoteOneAdmin(t *testing.T) {
	conn := freshDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	verifier := fakeVerifier{
		"tok-a": {UID: "fb-a", Claims: map[string]any{"email": "a@example.dev", "name": "A"}},
		"tok-b": {UID: "fb-b", Claims: map[string]any{"email": "b@example.dev", "name": "B"}},
	}
	handler := Auth(verifier, q, conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(GetUser(r.Context()).Role))
	}))

	holder, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback() }()
	if err := sqlcdb.New(holder).LockFirstAdmin(ctx); err != nil {
		t.Fatalf("hold first-admin lock: %v", err)
	}

	roles := make(chan string, len(verifier))
	for token := range verifier {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			roles <- fmt.Sprintf("%d %s", rec.Code, rec.Body)
		}()
	}
	waitForLockWaiters(t, conn, len(verifier))
	if err := holder.Rollback(); err != nil {
		t.Fatalf("release first-admin lock: %v", err)
	}

	got := map[string]int{}
	for range verifier {
		got[<-roles]++
	}
	if got["200 admin"] != 1 || got["200 member"] != 1 {
		t.Fatalf("sign-in results = %v, want one 200 admin and one 200 member", got)
	}
	n, err := q.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if n != 1 {
		t.Fatalf("admins after concurrent first sign-ins = %d, want 1", n)
	}
}

// TestAuth_ConcurrentFirstRequestsOfOneAccountBothAdmin replays a fresh
// account sending two requests at once while the club has no admin. Both
// pass the unlocked admin count; the one that waits for the first-admin
// lock must still report admin once the other has promoted the account.
func TestAuth_ConcurrentFirstRequestsOfOneAccountBothAdmin(t *testing.T) {
	conn := freshDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	verifier := fakeVerifier{
		"tok-a": {UID: "fb-a", Claims: map[string]any{"email": "a@example.dev", "name": "A"}},
	}
	handler := Auth(verifier, q, conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(GetUser(r.Context()).Role))
	}))
	signIn := func() string {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
		req.Header.Set("Authorization", "Bearer tok-a")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return fmt.Sprintf("%d %s", rec.Code, rec.Body)
	}
	holder, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback() }()
	if err := sqlcdb.New(holder).LockFirstAdmin(ctx); err != nil {
		t.Fatalf("hold first-admin lock: %v", err)
	}
	first := make(chan string, 1)
	go func() { first <- signIn() }()
	waitForLockWaiters(t, conn, 1)

	second := make(chan string, 1)
	go func() { second <- signIn() }()
	waitForLockWaiters(t, conn, 2)
	if err := holder.Rollback(); err != nil {
		t.Fatalf("release first-admin lock: %v", err)
	}

	for _, got := range []string{<-first, <-second} {
		if got != "200 admin" {
			t.Fatalf("sign-in result = %q, want 200 admin for both requests", got)
		}
	}
}
