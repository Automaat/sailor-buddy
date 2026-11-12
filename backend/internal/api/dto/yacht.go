package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Yacht is the API representation of a yacht.
type Yacht struct {
	ID             int64     `json:"id"`
	CreatedBy      *int64    `json:"created_by,omitempty"`
	Name           string    `json:"name"`
	RegistrationNo *string   `json:"registration_no,omitempty"`
	YachtType      *string   `json:"yacht_type,omitempty"`
	Berths         *int64    `json:"berths,omitempty"`
	LengthM        *float64  `json:"length_m,omitempty"`
	BuildYear      *int64    `json:"build_year,omitempty"`
	Engine         *string   `json:"engine,omitempty"`
	OwnerName      *string   `json:"owner_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// YachtBody is the create/update request payload for a yacht.
type YachtBody struct {
	Name           string  `json:"name" minLength:"1" doc:"Yacht name"`
	RegistrationNo *string `json:"registration_no,omitempty"`
	YachtType      *string `json:"yacht_type,omitempty"`
	// Berths is the bunk count; it becomes a new trip's default crew limit.
	Berths    *int64   `json:"berths,omitempty" minimum:"1" doc:"Number of berths"`
	LengthM   *float64 `json:"length_m,omitempty" exclusiveMinimum:"0" doc:"Length overall in metres"`
	BuildYear *int64   `json:"build_year,omitempty" minimum:"1800" maximum:"2100" doc:"Build year"`
	Engine    *string  `json:"engine,omitempty" doc:"Engine, e.g. Volvo Penta D2-55"`
	OwnerName *string  `json:"owner_name,omitempty" doc:"Charter owner (armator)"`
}

// YachtFromDB maps a database row to the API model.
func YachtFromDB(y sqlcdb.Yacht) Yacht {
	return Yacht{
		ID:             y.ID,
		CreatedBy:      intPtr(y.CreatedBy),
		Name:           y.Name,
		RegistrationNo: strPtr(y.RegistrationNo),
		YachtType:      strPtr(y.YachtType),
		Berths:         intPtr(y.Berths),
		LengthM:        floatPtr(y.LengthM),
		BuildYear:      intPtr(y.BuildYear),
		Engine:         strPtr(y.Engine),
		OwnerName:      strPtr(y.OwnerName),
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
