package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
)

func TestAuthHandler_Me(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		h := NewAuthHandler()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		ctx := context.WithValue(req.Context(), middleware.UserCtxKey, &auth.Claims{
			UserID: 42, Email: "test@example.com", Name: "Test", AvatarUrl: "https://example.com/avatar.png",
		})
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		h.Me(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("got content-type %q, want application/json", ct)
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		h := NewAuthHandler()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.Me(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}
