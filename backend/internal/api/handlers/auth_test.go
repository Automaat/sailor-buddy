package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestAuthHandler_Me(t *testing.T) {
	_, api := humatest.New(t)
	m := &mockQuerier{
		getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
			return sqlcdb.User{ID: id, Email: "test@example.com", Name: "Test User", Role: "admin"}, nil
		},
	}
	RegisterAuthRoutes(api, m)

	t.Run("authenticated", func(t *testing.T) {
		resp := api.GetCtx(userCtx(context.Background()), "/auth/me")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if ct := resp.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("got content-type %q, want application/json", ct)
		}
		var me dto.Me
		if err := json.Unmarshal(resp.Body.Bytes(), &me); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if me.Role != "admin" {
			t.Fatalf("role = %q, want %q", me.Role, "admin")
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		resp := api.Get("/auth/me")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.Code)
		}
	})
}

func TestAuthHandler_UpdateMe(t *testing.T) {
	_, api := humatest.New(t)
	var saved sqlcdb.UpdateUserPatentParams
	m := &mockQuerier{
		updateUserPatentFn: func(_ context.Context, arg sqlcdb.UpdateUserPatentParams) error {
			saved = arg
			return nil
		},
		getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
			return sqlcdb.User{ID: id, Email: "test@example.com", Name: "Test User"}, nil
		},
	}
	RegisterAuthRoutes(api, m)

	t.Run("valid patent", func(t *testing.T) {
		resp := api.PutCtx(userCtx(context.Background()), "/auth/me", map[string]any{
			"patent_type":   "kapitan_jachtowy",
			"patent_number": "PL-123",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
		if saved.ID != 1 {
			t.Fatalf("saved.ID = %d, want 1", saved.ID)
		}
		if saved.PatentType.String != "kapitan_jachtowy" || saved.PatentNumber.String != "PL-123" {
			t.Fatalf("saved patent params = %+v", saved)
		}
	})

	t.Run("rejects unknown patent type", func(t *testing.T) {
		resp := api.PutCtx(userCtx(context.Background()), "/auth/me", map[string]any{
			"patent_type": "admiral",
		})
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422; body=%s", resp.Code, resp.Body)
		}
	})
}
