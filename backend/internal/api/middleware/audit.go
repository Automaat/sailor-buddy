package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type auditContextKey struct{}

type auditMetadata struct {
	actor   int64
	targets map[string]int64
}

type auditRoute struct {
	method, path, action string
	parts                []string
}

// RequestAudit records safe HTTP outcomes and sensitive action outcomes.
type RequestAudit struct {
	logger   *slog.Logger
	fallback io.Writer
	routes   []auditRoute
}

// NewRequestAudit uses fallback only for a fixed, credential-free write-failure notice.
func NewRequestAudit(logger *slog.Logger, fallback io.Writer) *RequestAudit {
	return &RequestAudit{logger: logger, fallback: fallback}
}

// Register freezes route identities from the API contract before serving requests.
func (a *RequestAudit) Register(spec *huma.OpenAPI) {
	for path, item := range spec.Paths {
		for method, op := range map[string]*huma.Operation{
			http.MethodGet: item.Get, http.MethodPost: item.Post, http.MethodPut: item.Put, http.MethodDelete: item.Delete,
			http.MethodPatch: item.Patch, http.MethodHead: item.Head, http.MethodOptions: item.Options, http.MethodTrace: item.Trace,
		} {
			if op != nil {
				a.routes = append(a.routes, auditRoute{method: method, path: "/api" + path, action: op.OperationID, parts: strings.Split("/api"+path, "/")})
			}
		}
	}
	slices.SortFunc(a.routes, func(x, y auditRoute) int { return strings.Compare(x.path, y.path) })
}

// AuditIdentity records the authenticated database account before admission checks.
func AuditIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if meta, ok := r.Context().Value(auditContextKey{}).(*auditMetadata); ok {
			if user := GetUser(r.Context()); user != nil {
				meta.actor = user.UserID
			}
		}
		next.ServeHTTP(w, r)
	})
}

// AuditTarget adds only a numeric account/person target, never caller-supplied text.
func AuditTarget(ctx context.Context, key string, id int64) {
	if id <= 0 || !slices.Contains([]string{"user_id", "person_id", "duplicate_id", "record_id"}, key) {
		return
	}
	if meta, ok := ctx.Value(auditContextKey{}).(*auditMetadata); ok {
		meta.targets[key] = id
	}
}

func (a *RequestAudit) identify(method, path string) (matched auditRoute, ids map[string]int64) {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	for _, route := range a.routes {
		if method != route.method || len(parts) != len(route.parts) {
			continue
		}
		targets := map[string]int64{}
		matches := true
		for i, part := range route.parts {
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
				if !strings.Contains(strings.ToLower(name), "token") && (name == "id" || strings.HasSuffix(name, "ID")) {
					if id, err := strconv.ParseInt(parts[i], 10, 64); err == nil && id > 0 {
						key := strings.TrimSuffix(name, "ID") + "_id"
						if name == "id" {
							key = "record_id"
						}
						targets[key] = id
					}
				}
			} else if part != parts[i] {
				matches = false
				break
			}
		}
		if matches {
			return route, targets
		}
	}
	if strings.HasPrefix(path, "/api/uploads/") {
		return auditRoute{path: "/api/uploads/*", action: "read-upload"}, map[string]int64{}
	}
	if path == "/healthz" || path == "/readyz" {
		return auditRoute{path: path, action: "health-check"}, map[string]int64{}
	}
	return auditRoute{path: "unmatched", action: "unmatched"}, map[string]int64{}
}

// Middleware omits raw URLs, headers, bodies, panic values, and client request IDs.
func (a *RequestAudit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := uuid.NewString()
		method := r.Method
		if !slices.Contains([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodTrace}, method) {
			method = "OTHER"
		}
		route, targets := a.identify(method, r.URL.Path)
		meta := &auditMetadata{targets: targets}
		r = r.WithContext(context.WithValue(r.Context(), auditContextKey{}, meta))
		w.Header().Set("X-Request-ID", requestID)
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			panicked := recover()
			aborted := recoverAuditResponse(panicked, wrapped)
			status := wrapped.Status()
			if status == 0 && !aborted {
				status = http.StatusOK
			}
			result := "success"
			switch {
			case panicked != nil || status >= 500:
				result = "failure"
			case status == 401 || status == 403:
				result = "denied"
			case status >= 300:
				result = "rejected"
			}
			if meta.actor > 0 && strings.HasPrefix(route.path, "/api/auth/me") {
				meta.targets["user_id"] = meta.actor
			}
			attrs := []slog.Attr{slog.String("request_id", requestID), slog.Int64("actor_id", meta.actor), slog.String("method", method), slog.String("route", route.path), slog.String("action", route.action), slog.Any("targets", meta.targets), slog.String("result", result), slog.Int("status", status)}
			a.emit(r.Context(), "http_request", append(attrs, slog.Int64("duration_ms", time.Since(started).Milliseconds()))...)
			if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete || status >= 400 || panicked != nil {
				a.emit(r.Context(), "security_audit", attrs...)
			}
			if aborted {
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

func recoverAuditResponse(value any, writer chimiddleware.WrapResponseWriter) bool {
	if err, ok := value.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		return true
	}
	if value != nil && writer.Status() == 0 {
		http.Error(writer, `{"detail":"internal server error"}`, http.StatusInternalServerError)
		return false
	}
	return false
}

func (a *RequestAudit) emit(ctx context.Context, event string, attrs ...slog.Attr) {
	record := slog.NewRecord(time.Now(), slog.LevelInfo, event, 0)
	record.AddAttrs(slog.String("event", event))
	record.AddAttrs(attrs...)
	if err := a.logger.Handler().Handle(ctx, record); err != nil {
		_, _ = fmt.Fprintln(a.fallback, `{"event":"security_log_write_failed"}`)
	}
}
