package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
)

type ctxKey string

const UserCtxKey ctxKey = "user"

func Auth(jwtSecret string) func(http.Handler) http.Handler {
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

			claims, err := auth.ValidateAccessToken(jwtSecret, token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
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
