package middleware

import (
	"net/http"
	"strings"
)

// Admission restricts club routes to approved accounts with verified email.
func Admission(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			next.ServeHTTP(w, r)
			return
		}
		if user.Suspended {
			denyAccess(w, "account_suspended", "account suspended")
			return
		}
		if selfService(r) || user.Approved && user.EmailVerified {
			next.ServeHTTP(w, r)
			return
		}
		if !user.EmailVerified {
			denyAccess(w, "email_unverified", "verify your email before accessing club data")
			return
		}
		denyAccess(w, "approval_pending", "administrator approval required")
	})
}

func selfService(r *http.Request) bool {
	if r.URL.Path == "/api/auth/me" {
		return r.Method == http.MethodGet || r.Method == http.MethodPut
	}
	token, ok := strings.CutPrefix(r.URL.Path, "/api/enroll/")
	if !ok {
		return false
	}
	parts := strings.Split(token, "/")
	if parts[0] == "" {
		return false
	}
	return len(parts) == 1 && (r.Method == http.MethodGet || r.Method == http.MethodPost) || len(parts) == 2 && parts[1] == "legs" && r.Method == http.MethodPost
}

func denyAccess(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"code":"` + code + `","detail":"` + message + `"}`))
}
