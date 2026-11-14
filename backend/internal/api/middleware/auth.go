package middleware

import (
	"context"
	"database/sql"
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

// TokenVerifier verifies a Firebase ID token; *fbauth.Client implements it.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*fbauth.Token, error)
}

// Auth verifies the Firebase ID token, provisions the account on first
// sign-in and puts its claims on the request context. db runs the
// first-admin promotion in a transaction of its own.
func Auth(fbClient TokenVerifier, q sqlcdb.Querier, db *sql.DB) func(http.Handler) http.Handler {
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

			// Every account has exactly one person (crew record); the first
			// request of a new account creates it.
			if err := q.EnsureUserPerson(r.Context(), user.ID); err != nil {
				slog.Error("ensure user person", "user_id", user.ID, "err", err)
				http.Error(w, `{"error":"failed to provision user"}`, http.StatusInternalServerError)
				return
			}

			claims := &auth.Claims{
				UserID:    user.ID,
				Email:     user.Email,
				Name:      user.Name,
				AvatarUrl: user.AvatarUrl.String,
				Role:      firstAdminRole(r.Context(), q, db, user),
			}
			ctx := context.WithValue(r.Context(), UserCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// firstAdminRole returns the user's role after first-user-admin: the first
// account ever provisioned becomes the club admin. Self-healing — it only
// promotes while no admin exists. A failed promotion keeps the stored role.
func firstAdminRole(ctx context.Context, q sqlcdb.Querier, db *sql.DB, user sqlcdb.User) string {
	if user.Role == "admin" {
		return user.Role
	}
	if n, err := q.CountAdmins(ctx); err != nil || n != 0 {
		return user.Role
	}
	promoted, err := promoteFirstAdmin(ctx, db, user.ID)
	if err != nil {
		slog.Error("promote first admin", "user_id", user.ID, "err", err)
		return user.Role
	}
	if promoted {
		return "admin"
	}
	return user.Role
}

// promoteFirstAdmin makes userID the club admin when no admin exists and
// reports whether it did. The unlocked admin count in firstAdminRole is a
// fast path only: two first sign-ins can both see zero, so this re-checks
// under a transaction-scoped advisory lock and only one of them wins.
func promoteFirstAdmin(ctx context.Context, db *sql.DB, userID int64) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := sqlcdb.New(tx)
	if err := q.LockFirstAdmin(ctx); err != nil {
		return false, err
	}
	n, err := q.PromoteIfNoAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

func GetUser(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(UserCtxKey).(*auth.Claims)
	return claims
}
