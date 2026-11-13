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

// deadlockAttempts bounds how often retryDeadlock and onDeadlockRetry run
// their work.
const deadlockAttempts = 3

// retryDeadlock wraps run so a transaction PostgreSQL aborted as a deadlock
// victim (40P01) runs again, up to deadlockAttempts times in all. The
// rollback drops every lock, so each attempt takes them afresh.
func retryDeadlock(run txRunner) txRunner {
	return func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		return onDeadlockRetry(func() error { return run(ctx, fn) })
	}
}

// onDeadlockRetry runs fn again while it fails as a deadlock victim (40P01),
// up to deadlockAttempts times in all. fn must be a whole transaction or a
// single autocommit statement, so a failed attempt leaves nothing behind.
func onDeadlockRetry(fn func() error) error {
	var err error
	for range deadlockAttempts {
		err = fn()
		if !isDeadlock(err) {
			return err
		}
	}
	return err
}
