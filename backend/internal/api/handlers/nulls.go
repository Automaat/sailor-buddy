package handlers

import (
	"github.com/marcinskalski/sailor-buddy/backend/internal/location"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func nullString(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	return types.NullString{String: *s, Valid: true}
}

// nullPortName stores an optional port name, such as a leg's start or end
// port, by the location name rule: a blank name is NULL. The DTO's Resolve
// has already rejected a name that is too long.
func nullPortName(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	n := location.NormalizeName(*s)
	return types.NullString{String: n, Valid: n != ""}
}

// portCoordinates returns a port's coordinates as nullable columns, both
// NULL for a port known by name only.
func portCoordinates(p location.Port) (lat, lon types.NullFloat64) {
	la, lo := p.LatLon()
	return nullFloat64(la), nullFloat64(lo)
}

func nullInt64(i *int64) types.NullInt64 {
	if i == nil {
		return types.NullInt64{}
	}
	return types.NullInt64{Int64: *i, Valid: true}
}

func nullFloat64(f *float64) types.NullFloat64 {
	if f == nil {
		return types.NullFloat64{}
	}
	return types.NullFloat64{Float64: *f, Valid: true}
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
