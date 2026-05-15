package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Yacht is the API representation of a yacht.
type Yacht struct {
	ID             int64     `json:"id"`
	OwnerID        int64     `json:"owner_id"`
	OrgID          *int64    `json:"org_id,omitempty"`
	Name           string    `json:"name"`
	RegistrationNo *string   `json:"registration_no,omitempty"`
	YachtType      *string   `json:"yacht_type,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// YachtBody is the create/update request payload for a yacht.
type YachtBody struct {
	Name           string  `json:"name" minLength:"1" doc:"Yacht name"`
	RegistrationNo *string `json:"registration_no,omitempty"`
	YachtType      *string `json:"yacht_type,omitempty"`
}

// YachtFromDB maps a database row to the API model.
func YachtFromDB(y sqlcdb.Yacht) Yacht {
	return Yacht{
		ID:             y.ID,
		OwnerID:        y.OwnerID,
		OrgID:          intPtr(y.OrgID),
		Name:           y.Name,
		RegistrationNo: strPtr(y.RegistrationNo),
		YachtType:      strPtr(y.YachtType),
		CreatedAt:      timeVal(y.CreatedAt),
		UpdatedAt:      timeVal(y.UpdatedAt),
	}
}

// YachtsFromDB maps a slice of database rows, returning a non-nil slice.
func YachtsFromDB(ys []sqlcdb.Yacht) []Yacht {
	out := make([]Yacht, len(ys))
	for i := range ys {
		out[i] = YachtFromDB(ys[i])
	}
	return out
}
