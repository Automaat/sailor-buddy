package auth

import (
	"context"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
}

func NewFirebaseAuth(ctx context.Context, projectID string) (*fbauth.Client, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	return app.Auth(ctx)
}
