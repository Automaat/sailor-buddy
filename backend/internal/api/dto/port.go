package dto

import (
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
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
	Name      string   `json:"name" minLength:"1" doc:"Port / town name"`
	Latitude  *float64 `json:"latitude,omitempty" minimum:"-90" maximum:"90" doc:"Latitude; omit with longitude for a name-only port"`
	Longitude *float64 `json:"longitude,omitempty" minimum:"-180" maximum:"180" doc:"Longitude; omit with latitude for a name-only port"`
	Anchorage bool     `json:"anchorage,omitempty" doc:"An anchorage rather than a harbour"`
	Position  *int64   `json:"position,omitempty" minimum:"0" doc:"Order in the visited sequence"`
}

// Resolve rejects a port with only one of its two coordinates.
func (b *VoyagePortBody) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	if (b.Latitude == nil) == (b.Longitude == nil) {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  "latitude and longitude must be given together",
		Location: prefix.With("latitude"),
		Value:    b.Latitude,
	}}
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
// Name is the short port name stored on the voyage; Label is the full
// administrative path (town, region, country) shown in the picker so that
// otherwise identically named matches can be told apart.
type GeocodeResult struct {
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
