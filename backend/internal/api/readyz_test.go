package api

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
)

// stubConnector hands out connections whose Ping returns pingErr.
type stubConnector struct{ pingErr error }

func (c stubConnector) Connect(context.Context) (driver.Conn, error) {
	return stubConn(c), nil
}

func (stubConnector) Driver() driver.Driver { return nil }

type stubConn struct{ pingErr error }

func (c stubConn) Ping(context.Context) error { return c.pingErr }

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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := NewRouter(tc.db, &config.Config{}, nil)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", http.NoBody))
			if w.Code != tc.want {
				t.Errorf("GET /readyz = %d, want %d", w.Code, tc.want)
			}

			w = httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", http.NoBody))
			if w.Code != http.StatusOK {
				t.Errorf("GET /healthz = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}
