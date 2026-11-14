package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type securityVerifier map[string]*fbauth.Token

func (v securityVerifier) VerifyIDToken(_ context.Context, token string) (*fbauth.Token, error) {
	tokenValue, ok := v[token]
	if !ok {
		return nil, fmt.Errorf("invalid test token")
	}
	return tokenValue, nil
}

func securityDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SAILOR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("local test database not configured")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	name := "security_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + name + " WITH (FORCE)") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	conn, err := db.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	return conn
}

func securityRouter(t *testing.T, conn *sql.DB) (http.Handler, *os.Root) {
	t.Helper()
	q := sqlcdb.New(conn)
	verifier := securityVerifier{}
	for _, token := range []string{"pending", "approved", "unverified", "suspended", "captain", "subject", "admin"} {
		verifier[token] = &fbauth.Token{UID: token, Claims: map[string]any{"email": token + "@example.dev", "email_verified": token != "unverified"}}
	}
	uploads, err := handlers.OpenUploadRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = uploads.Close() })
	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth(verifier, q), middleware.Admission)
		registerHumaRoutes(humachi.New(r, humaConfig()), q, conn, uploads)
	})
	for token := range verifier {
		resp := securityRequest(r, token, http.MethodGet, "/api/auth/me", "")
		if resp.Code != 200 {
			t.Fatalf("provision %s: %d %s", token, resp.Code, resp.Body)
		}
	}
	if _, err := conn.Exec(`UPDATE users SET access_approved = firebase_uid <> 'pending', suspended = firebase_uid = 'suspended'`); err != nil {
		t.Fatal(err)
	}
	if err := auth.BootstrapAdmin(t.Context(), conn, "admin"); err != nil {
		t.Fatal(err)
	}
	return r, uploads
}

func securityRequest(h http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestAdmissionMatrix(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	cases := []struct {
		name, token, method, path, body string
		want                            int
	}{
		{"anonymous club read", "", http.MethodGet, "/api/crew", "", 401},
		{"pending club read", "pending", http.MethodGet, "/api/crew", "", 403},
		{"unverified club read", "unverified", http.MethodGet, "/api/crew", "", 403},
		{"suspended profile", "suspended", http.MethodGet, "/api/auth/me", "", 403},
		{"pending profile", "pending", http.MethodGet, "/api/auth/me", "", 200},
		{"pending onboarding", "pending", http.MethodPut, "/api/auth/me", `{"first_name":"New","last_name":"Member"}`, 200},
		{"pending token preview", "pending", http.MethodGet, "/api/enroll/missing", "", 404},
		{"pending enroll", "pending", http.MethodPost, "/api/enroll/missing", "{}", 404},
		{"pending enroll legs", "pending", http.MethodPost, "/api/enroll/missing/legs", `{"trip_ids":[1]}`, 404},
		{"pending trainings", "pending", http.MethodGet, "/api/trainings", "", 403},
		{"pending candidates", "pending", http.MethodGet, "/api/auth/me/person-candidates", "", 403},
		{"approved directory", "approved", http.MethodGet, "/api/crew", "", 200},
		{"approved cannot mutate", "approved", http.MethodPost, "/api/crew", `{"full_name":"Injected"}`, 403},
		{"admin directory", "admin", http.MethodGet, "/api/crew", "", 200},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rr := securityRequest(h, tt.token, tt.method, tt.path, tt.body)
			if rr.Code != tt.want {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tt.want, rr.Body)
			}
		})
	}
}

func TestSensitiveDataAccess(t *testing.T) {
	conn := securityDatabase(t)
	h, uploads := securityRouter(t, conn)
	var captain, subject, outsider, cruise, voyage, opinion int64
	for uid, dest := range map[string]*int64{"captain": &captain, "subject": &subject, "approved": &outsider} {
		if err := conn.QueryRow(`SELECT cm.id FROM crew_members cm JOIN users u ON u.id=cm.user_id WHERE u.firebase_uid=$1`, uid).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`UPDATE crew_members SET emergency_contact_name='PRIVATE-NAME',emergency_contact_phone='PRIVATE-PHONE' WHERE id=$1`, subject); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`INSERT INTO cruises(name,group_chat_url,enroll_token) VALUES('Event','https://chat.example/private','share') RETURNING id`).Scan(&cruise); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`INSERT INTO voyages(name,cruise_id,captain_person_id) VALUES('Voyage',$1,$2) RETURNING id`, cruise, captain).Scan(&voyage); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO crew_assignments(voyage_id,crew_member_id,role) VALUES($1,$2,'zalogant')`, voyage, subject); err != nil {
		t.Fatal(err)
	}
	if err := uploads.MkdirAll("opinions", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := uploads.WriteFile("opinions/test.pdf", []byte("private-document"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`INSERT INTO voyage_opinions(voyage_id,crew_member_id,file_path,file_format) VALUES($1,$2,$3,'pdf') RETURNING id`, voyage, subject, filepath.Join(uploads.Name(), "opinions/test.pdf")).Scan(&opinion); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"approved", "subject", "captain", "admin"} {
		t.Run(token, func(t *testing.T) {
			rr := securityRequest(h, token, "GET", "/api/crew", "")
			contains := strings.Contains(rr.Body.String(), "PRIVATE-PHONE")
			want := token == "captain" || token == "admin"
			if rr.Code != 200 || contains != want {
				t.Fatalf("directory status=%d exposed=%v want=%v", rr.Code, contains, want)
			}
			path := fmt.Sprintf("/api/crew/%d", subject)
			rr = securityRequest(h, token, "GET", path, "")
			if rr.Code != 200 || strings.Contains(rr.Body.String(), "PRIVATE-PHONE") != want {
				t.Fatalf("detail privacy: %d %s", rr.Code, rr.Body)
			}
			rr = securityRequest(h, token, "GET", fmt.Sprintf("/api/voyages/%d/opinions", voyage), "")
			documentAllowed := token != "approved"
			if rr.Code != 200 || strings.Contains(rr.Body.String(), `"file_format":"pdf"`) != documentAllowed {
				t.Fatalf("document list: %d %s", rr.Code, rr.Body)
			}
			rr = securityRequest(h, token, "GET", fmt.Sprintf("/api/voyages/%d/opinions/%d/download", voyage, opinion), "")
			expected := 403
			if documentAllowed {
				expected = 200
			}
			if rr.Code != expected {
				t.Fatalf("download=%d want=%d", rr.Code, expected)
			}
		})
	}
	for _, token := range []string{"pending", "unverified", "approved"} {
		rr := securityRequest(h, token, "GET", "/api/enroll/share", "")
		if rr.Code != 200 || strings.Contains(rr.Body.String(), "https://chat.example/private") != (token == "approved") {
			t.Fatalf("chat access %s: %d %s", token, rr.Code, rr.Body)
		}
	}
}

func TestAccountApprovalControls(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	var target, admin int64
	if err := conn.QueryRow("SELECT id FROM users WHERE firebase_uid='pending'").Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("SELECT id FROM users WHERE firebase_uid='admin'").Scan(&admin); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/members/%d/access", target)
	rr := securityRequest(h, "approved", "PUT", path, `{"approved":true,"suspended":false}`)
	if rr.Code != 403 {
		t.Fatalf("member approval=%d", rr.Code)
	}
	rr = securityRequest(h, "admin", "PUT", path, `{"approved":true,"suspended":false}`)
	if rr.Code != 204 {
		t.Fatalf("admin approval=%d %s", rr.Code, rr.Body)
	}
	rr = securityRequest(h, "pending", "GET", "/api/crew", "")
	if rr.Code != 200 {
		t.Fatalf("approved caller=%d", rr.Code)
	}
	rr = securityRequest(h, "admin", "PUT", path, `{"approved":true,"suspended":true}`)
	if rr.Code != 204 {
		t.Fatalf("suspension=%d %s", rr.Code, rr.Body)
	}

	rr = securityRequest(h, "admin", "PUT", fmt.Sprintf("/api/members/%d/role", target), `{"role":"admin"}`)
	if rr.Code != 409 {
		t.Fatalf("promoted suspended account: %d %s", rr.Code, rr.Body)
	}
	rr = securityRequest(h, "pending", "GET", "/api/auth/me", "")
	if rr.Code != 403 {
		t.Fatalf("retained token after suspension=%d", rr.Code)
	}
	rr = securityRequest(h, "admin", "PUT", fmt.Sprintf("/api/members/%d/access", admin), `{"approved":false,"suspended":true}`)
	if rr.Code != 409 {
		t.Fatalf("last admin restriction=%d %s", rr.Code, rr.Body)
	}
}

func TestApprovalMigrationPreservesAdminsOnly(t *testing.T) {
	conn := securityDatabase(t)
	if _, err := conn.Exec(`ALTER TABLE users DROP COLUMN access_approved, DROP COLUMN suspended;
 DELETE FROM schema_migrations WHERE version='030_account_access.sql';
 INSERT INTO users(email,name,role) VALUES ('admin@example.dev','Admin','admin'),('member@example.dev','Member','member');`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	var approved int
	if err := conn.QueryRow(`SELECT count(*) FROM users WHERE access_approved AND role='admin' AND NOT suspended`).Scan(&approved); err != nil {
		t.Fatal(err)
	}
	if approved != 1 {
		t.Fatalf("approved admins=%d", approved)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM users WHERE access_approved AND role='member'`).Scan(&approved); err != nil {
		t.Fatal(err)
	}
	if approved != 0 {
		t.Fatalf("approved members=%d", approved)
	}
}
