package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type ctxKey string

const UserCtxKey ctxKey = "user"

// optionalAuthPath reports whether a request may proceed without
// authentication. Only the enrollment preview (GET /api/enroll/{token}, a
// single token segment) is public — it is shared with people who do not yet
// have an account, so a missing or invalid token resolves to an anonymous
// request instead of a 401. Matching the exact route shape keeps any future
// /api/enroll/* subroute authenticated by default.
func optionalAuthPath(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	token, ok := strings.CutPrefix(r.URL.Path, "/api/enroll/")
	return ok && token != "" && !strings.Contains(token, "/")
}

func Auth(fbClient *fbauth.Client, q sqlcdb.Querier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			optional := optionalAuthPath(r)

			header := r.Header.Get("Authorization")
			if header == "" {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			fbToken, err := fbClient.VerifyIDToken(r.Context(), token)
			if err != nil {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			email, _ := fbToken.Claims["email"].(string)
			if email == "" {
				http.Error(w, `{"error":"missing email claim"}`, http.StatusUnauthorized)
				return
			}

			name, _ := fbToken.Claims["name"].(string)

			fbUID := types.NullString{String: fbToken.UID, Valid: true}
			user, err := q.UpsertUserByFirebaseUID(r.Context(), sqlcdb.UpsertUserByFirebaseUIDParams{
				Email:       email,
				Name:        name,
				FirebaseUid: fbUID,
			})
			if err != nil {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
					slog.Error("upsert user", "email", email, "uid", fbToken.UID, "err", err)
					http.Error(w, `{"error":"failed to provision user"}`, http.StatusInternalServerError)
					return
				}
				emailVerified, _ := fbToken.Claims["email_verified"].(bool)
				if !emailVerified {
					slog.Warn("email not verified, refusing link by email", "email", email, "uid", fbToken.UID)
					http.Error(w, `{"error":"email not verified"}`, http.StatusUnauthorized)
					return
				}
				slog.Error("upsert user, trying link by email", "email", email, "uid", fbToken.UID, "err", err)
				user, err = q.LinkFirebaseUIDByEmail(r.Context(), sqlcdb.LinkFirebaseUIDByEmailParams{
					FirebaseUid: fbUID,
					NewName:     name,
					Email:       email,
				})
			}
			if err != nil {
				slog.Error("provision user", "email", email, "uid", fbToken.UID, "err", err)
				http.Error(w, `{"error":"failed to provision user"}`, http.StatusInternalServerError)
				return
			}

			// First-user-admin: the first account ever provisioned becomes the
			// club admin. Self-healing — only fires while no admin exists.
			role := user.Role
			if role != "admin" {
				if n, cerr := q.CountAdmins(r.Context()); cerr == nil && n == 0 {
					if uerr := q.UpdateUserRole(r.Context(), sqlcdb.UpdateUserRoleParams{Role: "admin", ID: user.ID}); uerr == nil {
						role = "admin"
					}
				}
			}

			claims := &auth.Claims{
				UserID:    user.ID,
				Email:     user.Email,
				Name:      user.Name,
				AvatarUrl: user.AvatarUrl.String,
				Role:      role,
			}
			ctx := context.WithValue(r.Context(), UserCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(UserCtxKey).(*auth.Claims)
	return claims
}
