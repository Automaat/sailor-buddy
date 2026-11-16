package dto

import (
	"errors"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/location"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// VoyagePort is the API representation of a port visited during a voyage.
// A port recorded by name only has no coordinates and is not drawn on the map.
type VoyagePort struct {
	ID        int64     `json:"id"`
	VoyageID  int64     `json:"voyage_id"`
	Name      string    `json:"name"`
	Latitude  *float64  `json:"latitude,omitempty"`
	Longitude *float64  `json:"longitude,omitempty"`
	Anchorage bool      `json:"anchorage"`
	Position  int64     `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// VoyagePortBody is the create payload for a visited port. It also carries
// the ports supplied when a trip is completed into a voyage. Coordinates are
// optional but come as a pair.
type VoyagePortBody struct {
	Name      string   `json:"name" minLength:"1" doc:"Port / town name, at most 100 characters; surrounding blanks are dropped and inner runs of blanks become one space"`
	Latitude  *float64 `json:"latitude,omitempty" minimum:"-90" maximum:"90" doc:"Latitude; omit with longitude for a name-only port"`
	Longitude *float64 `json:"longitude,omitempty" minimum:"-180" maximum:"180" doc:"Longitude; omit with latitude for a name-only port"`
	Anchorage bool     `json:"anchorage,omitempty" doc:"An anchorage rather than a harbour"`
	Position  *int64   `json:"position,omitempty" minimum:"0" maximum:"10000" doc:"Order in the visited sequence, at most 10000 so the next append cannot overflow; omit to add the port after the last one"`
}

// Port applies the location rules to the port's name and coordinates.
func (b *VoyagePortBody) Port() (location.Port, error) {
	return location.NewPort(b.Name, b.Latitude, b.Longitude)
}

// Resolve rejects a port that Port rejects: a blank or too long name, or
// coordinates out of range or without their pair.
func (b *VoyagePortBody) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if _, err := b.Port(); err != nil {
		return []error{locationError(prefix, err, b.Name, b.Latitude, b.Longitude)}
	}
	return nil
}

// locationError reports a rejected port at the field the location rule
// names, with the value it rejected.
func locationError(prefix *huma.PathBuffer, err error, name string, lat, lon *float64) error {
	le, ok := errors.AsType[*location.Error](err)
	if !ok {
		return err
	}
	var value any = name
	switch le.Field {
	case location.FieldLatitude:
		value = lat
	case location.FieldLongitude:
		value = lon
	case location.FieldName:
	}
	return &huma.ErrorDetail{Message: le.Message, Location: prefix.With(string(le.Field)), Value: value}
}

// Port is a leg's start or end port: a name with optional coordinates. A
// port known by name only has none and is not drawn on a map.
type Port struct {
	Name      string   `json:"name" doc:"Port / town name, at most 100 characters; surrounding blanks are dropped and inner runs of blanks become one space. In a request, a blank name without coordinates means no port"`
	Latitude  *float64 `json:"latitude,omitempty" minimum:"-90" maximum:"90" doc:"Latitude; omit with longitude for a name-only port"`
	Longitude *float64 `json:"longitude,omitempty" minimum:"-180" maximum:"180" doc:"Longitude; omit with latitude for a name-only port"`
}

// Location applies the location rules to the port. ok is false for a blank
// name without coordinates, which means no port.
func (p *Port) Location() (port location.Port, ok bool, err error) {
	return location.ParseOptionalPort(p.Name, p.Latitude, p.Longitude)
}

// portErrors rejects a start or end port that Location rejects: a too long
// name, coordinates out of range or without their pair, or coordinates
// without a name, reported at the port's field under prefix. Each body
// checks its ports in its own Resolve: a resolver on Port would not run
// inside a body that embeds a struct with its own Resolve, such as
// TripFields.
func portErrors(prefix *huma.PathBuffer, field string, p *Port) []error {
	if p == nil {
		return nil
	}
	if _, _, err := p.Location(); err != nil {
		prefix.Push(field)
		defer prefix.Pop()
		return []error{locationError(prefix, err, p.Name, p.Latitude, p.Longitude)}
	}
	return nil
}

// portFromLocation maps a port to the API model.
func portFromLocation(p location.Port) Port {
	lat, lon := p.LatLon()
	return Port{Name: p.Name, Latitude: lat, Longitude: lon}
}

// portFromDB maps a stored start or end port, nil when the leg has none.
// Coordinates count only as a pair, as the table's checks store them.
func portFromDB(name types.NullString, lat, lon types.NullFloat64) *Port {
	if !name.Valid {
		return nil
	}
	p := location.Port{Name: name.String}
	if lat.Valid && lon.Valid {
		p.Coordinates = &location.Coordinates{Latitude: lat.Float64, Longitude: lon.Float64}
	}
	out := portFromLocation(p)
	return &out
}

// VoyagePortOrderBody carries port IDs in the desired visit order. The
// reorder endpoint rewrites each port's position to its index in this list.
type VoyagePortOrderBody struct {
	PortIDs []int64 `json:"port_ids" minItems:"1" doc:"Port IDs in the desired visit order"`
}

// VoyagePortFromDB maps a database row to the API model.
func VoyagePortFromDB(p sqlcdb.VoyagePort) VoyagePort {
	return VoyagePort{
		ID:        p.ID,
		VoyageID:  p.VoyageID,
		Name:      p.Name,
		Latitude:  floatPtr(p.Latitude),
		Longitude: floatPtr(p.Longitude),
		Anchorage: p.Anchorage,
		Position:  p.Position,
		CreatedAt: timeVal(p.CreatedAt),
	}
}

// VoyagePortsFromDB maps a slice of database rows, returning a non-nil slice so
// the JSON response serializes to [] rather than null when empty.
func VoyagePortsFromDB(ps []sqlcdb.VoyagePort) []VoyagePort {
	out := make([]VoyagePort, len(ps))
	for i := range ps {
		out[i] = VoyagePortFromDB(ps[i])
	}
	return out
}

// GeocodeResult is a single town/place match returned by the geocoding proxy.
// Name is the short port name stored on the voyage; Label is the
// administrative path (town, county, region, country) shown in the picker so
// that otherwise identically named matches can be told apart. Source names
// the OpenStreetMap search service that found it, for attribution.
type GeocodeResult struct {
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Source    string  `json:"source" enum:"nominatim,photon" doc:"Search service that found the place"`
}

// PortSuggestion is a port name the club already entered: a voyage's
// visited port, or a trip's or voyage's start or end port. Its coordinates
// come from the latest entry with this name that has them, from any of
// those sources; a name never entered with a location has none. Uses counts the entries with this
// name, ignoring case and accents.
type PortSuggestion struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Uses      int64    `json:"uses" doc:"How many visited, start and end ports carry this name"`
}

// PortSuggestionsFromDB maps suggestion rows, returning a non-nil slice so an
// empty answer serializes to [].
func PortSuggestionsFromDB(rows []sqlcdb.SuggestPortsRow) []PortSuggestion {
	out := make([]PortSuggestion, len(rows))
	for i, r := range rows {
		out[i] = PortSuggestion{
			Name:      r.Name,
			Latitude:  floatPtr(r.Latitude),
			Longitude: floatPtr(r.Longitude),
			Uses:      r.Uses,
		}
	}
	return out
}
