package auth

import (
	"context"
	"errors"
	"os"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
)

type Claims struct {
	Approved      bool   `json:"approved"`
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
		return nil, errors.New("Firebase emulator is forbidden in production")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	return app.Auth(ctx)
}
