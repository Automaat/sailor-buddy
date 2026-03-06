package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

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
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
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
