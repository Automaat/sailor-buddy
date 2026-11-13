package handlers

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// QueryError wraps a database operation failure with the operation name,
// enabling structured extraction via errors.As in logging middleware.
type QueryError struct {
	Op  string
	Err error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query %s failed: %v", e.Op, e.Err)
}

func (e *QueryError) Unwrap() error {
	return e.Err
}

// isForeignKeyViolation reports whether err is a PostgreSQL foreign key
// violation of the named constraint, e.g. a referenced row deleted between
// a check and the write.
func isForeignKeyViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "23503" && pgErr.ConstraintName == constraint
}

// isDeadlock reports whether err is a PostgreSQL deadlock abort (40P01): the
// transaction was chosen as the victim and can be retried.
func isDeadlock(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr != nil && pgErr.Code == "40P01"
}
