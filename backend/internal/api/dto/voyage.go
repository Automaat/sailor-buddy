package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Voyage is the API representation of a completed, logged sailing record.
type Voyage struct {
	ID            int64     `json:"id"`
	CreatedBy     *int64    `json:"created_by,omitempty"`
	CruiseID      *int64    `json:"cruise_id,omitempty"`
	Name          string    `json:"name"`
	Year          *int64    `json:"year,omitempty"`
	EmbarkDate    *string   `json:"embark_date,omitempty"`
	DisembarkDate *string   `json:"disembark_date,omitempty"`
	Countries     *string   `json:"countries,omitempty"`
	StartPort     *string   `json:"start_port,omitempty"`
	EndPort       *string   `json:"end_port,omitempty"`
	CaptainName   *string   `json:"captain_name,omitempty"`
	CaptainUserID *int64    `json:"captain_user_id,omitempty"`
	CaptainNotes  *string   `json:"captain_notes,omitempty"`
	YachtID       *int64    `json:"yacht_id,omitempty"`
	HoursTotal    float64   `json:"hours_total"`
	HoursSail     float64   `json:"hours_sail"`
	HoursEngine   float64   `json:"hours_engine"`
	HoursOver6bf  float64   `json:"hours_over_6bf"`
	Miles         float64   `json:"miles"`
	Days          int64     `json:"days"`
	TidalWaters   int64     `json:"tidal_waters"`
	CostTotal     *float64  `json:"cost_total,omitempty"`
	CostPerPerson *float64  `json:"cost_per_person,omitempty"`
	ImageLogoUrl  *string   `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string   `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string   `json:"image_route_url,omitempty"`
	Description   *string   `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// VoyageBody is the create/update request payload for a voyage.
type VoyageBody struct {
	Name          string   `json:"name" minLength:"1" doc:"Voyage name"`
	Year          *int64   `json:"year,omitempty"`
	EmbarkDate    *string  `json:"embark_date,omitempty"`
	DisembarkDate *string  `json:"disembark_date,omitempty"`
	Countries     *string  `json:"countries,omitempty"`
	StartPort     *string  `json:"start_port,omitempty"`
	EndPort       *string  `json:"end_port,omitempty"`
	CaptainName   *string  `json:"captain_name,omitempty" doc:"Free-text captain; replaced by the user's name when captain_user_id is set"`
	CaptainUserID *int64   `json:"captain_user_id,omitempty" doc:"User ID of the assigned captain (must be in the captain group)"`
	YachtID       *int64   `json:"yacht_id,omitempty"`
	HoursTotal    *float64 `json:"hours_total,omitempty"`
	HoursSail     *float64 `json:"hours_sail,omitempty"`
	HoursEngine   *float64 `json:"hours_engine,omitempty"`
	HoursOver6bf  *float64 `json:"hours_over_6bf,omitempty"`
	Miles         *float64 `json:"miles,omitempty"`
	Days          *int64   `json:"days,omitempty"`
	TidalWaters   *int64   `json:"tidal_waters,omitempty"`
	CostTotal     *float64 `json:"cost_total,omitempty"`
	CostPerPerson *float64 `json:"cost_per_person,omitempty"`
	ImageLogoUrl  *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl *string  `json:"image_route_url,omitempty"`
	Description   *string  `json:"description,omitempty"`
	CruiseID      *int64   `json:"cruise_id,omitempty"`
}

// VoyageLogBody is the post-cruise log a voyage's captain (or an admin)
// submits: sailing statistics and a free-form note. Ports are logged through
// the voyage ports endpoints.
type VoyageLogBody struct {
	HoursTotal   *float64 `json:"hours_total,omitempty" minimum:"0"`
	HoursSail    *float64 `json:"hours_sail,omitempty" minimum:"0"`
	HoursEngine  *float64 `json:"hours_engine,omitempty" minimum:"0"`
	HoursOver6bf *float64 `json:"hours_over_6bf,omitempty" minimum:"0"`
	Miles        *float64 `json:"miles,omitempty" minimum:"0"`
	Days         *int64   `json:"days,omitempty" minimum:"0"`
	TidalWaters  *int64   `json:"tidal_waters,omitempty" minimum:"0"`
	CaptainNotes *string  `json:"captain_notes,omitempty" doc:"Captain's note on the cruise"`
}

// VoyagesFromDB maps a slice of database rows, returning a non-nil slice.
func VoyagesFromDB(vs []sqlcdb.Voyage) []Voyage {
	out := make([]Voyage, len(vs))
	for i := range vs {
		out[i] = VoyageFromDB(vs[i])
	}
	return out
}

// VoyageFromDB maps a database row to the API model.
func VoyageFromDB(v sqlcdb.Voyage) Voyage {
	return Voyage{
		ID:            v.ID,
		CreatedBy:     intPtr(v.CreatedBy),
		CruiseID:      intPtr(v.CruiseID),
		Name:          v.Name,
		Year:          intPtr(v.Year),
		EmbarkDate:    strPtr(v.EmbarkDate),
		DisembarkDate: strPtr(v.DisembarkDate),
		Countries:     strPtr(v.Countries),
		StartPort:     strPtr(v.StartPort),
		EndPort:       strPtr(v.EndPort),
		CaptainName:   strPtr(v.CaptainName),
		CaptainUserID: intPtr(v.CaptainUserID),
		CaptainNotes:  strPtr(v.CaptainNotes),
		YachtID:       intPtr(v.YachtID),
		HoursTotal:    v.HoursTotal,
		HoursSail:     v.HoursSail,
		HoursEngine:   v.HoursEngine,
		HoursOver6bf:  v.HoursOver6bf,
		Miles:         v.Miles,
		Days:          v.Days,
		TidalWaters:   v.TidalWaters,
		CostTotal:     floatPtr(v.CostTotal),
		CostPerPerson: floatPtr(v.CostPerPerson),
		ImageLogoUrl:  strPtr(v.ImageLogoUrl),
		ImagePhotoUrl: strPtr(v.ImagePhotoUrl),
		ImageRouteUrl: strPtr(v.ImageRouteUrl),
		Description:   strPtr(v.Description),
		CreatedAt:     timeVal(v.CreatedAt),
		UpdatedAt:     timeVal(v.UpdatedAt),
	}
}
