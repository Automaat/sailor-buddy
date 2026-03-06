package middleware

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type ctxKey string

const UserCtxKey ctxKey = "user"

func Auth(fbClient *fbauth.Client, q *sqlcdb.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			fbToken, err := fbClient.VerifyIDToken(r.Context(), token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			email, _ := fbToken.Claims["email"].(string)
			if email == "" {
				http.Error(w, `{"error":"missing email claim"}`, http.StatusUnauthorized)
				return
			}

			name, _ := fbToken.Claims["name"].(string)

			fbUID := sql.NullString{String: fbToken.UID, Valid: true}
			user, err := q.UpsertUserByFirebaseUID(r.Context(), sqlcdb.UpsertUserByFirebaseUIDParams{
				Email:       email,
				Name:        name,
				FirebaseUid: fbUID,
			})
			if err != nil {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
					log.Printf("upsert failed (email=%s uid=%s): %v", email, fbToken.UID, err)
					http.Error(w, `{"error":"failed to provision user"}`, http.StatusInternalServerError)
					return
				}
				emailVerified, _ := fbToken.Claims["email_verified"].(bool)
				if !emailVerified {
					log.Printf("email not verified, refusing link by email (email=%s uid=%s)", email, fbToken.UID)
					http.Error(w, `{"error":"email not verified"}`, http.StatusUnauthorized)
					return
				}
				log.Printf("upsert failed (email=%s uid=%s): %v — trying link by email", email, fbToken.UID, err)
				user, err = q.LinkFirebaseUIDByEmail(r.Context(), sqlcdb.LinkFirebaseUIDByEmailParams{
					FirebaseUid: fbUID,
					NewName:     name,
					Email:       email,
				})
			}
			if err != nil {
				log.Printf("provision failed (email=%s uid=%s): %v", email, fbToken.UID, err)
				http.Error(w, `{"error":"failed to provision user"}`, http.StatusInternalServerError)
				return
			}

			claims := &auth.Claims{UserID: user.ID, Email: user.Email, Name: user.Name, AvatarUrl: user.AvatarUrl.String}
			ctx := context.WithValue(r.Context(), UserCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(UserCtxKey).(*auth.Claims)
	return claims
}
