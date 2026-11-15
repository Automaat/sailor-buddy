package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func captainGroupCtx() context.Context {
	return context.WithValue(context.Background(), middleware.UserCtxKey, &auth.Claims{
		Approved: true, EmailVerified: true, UserID: 7, Role: "member", IsCaptain: true,
	})
}

// TestSuggestPorts_Visibility checks that only admins and the captain group
// get suggestions from hidden voyages; members and an unidentified caller
// get visible ones only.
func TestSuggestPorts_Visibility(t *testing.T) {
	tests := []struct {
		name          string
		ctx           context.Context
		includeHidden bool
	}{
		{"admin", userCtxRole(context.Background(), "admin"), true},
		{"captain group", captainGroupCtx(), true},
		{"member", userCtxRole(context.Background(), "member"), false},
		{"no caller", context.Background(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sqlcdb.SuggestPortsParams
			m := &mockQuerier{suggestPortsFn: func(_ context.Context, arg sqlcdb.SuggestPortsParams) ([]sqlcdb.SuggestPortsRow, error) {
				got = arg
				return nil, nil
			}}
			resp := voyagePortTestAPI(t, m).GetCtx(tt.ctx, "/ports/suggest?q=%20Hel%20")
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
			}
			want := sqlcdb.SuggestPortsParams{Query: "Hel", IncludeHidden: tt.includeHidden, RowLimit: 8}
			if got != want {
				t.Fatalf("params = %+v, want %+v", got, want)
			}
			if body := resp.Body.String(); body != "[]\n" && body != "[]" {
				t.Fatalf("body = %q, want []", body)
			}
		})
	}
}

func TestSuggestPorts_MapsRows(t *testing.T) {
	m := &mockQuerier{suggestPortsFn: func(context.Context, sqlcdb.SuggestPortsParams) ([]sqlcdb.SuggestPortsRow, error) {
		return []sqlcdb.SuggestPortsRow{
			{Name: "Hel", Latitude: types.NullFloat64{Float64: 54.6, Valid: true}, Longitude: types.NullFloat64{Float64: 18.8, Valid: true}, Uses: 3},
			{Name: "Zatoka Pucka", Uses: 1},
		}, nil
	}}
	resp := voyagePortTestAPI(t, m).GetCtx(userCtxRole(context.Background(), "member"), "/ports/suggest?q=he")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var got []dto.PortSuggestion
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got[0].Name != "Hel" || got[0].Latitude == nil || *got[0].Latitude != 54.6 || got[0].Uses != 3 {
		t.Fatalf("first suggestion = %+v", got)
	}
	if got[1].Latitude != nil || got[1].Longitude != nil || got[1].Uses != 1 {
		t.Fatalf("name-only suggestion = %+v", got[1])
	}
}

// TestSuggestPorts_Validation checks the query and limit rules: a missing,
// bare, blank or one-character query and a limit outside 1..20 answer 422
// without a database call.
func TestSuggestPorts_Validation(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantCode  int
		wantLimit int32
	}{
		{"missing q", "/ports/suggest", http.StatusUnprocessableEntity, 0},
		{"bare q", "/ports/suggest?q", http.StatusUnprocessableEntity, 0},
		{"empty q", "/ports/suggest?q=", http.StatusUnprocessableEntity, 0},
		{"blank q", "/ports/suggest?q=%20%20%20", http.StatusUnprocessableEntity, 0},
		{"one character", "/ports/suggest?q=%20h%20", http.StatusUnprocessableEntity, 0},
		{"too long", "/ports/suggest?q=" + strings.Repeat("a", 101), http.StatusUnprocessableEntity, 0},
		{"limit zero", "/ports/suggest?q=he&limit=0", http.StatusUnprocessableEntity, 0},
		{"limit above max", "/ports/suggest?q=he&limit=21", http.StatusUnprocessableEntity, 0},
		{"default limit", "/ports/suggest?q=he", http.StatusOK, 8},
		{"explicit limit", "/ports/suggest?q=he&limit=3", http.StatusOK, 3},
		{"max limit", "/ports/suggest?q=he&limit=20", http.StatusOK, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var limit int32
			m := &mockQuerier{suggestPortsFn: func(_ context.Context, arg sqlcdb.SuggestPortsParams) ([]sqlcdb.SuggestPortsRow, error) {
				calls++
				limit = arg.RowLimit
				return nil, nil
			}}
			resp := voyagePortTestAPI(t, m).GetCtx(userCtxRole(context.Background(), "member"), tt.path)
			if resp.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, tt.wantCode, resp.Body)
			}
			if tt.wantCode != http.StatusOK {
				if calls != 0 {
					t.Fatalf("database called %d times for a rejected request", calls)
				}
				return
			}
			if calls != 1 || limit != tt.wantLimit {
				t.Fatalf("calls = %d, limit = %d, want 1 call with limit %d", calls, limit, tt.wantLimit)
			}
		})
	}
}

func TestSuggestPorts_DatabaseError(t *testing.T) {
	m := &mockQuerier{suggestPortsFn: func(context.Context, sqlcdb.SuggestPortsParams) ([]sqlcdb.SuggestPortsRow, error) {
		return nil, errors.New("boom")
	}}
	resp := voyagePortTestAPI(t, m).GetCtx(userCtxRole(context.Background(), "member"), "/ports/suggest?q=he")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", resp.Code, resp.Body)
	}
}
