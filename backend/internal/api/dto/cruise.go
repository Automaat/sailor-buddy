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
	Kind          string    `json:"kind" enum:"sea,inland" doc:"Cruise kind: sea (morski) or inland (śródlądowy)"`
	SeriesID      *int64    `json:"series_id,omitempty" doc:"Series this cruise is an edition of"`
	Edition       *string   `json:"edition,omitempty" doc:"Edition label within the series, e.g. 2025 or 2026/27"`
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
	GroupChatUrl  *string   `json:"group_chat_url,omitempty" doc:"Link to the crew group chat (http or https)"`
	TravelNotes   *string   `json:"travel_notes,omitempty" doc:"How to get there: flights, transport, meeting point"`
	MinAge        *int64    `json:"min_age,omitempty" doc:"Minimum crew age, shown as information only"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CruiseBody is the create/update request payload for a cruise.
type CruiseBody struct {
	Name          string   `json:"name" minLength:"1" doc:"Cruise name"`
	Kind          string   `json:"kind,omitempty" enum:"sea,inland" default:"sea" doc:"Cruise kind: sea (morski) or inland (śródlądowy)"`
	SeriesID      *int64   `json:"series_id,omitempty" doc:"Series this cruise is an edition of; omit or null for none"`
	Edition       *string  `json:"edition,omitempty" maxLength:"50" doc:"Edition label within the series, e.g. 2025 or 2026/27"`
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
	GroupChatUrl  *string  `json:"group_chat_url,omitempty" maxLength:"2000" doc:"Link to the crew group chat; must be an http or https URL, blank clears it"`
	TravelNotes   *string  `json:"travel_notes,omitempty" maxLength:"5000" doc:"How to get there: flights, transport, meeting point; blank clears it"`
	MinAge        *int64   `json:"min_age,omitempty" minimum:"0" maximum:"99" doc:"Minimum crew age, shown as information only; the app does not check it"`
}

// CruiseFromDB maps a database row to the API model.
func CruiseFromDB(c sqlcdb.Cruise) Cruise {
	return Cruise{
		ID:            c.ID,
		CreatedBy:     intPtr(c.CreatedBy),
		Name:          c.Name,
		Kind:          c.Kind,
		SeriesID:      intPtr(c.SeriesID),
		Edition:       strPtr(c.Edition),
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
		GroupChatUrl:  strPtr(c.GroupChatUrl),
		TravelNotes:   strPtr(c.TravelNotes),
		MinAge:        intPtr(c.MinAge),
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
