package cli

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

const testUsage = "Usage: cmd [-h]\n"

func TestParseNoArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantStdout string
		wantStderr []string
	}{
		{name: "no arguments", args: nil},
		{name: "empty arguments", args: []string{}},
		{name: "short help", args: []string{"-h"}, wantErr: flag.ErrHelp, wantStdout: testUsage},
		{name: "long help", args: []string{"--help"}, wantErr: flag.ErrHelp, wantStdout: testUsage},
		{name: "single-dash long help", args: []string{"-help"}, wantErr: flag.ErrHelp, wantStdout: testUsage},
		{name: "help after terminator is positional", args: []string{"--", "-h"}, wantErr: ErrUsage, wantStderr: []string{`unexpected argument "-h"`, testUsage}},
		{name: "unknown flag", args: []string{"-x"}, wantErr: ErrUsage, wantStderr: []string{"flag provided but not defined: -x", testUsage}},
		{name: "unknown long flag", args: []string{"--down"}, wantErr: ErrUsage, wantStderr: []string{"flag provided but not defined: -down", testUsage}},
		{name: "positional argument", args: []string{"up"}, wantErr: ErrUsage, wantStderr: []string{`unexpected argument "up"`, testUsage}},
		{name: "terminator only", args: []string{"--"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := ParseNoArgs("cmd", testUsage, tc.args, &stdout, &stderr)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseNoArgs(%q) = %v, want %v", tc.args, err, tc.wantErr)
			}
			if stdout.String() != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantStdout)
			}
			if len(tc.wantStderr) == 0 && stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
		})
	}
}
