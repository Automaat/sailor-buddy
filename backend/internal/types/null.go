// Package types provides nullable column types that marshal to plain JSON
// values (or null) instead of the {"Valid":...,...} shape that the
// database/sql Null* types produce. They are wired into sqlc via type
// overrides in sqlc.yaml, so generated structs use them directly.
package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// NullString is a nullable string. Field layout matches sql.NullString so
// struct literals and field access stay source-compatible.
type NullString struct {
	String string
	Valid  bool
}

// Scan implements sql.Scanner.
func (n *NullString) Scan(value any) error {
	var s sql.NullString
	if err := s.Scan(value); err != nil {
		return err
	}
	n.String, n.Valid = s.String, s.Valid
	return nil
}

// Value implements driver.Valuer.
func (n NullString) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.String, nil
}

// MarshalJSON emits the string value, or null when invalid.
func (n NullString) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.String)
}

// UnmarshalJSON accepts a JSON string or null.
func (n *NullString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.String, n.Valid = "", false
		return nil
	}
	if err := json.Unmarshal(b, &n.String); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// NullInt64 is a nullable int64.
type NullInt64 struct {
	Int64 int64
	Valid bool
}

// Scan implements sql.Scanner.
func (n *NullInt64) Scan(value any) error {
	var i sql.NullInt64
	if err := i.Scan(value); err != nil {
		return err
	}
	n.Int64, n.Valid = i.Int64, i.Valid
	return nil
}

// Value implements driver.Valuer.
func (n NullInt64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Int64, nil
}

// MarshalJSON emits the int64 value, or null when invalid.
func (n NullInt64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Int64)
}

// UnmarshalJSON accepts a JSON number or null.
func (n *NullInt64) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.Int64, n.Valid = 0, false
		return nil
	}
	if err := json.Unmarshal(b, &n.Int64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// NullFloat64 is a nullable float64.
type NullFloat64 struct {
	Float64 float64
	Valid   bool
}

// Scan implements sql.Scanner.
func (n *NullFloat64) Scan(value any) error {
	var f sql.NullFloat64
	if err := f.Scan(value); err != nil {
		return err
	}
	n.Float64, n.Valid = f.Float64, f.Valid
	return nil
}

// Value implements driver.Valuer.
func (n NullFloat64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Float64, nil
}

// MarshalJSON emits the float64 value, or null when invalid.
func (n NullFloat64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Float64)
}

// UnmarshalJSON accepts a JSON number or null.
func (n *NullFloat64) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.Float64, n.Valid = 0, false
		return nil
	}
	if err := json.Unmarshal(b, &n.Float64); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// NullTime is a nullable time.Time.
type NullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner.
func (n *NullTime) Scan(value any) error {
	var t sql.NullTime
	if err := t.Scan(value); err != nil {
		return err
	}
	n.Time, n.Valid = t.Time, t.Valid
	return nil
}

// Value implements driver.Valuer.
func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

// MarshalJSON emits the time value, or null when invalid.
func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.Time)
}

// UnmarshalJSON accepts a JSON time string or null.
func (n *NullTime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	if err := json.Unmarshal(b, &n.Time); err != nil {
		return err
	}
	n.Valid = true
	return nil
}
