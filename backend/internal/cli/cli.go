// Package cli parses the command line of the database commands (migrate,
// seed-dev), which take no arguments besides -h.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// ErrUsage marks invalid command-line arguments. ParseNoArgs has already
// printed the reason and the usage text to stderr.
var ErrUsage = errors.New("invalid arguments")

// ParseNoArgs parses args for a command named name that accepts only -h and
// -help. Help prints usage to stdout and returns flag.ErrHelp. An unknown
// flag or any positional argument prints the reason and usage to stderr and
// returns ErrUsage. A nil result means the command may run.
func ParseNoArgs(name, usage string, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usage)
			return err
		}
		_, _ = io.WriteString(stderr, usage)
		return ErrUsage
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "unexpected argument %q\n%s", fs.Arg(0), usage)
		return ErrUsage
	}
	return nil
}
