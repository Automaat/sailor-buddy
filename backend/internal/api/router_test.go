package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

// TestCORSPreflight verifies that NewRouter wires CORS from cfg.CORSAllowedOrigins:
// listed origins get Access-Control-Allow-Origin, unlisted ones do not.
func TestCORSPreflight(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		CORSAllowedOrigins: []string{"https://allowed.example.com", "http://localhost:5173"},
	}
	r := NewRouter(nil, cfg, nil)

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
