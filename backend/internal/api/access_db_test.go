package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
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
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type securityVerifier map[string]*fbauth.Token

func (v securityVerifier) VerifyIDTokenAndCheckRevoked(_ context.Context, token string) (*fbauth.Token, error) {
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
	for _, token := range []string{"member", "unverified", "suspended", "captain", "subject", "admin"} {
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
		registerHumaRoutes(humachi.New(r, humaConfig()), q, conn, uploads, nil)
	})
	for token := range verifier {
		resp := securityRequest(r, token, http.MethodGet, "/api/auth/me", "")
		if resp.Code != 200 {
			t.Fatalf("provision %s: %d %s", token, resp.Code, resp.Body)
		}
	}
	if _, err := conn.Exec(`UPDATE users SET suspended = firebase_uid = 'suspended'`); err != nil {
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
		{"unverified club read", "unverified", http.MethodGet, "/api/crew", "", 403},
		{"suspended club read", "suspended", http.MethodGet, "/api/crew", "", 403},
		{"suspended profile", "suspended", http.MethodGet, "/api/auth/me", "", 403},
		{"unverified profile", "unverified", http.MethodGet, "/api/auth/me", "", 200},
		{"unverified onboarding", "unverified", http.MethodPut, "/api/auth/me", `{"first_name":"New","last_name":"Member"}`, 200},
		{"unverified token preview", "unverified", http.MethodGet, "/api/enroll/missing", "", 404},
		{"unverified enroll", "unverified", http.MethodPost, "/api/enroll/missing", "{}", 404},
		{"unverified enroll legs", "unverified", http.MethodPost, "/api/enroll/missing/legs", `{"trip_ids":[1]}`, 404},
		{"unverified trainings", "unverified", http.MethodGet, "/api/trainings", "", 403},
		{"unverified candidates", "unverified", http.MethodGet, "/api/auth/me/person-candidates", "", 403},
		{"new verified account directory", "member", http.MethodGet, "/api/crew", "", 200},
		{"new verified account trainings", "member", http.MethodGet, "/api/trainings", "", 200},
		{"new verified account candidates", "member", http.MethodGet, "/api/auth/me/person-candidates", "", 200},
		{"member cannot mutate", "member", http.MethodPost, "/api/crew", `{"full_name":"Injected"}`, 403},
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

// TestAdmissionDenialCodes pins the reason the UI shows for each denied
// caller: unverified email or suspension. No approval step exists.
func TestAdmissionDenialCodes(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	for token, want := range map[string]string{"unverified": "email_unverified", "suspended": "account_suspended"} {
		rr := securityRequest(h, token, http.MethodGet, "/api/crew", "")
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: decode %s: %v", token, rr.Body, err)
		}
		if rr.Code != 403 || body.Code != want {
			t.Errorf("%s: status=%d code=%q, want 403 %q", token, rr.Code, body.Code, want)
		}
	}
	var me dto.Me
	rr := securityRequest(h, "member", http.MethodGet, "/api/auth/me", "")
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if !me.EmailVerified || me.Suspended || strings.Contains(rr.Body.String(), `"approved"`) {
		t.Errorf("profile of a new verified account: %s", rr.Body)
	}
}

func TestSensitiveDataAccess(t *testing.T) {
	conn := securityDatabase(t)
	h, uploads := securityRouter(t, conn)
	var captain, subject, outsider, cruise, voyage, opinion int64
	for uid, dest := range map[string]*int64{"captain": &captain, "subject": &subject, "member": &outsider} {
		if err := conn.QueryRow(`SELECT cm.id FROM crew_members cm JOIN users u ON u.id=cm.user_id WHERE u.firebase_uid=$1`, uid).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(`UPDATE crew_members SET emergency_contact_name='PRIVATE-NAME',emergency_contact_phone='PRIVATE-PHONE' WHERE id=$1`, subject); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`INSERT INTO cruises(name,enroll_token,published_at) VALUES('Event','share',now()) RETURNING id`).Scan(&cruise); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`INSERT INTO voyages(name,cruise_id,captain_person_id,published_at) VALUES('Voyage',$1,$2,now()) RETURNING id`, cruise, captain).Scan(&voyage); err != nil {
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
	for _, token := range []string{"member", "subject", "captain", "admin"} {
		t.Run(token, func(t *testing.T) {
			rr := securityRequest(h, token, "GET", "/api/crew", "")
			contains := strings.Contains(rr.Body.String(), "PRIVATE-PHONE")
			want := token == "captain" || token == "admin"
			if rr.Code != 200 || contains != want {
				t.Fatalf("directory status=%d exposed=%v want=%v", rr.Code, contains, want)
			}

			cursor := ""
			seen := false
			for range 20 {
				rr = securityRequest(h, token, "GET", "/api/crew?limit=1&after="+url.QueryEscape(cursor), "")
				var page dto.Page[dto.CrewMember]
				if rr.Code != 200 {
					t.Fatalf("page status=%d %s", rr.Code, rr.Body)
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &page); err != nil {
					t.Fatal(err)
				}
				for _, person := range page.Items {
					if person.ID == subject {
						seen = true
						if (person.EmergencyContactPhone != nil) != want {
							t.Fatalf("paginated contact exposed=%v want=%v", person.EmergencyContactPhone != nil, want)
						}
					}
				}
				if !page.HasMore {
					break
				}
				if page.NextCursor == "" {
					t.Fatal("missing next cursor")
				}
				cursor = page.NextCursor
			}
			if !seen {
				t.Fatal("subject missing from paginated directory")
			}
			path := fmt.Sprintf("/api/crew/%d", subject)
			rr = securityRequest(h, token, "GET", path, "")
			if rr.Code != 200 || strings.Contains(rr.Body.String(), "PRIVATE-PHONE") != want {
				t.Fatalf("detail privacy: %d %s", rr.Code, rr.Body)
			}
			rr = securityRequest(h, token, "GET", fmt.Sprintf("/api/voyages/%d/opinions", voyage), "")
			documentAllowed := token != "member"
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
	for _, token := range []string{"unverified", "member"} {
		rr := securityRequest(h, token, "GET", "/api/enroll/share", "")
		if rr.Code != 200 {
			t.Fatalf("enroll preview %s: %d %s", token, rr.Code, rr.Body)
		}
	}
}

// TestAccountSuspensionControls covers the admin-only suspend/restore
// control. A suspension denies the account's existing token at once, and an
// admin must be demoted before suspension.
func TestAccountSuspensionControls(t *testing.T) {
	conn := securityDatabase(t)
	h, _ := securityRouter(t, conn)
	var target, admin int64
	if err := conn.QueryRow("SELECT id FROM users WHERE firebase_uid='subject'").Scan(&target); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("SELECT id FROM users WHERE firebase_uid='admin'").Scan(&admin); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/members/%d/access", target)
	rr := securityRequest(h, "member", "PUT", path, `{"suspended":true}`)
	if rr.Code != 403 {
		t.Fatalf("member suspension=%d", rr.Code)
	}
	rr = securityRequest(h, "subject", "GET", "/api/crew", "")
	if rr.Code != 200 {
		t.Fatalf("verified caller before suspension=%d", rr.Code)
	}
	rr = securityRequest(h, "admin", "PUT", path, `{"suspended":true}`)
	if rr.Code != 204 {
		t.Fatalf("suspension=%d %s", rr.Code, rr.Body)
	}
	for _, route := range []string{"/api/crew", "/api/auth/me"} {
		rr = securityRequest(h, "subject", "GET", route, "")
		if rr.Code != 403 {
			t.Fatalf("retained token after suspension %s=%d", route, rr.Code)
		}
	}
	rr = securityRequest(h, "admin", "PUT", fmt.Sprintf("/api/members/%d/role", target), `{"role":"admin"}`)
	if rr.Code != 409 {
		t.Fatalf("promoted suspended account: %d %s", rr.Code, rr.Body)
	}
	rr = securityRequest(h, "admin", "PUT", path, `{"suspended":false}`)
	if rr.Code != 204 {
		t.Fatalf("restore=%d %s", rr.Code, rr.Body)
	}
	rr = securityRequest(h, "subject", "GET", "/api/crew", "")
	if rr.Code != 200 {
		t.Fatalf("restored caller=%d", rr.Code)
	}
	rr = securityRequest(h, "admin", "PUT", fmt.Sprintf("/api/members/%d/access", admin), `{"suspended":true}`)
	if rr.Code != 409 {
		t.Fatalf("last admin suspension=%d %s", rr.Code, rr.Body)
	}
	rr = securityRequest(h, "admin", "PUT", "/api/members/999999/access", `{"suspended":true}`)
	if rr.Code != 404 {
		t.Fatalf("unknown account=%d %s", rr.Code, rr.Body)
	}
}
