package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
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
