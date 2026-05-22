package handlers

import (
	"context"
	"database/sql"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// txRunner runs fn inside a database transaction, passing it a Querier bound
// to that transaction. It lets write paths stay testable with a fake querier
// while running atomically in production.
type txRunner func(ctx context.Context, fn func(sqlcdb.Querier) error) error

// sqlTxRunner returns a txRunner backed by a real database connection.
func sqlTxRunner(db *sql.DB) txRunner {
	return func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return &QueryError{Op: "BeginTx", Err: err}
		}
		defer func() { _ = tx.Rollback() }()
		if err := fn(sqlcdb.New(tx)); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return &QueryError{Op: "Commit", Err: err}
		}
		return nil
	}
}
