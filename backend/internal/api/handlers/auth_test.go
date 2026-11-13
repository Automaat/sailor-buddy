package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestAuthHandler_Me(t *testing.T) {
	_, api := humatest.New(t)
	m := &mockQuerier{
		getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
			return sqlcdb.User{ID: id, Email: "test@example.com", Name: "Test User", Role: "admin"}, nil
		},
		getUserPersonFn: func(_ context.Context, userID types.NullInt64) (sqlcdb.CrewMember, error) {
			return sqlcdb.CrewMember{ID: 40, UserID: userID}, nil
		},
	}
	registerAuthRoutes(api, authHandlerFor(m))

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
		if me.PersonID == nil || *me.PersonID != 40 {
			t.Fatalf("person_id = %v, want 40", me.PersonID)
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
	tests := []struct {
		name      string
		body      map[string]any
		isCaptain bool
		want      int
		wantSaved *sqlcdb.UpdateUserProfileParams
	}{
		{
			name: "onboarding with patent",
			body: map[string]any{
				"first_name": " Jan ", "last_name": "Kowalski", "phone": "+48 600 100 200",
				"patent_type": "kapitan_jachtowy", "patent_number": " PL-123 ",
			},
			want: http.StatusOK,
			wantSaved: &sqlcdb.UpdateUserProfileParams{
				ID: 1, FirstName: "Jan", LastName: "Kowalski",
				Phone:        types.NullString{String: "+48 600 100 200", Valid: true},
				PatentType:   types.NullString{String: "kapitan_jachtowy", Valid: true},
				PatentNumber: types.NullString{String: "PL-123", Valid: true},
			},
		},
		{
			name: "number without patent type is dropped",
			body: map[string]any{"first_name": "Jan", "last_name": "Kowalski", "patent_number": "PL-1"},
			want: http.StatusOK,
			wantSaved: &sqlcdb.UpdateUserProfileParams{
				ID: 1, FirstName: "Jan", LastName: "Kowalski",
			},
		},
		{
			name: "patent type needs a number",
			body: map[string]any{"first_name": "Jan", "last_name": "Kowalski", "patent_type": "zeglarz_jachtowy", "patent_number": "  "},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "blank last name",
			body: map[string]any{"first_name": "Jan", "last_name": "   "},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "missing first name",
			body: map[string]any{"last_name": "Kowalski"},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "invalid phone",
			body: map[string]any{"first_name": "Jan", "last_name": "Kowalski", "phone": "call me"},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "unknown patent type",
			body: map[string]any{"first_name": "Jan", "last_name": "Kowalski", "patent_type": "admiral"},
			want: http.StatusUnprocessableEntity,
		},
		{
			name:      "captain cannot drop patent",
			body:      map[string]any{"first_name": "Jan", "last_name": "Kowalski"},
			isCaptain: true,
			want:      http.StatusUnprocessableEntity,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			var saved *sqlcdb.UpdateUserProfileParams
			m := &mockQuerier{
				getUserByIDFn: func(_ context.Context, id int64) (sqlcdb.User, error) {
					return sqlcdb.User{ID: id, IsCaptain: tt.isCaptain}, nil
				},
				lockUserForUpdateFn: func(_ context.Context, id int64) (int64, error) { return id, nil },
				getUserPersonFn: func(context.Context, types.NullInt64) (sqlcdb.CrewMember, error) {
					return sqlcdb.CrewMember{}, sql.ErrNoRows
				},
				updateUserProfileFn: func(_ context.Context, arg sqlcdb.UpdateUserProfileParams) (sqlcdb.User, error) {
					saved = &arg
					return sqlcdb.User{
						ID: arg.ID, FirstName: arg.FirstName, LastName: arg.LastName,
						Name:        arg.FirstName + " " + arg.LastName,
						OnboardedAt: types.NullTime{Valid: true},
					}, nil
				},
			}
			registerAuthRoutes(api, authHandlerFor(m))

			resp := api.PutCtx(userCtx(context.Background()), "/auth/me", tt.body)
			if resp.Code != tt.want {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, tt.want, resp.Body)
			}
			if tt.wantSaved == nil {
				if saved != nil {
					t.Fatalf("profile saved despite rejection: %+v", saved)
				}
				return
			}
			if saved == nil || *saved != *tt.wantSaved {
				t.Fatalf("saved = %+v, want %+v", saved, tt.wantSaved)
			}
			var me dto.Me
			if err := json.Unmarshal(resp.Body.Bytes(), &me); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !me.Onboarded || me.Name != "Jan Kowalski" {
				t.Fatalf("me = %+v, want onboarded Jan Kowalski", me)
			}
		})
	}
}

func TestAuthHandler_MyCruises(t *testing.T) {
	tests := []struct {
		name string
		ids  []int64
		want string
	}{
		{name: "participated", ids: []int64{3, 7}, want: "[3,7]"},
		{name: "none is an empty list", ids: nil, want: "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, api := humatest.New(t)
			var asked int64
			registerAuthRoutes(api, authHandlerFor(&mockQuerier{
				listUserCruiseIDsFn: func(_ context.Context, userID int64) ([]int64, error) {
					asked = userID
					return tt.ids, nil
				},
			}))
			resp := api.GetCtx(userCtx(context.Background()), "/auth/me/cruises")
			if resp.Code != http.StatusOK {
				t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
			}
			var body struct {
				CruiseIDs json.RawMessage `json:"cruise_ids"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if asked != 1 || string(body.CruiseIDs) != tt.want {
				t.Fatalf("asked user %d, cruise_ids %s; want user 1, %s", asked, body.CruiseIDs, tt.want)
			}
		})
	}

	t.Run("not authenticated", func(t *testing.T) {
		_, api := humatest.New(t)
		registerAuthRoutes(api, authHandlerFor(&mockQuerier{}))
		if resp := api.Get("/auth/me/cruises"); resp.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", resp.Code)
		}
	})
}

func authHandlerFor(m *mockQuerier) *AuthHandler {
	return &AuthHandler{q: m, runTx: directTxRunner(m)}
}
