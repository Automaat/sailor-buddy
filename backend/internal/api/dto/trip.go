package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Trip is the API representation of a planned trip.
type Trip struct {
	ID                     int64     `json:"id"`
	CreatedBy              *int64    `json:"created_by,omitempty"`
	CruiseID               int64     `json:"cruise_id"`
	Name                   string    `json:"name"`
	Status                 string    `json:"status" enum:"planned,cancelled"`
	EmbarkDate             *string   `json:"embark_date,omitempty"`
	DisembarkDate          *string   `json:"disembark_date,omitempty"`
	Countries              *string   `json:"countries,omitempty"`
	StartPort              *string   `json:"start_port,omitempty"`
	EndPort                *string   `json:"end_port,omitempty"`
	CaptainName            *string   `json:"captain_name,omitempty"`
	CaptainPersonID        *int64    `json:"captain_person_id,omitempty" doc:"Person (crew record) who captains the trip"`
	YachtID                *int64    `json:"yacht_id,omitempty"`
	CostTotal              *float64  `json:"cost_total,omitempty"`
	CostPerPerson          *float64  `json:"cost_per_person,omitempty" doc:"Member price per person (czlonek, sympatyk)"`
	Currency               string    `json:"currency" enum:"PLN,EUR" doc:"Currency of every price on the trip"`
	CostPerPersonNonMember *float64  `json:"cost_per_person_non_member,omitempty" doc:"Non-member (gosc) price per person"`
	YachtFund              *float64  `json:"yacht_fund,omitempty" doc:"Estimated yacht fund (kasa jachtowa) per person: food, fuel, marinas"`
	Deposit                *float64  `json:"deposit,omitempty" doc:"Refundable deposit (kaucja) per person"`
	MaxCrew                *int64    `json:"max_crew,omitempty"`
	ImageLogoUrl           *string   `json:"image_logo_url,omitempty"`
	ImagePhotoUrl          *string   `json:"image_photo_url,omitempty"`
	ImageRouteUrl          *string   `json:"image_route_url,omitempty"`
	Description            *string   `json:"description,omitempty"`
	EnrollToken            *string   `json:"enroll_token,omitempty"`
	SignUpDeadline         *string   `json:"sign_up_deadline,omitempty" format:"date" doc:"Last day (Europe/Warsaw, inclusive) the leg takes sign-ups; omitted when none"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// TripBody is the create/update request payload for a trip.
type TripBody struct {
	Name                   string   `json:"name" minLength:"1" doc:"Trip name"`
	EmbarkDate             *string  `json:"embark_date,omitempty"`
	DisembarkDate          *string  `json:"disembark_date,omitempty"`
	Countries              *string  `json:"countries,omitempty"`
	StartPort              *string  `json:"start_port,omitempty"`
	EndPort                *string  `json:"end_port,omitempty"`
	CaptainName            *string  `json:"captain_name,omitempty" doc:"Free-text captain; replaced by the person's name when captain_person_id is set"`
	CaptainPersonID        *int64   `json:"captain_person_id,omitempty" doc:"Person (crew record) who captains the trip; any person may be chosen"`
	YachtID                *int64   `json:"yacht_id,omitempty"`
	CostTotal              *float64 `json:"cost_total,omitempty"`
	CostPerPerson          *float64 `json:"cost_per_person,omitempty" minimum:"0" doc:"Member price per person (czlonek, sympatyk)"`
	Currency               string   `json:"currency,omitempty" enum:"PLN,EUR" doc:"Currency of every price on the trip; PLN when omitted"`
	CostPerPersonNonMember *float64 `json:"cost_per_person_non_member,omitempty" minimum:"0" doc:"Non-member (gosc) price per person"`
	YachtFund              *float64 `json:"yacht_fund,omitempty" minimum:"0" doc:"Estimated yacht fund (kasa jachtowa) per person: food, fuel, marinas"`
	Deposit                *float64 `json:"deposit,omitempty" minimum:"0" doc:"Refundable deposit (kaucja) per person"`
	MaxCrew                *int64   `json:"max_crew,omitempty"`
	ImageLogoUrl           *string  `json:"image_logo_url,omitempty"`
	ImagePhotoUrl          *string  `json:"image_photo_url,omitempty"`
	ImageRouteUrl          *string  `json:"image_route_url,omitempty"`
	Description            *string  `json:"description,omitempty"`
	SignUpDeadline         *string  `json:"sign_up_deadline,omitempty" format:"date" doc:"Last day (YYYY-MM-DD, Europe/Warsaw, inclusive) the leg takes sign-ups; no later than embark_date; omit for no deadline"`
	CruiseID               int64    `json:"cruise_id" minimum:"1" doc:"Club event (cruise) this trip is a leg of"`
}

// CompleteTripBody is the request payload for completing a trip into a voyage.
// All fields are optional; year falls back to the trip embark date.
type CompleteTripBody struct {
	Year         *int64           `json:"year,omitempty"`
	HoursTotal   *float64         `json:"hours_total,omitempty"`
	HoursSail    *float64         `json:"hours_sail,omitempty"`
	HoursEngine  *float64         `json:"hours_engine,omitempty"`
	HoursOver6bf *float64         `json:"hours_over_6bf,omitempty"`
	Miles        *float64         `json:"miles,omitempty"`
	Days         *int64           `json:"days,omitempty"`
	TidalWaters  *int64           `json:"tidal_waters,omitempty"`
	Ports        []VoyagePortBody `json:"ports,omitempty" doc:"Ports visited during the trip"`
}

// TripFromDB maps a database row to the API model.
func TripFromDB(t sqlcdb.Trip) Trip {
	return Trip{
		ID:                     t.ID,
		CreatedBy:              intPtr(t.CreatedBy),
		CruiseID:               t.CruiseID,
		Name:                   t.Name,
		Status:                 string(t.Status),
		EmbarkDate:             strPtr(t.EmbarkDate),
		DisembarkDate:          strPtr(t.DisembarkDate),
		Countries:              strPtr(t.Countries),
		StartPort:              strPtr(t.StartPort),
		EndPort:                strPtr(t.EndPort),
		CaptainName:            strPtr(t.CaptainName),
		CaptainPersonID:        intPtr(t.CaptainPersonID),
		YachtID:                intPtr(t.YachtID),
		CostTotal:              floatPtr(t.CostTotal),
		CostPerPerson:          floatPtr(t.CostPerPerson),
		Currency:               t.Currency,
		CostPerPersonNonMember: floatPtr(t.CostPerPersonNonMember),
		YachtFund:              floatPtr(t.YachtFund),
		Deposit:                floatPtr(t.Deposit),
		MaxCrew:                intPtr(t.MaxCrew),
		ImageLogoUrl:           strPtr(t.ImageLogoUrl),
		ImagePhotoUrl:          strPtr(t.ImagePhotoUrl),
		ImageRouteUrl:          strPtr(t.ImageRouteUrl),
		Description:            strPtr(t.Description),
		EnrollToken:            strPtr(t.EnrollToken),
		SignUpDeadline:         strPtr(t.SignUpDeadline),
		CreatedAt:              timeVal(t.CreatedAt),
		UpdatedAt:              timeVal(t.UpdatedAt),
	}
}

// TripsFromDB maps a slice of database rows, returning a non-nil slice so the
// JSON response serializes to [] rather than null when empty.
func TripsFromDB(ts []sqlcdb.Trip) []Trip {
	out := make([]Trip, len(ts))
	for i := range ts {
		out[i] = TripFromDB(ts[i])
	}
	return out
}
