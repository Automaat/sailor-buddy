package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

// stubConnector hands out connections whose Ping returns pingErr, or blocks
// until its context ends when hang is set.
type stubConnector struct {
	pingErr error
	hang    bool
}

func (c stubConnector) Connect(context.Context) (driver.Conn, error) {
	return stubConn(c), nil
}

func (stubConnector) Driver() driver.Driver { return nil }

type stubConn struct {
	pingErr error
	hang    bool
}

func (c stubConn) Ping(ctx context.Context) error {
	if c.hang {
		<-ctx.Done()
		return ctx.Err()
	}
	return c.pingErr
}

func (stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (stubConn) Close() error              { return nil }
func (stubConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

// TestReadyz checks that /readyz tracks database reachability while /healthz
// stays a pure liveness probe.
func TestReadyz(t *testing.T) {
	t.Parallel()

	closed := sql.OpenDB(stubConnector{})
	if err := closed.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	tests := []struct {
		name string
		db   *sql.DB
		want int
	}{
		{"database reachable", sql.OpenDB(stubConnector{}), http.StatusOK},
		{"ping fails", sql.OpenDB(stubConnector{pingErr: errors.New("connection refused")}), http.StatusServiceUnavailable},
		{"database closed", closed, http.StatusServiceUnavailable},
		{"no database", nil, http.StatusServiceUnavailable},
		{"ping hangs", sql.OpenDB(stubConnector{hang: true}), http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := NewRouter(tc.db, &config.Config{}, nil, nil)

			w := httptest.NewRecorder()
			start := time.Now()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
			if w.Code != tc.want {
				t.Errorf("GET /readyz = %d, want %d", w.Code, tc.want)
			}
			if elapsed := time.Since(start); elapsed > readyTimeout+time.Second {
				t.Errorf("GET /readyz took %v, want at most %v", elapsed, readyTimeout+time.Second)
			}

			w = httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
			if w.Code != http.StatusOK {
				t.Errorf("GET /healthz = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}
