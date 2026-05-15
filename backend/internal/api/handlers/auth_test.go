package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestAuthHandler_Me(t *testing.T) {
	_, api := humatest.New(t)
	RegisterAuthRoutes(api)

	t.Run("authenticated", func(t *testing.T) {
		resp := api.GetCtx(userCtx(context.Background()), "/auth/me")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("got content-type %q, want application/json", ct)
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		resp := api.Get("/auth/me")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.Code)
		}
	})
}
