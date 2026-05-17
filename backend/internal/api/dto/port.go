package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// VoyagePort is the API representation of a port visited during a voyage.
type VoyagePort struct {
	ID        int64     `json:"id"`
	VoyageID  int64     `json:"voyage_id"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Position  int64     `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// VoyagePortBody is the create payload for a visited port. It also carries
// the ports supplied when a trip is completed into a voyage.
type VoyagePortBody struct {
	Name      string  `json:"name" minLength:"1" doc:"Port / town name"`
	Latitude  float64 `json:"latitude" minimum:"-90" maximum:"90" doc:"Latitude"`
	Longitude float64 `json:"longitude" minimum:"-180" maximum:"180" doc:"Longitude"`
	Position  *int64  `json:"position,omitempty" minimum:"0" doc:"Order in the visited sequence"`
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
		Latitude:  p.Latitude,
		Longitude: p.Longitude,
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
