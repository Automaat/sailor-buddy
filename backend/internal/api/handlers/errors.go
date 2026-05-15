package handlers

import "fmt"

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
