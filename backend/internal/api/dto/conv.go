// Package dto holds the API wire models. They are kept separate from the
// sqlc-generated database row structs so the HTTP contract can evolve
// independently of the schema. huma reflects these structs into the OpenAPI
// spec, which in turn drives the generated frontend types.
package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// strPtr converts a nullable DB string into an optional API field.
func strPtr(n types.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

// intPtr converts a nullable DB int into an optional API field.
func intPtr(n types.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	i := n.Int64
	return &i
}

// floatPtr converts a nullable DB float into an optional API field.
func floatPtr(n types.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	f := n.Float64
	return &f
}

// timeVal returns the time value, or the zero time when the column is null.
func timeVal(n types.NullTime) time.Time {
	return n.Time
}
