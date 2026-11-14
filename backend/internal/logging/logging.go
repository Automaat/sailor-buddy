// Package logging emits diagnostics without request credentials or personal text.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// New builds the JSON logger used by the API and its audit stream.
func New(writer io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{ReplaceAttr: protectAttribute}))
}

func protectAttribute(_ []string, attr slog.Attr) slog.Attr {
	switch strings.ToLower(attr.Key) {
	case "email", "uid", "name", "query", "path", "authorization", "password", "token", "body":
		return slog.Attr{}
	case "err":
		err, _ := attr.Value.Any().(error)
		return slog.String("error_kind", errorKind(err))
	default:
		return attr
	}
}

func errorKind(err error) string {
	if err == nil {
		return "redacted"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if pg, ok := errors.AsType[*pgconn.PgError](err); ok && pg != nil && len(pg.Code) == 5 && strings.IndexFunc(pg.Code, func(r rune) bool { return (r < '0' || r > '9') && (r < 'A' || r > 'Z') }) == -1 {
		return "postgres_" + pg.Code
	}
	return fmt.Sprintf("%T", err)
}
