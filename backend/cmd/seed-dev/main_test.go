package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

func TestRun(t *testing.T) {
	const dsn = "postgres://u:p@db.example:5432/x?sslmode=disable"
	tests := []struct {
		name       string
		args       []string
		env        string
		wantErr    error
		wantSeed   bool
		wantStdout string
	}{
		{name: "help", args: []string{"-h"}, env: dsn, wantErr: flag.ErrHelp, wantStdout: "Usage: seed-dev"},
		{name: "long help", args: []string{"--help"}, env: dsn, wantErr: flag.ErrHelp, wantStdout: "Usage: seed-dev"},
		{name: "unknown flag", args: []string{"-x"}, env: dsn, wantErr: cli.ErrUsage},
		{name: "positional argument", args: []string{"now"}, env: dsn, wantErr: cli.ErrUsage},
		{name: "unset database URL", env: "", wantErr: config.ErrDatabaseURLUnset},
		{name: "blank database URL", env: "  ", wantErr: config.ErrDatabaseURLUnset},
		{name: "explicit database URL", env: dsn, wantSeed: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", tc.env)
			var stdout, stderr bytes.Buffer
			var gotDSN string
			called := false
			apply := func(_ context.Context, dsn string) error {
				called = true
				gotDSN = dsn
				return nil
			}
			err := run(context.Background(), tc.args, &stdout, &stderr, apply)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("run(%q) = %v, want %v", tc.args, err, tc.wantErr)
			}
			if called != tc.wantSeed {
				t.Fatalf("seed called = %v, want %v", called, tc.wantSeed)
			}
			if tc.wantSeed && gotDSN != dsn {
				t.Errorf("seed dsn = %q, want %q", gotDSN, dsn)
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantStdout)
			}
		})
	}
}
