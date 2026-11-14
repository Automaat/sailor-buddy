package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	VerifyIDTokenAndCheckRevoked(ctx context.Context, idToken string) (*fbauth.Token, error)
}

// Auth verifies the Firebase ID token, provisions the account on first
// sign-in and puts its stored role on the request context.
func Auth(fbClient TokenVerifier, q sqlcdb.Querier) func(http.Handler) http.Handler {
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

			verifyCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			fbToken, err := fbClient.VerifyIDTokenAndCheckRevoked(verifyCtx, token)
			cancel()
			if err != nil || fbToken == nil {
				if optional {
					next.ServeHTTP(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if invalidSession(err) {
					http.Error(w, `{"detail":"invalid or expired session"}`, http.StatusUnauthorized)
				} else {
					http.Error(w, `{"detail":"authentication service unavailable; try again"}`, http.StatusServiceUnavailable)
				}
				return
			}

			email, _ := fbToken.Claims["email"].(string)
			if email == "" {
				http.Error(w, `{"error":"missing email claim"}`, http.StatusUnauthorized)
				return
			}

			name, _ := fbToken.Claims["name"].(string)
			emailVerified, _ := fbToken.Claims["email_verified"].(bool)

			fbUID := types.NullString{String: fbToken.UID, Valid: true}
			if user, ok := knownUser(r.Context(), q, fbUID, email, name); ok {
				next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), user, user.Role, emailVerified)))
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
				user, err = linkByEmail(r.Context(), q, sqlcdb.LinkFirebaseUIDByEmailParams{
					FirebaseUid: fbUID,
					NewName:     name,
					Email:       email,
				})
			}
			if errors.Is(err, errEmailTaken) {
				slog.Warn("email belongs to another account", "email", email, "uid", fbToken.UID)
				http.Error(w, `{"error":"email belongs to another account"}`, http.StatusConflict)
				return
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

			next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), user, user.Role, emailVerified)))
		})
	}
}

// knownUser returns the stored account of fbUID when the request needs no
// write: the upsert would keep the same email and name (a token name only
// replaces a non-empty, not yet onboarded one), the account has its person,
// and no profile change is needed. Any other case, including a failed
// lookup, falls through to provisioning.
func knownUser(ctx context.Context, q sqlcdb.Querier, fbUID types.NullString, email, name string) (sqlcdb.User, bool) {
	row, err := q.GetAuthUserByFirebaseUID(ctx, fbUID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("look up user", "uid", fbUID.String, "err", err)
		}
		return sqlcdb.User{}, false
	}
	user := row.User
	if user.Suspended {
		return user, true
	}
	renamed := name != "" && !user.OnboardedAt.Valid && user.Name != name
	if user.Email != email || renamed || !row.HasPerson {
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

// errEmailTaken reports that another Firebase account owns the email.
var errEmailTaken = errors.New("email belongs to another account")

// linkByEmail gives the account of a verified email to a new Firebase UID.
// It only claims an account without a UID, and accounts are never moved
// between sign-ins automatically, so it returns errEmailTaken and changes
// nothing when:
//   - another Firebase account owns the email (no row matches), or
//   - the UID already has an account of its own with another email, so the
//     link breaks the firebase_uid unique index. The link writes no email,
//     so that index is the only one it can break.
func linkByEmail(ctx context.Context, q sqlcdb.Querier, arg sqlcdb.LinkFirebaseUIDByEmailParams) (sqlcdb.User, error) {
	user, err := q.LinkFirebaseUIDByEmail(ctx, arg)
	if errors.Is(err, sql.ErrNoRows) || isUniqueViolation(err) {
		return sqlcdb.User{}, errEmailTaken
	}
	return user, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}

func withClaims(ctx context.Context, user sqlcdb.User, role string, verified bool) context.Context {
	return context.WithValue(ctx, UserCtxKey, &auth.Claims{
		Approved:      user.AccessApproved || role == "admin",
		Suspended:     user.Suspended,
		EmailVerified: verified,
		UserID:        user.ID,
		Email:         user.Email,
		Name:          user.Name,
		AvatarUrl:     user.AvatarUrl.String,
		Role:          role,
	})
}

func GetUser(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(UserCtxKey).(*auth.Claims)
	return claims
}

func invalidSession(err error) bool {
	return fbauth.IsIDTokenRevoked(err) || fbauth.IsUserDisabled(err) || fbauth.IsIDTokenExpired(err) || fbauth.IsIDTokenInvalid(err) || fbauth.IsUserNotFound(err) || fbauth.IsTenantIDMismatch(err)
}
