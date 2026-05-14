package handlers

import "database/sql"

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullInt64(i *int64) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *i, Valid: true}
}

func nullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func valOrZeroFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func valOrZeroInt(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}
