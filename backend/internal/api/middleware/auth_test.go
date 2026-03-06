package middleware

import (
	"context"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
)

func TestGetUser_ValidClaims(t *testing.T) {
	want := &auth.Claims{
		UserID:    42,
		Email:     "sailor@example.com",
		Name:      "Captain Hook",
		AvatarUrl: "https://example.com/avatar.png",
	}
	ctx := context.WithValue(context.Background(), UserCtxKey, want)

	got := GetUser(ctx)
	if got == nil {
		t.Fatal("expected non-nil claims")
	}
	if got.UserID != want.UserID {
		t.Errorf("UserID = %d, want %d", got.UserID, want.UserID)
	}
	if got.Email != want.Email {
		t.Errorf("Email = %q, want %q", got.Email, want.Email)
	}
	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if got.AvatarUrl != want.AvatarUrl {
		t.Errorf("AvatarUrl = %q, want %q", got.AvatarUrl, want.AvatarUrl)
	}
}

func TestGetUser_EmptyContext(t *testing.T) {
	got := GetUser(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestGetUser_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserCtxKey, "not-a-claims-struct")

	got := GetUser(ctx)
	if got != nil {
		t.Errorf("expected nil for wrong type, got %+v", got)
	}
}

func TestGetUser_WrongKey(t *testing.T) {
	claims := &auth.Claims{UserID: 1, Email: "a@b.com"}
	ctx := context.WithValue(context.Background(), ctxKey("other"), claims)

	got := GetUser(ctx)
	if got != nil {
		t.Errorf("expected nil for wrong key, got %+v", got)
	}
}
