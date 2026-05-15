package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Trip is the API representation of a planned trip.
type Trip struct {
	ID            int64     `json:"id"`
	OwnerID       int64     `json:"owner_id"`
	OrgID         *int64    `json:"org_id,omitempty"`
	CruiseID      *int64    `json:"cruise_id,omitempty"`
	Name          string    `json:"name"`
	Status        string    `json:"status" enum:"planned,cancelled"`
	EmbarkDate    *string   `json:"embark_date,omitempty"`
	DisembarkDate *string   `json:"disembark_date,omitempty"`
	Countries     *string   `json:"countries,omitempty"`
	StartPort     *string   `json:"start_port,omitempty"`
	EndPort       *string   `json:"end_port,omitempty"`
	CaptainName   *string   `json:"captain_name,omitempty"`
	YachtID       *int64    `json:"yacht_id,omitempty"`
	CostTotal     *float64  `json:"cost_total,omitempty"`
	CostPerPerson *float64  `json:"cost_per_person,omitempty"`
	MaxCrew       *int64    `json:"max_crew,omitempty"`
	ImageLogoUrl  *string   `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string   `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string   `json:"image_route_url,omitempty"`
	Description   *string   `json:"description,omitempty"`
	EnrollToken   *string   `json:"enroll_token,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TripBody is the create/update request payload for a trip.
type TripBody struct {
	Name          string   `json:"name" minLength:"1" doc:"Trip name"`
	EmbarkDate    *string  `json:"embark_date,omitempty"`
	DisembarkDate *string  `json:"disembark_date,omitempty"`
	Countries     *string  `json:"countries,omitempty"`
	StartPort     *string  `json:"start_port,omitempty"`
	EndPort       *string  `json:"end_port,omitempty"`
	CaptainName   *string  `json:"captain_name,omitempty"`
	YachtID       *int64   `json:"yacht_id,omitempty"`
	CostTotal     *float64 `json:"cost_total,omitempty"`
	CostPerPerson *float64 `json:"cost_per_person,omitempty"`
	MaxCrew       *int64   `json:"max_crew,omitempty"`
	ImageLogoUrl  *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string  `json:"image_route_url,omitempty"`
	Description   *string  `json:"description,omitempty"`
	CruiseID      *int64   `json:"cruise_id,omitempty"`
}

// CompleteTripBody is the request payload for completing a trip into a voyage.
// All fields are optional; year falls back to the trip embark date.
type CompleteTripBody struct {
	Year         *int64   `json:"year,omitempty"`
	HoursTotal   *float64 `json:"hours_total,omitempty"`
	HoursSail    *float64 `json:"hours_sail,omitempty"`
	HoursEngine  *float64 `json:"hours_engine,omitempty"`
	HoursOver6bf *float64 `json:"hours_over_6bf,omitempty"`
	Miles        *float64 `json:"miles,omitempty"`
	Days         *int64   `json:"days,omitempty"`
	TidalWaters  *int64   `json:"tidal_waters,omitempty"`
}

// TripFromDB maps a database row to the API model.
func TripFromDB(t sqlcdb.Trip) Trip {
	return Trip{
		ID:            t.ID,
		OwnerID:       t.OwnerID,
		OrgID:         intPtr(t.OrgID),
		CruiseID:      intPtr(t.CruiseID),
		Name:          t.Name,
		Status:        string(t.Status),
		EmbarkDate:    strPtr(t.EmbarkDate),
		DisembarkDate: strPtr(t.DisembarkDate),
		Countries:     strPtr(t.Countries),
		StartPort:     strPtr(t.StartPort),
		EndPort:       strPtr(t.EndPort),
		CaptainName:   strPtr(t.CaptainName),
		YachtID:       intPtr(t.YachtID),
		CostTotal:     floatPtr(t.CostTotal),
		CostPerPerson: floatPtr(t.CostPerPerson),
		MaxCrew:       intPtr(t.MaxCrew),
		ImageLogoUrl:  strPtr(t.ImageLogoUrl),
		ImagePhotoUrl: strPtr(t.ImagePhotoUrl),
		ImageRouteUrl: strPtr(t.ImageRouteUrl),
		Description:   strPtr(t.Description),
		EnrollToken:   strPtr(t.EnrollToken),
		CreatedAt:     timeVal(t.CreatedAt),
		UpdatedAt:     timeVal(t.UpdatedAt),
	}
}

// TripsFromDB maps a slice of database rows, returning a non-nil slice so the
// JSON response serializes to [] rather than null when empty.
func TripsFromDB(ts []sqlcdb.Trip) []Trip {
	out := make([]Trip, len(ts))
	for i, t := range ts {
		out[i] = TripFromDB(t)
	}
	return out
}
