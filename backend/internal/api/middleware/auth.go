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
			if user, ok := knownUser(r.Context(), q, fbUID, email, name); ok {
				next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), user, user.Role)))
				return
			}

			user, err := upsertAccount(r.Context(), q, sqlcdb.UpsertUserByFirebaseUIDParams{
				Email:       email,
				Name:        name,
				FirebaseUid: fbUID,
			})
			if err != nil {
				if !isUniqueViolation(err) {
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

			role := firstAdminRole(r.Context(), q, db, user)
			next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), user, role)))
		})
	}
}

// knownUser returns the stored account of fbUID when the request needs no
// write: the upsert would keep the same email and name (a token name only
// replaces a non-empty, not yet onboarded one), the account has its person,
// and an admin exists, so first-user-admin has nothing to do. Any other
// case, including a failed lookup, falls through to provisioning.
func knownUser(ctx context.Context, q sqlcdb.Querier, fbUID types.NullString, email, name string) (sqlcdb.User, bool) {
	row, err := q.GetAuthUserByFirebaseUID(ctx, fbUID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("look up user", "uid", fbUID.String, "err", err)
		}
		return sqlcdb.User{}, false
	}
	user := row.User
	renamed := name != "" && !user.OnboardedAt.Valid && user.Name != name
	if user.Email != email || renamed || !row.HasPerson || !row.AdminExists {
		return sqlcdb.User{}, false
	}
	return user, true
}

// upsertAccount inserts or updates the account of a Firebase UID. A
// concurrent first request of the same account can insert it after this
// upsert's firebase_uid conflict check, and the email unique index then
// rejects the insert. That account is committed once the violation is
// reported, so one retry updates it instead. A second violation means the
// email belongs to another account.
func upsertAccount(ctx context.Context, q sqlcdb.Querier, arg sqlcdb.UpsertUserByFirebaseUIDParams) (sqlcdb.User, error) {
	user, err := q.UpsertUserByFirebaseUID(ctx, arg)
	if isUniqueViolation(err) {
		return q.UpsertUserByFirebaseUID(ctx, arg)
	}
	return user, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func withClaims(ctx context.Context, user sqlcdb.User, role string) context.Context {
	return context.WithValue(ctx, UserCtxKey, &auth.Claims{
		UserID:    user.ID,
		Email:     user.Email,
		Name:      user.Name,
		AvatarUrl: user.AvatarUrl.String,
		Role:      role,
	})
}

// firstAdminRole returns the user's role after first-user-admin: the first
// account ever provisioned becomes the club admin. Self-healing — it only
// promotes while no admin exists. A failed promotion keeps the stored role.
//
// user.Role may be stale: a concurrent first request of the same account
// can promote it after user was read. The fast path therefore ignores the
// user, so that case still reaches PromoteIfNoAdmin, which ignores it too
// and reports the promotion.
func firstAdminRole(ctx context.Context, q sqlcdb.Querier, db *sql.DB, user sqlcdb.User) string {
	if user.Role == "admin" {
		return user.Role
	}
	if n, err := q.CountOtherAdmins(ctx, user.ID); err != nil || n != 0 {
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
