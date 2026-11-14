// Command migrate applies pending database migrations to the database named
// by SAILOR_DATABASE_URL.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

const usage = `Usage: migrate [-h]

Apply every pending migration to the database named by SAILOR_DATABASE_URL.
SAILOR_DATABASE_URL is required; migrate never falls back to a default.
`

// applyFunc opens the database at dsn and applies pending migrations.
type applyFunc func(ctx context.Context, dsn string) error

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, applyMigrations)
	switch {
	case errors.Is(err, flag.ErrHelp):
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	case err != nil:
		log.Fatal(err)
	default:
		log.Println("migrations applied successfully")
	}
}

// run parses args and, only when they are valid and SAILOR_DATABASE_URL is
// set, calls apply; see cli.ParseNoArgs for help and bad-argument results.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, apply applyFunc) error {
	if err := cli.ParseNoArgs("migrate", usage, args, stdout, stderr); err != nil {
		return err
	}

	dsn, err := config.RequireDatabaseURL()
	if err != nil {
		return err
	}
	return apply(ctx, dsn)
}

func applyMigrations(ctx context.Context, dsn string) error {
	database, err := db.OpenContext(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.Migrate(ctx, database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
