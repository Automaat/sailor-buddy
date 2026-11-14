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
	"strings"
	"sync"
	"testing"
	"time"

	fbauth "firebase.google.com/go/v4/auth"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
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

// authProbe runs Auth over conn and returns a sign-in helper that reports
// the claims the handler saw.
func authProbe(t *testing.T, conn *sql.DB, verifier fakeVerifier) func(token string) auth.Claims {
	t.Helper()
	var seen *auth.Claims
	handler := Auth(verifier, sqlcdb.New(conn), conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = GetUser(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	return func(token string) auth.Claims {
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
}

func userUpdatedAt(t *testing.T, conn *sql.DB, id int64) time.Time {
	t.Helper()
	var at time.Time
	if err := conn.QueryRow(`SELECT updated_at FROM users WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("updated_at of user %d: %v", id, err)
	}
	return at
}

func TestAuth_KnownAccountIsNotWritten(t *testing.T) {
	conn := freshDB(t)
	signIn := authProbe(t, conn, fakeVerifier{
		"tok-admin": {UID: "fb-admin", Claims: map[string]any{"email": "admin@example.dev", "name": "Ada"}},
		"tok-ola":   {UID: "fb-ola", Claims: map[string]any{"email": "ola@example.dev", "name": "Ola Nowak"}},
	})

	for _, token := range []string{"tok-admin", "tok-ola"} {
		first := signIn(token)
		before := userUpdatedAt(t, conn, first.UserID)
		again := signIn(token)
		if again != first {
			t.Errorf("%s: repeat claims = %+v, want %+v", token, again, first)
		}
		if after := userUpdatedAt(t, conn, first.UserID); !after.Equal(before) {
			t.Errorf("%s: users.updated_at moved from %v to %v on a repeat request", token, before, after)
		}
	}
	if got := signIn("tok-ola").Role; got != "member" {
		t.Errorf("second account role = %q, want member", got)
	}
}

func TestAuth_ChangedAccountIsProvisioned(t *testing.T) {
	cases := []struct {
		name   string
		change string
		claims map[string]any
		want   auth.Claims
	}{
		{
			name:   "new token name before onboarding",
			claims: map[string]any{"email": "ola@example.dev", "name": "Ola Kowalska"},
			want:   auth.Claims{Email: "ola@example.dev", Name: "Ola Kowalska", Role: "admin"},
		},
		{
			name:   "new token name after onboarding",
			change: `UPDATE users SET onboarded_at = now() WHERE firebase_uid = 'fb-ola'`,
			claims: map[string]any{"email": "ola@example.dev", "name": "Ola Kowalska"},
			want:   auth.Claims{Email: "ola@example.dev", Name: "Ola Nowak", Role: "admin"},
		},
		{
			name:   "new token email",
			claims: map[string]any{"email": "ola@sail.example.dev", "name": "Ola Nowak"},
			want:   auth.Claims{Email: "ola@sail.example.dev", Name: "Ola Nowak", Role: "admin"},
		},
		{
			name:   "person removed",
			change: `DELETE FROM crew_members WHERE user_id = (SELECT id FROM users WHERE firebase_uid = 'fb-ola')`,
			claims: map[string]any{"email": "ola@example.dev", "name": "Ola Nowak"},
			want:   auth.Claims{Email: "ola@example.dev", Name: "Ola Nowak", Role: "admin"},
		},
		{
			name:   "no admin left",
			change: `UPDATE users SET role = 'member'`,
			claims: map[string]any{"email": "ola@example.dev", "name": "Ola Nowak"},
			want:   auth.Claims{Email: "ola@example.dev", Name: "Ola Nowak", Role: "admin"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := freshDB(t)
			signIn := authProbe(t, conn, fakeVerifier{
				"tok-ola":     {UID: "fb-ola", Claims: map[string]any{"email": "ola@example.dev", "name": "Ola Nowak"}},
				"tok-changed": {UID: "fb-ola", Claims: tc.claims},
			})
			tc.want.UserID = signIn("tok-ola").UserID
			if tc.change != "" {
				if _, err := conn.Exec(tc.change); err != nil {
					t.Fatalf("apply change: %v", err)
				}
			}
			for _, attempt := range []string{"first", "repeat"} {
				if got := signIn("tok-changed"); got != tc.want {
					t.Errorf("%s request claims = %+v, want %+v", attempt, got, tc.want)
				}
			}
			var people int
			if err := conn.QueryRow(`SELECT COUNT(*) FROM crew_members WHERE user_id = $1`, tc.want.UserID).Scan(&people); err != nil {
				t.Fatalf("count people: %v", err)
			}
			if people != 1 {
				t.Errorf("people of the account = %d, want 1", people)
			}
		})
	}
}

// roleProbe runs Auth over conn and returns a handler that writes the role
// the request ran with.
func roleProbe(conn *sql.DB, verifier fakeVerifier) http.Handler {
	return Auth(verifier, sqlcdb.New(conn), conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(GetUser(r.Context()).Role))
	}))
}

// parallelSignIns sends one GET /api/auth/me per token, all released at
// once, and returns "<status> <body>" per request in token order.
func parallelSignIns(handler http.Handler, tokens []string) []string {
	start := make(chan struct{})
	results := make([]string, len(tokens))
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			<-start
			handler.ServeHTTP(rec, req)
			results[i] = fmt.Sprintf("%d %s", rec.Code, strings.TrimSpace(rec.Body.String()))
		})
	}
	close(start)
	wg.Wait()
	return results
}

func repeat(token string, n int) []string {
	tokens := make([]string, n)
	for i := range tokens {
		tokens[i] = token
	}
	return tokens
}

func emptyClub(t *testing.T, conn *sql.DB) {
	t.Helper()
	if _, err := conn.Exec(`TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("empty club: %v", err)
	}
}

// TestFirstAdminRole_StaleRole passes a role read before the request's
// promotion check. A sibling request of the same account may have promoted
// it in between; that account must still run as admin.
func TestFirstAdminRole_StaleRole(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		want  string
	}{
		{name: "no admin", want: "admin"},
		{name: "account promoted by a sibling request", setup: `UPDATE users SET role = 'admin' WHERE firebase_uid = 'fb-new'`, want: "admin"},
		{
			name:  "another account is admin",
			setup: `INSERT INTO users (email, name, password_hash, role) VALUES ('boss@example.dev', 'Boss', '', 'admin')`,
			want:  "member",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := freshDB(t)
			ctx := context.Background()
			q := sqlcdb.New(conn)
			stale, err := q.UpsertUserByFirebaseUID(ctx, sqlcdb.UpsertUserByFirebaseUIDParams{
				Email:       "new@example.dev",
				Name:        "New",
				FirebaseUid: types.NullString{String: "fb-new", Valid: true},
			})
			if err != nil {
				t.Fatalf("insert account: %v", err)
			}
			if stale.Role != "member" {
				t.Fatalf("new account role = %q, want member", stale.Role)
			}
			if tc.setup != "" {
				if _, err := conn.Exec(tc.setup); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			if got := firstAdminRole(ctx, q, conn, stale); got != tc.want {
				t.Errorf("firstAdminRole = %q, want %q", got, tc.want)
			}
			n, err := q.CountAdmins(ctx)
			if err != nil {
				t.Fatalf("count admins: %v", err)
			}
			if n != 1 {
				t.Errorf("admins = %d, want 1", n)
			}
		})
	}
}

// TestAuth_ParallelFirstRequestsOfNewAccountAllAdmin sends 8 requests of
// one new account at once to an empty club; every one runs as admin.
func TestAuth_ParallelFirstRequestsOfNewAccountAllAdmin(t *testing.T) {
	conn := freshDB(t)
	handler := roleProbe(conn, fakeVerifier{
		"tok-a": {UID: "fb-a", Claims: map[string]any{"email": "a@example.dev", "name": "A"}},
	})
	for round := range 50 {
		emptyClub(t, conn)
		for i, got := range parallelSignIns(handler, repeat("tok-a", 8)) {
			if got != "200 admin" {
				t.Fatalf("round %d request %d = %q, want 200 admin", round, i, got)
			}
		}
	}
}

// TestAuth_ParallelFirstRequestsOfTwoAccountsOneAdmin sends 4 requests of
// each of two new accounts at once to an empty club; exactly one account
// becomes admin and all its requests run as admin.
func TestAuth_ParallelFirstRequestsOfTwoAccountsOneAdmin(t *testing.T) {
	conn := freshDB(t)
	handler := roleProbe(conn, fakeVerifier{
		"tok-a": {UID: "fb-a", Claims: map[string]any{"email": "a@example.dev", "name": "A"}},
		"tok-b": {UID: "fb-b", Claims: map[string]any{"email": "b@example.dev", "name": "B"}},
	})
	for round := range 25 {
		emptyClub(t, conn)
		results := parallelSignIns(handler, append(repeat("tok-a", 4), repeat("tok-b", 4)...))
		roles := map[string]map[string]int{"fb-a": {}, "fb-b": {}}
		for i, got := range results {
			roles[[]string{"fb-a", "fb-b"}[i/4]][got]++
		}
		var stored []string
		rows, err := conn.Query(`SELECT firebase_uid FROM users WHERE role = 'admin'`)
		if err != nil {
			t.Fatalf("list admins: %v", err)
		}
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				t.Fatalf("scan admin: %v", err)
			}
			stored = append(stored, uid)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close admins: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("round %d: admins = %v, want exactly one", round, stored)
		}
		for uid, got := range roles {
			want := "200 member"
			if uid == stored[0] {
				want = "200 admin"
			}
			if got[want] != 4 {
				t.Fatalf("round %d: %s results = %v, want 4 × %q", round, uid, got, want)
			}
		}
	}
}

// TestAuth_ParallelFirstRequestsNeverUnauthorized sends 16 first requests
// of a new account with an unverified email at once; none may fail.
func TestAuth_ParallelFirstRequestsNeverUnauthorized(t *testing.T) {
	conn := freshDB(t)
	verifier := fakeVerifier{}
	for round := range 100 {
		verifier[fmt.Sprintf("tok-%d", round)] = &fbauth.Token{
			UID:    fmt.Sprintf("fb-%d", round),
			Claims: map[string]any{"email": fmt.Sprintf("u%d@example.dev", round), "email_verified": false},
		}
	}
	handler := roleProbe(conn, verifier)
	for round := range 100 {
		for i, got := range parallelSignIns(handler, repeat(fmt.Sprintf("tok-%d", round), 16)) {
			if !strings.HasPrefix(got, "200 ") {
				t.Fatalf("round %d request %d = %q, want 200", round, i, got)
			}
		}
	}
	var accounts int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&accounts); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 100 {
		t.Errorf("accounts = %d, want 100", accounts)
	}
}

// TestAuth_FirstRequestRacingSameAccountInsert replays the insert race
// deterministically: a sibling request's account row is inserted but not
// committed when this request upserts, so the email index blocks it. Once
// the sibling commits the account with the same UID, the request must use
// that account instead of treating the email as taken.
func TestAuth_FirstRequestRacingSameAccountInsert(t *testing.T) {
	conn := freshDB(t)
	ctx := context.Background()
	handler := roleProbe(conn, fakeVerifier{
		"tok-a": {UID: "fb-a", Claims: map[string]any{"email": "a@example.dev", "name": "A"}},
	})
	sibling, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin sibling: %v", err)
	}
	defer func() { _ = sibling.Rollback() }()
	if _, err := sibling.Exec(`INSERT INTO users (email, name, password_hash) VALUES ('a@example.dev', 'A', '')`); err != nil {
		t.Fatalf("sibling insert: %v", err)
	}

	result := make(chan string, 1)
	go func() { result <- parallelSignIns(handler, []string{"tok-a"})[0] }()
	waitForLockWaiters(t, conn, 1)
	if _, err := sibling.Exec(`UPDATE users SET firebase_uid = 'fb-a' WHERE email = 'a@example.dev'`); err != nil {
		t.Fatalf("sibling sets uid: %v", err)
	}
	if err := sibling.Commit(); err != nil {
		t.Fatalf("sibling commit: %v", err)
	}

	if got := <-result; got != "200 admin" {
		t.Fatalf("racing first request = %q, want 200 admin", got)
	}
	var accounts int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&accounts); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accounts != 1 {
		t.Errorf("accounts = %d, want 1", accounts)
	}
}

// TestAuth_EmailOfExistingAccount keeps link-by-email: a verified token
// links an account that has the email but no UID; an unverified one is
// refused.
func TestAuth_EmailOfExistingAccount(t *testing.T) {
	cases := []struct {
		name     string
		verified bool
		want     int
	}{
		{name: "verified email links", verified: true, want: http.StatusOK},
		{name: "unverified email refused", verified: false, want: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := freshDB(t)
			var id int64
			if err := conn.QueryRow(`INSERT INTO users (email, name, password_hash, role)
				VALUES ('ola@example.dev', 'Ola', '', 'admin') RETURNING id`).Scan(&id); err != nil {
				t.Fatalf("insert account: %v", err)
			}
			var seen int64
			handler := Auth(fakeVerifier{
				"tok-ola": {UID: "fb-ola", Claims: map[string]any{"email": "ola@example.dev", "email_verified": tc.verified}},
			}, sqlcdb.New(conn), conn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = GetUser(r.Context()).UserID
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
			req.Header.Set("Authorization", "Bearer tok-ola")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body)
			}
			if tc.verified && seen != id {
				t.Errorf("linked user = %d, want %d", seen, id)
			}
		})
	}
}
