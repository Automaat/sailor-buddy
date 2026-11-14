package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
	"github.com/marcinskalski/sailor-buddy/backend/internal/docgen"
)

// shutdownTimeout bounds how long in-flight requests may run after a stop
// signal before the server closes their connections.
const shutdownTimeout = 30 * time.Second

// errStopped marks a startup step cut short by a stop signal: a requested
// exit, not a failure.
var errStopped = errors.New("stop signal received")

func main() {
	err := run()
	switch {
	case errors.Is(err, errStopped):
		log.Print(err)
	case err != nil:
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signalContext()
	defer stop()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	database, err := db.OpenContext(ctx, cfg.DatabaseURL)
	if err != nil {
		return startupError(ctx, "open database", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.Migrate(ctx, database); err != nil {
		return startupError(ctx, "run migrations", err)
	}

	fbClient, err := auth.NewFirebaseAuth(ctx, cfg.FirebaseProjectID)
	if err != nil {
		return fmt.Errorf("init firebase auth: %w", err)
	}

	uploads, err := handlers.OpenUploadRoot(cfg.UploadDir)
	if err != nil {
		return err
	}
	defer func() {
		if err := uploads.Close(); err != nil {
			log.Printf("failed to close upload directory: %v", err)
		}
	}()

	if os.Getenv("CHROME_PATH") != "" {
		go checkPDFRenderer(ctx)
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := newServer(cfg.ListenAddr, api.NewRouter(database, cfg, fbClient, uploads))
	log.Printf("listening on %s", ln.Addr())
	return serve(ctx, srv, ln, shutdownTimeout)
}

// checkPDFRenderer renders one page at startup, so a container whose
// seccomp or AppArmor settings block the Chromium sandbox shows up in the
// log at boot rather than on the first PDF opinion.
func checkPDFRenderer(ctx context.Context) {
	_, err := docgen.GeneratePDF(ctx, []byte("<p>startup check</p>"))
	if err != nil && ctx.Err() == nil {
		sandboxOff, _ := strconv.ParseBool(os.Getenv("SAILOR_CHROME_NO_SANDBOX"))
		log.Printf("PDF renderer unavailable, PDF opinions will fail (%s): %v",
			pdfRendererHint("/proc", sandboxOff), err)
	}
}

// pdfRendererHint names the container settings most likely to block the
// Chromium sandbox. procDir is the procfs mount, a parameter for tests.
// sandboxOff reports SAILOR_CHROME_NO_SANDBOX; with the sandbox off neither
// seccomp nor AppArmor can be the cause.
//
// Hosts with kernel.apparmor_restrict_unprivileged_userns=1 (Ubuntu 24.04+)
// move an AppArmor-unconfined process into the unprivileged_userns profile
// when Chromium creates its sandbox namespace, and the sandbox cannot start.
// Docker's default AppArmor profile is not affected, so the fix is to drop
// apparmor=unconfined. Docker's default seccomp profile blocks the namespace
// before AppArmor sees it, so that hint also names the seccomp profile.
func pdfRendererHint(procDir string, sandboxOff bool) string {
	const seccompHint = "run with --security-opt seccomp=backend/seccomp-chromium.json"
	switch {
	case sandboxOff:
		return "Chromium sandbox is off (SAILOR_CHROME_NO_SANDBOX); check CHROME_PATH"
	case apparmorUnconfined(procDir) && userNSRestricted(procDir):
		return "AppArmor restricts user namespaces on this host and the container " +
			"runs unconfined: drop --security-opt apparmor=unconfined to keep " +
			"Docker's default profile, and " + seccompHint + "; see README"
	default:
		return "Docker needs to " + seccompHint + "; see README"
	}
}

// apparmorUnconfined reports whether this process runs without an AppArmor
// profile. Newer kernels expose the label under attr/apparmor/.
func apparmorUnconfined(procDir string) bool {
	for _, name := range []string{"self/attr/apparmor/current", "self/attr/current"} {
		b, err := os.ReadFile(filepath.Join(procDir, name))
		if err == nil {
			return strings.TrimRight(string(b), "\x00\n ") == "unconfined"
		}
	}
	return false
}

// userNSRestricted reports whether AppArmor restricts unprivileged user
// namespaces on this host.
func userNSRestricted(procDir string) bool {
	b, err := os.ReadFile(filepath.Join(procDir, "sys/kernel/apparmor_restrict_unprivileged_userns"))
	return err == nil && strings.TrimSpace(string(b)) == "1"
}

// signalContext returns a context cancelled by SIGINT or SIGTERM. After the
// first signal it restores default handling, so a second signal kills the
// process instead of waiting for the drain.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop)
	return ctx, stop
}

// startupError wraps a failed startup step, marking it errStopped when a stop
// signal caused the failure.
func startupError(ctx context.Context, step string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w during %s: %w", errStopped, step, err)
	}
	return fmt.Errorf("%s: %w", step, err)
}

// newServer builds the HTTP server with bounded timeouts. WriteTimeout covers
// the 30 s PDF render plus response write.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// serve runs srv on ln until ctx is done, then shuts it down gracefully,
// waiting up to timeout for in-flight requests before closing their
// connections. A serve failure before ctx is done is returned at once.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Printf("shutting down, waiting up to %s for in-flight requests", timeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		closeErr := srv.Close()
		return fmt.Errorf("shutdown: %w", errors.Join(err, closeErr))
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
