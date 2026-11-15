package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Cruise is the API representation of a club cruise event: an edition of a
// series, or the wrapper of a standalone trip. It holds what all its legs
// share; dates, ports, crew limits and prices live on each leg.
type Cruise struct {
	ID            int64      `json:"id"`
	CreatedBy     *int64     `json:"created_by,omitempty"`
	Name          string     `json:"name"`
	SeriesID      *int64     `json:"series_id,omitempty" doc:"Series this cruise is an edition of"`
	Edition       *string    `json:"edition,omitempty" doc:"Edition label within the series, e.g. 2025 or 2026/27"`
	Countries     *string    `json:"countries,omitempty"`
	Description   *string    `json:"description,omitempty"`
	ImageLogoUrl  *string    `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string    `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string    `json:"image_route_url,omitempty"`
	Included      []string   `json:"included" doc:"What every leg's price covers"`
	NotIncluded   []string   `json:"not_included" doc:"What no leg's price covers"`
	EnrollToken   *string    `json:"enroll_token,omitempty"`
	TravelNotes   *string    `json:"travel_notes,omitempty" doc:"How to get there: flights, transport, meeting point"`
	MinAge        *int64     `json:"min_age,omitempty" doc:"Minimum crew age, shown as information only"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty" doc:"When the cruise was published; absent while it is a hidden draft"`
}

// CruiseBody is the create/update request payload for a cruise.
type CruiseBody struct {
	Name          string   `json:"name" minLength:"1" doc:"Cruise name"`
	SeriesID      *int64   `json:"series_id,omitempty" doc:"Series this cruise is an edition of; omit or null for none"`
	Edition       *string  `json:"edition,omitempty" maxLength:"50" doc:"Edition label within the series, e.g. 2025 or 2026/27"`
	Countries     *string  `json:"countries,omitempty"`
	Description   *string  `json:"description,omitempty"`
	ImageLogoUrl  *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string  `json:"image_route_url,omitempty"`
	Included      []string `json:"included,omitempty" maxItems:"30" doc:"What every leg's price covers; blank entries are dropped, each at most 200 characters"`
	NotIncluded   []string `json:"not_included,omitempty" maxItems:"30" doc:"What no leg's price covers; blank entries are dropped, each at most 200 characters"`
	TravelNotes   *string  `json:"travel_notes,omitempty" maxLength:"5000" doc:"How to get there: flights, transport, meeting point; blank clears it"`
	MinAge        *int64   `json:"min_age,omitempty" minimum:"0" maximum:"99" doc:"Minimum crew age, shown as information only; the app does not check it"`
}

// CruiseFromDB maps a database row to the API model.
func CruiseFromDB(c sqlcdb.Cruise) Cruise {
	return Cruise{
		ID:            c.ID,
		CreatedBy:     intPtr(c.CreatedBy),
		Name:          c.Name,
		SeriesID:      intPtr(c.SeriesID),
		Edition:       strPtr(c.Edition),
		Countries:     strPtr(c.Countries),
		Description:   strPtr(c.Description),
		ImageLogoUrl:  strPtr(c.ImageLogoUrl),
		ImagePhotoUrl: strPtr(c.ImagePhotoUrl),
		ImageRouteUrl: strPtr(c.ImageRouteUrl),
		Included:      stringList(c.Included),
		NotIncluded:   stringList(c.NotIncluded),
		EnrollToken:   strPtr(c.EnrollToken),
		TravelNotes:   strPtr(c.TravelNotes),
		MinAge:        intPtr(c.MinAge),
		CreatedAt:     timeVal(c.CreatedAt),
		UpdatedAt:     timeVal(c.UpdatedAt),
		PublishedAt:   timePtr(c.PublishedAt),
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

// PublishCruiseBody is the optional request payload for publishing a cruise.
type PublishCruiseBody struct {
	IncludeLegs bool `json:"include_legs,omitempty" doc:"Also publish the cruise's hidden trips and voyages"`
}
