package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/logging"
)

func auditRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var record map[string]any
		if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
			return records
		} else if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
}

func TestAuditRedactsSecretsAcrossOutcomes(t *testing.T) {
	const secret = "private-enrollment-bearer-password"
	for _, status := range []int{200, 400, 401, 403, 422, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var output, fallback bytes.Buffer
			audit := NewRequestAudit(logging.New(&output), &fallback)
			audit.Register(&huma.OpenAPI{Paths: map[string]*huma.PathItem{"/enroll/{token}": {Post: &huma.Operation{OperationID: "enroll-by-token"}}}})
			h := audit.Middleware(AuditIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				AuditTarget(r.Context(), "user_id", 99)
				AuditTarget(r.Context(), "actor_id", 999)
				w.WriteHeader(status)
			})))
			r := httptest.NewRequest(http.MethodPost, "/api/enroll/"+secret+"?password="+secret, strings.NewReader(`{"password":"`+secret+`"}`))
			r.Header.Set("Authorization", "Bearer "+secret)
			r.Header.Set("X-Request-ID", secret)
			r = r.WithContext(context.WithValue(r.Context(), UserCtxKey, &auth.Claims{UserID: 7}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if strings.Contains(output.String(), secret) || strings.Contains(fallback.String(), secret) {
				t.Fatal("credentials in request/audit logs")
			}
			records := auditRecords(t, output.Bytes())
			if len(records) != 2 {
				t.Fatalf("records=%v", records)
			}
			if records[0]["event"] != "http_request" || records[1]["event"] != "security_audit" {
				t.Fatalf("records=%v", records)
			}
			for _, record := range records {
				if record["request_id"] != w.Header().Get("X-Request-ID") || record["actor_id"] != float64(7) || record["route"] != "/api/enroll/{token}" || record["action"] != "enroll-by-token" {
					t.Fatalf("record=%v", record)
				}
				if record["targets"].(map[string]any)["user_id"] != float64(99) {
					t.Fatalf("target=%v", record)
				}
			}
			want := "success"
			switch {
			case status >= 500:
				want = "failure"
			case status == 401 || status == 403:
				want = "denied"
			case status >= 300:
				want = "rejected"
			}
			if records[1]["result"] != want {
				t.Fatalf("record=%v", records[1])
			}
		})
	}
}

func TestAuditPanicValuesAndUnmatchedPathsStayPrivate(t *testing.T) {
	const secret = "private-token-in-unknown-url-and-panic"
	for _, path := range []string{"/api/enroll/" + secret + "/unknown", "/" + secret} {
		var output bytes.Buffer
		audit := NewRequestAudit(logging.New(&output), io.Discard)
		h := audit.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(secret) }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, path, http.NoBody))
		if w.Code != 500 || strings.Contains(output.String(), secret) || strings.Contains(w.Body.String(), secret) {
			t.Fatalf("status=%d logs=%s body=%s", w.Code, output.String(), w.Body)
		}
		if records := auditRecords(t, output.Bytes()); len(records) != 2 || records[1]["result"] != "failure" || records[1]["route"] != "unmatched" {
			t.Fatalf("records=%v", records)
		}
	}
}

type brokenAuditHandler struct{}

func (brokenAuditHandler) Enabled(context.Context, slog.Level) bool { return true }
func (brokenAuditHandler) Handle(context.Context, slog.Record) error {
	return errors.New("secret sink credentials")
}
func (h brokenAuditHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h brokenAuditHandler) WithGroup(string) slog.Handler      { return h }

func TestAuditWriteFailureKeepsHTTPResultAndUsesSafeFallback(t *testing.T) {
	var fallback bytes.Buffer
	audit := NewRequestAudit(slog.New(brokenAuditHandler{}), &fallback)
	w := httptest.NewRecorder()
	audit.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/members/99/role", http.NoBody))
	if w.Code != 204 || strings.Contains(fallback.String(), "secret") || !strings.Contains(fallback.String(), "security_log_write_failed") {
		t.Fatalf("status=%d fallback=%s", w.Code, fallback.String())
	}
}

func TestAuditAbortClosesConnectionWithoutInventingSuccess(t *testing.T) {
	var output bytes.Buffer
	audit := NewRequestAudit(logging.New(&output), io.Discard)
	done := make(chan struct{})
	handler := audit.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(done); handler.ServeHTTP(w, r) }))
	defer server.Close()
	client := server.Client()
	client.Timeout = time.Second
	response, err := client.Get(server.URL + "/api/private-abort-token")
	if response != nil {
		_ = response.Body.Close()
	}
	<-done
	if err == nil {
		t.Fatal("aborted request returned a response")
	}
	records := auditRecords(t, output.Bytes())
	if len(records) != 2 || records[1]["status"] != float64(0) || records[1]["result"] != "failure" || strings.Contains(output.String(), "private-abort-token") {
		t.Fatalf("records=%v", records)
	}
}
