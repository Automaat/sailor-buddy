package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrivateFileRangeErrorsCannotClearCachePolicy(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "private.png", time.Now(), bytes.NewReader([]byte("image")))
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/uploads/1/images/private.png", http.NoBody)
	req.Header.Set("Range", "bytes=100-200")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusRequestedRangeNotSatisfiable || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
}

func TestPrivateResponseHeaders(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusPartialContent, http.StatusNotModified, http.StatusForbidden, http.StatusInternalServerError} {
		w := httptest.NewRecorder()
		handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Disposition", `attachment; filename="opinion.pdf"`)
			w.WriteHeader(status)
		}))
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/voyages/1/opinions/1/download", http.NoBody))
		if w.Code != status || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("Content-Disposition") == "" {
			t.Fatalf("status=%d headers=%v", w.Code, w.Header())
		}
	}
}
