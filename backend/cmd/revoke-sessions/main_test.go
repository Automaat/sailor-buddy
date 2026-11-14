package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
)

func TestRevokeInvocation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         []string
		project, uid string
		want         error
	}{
		{name: "help", args: []string{"-h"}, want: flag.ErrHelp},
		{name: "unknown argument", args: []string{"somebody"}, want: cli.ErrUsage},
		{name: "missing identity", project: "sailor-club"},
		{name: "missing project", uid: "exact-uid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_FIREBASE_PROJECT_ID", tc.project)
			t.Setenv("SAILOR_REVOKE_FIREBASE_UID", tc.uid)
			var stdout, stderr bytes.Buffer
			err := run(t.Context(), tc.args, &stdout, &stderr, func(context.Context, string, string) error { t.Fatal("connected unexpectedly"); return nil })
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
		})
	}
}
