package handlers

import (
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/location"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func nullString(s *string) types.NullString {
	if s == nil {
		return types.NullString{}
	}
	return types.NullString{String: *s, Valid: true}
}

// portColumns is a leg's start or end port as its table stores it: a
// nullable name and nullable, paired coordinates.
type portColumns struct {
	name     types.NullString
	lat, lon types.NullFloat64
}

// legPort stores an optional start or end port by the location rules: an
// absent port, or a blank name without coordinates, is all NULL. The
// port's Resolve has already rejected one the rules refuse.
func legPort(p *dto.Port) portColumns {
	if p == nil {
		return portColumns{}
	}
	lp, ok, err := p.Location()
	if err != nil || !ok {
		return portColumns{}
	}
	lat, lon := portCoordinates(lp)
	return portColumns{name: types.NullString{String: lp.Name, Valid: true}, lat: lat, lon: lon}
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
