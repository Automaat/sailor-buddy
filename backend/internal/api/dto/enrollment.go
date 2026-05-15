package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Enrollment is a user's enrollment in a trip or cruise. Exactly one of
// trip_id / cruise_id identifies the target.
type Enrollment struct {
	ID        int64     `json:"id"`
	TripID    *int64    `json:"trip_id,omitempty"`
	CruiseID  *int64    `json:"cruise_id,omitempty"`
	UserID    int64     `json:"user_id"`
	Note      *string   `json:"note,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TripEnrollmentDetail is a trip enrollment joined with the enrolled user.
type TripEnrollmentDetail struct {
	Enrollment
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

// EnrollTrip is the trip summary returned when resolving an enrollment token.
type EnrollTrip struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	EmbarkDate    *string `json:"embark_date,omitempty"`
	DisembarkDate *string `json:"disembark_date,omitempty"`
	Countries     *string `json:"countries,omitempty"`
	StartPort     *string `json:"start_port,omitempty"`
	EndPort       *string `json:"end_port,omitempty"`
	Description   *string `json:"description,omitempty"`
	MaxCrew       *int64  `json:"max_crew,omitempty"`
	CaptainName   *string `json:"captain_name,omitempty"`
	ImagePhotoUrl *string `json:"image_photo_url,omitempty"`
}

// EnrollCruise is the cruise summary returned when resolving an enrollment token.
type EnrollCruise struct {
	ID            int64    `json:"id"`
	OrgID         int64    `json:"org_id"`
	Name          string   `json:"name"`
	EmbarkDate    *string  `json:"embark_date,omitempty"`
	DisembarkDate *string  `json:"disembark_date,omitempty"`
	Countries     *string  `json:"countries,omitempty"`
	StartPort     *string  `json:"start_port,omitempty"`
	EndPort       *string  `json:"end_port,omitempty"`
	Description   *string  `json:"description,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	MaxCrew       *int64   `json:"max_crew,omitempty"`
	CostPerPerson *float64 `json:"cost_per_person,omitempty"`
}

// EnrollInfo is the resolved target of an enrollment share token. kind selects
// which of trip / cruise is populated.
type EnrollInfo struct {
	Kind          string        `json:"kind" enum:"trip,cruise"`
	Trip          *EnrollTrip   `json:"trip,omitempty"`
	Cruise        *EnrollCruise `json:"cruise,omitempty"`
	Trips         []Trip        `json:"trips,omitempty"`
	Enrolled      bool          `json:"enrolled"`
	Enrollment    *Enrollment   `json:"enrollment,omitempty"`
	AcceptedCount int64         `json:"accepted_count"`
	TotalCount    int64         `json:"total_count"`
}

// EnrollBody is the request payload for self-enrolling via a share token.
type EnrollBody struct {
	Note *string `json:"note,omitempty" doc:"Optional note to the organizer"`
}

// EnrollmentStatusBody updates an enrollment's review status.
type EnrollmentStatusBody struct {
	Status string `json:"status" enum:"accepted,rejected,waitlisted,pending" doc:"Enrollment status"`
}

// TripEnrollmentToDTO maps a trip enrollment row to the unified model.
func TripEnrollmentToDTO(e sqlcdb.TripEnrollment) Enrollment {
	tid := e.TripID
	return Enrollment{
		ID:        e.ID,
		TripID:    &tid,
		UserID:    e.UserID,
		Note:      strPtr(e.Note),
		Status:    e.Status,
		CreatedAt: timeVal(e.CreatedAt),
		UpdatedAt: timeVal(e.UpdatedAt),
	}
}

// CruiseEnrollmentToDTO maps a cruise enrollment row to the unified model.
func CruiseEnrollmentToDTO(e sqlcdb.CruiseEnrollment) Enrollment {
	cid := e.CruiseID
	return Enrollment{
		ID:        e.ID,
		CruiseID:  &cid,
		TripID:    intPtr(e.TripID),
		UserID:    e.UserID,
		Note:      strPtr(e.Note),
		Status:    e.Status,
		CreatedAt: timeVal(e.CreatedAt),
		UpdatedAt: timeVal(e.UpdatedAt),
	}
}

// TripEnrollmentsFromDB maps the joined trip-enrollment rows.
func TripEnrollmentsFromDB(rows []sqlcdb.ListTripEnrollmentsRow) []TripEnrollmentDetail {
	out := make([]TripEnrollmentDetail, len(rows))
	for i := range rows {
		r := rows[i]
		tid := r.TripID
		out[i] = TripEnrollmentDetail{
			Enrollment: Enrollment{
				ID:        r.ID,
				TripID:    &tid,
				UserID:    r.UserID,
				Note:      strPtr(r.Note),
				Status:    r.Status,
				CreatedAt: timeVal(r.CreatedAt),
				UpdatedAt: timeVal(r.UpdatedAt),
			},
			UserName:  r.UserName,
			UserEmail: r.UserEmail,
		}
	}
	return out
}

// EnrollTripFromRow maps the token-resolved trip row.
func EnrollTripFromRow(t sqlcdb.GetTripByEnrollTokenRow) EnrollTrip {
	return EnrollTrip{
		ID:            t.ID,
		Name:          t.Name,
		EmbarkDate:    strPtr(t.EmbarkDate),
		DisembarkDate: strPtr(t.DisembarkDate),
		Countries:     strPtr(t.Countries),
		StartPort:     strPtr(t.StartPort),
		EndPort:       strPtr(t.EndPort),
		Description:   strPtr(t.Description),
		MaxCrew:       intPtr(t.MaxCrew),
		CaptainName:   strPtr(t.CaptainName),
		ImagePhotoUrl: strPtr(t.ImagePhotoUrl),
	}
}

// EnrollCruiseFromRow maps the token-resolved cruise row.
func EnrollCruiseFromRow(c sqlcdb.GetCruiseByEnrollTokenRow) EnrollCruise {
	return EnrollCruise{
		ID:            c.ID,
		OrgID:         c.OrgID,
		Name:          c.Name,
		EmbarkDate:    strPtr(c.EmbarkDate),
		DisembarkDate: strPtr(c.DisembarkDate),
		Countries:     strPtr(c.Countries),
		StartPort:     strPtr(c.StartPort),
		EndPort:       strPtr(c.EndPort),
		Description:   strPtr(c.Description),
		ImagePhotoUrl: strPtr(c.ImagePhotoUrl),
		MaxCrew:       intPtr(c.MaxCrew),
		CostPerPerson: floatPtr(c.CostPerPerson),
	}
}
