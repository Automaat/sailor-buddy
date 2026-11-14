package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
)

func TestBootstrapCLIRejectsInvalidInvocationWithoutConnecting(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		dsn, uid string
		want     error
	}{
		{name: "help", args: []string{"-h"}, want: flag.ErrHelp},
		{name: "unexpected argument", args: []string{"user"}, want: cli.ErrUsage},
		{name: "unknown option", args: []string{"--uid=x"}, want: cli.ErrUsage},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", tt.dsn)
			t.Setenv("SAILOR_BOOTSTRAP_FIREBASE_UID", tt.uid)
			var stdout, stderr bytes.Buffer
			err := run(t.Context(), tt.args, &stdout, &stderr, func(context.Context, string) error { t.Fatal("connected unexpectedly"); return nil })
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
		})
	}
}

func TestBootstrapCLIRequiresDatabaseAndUID(t *testing.T) {
	for _, dsn := range []string{"", "postgres://sailor@localhost/sailor"} {
		t.Run(dsn, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", dsn)
			t.Setenv("SAILOR_BOOTSTRAP_FIREBASE_UID", "")
			var stdout, stderr bytes.Buffer
			if err := run(t.Context(), nil, &stdout, &stderr, func(context.Context, string) error { t.Fatal("connected without configuration"); return nil }); err == nil {
				t.Fatal("accepted missing configuration")
			}
		})
	}
}
