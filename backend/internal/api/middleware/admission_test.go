package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
)

func TestAdmission(t *testing.T) {
	verified := &auth.Claims{UserID: 1, Role: "member", EmailVerified: true}
	unverified := &auth.Claims{UserID: 2, Role: "member"}
	suspended := &auth.Claims{UserID: 3, Role: "member", EmailVerified: true, Suspended: true}
	unverifiedSuspended := &auth.Claims{UserID: 4, Role: "member", Suspended: true}
	tests := []struct {
		name     string
		claims   *auth.Claims
		method   string
		path     string
		wantCode string
	}{
		{"anonymous passes to the route's own auth", nil, http.MethodGet, "/api/crew", ""},
		{"verified reads club data", verified, http.MethodGet, "/api/crew", ""},
		{"verified admin reads club data", &auth.Claims{UserID: 5, Role: "admin", EmailVerified: true}, http.MethodGet, "/api/crew", ""},
		{"unverified club read", unverified, http.MethodGet, "/api/crew", "email_unverified"},
		{"unverified club write", unverified, http.MethodPost, "/api/crew", "email_unverified"},
		{"unverified admin club read", &auth.Claims{UserID: 6, Role: "admin"}, http.MethodGet, "/api/crew", "email_unverified"},
		{"unverified own profile", unverified, http.MethodGet, "/api/auth/me", ""},
		{"unverified onboarding", unverified, http.MethodPut, "/api/auth/me", ""},
		{"unverified token preview", unverified, http.MethodGet, "/api/enroll/abc", ""},
		{"unverified token enrollment", unverified, http.MethodPost, "/api/enroll/abc", ""},
		{"unverified token leg enrollment", unverified, http.MethodPost, "/api/enroll/abc/legs", ""},
		{"unverified person candidates", unverified, http.MethodGet, "/api/auth/me/person-candidates", "email_unverified"},
		{"suspended club read", suspended, http.MethodGet, "/api/crew", "account_suspended"},
		{"suspended own profile", suspended, http.MethodGet, "/api/auth/me", "account_suspended"},
		{"suspended token preview", suspended, http.MethodGet, "/api/enroll/abc", "account_suspended"},
		{"suspended and unverified", unverifiedSuspended, http.MethodGet, "/api/crew", "account_suspended"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached := false
			h := Admission(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			if tt.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), UserCtxKey, tt.claims))
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if tt.wantCode == "" {
				if !reached || rr.Code != http.StatusNoContent {
					t.Fatalf("status=%d reached=%v body=%s, want the route to run", rr.Code, reached, rr.Body)
				}
				return
			}
			if reached || rr.Code != http.StatusForbidden {
				t.Fatalf("status=%d reached=%v, want 403 before the route", rr.Code, reached)
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", rr.Body, err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code=%q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}
