package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Voyage is the API representation of a completed, logged sailing record.
type Voyage struct {
	ID            int64     `json:"id"`
	OwnerID       int64     `json:"owner_id"`
	OrgID         *int64    `json:"org_id,omitempty"`
	CruiseID      *int64    `json:"cruise_id,omitempty"`
	Name          string    `json:"name"`
	Year          *int64    `json:"year,omitempty"`
	EmbarkDate    *string   `json:"embark_date,omitempty"`
	DisembarkDate *string   `json:"disembark_date,omitempty"`
	Countries     *string   `json:"countries,omitempty"`
	StartPort     *string   `json:"start_port,omitempty"`
	EndPort       *string   `json:"end_port,omitempty"`
	CaptainName   *string   `json:"captain_name,omitempty"`
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

// VoyageFromDB maps a database row to the API model.
func VoyageFromDB(v sqlcdb.Voyage) Voyage {
	return Voyage{
		ID:            v.ID,
		OwnerID:       v.OwnerID,
		OrgID:         intPtr(v.OrgID),
		CruiseID:      intPtr(v.CruiseID),
		Name:          v.Name,
		Year:          intPtr(v.Year),
		EmbarkDate:    strPtr(v.EmbarkDate),
		DisembarkDate: strPtr(v.DisembarkDate),
		Countries:     strPtr(v.Countries),
		StartPort:     strPtr(v.StartPort),
		EndPort:       strPtr(v.EndPort),
		CaptainName:   strPtr(v.CaptainName),
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
