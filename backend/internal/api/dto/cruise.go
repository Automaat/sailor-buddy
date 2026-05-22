package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Cruise is the API representation of a club cruise event.
type Cruise struct {
	ID            int64     `json:"id"`
	CreatedBy     *int64    `json:"created_by,omitempty"`
	Name          string    `json:"name"`
	EmbarkDate    *string   `json:"embark_date,omitempty"`
	DisembarkDate *string   `json:"disembark_date,omitempty"`
	Countries     *string   `json:"countries,omitempty"`
	StartPort     *string   `json:"start_port,omitempty"`
	EndPort       *string   `json:"end_port,omitempty"`
	Description   *string   `json:"description,omitempty"`
	ImageLogoUrl  *string   `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string   `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string   `json:"image_route_url,omitempty"`
	MaxCrew       *int64    `json:"max_crew,omitempty"`
	CostPerPerson *float64  `json:"cost_per_person,omitempty"`
	EnrollToken   *string   `json:"enroll_token,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CruiseBody is the create/update request payload for a cruise.
type CruiseBody struct {
	Name          string   `json:"name" minLength:"1" doc:"Cruise name"`
	EmbarkDate    *string  `json:"embark_date,omitempty"`
	DisembarkDate *string  `json:"disembark_date,omitempty"`
	Countries     *string  `json:"countries,omitempty"`
	StartPort     *string  `json:"start_port,omitempty"`
	EndPort       *string  `json:"end_port,omitempty"`
	Description   *string  `json:"description,omitempty"`
	ImageLogoUrl  *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string  `json:"image_route_url,omitempty"`
	MaxCrew       *int64   `json:"max_crew,omitempty"`
	CostPerPerson *float64 `json:"cost_per_person,omitempty"`
}

// CruiseFromDB maps a database row to the API model.
func CruiseFromDB(c sqlcdb.Cruise) Cruise {
	return Cruise{
		ID:            c.ID,
		CreatedBy:     intPtr(c.CreatedBy),
		Name:          c.Name,
		EmbarkDate:    strPtr(c.EmbarkDate),
		DisembarkDate: strPtr(c.DisembarkDate),
		Countries:     strPtr(c.Countries),
		StartPort:     strPtr(c.StartPort),
		EndPort:       strPtr(c.EndPort),
		Description:   strPtr(c.Description),
		ImageLogoUrl:  strPtr(c.ImageLogoUrl),
		ImagePhotoUrl: strPtr(c.ImagePhotoUrl),
		ImageRouteUrl: strPtr(c.ImageRouteUrl),
		MaxCrew:       intPtr(c.MaxCrew),
		CostPerPerson: floatPtr(c.CostPerPerson),
		EnrollToken:   strPtr(c.EnrollToken),
		CreatedAt:     timeVal(c.CreatedAt),
		UpdatedAt:     timeVal(c.UpdatedAt),
	}
}

// CruisesFromDB maps a slice of database rows, returning a non-nil slice.
func CruisesFromDB(cs []sqlcdb.Cruise) []Cruise {
	out := make([]Cruise, len(cs))
	for i := range cs {
		out[i] = CruiseFromDB(cs[i])
	}
	return out
}
