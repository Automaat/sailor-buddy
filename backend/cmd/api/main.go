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
	"syscall"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

// shutdownTimeout bounds how long in-flight requests may run after a stop
// signal before the server closes their connections.
const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore default handling so a second one kills
	// the process instead of waiting for the drain.
	context.AfterFunc(ctx, stop)

	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.Migrate(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	fbClient, err := auth.NewFirebaseAuth(ctx, cfg.FirebaseProjectID)
	if err != nil {
		return fmt.Errorf("init firebase auth: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := newServer(cfg.ListenAddr, api.NewRouter(database, cfg, fbClient))
	log.Printf("listening on %s", ln.Addr())
	return serve(ctx, srv, ln, shutdownTimeout)
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
