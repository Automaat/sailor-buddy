package dto

import (
	"strings"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Enrollment is an enrollment in a trip or cruise. Exactly one of
// trip_id / cruise_id identifies the target. A trip (leg) sign-up belongs
// to a person (crew_member_id), who may have no account; a cruise
// enrollment belongs to an account (user_id).
type Enrollment struct {
	ID           int64   `json:"id"`
	TripID       *int64  `json:"trip_id,omitempty"`
	CruiseID     *int64  `json:"cruise_id,omitempty"`
	CrewMemberID *int64  `json:"crew_member_id,omitempty" doc:"Person the leg sign-up belongs to. Leg sign-ups only"`
	UserID       *int64  `json:"user_id,omitempty" doc:"Account of the enrollment: always set on a cruise enrollment; on a leg sign-up, the person's account, omitted when the person has none"`
	Note         *string `json:"note,omitempty"`
	Status       string  `json:"status"`
	// PaymentStatus is set on trip (leg) enrollments only; cruise-level
	// enrollments carry no payment status.
	PaymentStatus string    `json:"payment_status,omitempty" enum:"unpaid,deposit,paid" doc:"Manual payment status set by an admin: unpaid, deposit (first payment received) or paid (in full). Leg sign-ups only"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TripEnrollmentDetail is a trip enrollment joined with its person.
type TripEnrollmentDetail struct {
	Enrollment
	PersonName string  `json:"person_name" doc:"Full name of the person"`
	Email      *string `json:"email,omitempty" doc:"The person's e-mail, or the account's when the person has none; omitted when neither has one"`
}

// TripEnrollmentCreateBody is an admin's sign-up of a person for a leg.
type TripEnrollmentCreateBody struct {
	CrewMemberID int64   `json:"crew_member_id" minimum:"1" doc:"Person to sign up; may have no account"`
	Note         *string `json:"note,omitempty" maxLength:"2000" doc:"Optional note on the sign-up"`
}

// CruiseEnrollmentDetail is a cruise enrollment joined with the enrolled user
// and, when assigned, the trip it was placed on.
type CruiseEnrollmentDetail struct {
	Enrollment
	UserName         string  `json:"user_name"`
	UserEmail        string  `json:"user_email"`
	UserPhone        *string `json:"user_phone,omitempty"`
	UserPatentType   *string `json:"user_patent_type,omitempty"`
	UserPatentNumber *string `json:"user_patent_number,omitempty"`
	TripName         *string `json:"trip_name,omitempty"`
}

// CruiseEnrollmentsFromDB maps the joined cruise-enrollment rows.
func CruiseEnrollmentsFromDB(rows []sqlcdb.ListCruiseEnrollmentsRow) []CruiseEnrollmentDetail {
	out := make([]CruiseEnrollmentDetail, len(rows))
	for i := range rows {
		r := rows[i]
		cid := r.CruiseID
		out[i] = CruiseEnrollmentDetail{
			ID:               r.ID,
			CruiseID:         &cid,
			TripID:           intPtr(r.TripID),
			UserID:           &r.UserID,
			Note:             strPtr(r.Note),
			Status:           r.Status,
			CreatedAt:        timeVal(r.CreatedAt),
			UpdatedAt:        timeVal(r.UpdatedAt),
			UserName:         r.UserName,
			UserEmail:        r.UserEmail,
			UserPhone:        strPtr(r.UserPhone),
			UserPatentType:   strPtr(r.UserPatentType),
			UserPatentNumber: strPtr(r.UserPatentNumber),
			TripName:         strPtr(r.TripName),
		}
	}
	return out
}

// EnrollTrip is the trip summary returned when resolving an enrollment token.
type EnrollTrip struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	EmbarkDate             *string  `json:"embark_date,omitempty"`
	DisembarkDate          *string  `json:"disembark_date,omitempty"`
	Countries              []string `json:"countries" doc:"ISO 3166-1 alpha-2 country codes in display order; empty when none"`
	StartPort              *string  `json:"start_port,omitempty"`
	EndPort                *string  `json:"end_port,omitempty"`
	Description            *string  `json:"description,omitempty"`
	MaxCrew                *int64   `json:"max_crew,omitempty"`
	CaptainName            *string  `json:"captain_name,omitempty"`
	ImagePhotoUrl          *string  `json:"image_photo_url,omitempty"`
	CostPerPerson          *float64 `json:"cost_per_person,omitempty" doc:"Member price per person (czlonek, sympatyk)"`
	Currency               string   `json:"currency" enum:"PLN,EUR"`
	CostPerPersonNonMember *float64 `json:"cost_per_person_non_member,omitempty" doc:"Non-member (gosc) price per person"`
	YachtFund              *float64 `json:"yacht_fund,omitempty" doc:"Estimated yacht fund (kasa jachtowa) per person"`
	Deposit                *float64 `json:"deposit,omitempty" doc:"Refundable deposit (kaucja) per person"`
	SignUpDeadline         *string  `json:"sign_up_deadline,omitempty" format:"date" doc:"Last day (Europe/Warsaw, inclusive) the leg takes sign-ups; omitted when none"`
}

// EnrollLeg is the public summary of one cruise trip (leg) on the enroll
// page. It omits the trip's own enroll token, creator and total cost: the
// cruise link is shared publicly and must not reveal per-leg links.
type EnrollLeg struct {
	EnrollTrip
	Status string `json:"status" enum:"planned,cancelled"`
}

// EnrollCruise is the cruise summary returned when resolving an enrollment
// token. Dates, ports, crew limits and prices are on each leg (trips).
type EnrollCruise struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Countries     []string `json:"countries" doc:"ISO 3166-1 alpha-2 country codes in display order; empty when none"`
	Description   *string  `json:"description,omitempty"`
	ImagePhotoUrl *string  `json:"image_photo_url,omitempty"`
	Included      []string `json:"included" doc:"What every leg's price covers"`
	NotIncluded   []string `json:"not_included" doc:"What no leg's price covers"`
	TravelNotes   *string  `json:"travel_notes,omitempty" doc:"How to get there: flights, transport, meeting point"`
	MinAge        *int64   `json:"min_age,omitempty" doc:"Minimum crew age, shown as information only"`
}

// EnrollInfo is the resolved target of an enrollment share token. kind selects
// which of trip / cruise is populated.
type EnrollInfo struct {
	Kind          string        `json:"kind" enum:"trip,cruise"`
	Trip          *EnrollTrip   `json:"trip,omitempty"`
	Cruise        *EnrollCruise `json:"cruise,omitempty"`
	Trips         []EnrollLeg   `json:"trips,omitempty"`
	Enrolled      bool          `json:"enrolled"`
	Enrollment    *Enrollment   `json:"enrollment,omitempty"`
	AcceptedCount int64         `json:"accepted_count"`
	TotalCount    int64         `json:"total_count"`
	// LegEnrollments lists the caller's per-leg (trip) enrollments for a
	// cruise, in leg order. Empty for trip tokens.
	LegEnrollments []Enrollment `json:"leg_enrollments,omitempty"`
	// TripAccepted lists accepted crew per trip: the token's trip, or each
	// cruise trip. Free places on the enroll page derive from it.
	TripAccepted []TripAccepted `json:"trip_accepted,omitempty"`
}

// TripAccepted is the accepted crew count of one cruise trip: direct trip
// enrollments plus cruise enrollments assigned to it.
type TripAccepted struct {
	TripID        int64 `json:"trip_id"`
	AcceptedCount int64 `json:"accepted_count"`
}

// TripAcceptedFromDB maps the per-trip accepted counts.
func TripAcceptedFromDB(rows []sqlcdb.CountAcceptedByCruiseTripRow) []TripAccepted {
	out := make([]TripAccepted, len(rows))
	for i, r := range rows {
		out[i] = TripAccepted{TripID: r.TripID, AcceptedCount: r.Accepted}
	}
	return out
}

// EnrollBody is the request payload for self-enrolling via a share token.
type EnrollBody struct {
	Note *string `json:"note,omitempty" doc:"Optional note to the organizer"`
}

// EnrollLegsBody is the request payload for signing up for one or more legs
// (trips) of a cruise in one submission.
type EnrollLegsBody struct {
	TripIDs []int64 `json:"trip_ids" nullable:"false" minItems:"1" maxItems:"100" uniqueItems:"true" doc:"IDs of the cruise's trips (legs) to enroll in"`
	Note    *string `json:"note,omitempty" doc:"Optional note to the organizer, copied to every leg enrollment"`
}

// EnrollLegResult is the outcome of one requested leg: a new pending or
// waitlisted enrollment, or the user's existing enrollment for that leg.
type EnrollLegResult struct {
	TripID     int64      `json:"trip_id"`
	Result     string     `json:"result" enum:"created,already_enrolled"`
	Enrollment Enrollment `json:"enrollment"`
	// WaitlistPosition is set when the enrollment is on the leg's waiting list.
	WaitlistPosition *int64 `json:"waitlist_position,omitempty" doc:"1-based waiting-list position by sign-up time; omitted unless the enrollment is waitlisted"`
}

// EnrollLegsResponse lists one result per requested leg, in request order.
type EnrollLegsResponse struct {
	Results []EnrollLegResult `json:"results"`
}

// EnrollmentStatusBody updates an enrollment's review status.
type EnrollmentStatusBody struct {
	Status string `json:"status" enum:"accepted,rejected,waitlisted,pending" doc:"Enrollment status"`
}

// PaymentStatusBody sets a leg sign-up's manual payment status.
type PaymentStatusBody struct {
	PaymentStatus string `json:"payment_status" enum:"unpaid,deposit,paid" doc:"unpaid, deposit (first payment received) or paid (in full)"`
}

// AssignTripBody assigns a cruise enrollment to a trip; a null trip_id unassigns.
type AssignTripBody struct {
	TripID *int64 `json:"trip_id,omitempty" doc:"Trip ID, or null to unassign"`
}

// TripEnrollmentToDTO maps a trip enrollment row to the unified model. The
// row does not say which account the person has, so user_id is left out.
func TripEnrollmentToDTO(e sqlcdb.TripEnrollment) Enrollment {
	tid := e.TripID
	pid := e.CrewMemberID
	return Enrollment{
		ID:            e.ID,
		TripID:        &tid,
		CrewMemberID:  &pid,
		Note:          strPtr(e.Note),
		Status:        e.Status,
		PaymentStatus: e.PaymentStatus,
		CreatedAt:     timeVal(e.CreatedAt),
		UpdatedAt:     timeVal(e.UpdatedAt),
	}
}

// UserTripEnrollmentToDTO maps a sign-up of userID's person.
func UserTripEnrollmentToDTO(e sqlcdb.TripEnrollment, userID int64) Enrollment {
	out := TripEnrollmentToDTO(e)
	out.UserID = &userID
	return out
}

// CruiseEnrollmentToDTO maps a cruise enrollment row to the unified model.
func CruiseEnrollmentToDTO(e sqlcdb.CruiseEnrollment) Enrollment {
	cid := e.CruiseID
	uid := e.UserID
	return Enrollment{
		ID:        e.ID,
		CruiseID:  &cid,
		TripID:    intPtr(e.TripID),
		UserID:    &uid,
		Note:      strPtr(e.Note),
		Status:    e.Status,
		CreatedAt: timeVal(e.CreatedAt),
		UpdatedAt: timeVal(e.UpdatedAt),
	}
}

// UserTripEnrollmentsToDTO maps sign-ups of userID's person.
func UserTripEnrollmentsToDTO(rows []sqlcdb.TripEnrollment, userID int64) []Enrollment {
	out := make([]Enrollment, len(rows))
	for i := range rows {
		out[i] = UserTripEnrollmentToDTO(rows[i], userID)
	}
	return out
}

// TripEnrollmentDetailFromDB maps one joined trip-enrollment row. The
// e-mail is the person's, or the account's when the person has none.
func TripEnrollmentDetailFromDB(r sqlcdb.ListTripEnrollmentsRow) TripEnrollmentDetail {
	tid := r.TripID
	pid := r.CrewMemberID
	email := strPtr(r.PersonEmail)
	if email == nil || strings.TrimSpace(*email) == "" {
		email = strPtr(r.AccountEmail)
	}
	return TripEnrollmentDetail{
		ID:            r.ID,
		TripID:        &tid,
		CrewMemberID:  &pid,
		UserID:        intPtr(r.UserID),
		Note:          strPtr(r.Note),
		Status:        r.Status,
		PaymentStatus: r.PaymentStatus,
		CreatedAt:     timeVal(r.CreatedAt),
		UpdatedAt:     timeVal(r.UpdatedAt),
		PersonName:    r.PersonName,
		Email:         email,
	}
}

// TripEnrollmentsFromDB maps the joined trip-enrollment rows.
func TripEnrollmentsFromDB(rows []sqlcdb.ListTripEnrollmentsRow) []TripEnrollmentDetail {
	out := make([]TripEnrollmentDetail, len(rows))
	for i := range rows {
		out[i] = TripEnrollmentDetailFromDB(rows[i])
	}
	return out
}

// EnrollTripFromRow maps the token-resolved trip row.
func EnrollTripFromRow(t sqlcdb.GetTripByEnrollTokenRow) EnrollTrip {
	return EnrollTrip{
		ID:                     t.ID,
		Name:                   t.Name,
		EmbarkDate:             strPtr(t.EmbarkDate),
		DisembarkDate:          strPtr(t.DisembarkDate),
		Countries:              countryCodes(t.Countries),
		StartPort:              strPtr(t.StartPort),
		EndPort:                strPtr(t.EndPort),
		Description:            strPtr(t.Description),
		MaxCrew:                intPtr(t.MaxCrew),
		CaptainName:            strPtr(t.CaptainName),
		ImagePhotoUrl:          strPtr(t.ImagePhotoUrl),
		CostPerPerson:          floatPtr(t.CostPerPerson),
		Currency:               t.Currency,
		CostPerPersonNonMember: floatPtr(t.CostPerPersonNonMember),
		YachtFund:              floatPtr(t.YachtFund),
		Deposit:                floatPtr(t.Deposit),
		SignUpDeadline:         strPtr(t.SignUpDeadline),
	}
}

// EnrollCruiseFromRow maps the token-resolved cruise row.
func EnrollCruiseFromRow(c sqlcdb.GetCruiseByEnrollTokenRow) EnrollCruise {
	return EnrollCruise{
		ID:            c.ID,
		Name:          c.Name,
		Countries:     countryCodes(c.Countries),
		Description:   strPtr(c.Description),
		ImagePhotoUrl: strPtr(c.ImagePhotoUrl),
		Included:      stringList(c.Included),
		NotIncluded:   stringList(c.NotIncluded),
		TravelNotes:   strPtr(c.TravelNotes),
		MinAge:        intPtr(c.MinAge),
	}
}

// EnrollLegsFromDB maps a cruise's trips to their public leg summaries.
func EnrollLegsFromDB(ts []sqlcdb.Trip) []EnrollLeg {
	out := make([]EnrollLeg, len(ts))
	for i := range ts {
		t := ts[i]
		out[i] = EnrollLeg{
			ID:                     t.ID,
			Name:                   t.Name,
			Status:                 string(t.Status),
			EmbarkDate:             strPtr(t.EmbarkDate),
			DisembarkDate:          strPtr(t.DisembarkDate),
			Countries:              countryCodes(t.Countries),
			StartPort:              strPtr(t.StartPort),
			EndPort:                strPtr(t.EndPort),
			Description:            strPtr(t.Description),
			MaxCrew:                intPtr(t.MaxCrew),
			CaptainName:            strPtr(t.CaptainName),
			ImagePhotoUrl:          strPtr(t.ImagePhotoUrl),
			CostPerPerson:          floatPtr(t.CostPerPerson),
			Currency:               t.Currency,
			CostPerPersonNonMember: floatPtr(t.CostPerPersonNonMember),
			YachtFund:              floatPtr(t.YachtFund),
			Deposit:                floatPtr(t.Deposit),
			SignUpDeadline:         strPtr(t.SignUpDeadline),
		}
	}
	return out
}

// LegSignUp is a leg (trip) as the signed-in caller sees it for sign-up:
// whether it takes new sign-ups, how many crew are accepted, and the caller's
// own sign-up, if any.
type LegSignUp struct {
	TripID        int64       `json:"trip_id"`
	Open          bool        `json:"open" doc:"Whether the leg takes new sign-ups: planned, with sign-ups switched on (an enroll token) and its sign-up deadline not passed"`
	AcceptedCount int64       `json:"accepted_count" doc:"Accepted crew: direct leg sign-ups plus cruise enrollments assigned to the leg"`
	Enrollment    *Enrollment `json:"enrollment,omitempty" doc:"The caller's sign-up for this leg; omitted when not signed up"`
	// WaitlistPosition is set when the caller's sign-up is on the waiting list.
	WaitlistPosition *int64 `json:"waitlist_position,omitempty" doc:"1-based position of the caller's sign-up on the leg's waiting list, by sign-up time; omitted unless it is waitlisted"`
}

// SignUpNoteBody sets the note on the caller's own leg sign-up.
type SignUpNoteBody struct {
	Note *string `json:"note,omitempty" maxLength:"2000" doc:"Note to the organizer; omit or send an empty string to clear it"`
}

// CruiseTripEnrollmentsFromDB maps every leg enrollment of a cruise.
func CruiseTripEnrollmentsFromDB(rows []sqlcdb.ListCruiseTripEnrollmentsRow) []TripEnrollmentDetail {
	out := make([]TripEnrollmentDetail, len(rows))
	for i := range rows {
		out[i] = TripEnrollmentDetailFromDB(sqlcdb.ListTripEnrollmentsRow(rows[i]))
	}
	return out
}
