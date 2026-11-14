package middleware

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

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
	handler := Auth(verifier, q)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
