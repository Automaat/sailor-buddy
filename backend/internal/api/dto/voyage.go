package dto

import (
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Voyage is the API representation of a completed, logged sailing record.
type Voyage struct {
	ID              int64     `json:"id"`
	CreatedBy       *int64    `json:"created_by,omitempty"`
	CruiseID        int64     `json:"cruise_id"`
	Name            string    `json:"name"`
	Year            *int64    `json:"year,omitempty"`
	EmbarkDate      *string   `json:"embark_date,omitempty"`
	DisembarkDate   *string   `json:"disembark_date,omitempty"`
	Countries       *string   `json:"countries,omitempty"`
	StartPort       *string   `json:"start_port,omitempty"`
	EndPort         *string   `json:"end_port,omitempty"`
	CaptainName     *string   `json:"captain_name,omitempty"`
	CaptainPersonID *int64    `json:"captain_person_id,omitempty" doc:"Person (crew record) who captains the voyage"`
	CaptainNotes    *string   `json:"captain_notes,omitempty"`
	YachtID         *int64    `json:"yacht_id,omitempty"`
	HoursTotal      float64   `json:"hours_total"`
	HoursSail       float64   `json:"hours_sail"`
	HoursEngine     float64   `json:"hours_engine"`
	HoursOver6bf    float64   `json:"hours_over_6bf"`
	Miles           float64   `json:"miles"`
	Days            int64     `json:"days"`
	TidalWaters     int64     `json:"tidal_waters"`
	CostTotal       *float64  `json:"cost_total,omitempty"`
	CostPerPerson   *float64  `json:"cost_per_person,omitempty"`
	ImageLogoUrl    *string   `json:"image_logo_url,omitempty"`
	ImagePhotoUrl   *string   `json:"image_photo_url,omitempty"`
	ImageRouteUrl   *string   `json:"image_route_url,omitempty"`
	Description     *string   `json:"description,omitempty"`
	TrackUrl        *string   `json:"track_url,omitempty"`
	VoyageCard      *string   `json:"voyage_card,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// VoyageBody is the create/update request payload for a voyage.
type VoyageBody struct {
	Name            string   `json:"name" minLength:"1" doc:"Voyage name"`
	Year            *int64   `json:"year,omitempty"`
	EmbarkDate      *string  `json:"embark_date,omitempty"`
	DisembarkDate   *string  `json:"disembark_date,omitempty"`
	Countries       *string  `json:"countries,omitempty"`
	StartPort       *string  `json:"start_port,omitempty"`
	EndPort         *string  `json:"end_port,omitempty"`
	CaptainName     *string  `json:"captain_name,omitempty" doc:"Free-text captain; replaced by the person's name when captain_person_id is set"`
	CaptainPersonID *int64   `json:"captain_person_id,omitempty" doc:"Person (crew record) who captains the voyage; any person may be chosen"`
	YachtID         *int64   `json:"yacht_id,omitempty"`
	HoursTotal      *float64 `json:"hours_total,omitempty"`
	HoursSail       *float64 `json:"hours_sail,omitempty"`
	HoursEngine     *float64 `json:"hours_engine,omitempty"`
	HoursOver6bf    *float64 `json:"hours_over_6bf,omitempty"`
	Miles           *float64 `json:"miles,omitempty"`
	Days            *int64   `json:"days,omitempty"`
	TidalWaters     *int64   `json:"tidal_waters,omitempty"`
	CostTotal       *float64 `json:"cost_total,omitempty"`
	CostPerPerson   *float64 `json:"cost_per_person,omitempty"`
	ImageLogoUrl    *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl   *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl   *string  `json:"image_route_url,omitempty"`
	Description     *string  `json:"description,omitempty"`
	TrackUrl        *string  `json:"track_url,omitempty" maxLength:"2048" doc:"Link to the track recording (http or https)"`
	VoyageCard      *string  `json:"voyage_card,omitempty" doc:"Voyage card (Karta rejsu): who holds it or where it is"`
	CruiseID        int64    `json:"cruise_id" minimum:"1" doc:"Club event (cruise) this voyage was a leg of"`
}

// Resolve rejects a track link that is not an absolute http(s) URL.
func (b *VoyageBody) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return trackURLErrors(b.TrackUrl, prefix)
}

// VoyageLogBody is the post-cruise log a voyage's captain (or an admin)
// submits: sailing statistics, the yacht, the track link, the voyage card
// and a free-form note. Every field replaces the stored value. Ports are
// logged through the voyage ports endpoints.
type VoyageLogBody struct {
	HoursTotal   *float64 `json:"hours_total,omitempty" minimum:"0"`
	HoursSail    *float64 `json:"hours_sail,omitempty" minimum:"0"`
	HoursEngine  *float64 `json:"hours_engine,omitempty" minimum:"0"`
	HoursOver6bf *float64 `json:"hours_over_6bf,omitempty" minimum:"0"`
	Miles        *float64 `json:"miles,omitempty" minimum:"0"`
	Days         *int64   `json:"days,omitempty" minimum:"0"`
	TidalWaters  *int64   `json:"tidal_waters,omitempty" minimum:"0"`
	CaptainNotes *string  `json:"captain_notes,omitempty" doc:"Captain's note on the cruise"`
	YachtID      *int64   `json:"yacht_id,omitempty" doc:"Yacht sailed; omit to clear"`
	TrackUrl     *string  `json:"track_url,omitempty" maxLength:"2048" doc:"Link to the track recording (http or https)"`
	VoyageCard   *string  `json:"voyage_card,omitempty" doc:"Voyage card (Karta rejsu): who holds it or where it is"`
}

// Resolve rejects a track link that is not an absolute http(s) URL.
func (b *VoyageLogBody) Resolve(_ huma.Context, prefix *huma.PathBuffer) []error {
	return trackURLErrors(b.TrackUrl, prefix)
}

// trackURLErrors accepts an absent or empty link, or an absolute http(s)
// URL with a host. Anything else (javascript:, relative paths) is rejected
// because the link is rendered as an anchor on the voyage page.
func trackURLErrors(raw *string, prefix *huma.PathBuffer) []error {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(*raw))
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return nil
	}
	return []error{&huma.ErrorDetail{
		Message:  "track_url must be an http or https link",
		Location: prefix.With("track_url"),
		Value:    *raw,
	}}
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
		ID:              v.ID,
		CreatedBy:       intPtr(v.CreatedBy),
		CruiseID:        v.CruiseID,
		Name:            v.Name,
		Year:            intPtr(v.Year),
		EmbarkDate:      strPtr(v.EmbarkDate),
		DisembarkDate:   strPtr(v.DisembarkDate),
		Countries:       strPtr(v.Countries),
		StartPort:       strPtr(v.StartPort),
		EndPort:         strPtr(v.EndPort),
		CaptainName:     strPtr(v.CaptainName),
		CaptainPersonID: intPtr(v.CaptainPersonID),
		CaptainNotes:    strPtr(v.CaptainNotes),
		YachtID:         intPtr(v.YachtID),
		HoursTotal:      v.HoursTotal,
		HoursSail:       v.HoursSail,
		HoursEngine:     v.HoursEngine,
		HoursOver6bf:    v.HoursOver6bf,
		Miles:           v.Miles,
		Days:            v.Days,
		TidalWaters:     v.TidalWaters,
		CostTotal:       floatPtr(v.CostTotal),
		CostPerPerson:   floatPtr(v.CostPerPerson),
		ImageLogoUrl:    strPtr(v.ImageLogoUrl),
		ImagePhotoUrl:   strPtr(v.ImagePhotoUrl),
		ImageRouteUrl:   strPtr(v.ImageRouteUrl),
		Description:     strPtr(v.Description),
		TrackUrl:        strPtr(v.TrackUrl),
		VoyageCard:      strPtr(v.VoyageCard),
		CreatedAt:       timeVal(v.CreatedAt),
		UpdatedAt:       timeVal(v.UpdatedAt),
	}
}
