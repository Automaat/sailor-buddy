package auth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/auth/credentials"
	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Claims struct {
	Approved      bool   `json:"approved"`
	IsCaptain     bool   `json:"is_captain"`
	Suspended     bool   `json:"suspended"`
	EmailVerified bool   `json:"email_verified"`
	UserID        int64  `json:"user_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	AvatarUrl     string `json:"avatar_url"`
	Role          string `json:"role"`
}

func NewFirebaseAuth(ctx context.Context, projectID string) (*fbauth.Client, error) {
	if os.Getenv("SAILOR_ENV") == "production" && os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return nil, errors.New("emulator authentication is forbidden in production")
	}
	var options []option.ClientOption
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") == "" {
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			Client: &http.Client{Timeout: 5 * time.Second},
		})
		if err != nil {
			return nil, err
		}
		// The context-aware credential provider bounds refresh RPCs with the
		// verification request, unlike the legacy startup-context token source.
		options = append(options, option.WithAuthCredentials(creds))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, options...)
	if err != nil {
		return nil, err
	}
	return app.Auth(ctx)
}
