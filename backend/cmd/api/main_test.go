package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

type result struct {
	status int
	body   string
	err    error
}

// startSlowServer serves a handler that blocks until release is closed and
// returns the listener address, a channel closed once the handler is running,
// and serve's error channel.
func startSlowServer(ctx context.Context, t *testing.T, release <-chan struct{}, timeout time.Duration) (addr string, started <-chan struct{}, served <-chan error) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	startedCh := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(startedCh)
		<-release
		_, _ = io.WriteString(w, "done")
	})
	srv := newServer(ln.Addr().String(), handler)

	servedCh := make(chan error, 1)
	go func() { servedCh <- serve(ctx, srv, ln, timeout) }()
	return ln.Addr().String(), startedCh, servedCh
}

func get(addr string) <-chan result {
	out := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			out <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		out <- result{status: resp.StatusCode, body: string(body), err: err}
	}()
	return out
}

// waitListenerClosed blocks until addr refuses new connections, which shows
// Shutdown has started.
func waitListenerClosed(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("listener still accepting connections 5s after the stop signal")
}

// TestServeDrainsInFlightOnSIGTERM sends the process a real SIGTERM while a
// request is in flight and asserts the request still completes and serve
// reports a clean shutdown.
func TestServeDrainsInFlightOnSIGTERM(t *testing.T) {
	ctx, stop := signalContext()
	defer stop()

	release := make(chan struct{})
	addr, started, served := startSlowServer(ctx, t, release, 5*time.Second)
	resp := get(addr)

	<-started
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	waitListenerClosed(t, addr)

	select {
	case err := <-served:
		t.Fatalf("serve returned %v before the in-flight request finished", err)
	default:
	}
	close(release)

	r := <-resp
	if r.err != nil || r.status != http.StatusOK || r.body != "done" {
		t.Fatalf("in-flight request = (%d, %q, %v), want (200, \"done\", nil)", r.status, r.body, r.err)
	}
	if err := <-served; err != nil {
		t.Fatalf("serve = %v, want nil", err)
	}
}

// TestServeShutdownTimeout asserts a request that outlives the drain window
// makes serve return a deadline error and closes the connection.
func TestServeShutdownTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	release := make(chan struct{})
	defer close(release)
	addr, started, served := startSlowServer(ctx, t, release, 50*time.Millisecond)
	resp := get(addr)

	<-started
	cancel()

	err := <-served
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("serve = %v, want context.DeadlineExceeded", err)
	}
	if r := <-resp; r.err == nil {
		t.Fatalf("request after forced close = (%d, %q), want a connection error", r.status, r.body)
	}
}

// TestServeListenerFailure asserts serve returns a listener error at once
// instead of waiting for a stop signal.
func TestServeListenerFailure(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_ = ln.Close()

	done := make(chan error, 1)
	go func() { done <- serve(context.Background(), newServer("", http.NotFoundHandler()), ln, time.Second) }()

	select {
	case err := <-done:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("serve = %v, want a listener error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the listener failed")
	}
}

func TestNewServerTimeouts(t *testing.T) {
	t.Parallel()
	srv := newServer(":0", http.NotFoundHandler())
	got := []time.Duration{srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout}
	want := []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second, 120 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("timeouts = %v, want %v", got, want)
			break
		}
	}
}

// childModeEnv selects what TestChildProcess does when the test binary
// re-runs itself as a child process.
const childModeEnv = "SAILOR_API_TEST_CHILD"

// TestChildProcess is not a test on its own: the subprocess tests below run
// the test binary again with childModeEnv set, so real signals hit a separate
// process.
func TestChildProcess(t *testing.T) {
	switch os.Getenv(childModeEnv) {
	case "signal":
		ctx, stop := signalContext()
		_, _ = os.Stdout.WriteString("ready\n")
		<-ctx.Done()
		_, _ = os.Stdout.WriteString("draining\n")
		time.Sleep(30 * time.Second)
		stop()
		os.Exit(3)
	case "run":
		main()
		os.Exit(0)
	}
}

func startChild(t *testing.T, mode string, env ...string) (cmd *exec.Cmd, stdout *lineReader) {
	t.Helper()
	cmd = exec.Command(os.Args[0], "-test.run=^TestChildProcess$")
	cmd.Env = append(os.Environ(), append(env, childModeEnv+"="+mode)...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, newLineReader(out)
}

type lineReader struct{ lines chan string }

func newLineReader(r io.Reader) *lineReader {
	lr := &lineReader{lines: make(chan string, 16)}
	go func() {
		defer close(lr.lines)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lr.lines <- sc.Text()
		}
	}()
	return lr
}

func (lr *lineReader) waitFor(t *testing.T, want string) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case line, ok := <-lr.lines:
			if !ok {
				t.Fatalf("child output closed before %q", want)
			}
			if line == want {
				return
			}
		case <-timeout:
			t.Fatalf("child did not print %q within 10s", want)
		}
	}
}

func waitExit(t *testing.T, cmd *exec.Cmd) *os.ProcessState {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return cmd.ProcessState
	case <-time.After(10 * time.Second):
		t.Fatal("child still running 10s after the signal")
		return nil
	}
}

// waitExitResending sends sig until the child exits. Default handling is
// restored asynchronously after the first signal, so a signal sent in that
// instant may still be absorbed; a repeat must then kill the process.
func waitExitResending(t *testing.T, cmd *exec.Cmd, sig os.Signal) *os.ProcessState {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	timeout := time.After(10 * time.Second)
	for {
		_ = cmd.Process.Signal(sig)
		select {
		case <-done:
			return cmd.ProcessState
		case <-timeout:
			t.Fatal("child still running 10s after repeated signals")
			return nil
		case <-tick.C:
		}
	}
}

// TestSignalContextSecondSignalKills checks that both SIGINT and SIGTERM
// start the drain, and that a second signal during the drain kills the
// process instead of being swallowed.
func TestSignalContextSecondSignalKills(t *testing.T) {
	t.Parallel()
	for _, first := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(first.String(), func(t *testing.T) {
			t.Parallel()
			cmd, out := startChild(t, "signal")
			out.waitFor(t, "ready")

			if err := cmd.Process.Signal(first); err != nil {
				t.Fatalf("send %v: %v", first, err)
			}
			out.waitFor(t, "draining")
			state := waitExitResending(t, cmd, syscall.SIGTERM)
			ws, ok := state.Sys().(syscall.WaitStatus)
			if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGTERM {
				t.Fatalf("child exit = %v, want killed by SIGTERM", state)
			}
		})
	}
}

// TestRunStopsOnSIGTERMDuringStartup points the server at a TCP peer that
// accepts and never answers, so startup blocks in the database ping. A
// SIGTERM must end the process at once with a clean exit.
func TestRunStopsOnSIGTERMDuringStartup(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	dsn := "postgres://u:p@" + ln.Addr().String() + "/x?sslmode=disable"
	cmd, _ := startChild(t, "run", "SAILOR_DATABASE_URL="+dsn, "SAILOR_LISTEN_ADDR=127.0.0.1:0")

	select {
	case conn := <-accepted:
		t.Cleanup(func() { _ = conn.Close() })
	case <-time.After(10 * time.Second):
		t.Fatal("child never connected to the fake database")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	start := time.Now()
	state := waitExit(t, cmd)
	if state.ExitCode() != 0 {
		t.Fatalf("child exit = %v, want clean exit 0", state)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("child took %v to stop after SIGTERM, want under 5s", elapsed)
	}
}
