package types

import (
	"database/sql/driver"

	"github.com/jackc/pgx/v5/pgtype"
)

// TextArray is a PostgreSQL text[] column as a string slice. database/sql
// hands array columns over in their text form, so Scan and Value convert
// through pgtype. A NULL column scans to an empty slice and an empty or nil
// slice stores as '{}', so callers never see a nil list.
type TextArray []string

// Scan implements sql.Scanner.
func (a *TextArray) Scan(value any) error {
	var s []string
	if err := pgtype.NewMap().SQLScanner(&s).Scan(value); err != nil {
		return err
	}
	if s == nil {
		s = []string{}
	}
	*a = s
	return nil
}

// Value implements driver.Valuer with the array's text form.
func (a TextArray) Value() (driver.Value, error) {
	s := []string(a)
	if s == nil {
		s = []string{}
	}
	buf, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, s, nil)
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}
