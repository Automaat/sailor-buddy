package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const childEnv = "SAILOR_MIGRATE_TEST_CHILD"

// recorder is an applyFunc that records whether run reached the database step.
type recorder struct {
	called bool
	dsn    string
}

func (r *recorder) apply(_ context.Context, dsn string) error {
	r.called = true
	r.dsn = dsn
	return nil
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "-help"} {
		t.Run(arg, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", "postgres://u:p@127.0.0.1:1/x?sslmode=disable")
			var stdout, stderr bytes.Buffer
			var rec recorder
			err := run(context.Background(), []string{arg}, &stdout, &stderr, rec.apply)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("run(%s) = %v, want flag.ErrHelp", arg, err)
			}
			if rec.called {
				t.Error("help reached the database step")
			}
			if !strings.Contains(stdout.String(), "Usage: migrate") {
				t.Errorf("stdout = %q, want usage", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"-x"}, "flag provided but not defined: -x"},
		{"positional argument", []string{"up"}, `unexpected argument "up"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", "postgres://u:p@127.0.0.1:1/x?sslmode=disable")
			var stdout, stderr bytes.Buffer
			var rec recorder
			err := run(context.Background(), tc.args, &stdout, &stderr, rec.apply)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("run(%v) = %v, want cli.ErrUsage", tc.args, err)
			}
			if rec.called {
				t.Error("bad arguments reached the database step")
			}
			if got := stderr.String(); !strings.Contains(got, tc.want) || !strings.Contains(got, "Usage: migrate") {
				t.Errorf("stderr = %q, want %q and usage", got, tc.want)
			}
		})
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	for name, value := range map[string]string{"empty": "", "blank": "  "} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", value)
			var rec recorder
			err := run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, rec.apply)
			if !errors.Is(err, config.ErrDatabaseURLUnset) {
				t.Fatalf("run() = %v, want ErrDatabaseURLUnset", err)
			}
			if rec.called {
				t.Error("unset SAILOR_DATABASE_URL reached the database step")
			}
		})
	}
}

func TestRunUsesDatabaseURL(t *testing.T) {
	const dsn = "postgres://u:p@db.example:5432/x?sslmode=disable"
	t.Setenv("SAILOR_DATABASE_URL", dsn)
	var rec recorder
	if err := run(context.Background(), nil, &bytes.Buffer{}, &bytes.Buffer{}, rec.apply); err != nil {
		t.Fatalf("run() = %v", err)
	}
	if !rec.called || rec.dsn != dsn {
		t.Errorf("apply called=%v dsn=%q, want called with %q", rec.called, rec.dsn, dsn)
	}
}

// TestChildProcess runs main in a subprocess so exit codes can be checked.
// The command's arguments follow "--" on the test binary's command line.
func TestChildProcess(t *testing.T) {
	if os.Getenv(childEnv) != "1" {
		t.Skip("helper process")
	}
	os.Args = append([]string{"migrate"}, flag.Args()...)
	main()
	os.Exit(0)
}

type childResult struct {
	code           int
	stdout, stderr string
}

func runChild(t *testing.T, args []string, env ...string) childResult {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestChildProcess$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), append(env, childEnv+"=1")...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("run child: %v", err)
	}
	return childResult{code: cmd.ProcessState.ExitCode(), stdout: stdout.String(), stderr: stderr.String()}
}

// countingListener accepts TCP connections and counts them, standing in for a
// database that must never be contacted.
func countingListener(t *testing.T) (addr string, accepted *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accepted = &atomic.Int32{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()
	return ln.Addr().String(), accepted
}

func TestMainHelpExitsZeroWithoutConnecting(t *testing.T) {
	addr, accepted := countingListener(t)
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			res := runChild(t, []string{arg}, "SAILOR_DATABASE_URL=postgres://u:p@"+addr+"/x?sslmode=disable&connect_timeout=2")
			if res.code != 0 {
				t.Fatalf("exit code = %d, want 0 (stderr %q)", res.code, res.stderr)
			}
			if !strings.Contains(res.stdout, "Usage: migrate") {
				t.Errorf("stdout = %q, want usage", res.stdout)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	if n := accepted.Load(); n != 0 {
		t.Errorf("database listener accepted %d connections, want 0", n)
	}
}

func TestMainExitCodes(t *testing.T) {
	addr, accepted := countingListener(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	// An empty DSN makes pgx fall back to PGHOST/PGPORT, then localhost:5432.
	// Point the fallback at the listener so a regression is counted instead of
	// reaching a real local database.
	pgFallback := []string{"PGHOST=" + host, "PGPORT=" + port, "PGCONNECT_TIMEOUT=2"}
	dsn := "SAILOR_DATABASE_URL=postgres://u:p@" + addr + "/x?sslmode=disable&connect_timeout=2"
	tests := []struct {
		name   string
		args   []string
		env    string
		code   int
		stderr string
	}{
		{"unknown flag", []string{"-x"}, dsn, 2, "flag provided but not defined: -x"},
		{"positional argument", []string{"up"}, dsn, 2, `unexpected argument "up"`},
		{"unset database URL", nil, "SAILOR_DATABASE_URL=", 1, "SAILOR_DATABASE_URL is not set"},
		{"blank database URL", nil, "SAILOR_DATABASE_URL= ", 1, "SAILOR_DATABASE_URL is not set"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := runChild(t, tc.args, append([]string{tc.env}, pgFallback...)...)
			if res.code != tc.code {
				t.Fatalf("exit code = %d, want %d (stderr %q)", res.code, tc.code, res.stderr)
			}
			if !strings.Contains(res.stderr, tc.stderr) {
				t.Errorf("stderr = %q, want %q", res.stderr, tc.stderr)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	if n := accepted.Load(); n != 0 {
		t.Errorf("database listener accepted %d connections, want 0", n)
	}
}

// freshDatabaseURL creates an empty database of its own on the server named by
// SAILOR_TEST_DATABASE_URL, drops it after the test and returns its URL.
func freshDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SAILOR_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SAILOR_TEST_DATABASE_URL not set; skipping database test")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open admin db: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random db name: %v", err)
	}
	name := "migrate_cmd_" + hex.EncodeToString(b)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func appliedMigrations(t *testing.T, dsn string) int {
	t.Helper()
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = conn.Close() }()
	var n int
	if err := conn.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	return n
}

// TestMainAppliesMigrations runs the real binary against a fresh database:
// the explicit-URL path still applies every migration and stays idempotent.
func TestMainAppliesMigrations(t *testing.T) {
	dsn := freshDatabaseURL(t)

	res := runChild(t, nil, "SAILOR_DATABASE_URL="+dsn)
	if res.code != 0 {
		t.Fatalf("first run exit code = %d, stderr %q", res.code, res.stderr)
	}
	if !strings.Contains(res.stderr, "migrations applied successfully") {
		t.Errorf("stderr = %q, want success message", res.stderr)
	}
	first := appliedMigrations(t, dsn)
	if first == 0 {
		t.Fatal("no migrations recorded after the first run")
	}

	res = runChild(t, nil, "SAILOR_DATABASE_URL="+dsn)
	if res.code != 0 {
		t.Fatalf("second run exit code = %d, stderr %q", res.code, res.stderr)
	}
	if second := appliedMigrations(t, dsn); second != first {
		t.Errorf("second run recorded %d migrations, want %d", second, first)
	}
}
