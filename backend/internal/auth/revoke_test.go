package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	fbauth "firebase.google.com/go/v4/auth"
)

type failingRevoker struct {
	calls    int
	failCall int
	cutoff   int64
	err      error
}

func (r *failingRevoker) RevokeRefreshTokens(_ context.Context, _ string) error {
	r.calls++
	if r.calls == r.failCall {
		return r.err
	}
	return nil
}

func (r *failingRevoker) GetUser(context.Context, string) (*fbauth.UserRecord, error) {
	if r.failCall == 3 {
		return nil, r.err
	}
	return &fbauth.UserRecord{TokensValidAfterMillis: r.cutoff}, nil
}

func TestRevokeSessionsDoesNotReportUnconfirmedCutoff(t *testing.T) {
	for _, failCall := range []int{1, 2, 3, 0} {
		t.Run(string(rune('0'+failCall)), func(t *testing.T) {
			failure := errors.New("operator RPC failed")
			client := &failingRevoker{failCall: failCall, cutoff: time.Now().Unix() * 1000, err: failure}
			err := RevokeSessions(t.Context(), client, "exact-uid")
			if err == nil {
				t.Fatal("reported success without a confirmed later cutoff")
			}
			if failCall != 0 && !errors.Is(err, failure) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRevokeSessionsCancellationStopsSecondRevoke(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client := &failingRevoker{}
	if err := RevokeSessions(ctx, client, "exact-uid"); !errors.Is(err, context.Canceled) || client.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, client.calls)
	}
}

func TestRevokeSessionsRejectsInexactUIDBeforeRPC(t *testing.T) {
	for _, uid := range []string{"", " user", "user "} {
		client := &failingRevoker{}
		if err := RevokeSessions(t.Context(), client, uid); err == nil || client.calls != 0 {
			t.Fatalf("uid=%q error=%v calls=%d", uid, err, client.calls)
		}
	}
}
