// Command revoke-sessions invalidates sessions for an explicitly selected Firebase UID.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/cli"
)

const usage = `Usage: revoke-sessions [-h]

Revoke sessions for the exact SAILOR_REVOKE_FIREBASE_UID in
SAILOR_FIREBASE_PROJECT_ID, then verify that Firebase advanced its cutoff.
Requires an operator ADC identity with firebaseauth.users.get and .update.
Suspend/disable the account separately to prevent new sign-ins.
`

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr, revoke)
	cancel()
	switch {
	case errors.Is(err, flag.ErrHelp):
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	case err != nil:
		log.Fatal(err)
	default:
		log.Println("Firebase session cutoff confirmed; verify a retained token before restoring access")
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, apply func(context.Context, string, string) error) error {
	if err := cli.ParseNoArgs("revoke-sessions", usage, args, stdout, stderr); err != nil {
		return err
	}
	uid, project := os.Getenv("SAILOR_REVOKE_FIREBASE_UID"), os.Getenv("SAILOR_FIREBASE_PROJECT_ID")
	if strings.TrimSpace(uid) == "" || uid != strings.TrimSpace(uid) {
		return errors.New("SAILOR_REVOKE_FIREBASE_UID is required")
	}
	if strings.TrimSpace(project) == "" || project != strings.TrimSpace(project) {
		return errors.New("SAILOR_FIREBASE_PROJECT_ID is required")
	}
	return apply(ctx, project, uid)
}

func revoke(ctx context.Context, project, uid string) error {
	client, err := auth.NewFirebaseAuth(ctx, project)
	if err != nil {
		return err
	}
	return auth.RevokeSessions(ctx, client, uid)
}
