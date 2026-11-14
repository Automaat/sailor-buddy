// Command bootstrap-admin provisions an explicitly selected registered Firebase UID.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

const usage = `Usage: bootstrap-admin [-h]

Promote SAILOR_BOOTSTRAP_FIREBASE_UID only when no administrator exists.
The exact Firebase UID must already have signed in to the application.
SAILOR_DATABASE_URL is required; bootstrap-admin never falls back to a default.
`

// applyFunc runs the private bootstrap operation.
type applyFunc func(ctx context.Context, dsn string) error

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, bootstrapAdmin)
	switch {
	case errors.Is(err, flag.ErrHelp):
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	case err != nil:
		log.Fatal(err)
	default:
		log.Println("initial administrator provisioned")
	}
}

// run parses args and, only when they are valid and SAILOR_DATABASE_URL is
// set, calls apply; see cli.ParseNoArgs for help and bad-argument results.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, apply applyFunc) error {
	if err := cli.ParseNoArgs("bootstrap-admin", usage, args, stdout, stderr); err != nil {
		return err
	}

	dsn, err := config.RequireDatabaseURL()
	if err != nil {
		return err
	}
	uid := os.Getenv("SAILOR_BOOTSTRAP_FIREBASE_UID")
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(uid) != uid {
		return errors.New("SAILOR_BOOTSTRAP_FIREBASE_UID is required")
	}
	return apply(ctx, dsn)
}

func bootstrapAdmin(ctx context.Context, dsn string) error {
	database, err := db.OpenContext(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	return auth.BootstrapAdmin(ctx, database, os.Getenv("SAILOR_BOOTSTRAP_FIREBASE_UID"))
}
