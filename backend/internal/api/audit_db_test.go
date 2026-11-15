package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/logging"
)

func TestAuditRealRoleChangesAndEarlyDenials(t *testing.T) {
	conn := securityDatabase(t)
	var output bytes.Buffer
	verifier := securityVerifier{}
	for _, name := range []string{"actor", "target", "member", "pending"} {
		verifier[name] = &fbauth.Token{UID: name, Claims: map[string]any{"email": name + "@example.dev", "email_verified": true}}
	}
	h := newRouter(conn, &config.Config{}, verifier, nil, logging.New(&output))
	for name := range verifier {
		if response := securityRequest(h, name, http.MethodGet, "/api/auth/me", ""); response.Code != 200 {
			t.Fatalf("provision %s: %d %s", name, response.Code, response.Body)
		}
	}
	if err := auth.BootstrapAdmin(t.Context(), conn, "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`UPDATE users SET access_approved = true WHERE firebase_uid IN ('target','member')`); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for name := range verifier {
		var id int64
		if err := conn.QueryRow(`SELECT id FROM users WHERE firebase_uid=$1`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	for _, tc := range []struct {
		token, role, result string
		status              int
	}{
		{"actor", "admin", "success", 204},
		{"member", "member", "denied", 403},
		{"pending", "member", "denied", 403},
		{"actor", "private-validation-secret", "rejected", 422},
		{"", "member", "denied", 401},
	} {
		t.Run(tc.token+tc.result, func(t *testing.T) {
			output.Reset()
			path := fmt.Sprintf("/api/members/%d/role", ids["target"])
			response := securityRequest(h, tc.token, http.MethodPut, path, fmt.Sprintf(`{"role":%q}`, tc.role))
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if strings.Contains(output.String(), "private-validation-secret") || strings.Contains(output.String(), "@example.dev") {
				t.Fatalf("personal data in log: %s", output.String())
			}
			decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
			for range 2 {
				var record struct {
					Event     string           `json:"event"`
					Actor     int64            `json:"actor_id"`
					Action    string           `json:"action"`
					Result    string           `json:"result"`
					RequestID string           `json:"request_id"`
					Targets   map[string]int64 `json:"targets"`
				}
				if err := decoder.Decode(&record); err != nil {
					t.Fatalf("record: %v logs=%s", err, output.String())
				}
				if record.Actor != ids[tc.token] || record.Targets["user_id"] != ids["target"] || record.Action != "update-member-role" || record.Result != tc.result || record.RequestID != response.Header().Get("X-Request-ID") {
					t.Fatalf("record=%+v", record)
				}
			}
		})
	}
	var role string
	if err := conn.QueryRow(`SELECT role FROM users WHERE id=$1`, ids["target"]).Scan(&role); err != nil || role != "admin" {
		t.Fatalf("target role=%s err=%v", role, err)
	}
	create := func(path, body string) int64 {
		t.Helper()
		response := securityRequest(h, "actor", http.MethodPost, path, body)
		if response.Code != 201 {
			t.Fatalf("create %s: %d %s", path, response.Code, response.Body)
		}
		var entity struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &entity); err != nil {
			t.Fatal(err)
		}
		return entity.ID
	}
	check := func(method, path, body, action string, want int, targets map[string]int64) *httptest.ResponseRecorder {
		t.Helper()
		output.Reset()
		response := securityRequest(h, "actor", method, path, body)
		if response.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body)
		}
		decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
		for range 2 {
			var record struct {
				Actor   int64            `json:"actor_id"`
				Action  string           `json:"action"`
				Result  string           `json:"result"`
				Targets map[string]int64 `json:"targets"`
			}
			if err := decoder.Decode(&record); err != nil {
				t.Fatal(err)
			}
			if record.Actor != ids["actor"] || record.Action != action || record.Result != "success" {
				t.Fatalf("record=%+v", record)
			}
			for key, value := range targets {
				if record.Targets[key] != value {
					t.Fatalf("target %s=%d, want %d; record=%+v", key, record.Targets[key], value, record)
				}
			}
		}
		return response
	}
	seriesID := create("/api/series", `{"name":"Audit Series"}`)
	cruiseID := create("/api/cruises", fmt.Sprintf(`{"name":"Audit Cruise","series_id":%d,"edition":"2026"}`, seriesID))
	tripID := create("/api/trips", fmt.Sprintf(`{"name":"Audit Trip","cruise_id":%d}`, cruiseID))
	if response := securityRequest(h, "actor", http.MethodPost, fmt.Sprintf("/api/trips/%d/enroll-token", tripID), ""); response.Code != 200 {
		t.Fatalf("enable sign-up: %d %s", response.Code, response.Body)
	}
	if response := securityRequest(h, "actor", http.MethodPost, fmt.Sprintf("/api/cruises/%d/publish", cruiseID), `{"include_legs":true}`); response.Code != 200 {
		t.Fatalf("publish edition: %d %s", response.Code, response.Body)
	}
	signup := securityRequest(h, "target", http.MethodPost, fmt.Sprintf("/api/trips/%d/sign-up", tripID), `{}`)
	if signup.Code != 201 {
		t.Fatalf("sign-up: %d %s", signup.Code, signup.Body)
	}
	var enrollmentID int64
	if err := conn.QueryRow(`SELECT te.id FROM trip_enrollments te JOIN crew_members cm ON cm.id = te.crew_member_id
		WHERE te.trip_id=$1 AND cm.user_id=$2`, tripID, ids["target"]).Scan(&enrollmentID); err != nil {
		t.Fatal(err)
	}
	check(http.MethodPut, fmt.Sprintf("/api/trips/%d/enrollments/%d/payment-status", tripID, enrollmentID), `{"payment_status":"paid"}`, "update-trip-enrollment-payment-status", 200, map[string]int64{"trip_id": tripID, "record_id": enrollmentID})
	var personID int64
	if err := conn.QueryRow(`INSERT INTO crew_members(full_name) VALUES('Private Name') RETURNING id`).Scan(&personID); err != nil {
		t.Fatal(err)
	}
	check(http.MethodPost, fmt.Sprintf("/api/crew/%d/link", personID), fmt.Sprintf(`{"user_id":%d}`, ids["target"]), "link-crew-member", 200, map[string]int64{"user_id": ids["target"], "person_id": personID})
	var linkedID int64
	if err := conn.QueryRow(`SELECT user_id FROM crew_members WHERE id=$1`, personID).Scan(&linkedID); err != nil || linkedID != ids["target"] {
		t.Fatalf("linked account=%d err=%v", linkedID, err)
	}
	check(http.MethodDelete, fmt.Sprintf("/api/trips/%d/enrollments/%d", tripID, enrollmentID), "", "delete-trip-enrollment", 204, map[string]int64{"trip_id": tripID, "record_id": enrollmentID})
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM trip_enrollments WHERE id=$1`, enrollmentID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("enrollment count=%d err=%v", count, err)
	}
}
