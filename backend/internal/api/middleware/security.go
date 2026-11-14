package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders protects API responses, including authentication failures,
// enrollment previews, uploaded files, and downloads.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secured := &securityResponseWriter{ResponseWriter: w, private: r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/")}
		secured.protect()
		next.ServeHTTP(secured, r)
	})
}

type securityResponseWriter struct {
	http.ResponseWriter
	private bool
}

func (w *securityResponseWriter) protect() {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if w.private {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	}
}

func (w *securityResponseWriter) WriteHeader(status int) {
	// ServeContent clears cache headers on range errors before committing.
	w.protect()
	w.ResponseWriter.WriteHeader(status)
}

func (w *securityResponseWriter) Write(body []byte) (int, error) {
	w.protect()
	return w.ResponseWriter.Write(body)
}

func (w *securityResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
