package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestDiagnosticsKeepErrorClassWithoutPersonalOrSecretText(t *testing.T) {
	const secret = "secret-token-password@example.com"
	var output bytes.Buffer
	logger := New(&output)
	logger.Error("provision account", "email", secret, "uid", secret, "name", secret, "path", secret, "query", secret, "authorization", secret, "password", secret, "token", secret, "body", secret, "err", fmt.Errorf("request %s: %w", secret, &pgconn.PgError{Code: "23505", Message: secret, Detail: secret}), "user_id", int64(42))
	if strings.Contains(output.String(), secret) {
		t.Fatalf("secret in log: %s", output.String())
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["error_kind"] != "postgres_23505" || record["user_id"] != float64(42) {
		t.Fatalf("record=%v", record)
	}
	logger.LogAttrs(t.Context(), slog.LevelError, "provider request", slog.Any("err", context.DeadlineExceeded))
	if !strings.Contains(output.String(), "deadline_exceeded") {
		t.Fatal("missing safe timeout category")
	}
}
