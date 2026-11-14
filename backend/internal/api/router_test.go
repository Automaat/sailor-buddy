package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

func TestSecurityHeadersBeforeAuthentication(t *testing.T) {
	r := NewRouter(nil, &config.Config{}, nil, nil)
	for _, path := range []string{"/api/auth/me", "/api/uploads/1/images/file.png", "/api/voyages/1/opinions/1/download", "/api/does-not-exist"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, http.NoBody))
			if w.Code < 400 {
				t.Fatalf("status=%d; expected unauthenticated/error response", w.Code)
			}
			if w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
				t.Fatalf("headers=%v", w.Header())
			}
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("missing API framing restriction")
			}
		})
	}
}

// TestCORSPreflight verifies that NewRouter wires CORS from cfg.CORSAllowedOrigins:
// listed origins get Access-Control-Allow-Origin, unlisted ones do not.
func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		CORSAllowedOrigins: []string{"https://allowed.example.com", "http://localhost:5173"},
	}
	r := NewRouter(nil, cfg, nil, nil)

	tests := []struct {
		origin  string
		wantHdr bool
	}{
		{"https://allowed.example.com", true},
		{"http://localhost:5173", true},
		{"https://evil.example.com", false},
		{"https://notallowed.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/healthz", http.NoBody)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			got := w.Header().Get("Access-Control-Allow-Origin")
			if tc.wantHdr && got == "" {
				t.Errorf("origin %q: expected ACAO header, got none", tc.origin)
			}
			if !tc.wantHdr && got != "" {
				t.Errorf("origin %q: expected no ACAO header, got %q", tc.origin, got)
			}
		})
	}
}

// TestCruiseNestedRoutes verifies that /{cruiseID}/crew and /{cruiseID}/opinions
// are reachable and not captured by the /{id} subrouter.
func TestCruiseNestedRoutes(t *testing.T) {
	t.Parallel()

	var hit string

	r := chi.NewRouter()
	r.Route("/cruises", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			hit = "list"
			w.WriteHeader(http.StatusOK)
		})
		r.Route("/{cruiseID}/crew", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
				hit = "crew"
				w.WriteHeader(http.StatusOK)
			})
		})
		r.Route("/{cruiseID}/opinions", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
				hit = "opinions"
				w.WriteHeader(http.StatusOK)
			})
		})
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
				hit = "cruise"
				w.WriteHeader(http.StatusOK)
			})
		})
	})

	tests := []struct {
		path    string
		wantHit string
	}{
		{"/cruises/", "list"},
		{"/cruises/123/", "cruise"},
		{"/cruises/123/crew/", "crew"},
		{"/cruises/123/opinions/", "opinions"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			hit = ""
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if hit != tc.wantHit {
				t.Errorf("handler = %q, want %q", hit, tc.wantHit)
			}
		})
	}
}
