package middleware

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgContext struct {
	OrgID int64
	Slug  string
	Role  string
}

const OrgCtxKey ctxKey = "org"

func OrgFromSlug(q sqlcdb.Querier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "slug")
			if slug == "" {
				http.Error(w, `{"error":"missing org slug"}`, http.StatusBadRequest)
				return
			}

			org, err := q.GetOrganizationBySlug(r.Context(), slug)
			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, `{"error":"organization not found"}`, http.StatusNotFound)
					return
				}
				http.Error(w, `{"error":"failed to get organization"}`, http.StatusInternalServerError)
				return
			}

			user := GetUser(r.Context())
			membership, err := q.GetOrgMembership(r.Context(), sqlcdb.GetOrgMembershipParams{
				OrgID:  org.ID,
				UserID: user.UserID,
			})
			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, `{"error":"not a member of this organization"}`, http.StatusForbidden)
					return
				}
				http.Error(w, `{"error":"failed to check membership"}`, http.StatusInternalServerError)
				return
			}

			octx := &OrgContext{
				OrgID: org.ID,
				Slug:  slug,
				Role:  membership.Role,
			}
			ctx := context.WithValue(r.Context(), OrgCtxKey, octx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetOrg(ctx context.Context) *OrgContext {
	octx, _ := ctx.Value(OrgCtxKey).(*OrgContext)
	return octx
}

func RequireOrgRole(roles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			octx := GetOrg(r.Context())
			if octx == nil || !roleSet[octx.Role] {
				http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
