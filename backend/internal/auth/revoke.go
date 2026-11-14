package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fbauth "firebase.google.com/go/v4/auth"
)

// SessionRevoker is the operator-only Firebase account API.
type SessionRevoker interface {
	RevokeRefreshTokens(context.Context, string) error
	GetUser(context.Context, string) (*fbauth.UserRecord, error)
}

// RevokeSessions advances the Firebase cutoff beyond this operation's first
// second. The SDK accepts tokens issued in exactly the cutoff second.
func RevokeSessions(ctx context.Context, client SessionRevoker, uid string) error {
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(uid) != uid {
		return errors.New("an exact Firebase UID is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := client.RevokeRefreshTokens(ctx, uid); err != nil {
		return fmt.Errorf("initial revocation: %w", err)
	}
	firstSecond := time.Now().Unix()
	timer := time.NewTimer(time.Until(time.Unix(firstSecond+1, 0)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if err := client.RevokeRefreshTokens(ctx, uid); err != nil {
		return fmt.Errorf("final revocation: %w", err)
	}
	user, err := client.GetUser(ctx, uid)
	if err != nil {
		return fmt.Errorf("verify revocation: %w", err)
	}
	if user == nil || user.TokensValidAfterMillis <= firstSecond*1000 {
		return errors.New("revocation cutoff not confirmed; keep the account suspended and retry")
	}
	return nil
}
