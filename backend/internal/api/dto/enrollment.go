package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Enrollment is a user's enrollment in a trip or cruise. Exactly one of
// trip_id / cruise_id identifies the target.
type Enrollment struct {
	ID       int64   `json:"id"`
	TripID   *int64  `json:"trip_id,omitempty"`
	CruiseID *int64  `json:"cruise_id,omitempty"`
	UserID   int64   `json:"user_id"`
	Note     *string `json:"note,omitempty"`
	Status   string  `json:"status"`
	// PaymentStatus is set on trip (leg) enrollments only; cruise-level
	// enrollments carry no payment status.
	PaymentStatus string    `json:"payment_status,omitempty" enum:"unpaid,deposit,paid" doc:"Manual payment status set by an admin: unpaid, deposit (first payment received) or paid (in full). Leg sign-ups only"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TripEnrollmentDetail is a trip enrollment joined with the enrolled user.
type TripEnrollmentDetail struct {
	Enrollment
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
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
			UserID:           r.UserID,
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
	Countries              *string  `json:"countries,omitempty"`
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

// EnrollCruise is the cruise summary returned when resolving an enrollment token.
type EnrollCruise struct {
	ID                     int64    `json:"id"`
	Name                   string   `json:"name"`
	EmbarkDate             *string  `json:"embark_date,omitempty"`
	DisembarkDate          *string  `json:"disembark_date,omitempty"`
	Countries              *string  `json:"countries,omitempty"`
	StartPort              *string  `json:"start_port,omitempty"`
	EndPort                *string  `json:"end_port,omitempty"`
	Description            *string  `json:"description,omitempty"`
	ImagePhotoUrl          *string  `json:"image_photo_url,omitempty"`
	MaxCrew                *int64   `json:"max_crew,omitempty"`
	CostPerPerson          *float64 `json:"cost_per_person,omitempty" doc:"Member price per person (czlonek, sympatyk)"`
	Currency               string   `json:"currency" enum:"PLN,EUR"`
	CostPerPersonNonMember *float64 `json:"cost_per_person_non_member,omitempty" doc:"Non-member (gosc) price per person"`
	YachtFund              *float64 `json:"yacht_fund,omitempty" doc:"Estimated yacht fund (kasa jachtowa) per person"`
	Deposit                *float64 `json:"deposit,omitempty" doc:"Refundable deposit (kaucja) per person"`
	Included               []string `json:"included" doc:"What the price covers"`
	NotIncluded            []string `json:"not_included" doc:"What the price does not cover"`
	TravelNotes            *string  `json:"travel_notes,omitempty" doc:"How to get there: flights, transport, meeting point"`
	MinAge                 *int64   `json:"min_age,omitempty" doc:"Minimum crew age, shown as information only"`
	// GroupChatUrl is returned only to a logged-in caller; the enroll preview
	// is public and the chat link is for members.
	GroupChatUrl *string `json:"group_chat_url,omitempty" doc:"Link to the crew group chat; omitted for anonymous callers"`
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

// EnrollLegResult is the outcome of one requested leg: a new pending
// enrollment, or the user's existing enrollment for that leg.
type EnrollLegResult struct {
	TripID     int64      `json:"trip_id"`
	Result     string     `json:"result" enum:"created,already_enrolled"`
	Enrollment Enrollment `json:"enrollment"`
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

// TripEnrollmentToDTO maps a trip enrollment row to the unified model.
func TripEnrollmentToDTO(e sqlcdb.TripEnrollment) Enrollment {
	tid := e.TripID
	return Enrollment{
		ID:            e.ID,
		TripID:        &tid,
		UserID:        e.UserID,
		Note:          strPtr(e.Note),
		Status:        e.Status,
		PaymentStatus: e.PaymentStatus,
		CreatedAt:     timeVal(e.CreatedAt),
		UpdatedAt:     timeVal(e.UpdatedAt),
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

// TripEnrollmentsToDTO maps plain trip enrollment rows.
func TripEnrollmentsToDTO(rows []sqlcdb.TripEnrollment) []Enrollment {
	out := make([]Enrollment, len(rows))
	for i := range rows {
		out[i] = TripEnrollmentToDTO(rows[i])
	}
	return out
}

// TripEnrollmentsFromDB maps the joined trip-enrollment rows.
func TripEnrollmentsFromDB(rows []sqlcdb.ListTripEnrollmentsRow) []TripEnrollmentDetail {
	out := make([]TripEnrollmentDetail, len(rows))
	for i := range rows {
		r := rows[i]
		tid := r.TripID
		out[i] = TripEnrollmentDetail{
			ID:            r.ID,
			TripID:        &tid,
			UserID:        r.UserID,
			Note:          strPtr(r.Note),
			Status:        r.Status,
			PaymentStatus: r.PaymentStatus,
			CreatedAt:     timeVal(r.CreatedAt),
			UpdatedAt:     timeVal(r.UpdatedAt),
			UserName:      r.UserName,
			UserEmail:     r.UserEmail,
		}
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
		Countries:              strPtr(t.Countries),
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

// EnrollCruiseFromRow maps the token-resolved cruise row. The group chat link
// is included only for a logged-in caller (member).
func EnrollCruiseFromRow(c sqlcdb.GetCruiseByEnrollTokenRow, member bool) EnrollCruise {
	out := EnrollCruise{
		ID:                     c.ID,
		Name:                   c.Name,
		EmbarkDate:             strPtr(c.EmbarkDate),
		DisembarkDate:          strPtr(c.DisembarkDate),
		Countries:              strPtr(c.Countries),
		StartPort:              strPtr(c.StartPort),
		EndPort:                strPtr(c.EndPort),
		Description:            strPtr(c.Description),
		ImagePhotoUrl:          strPtr(c.ImagePhotoUrl),
		MaxCrew:                intPtr(c.MaxCrew),
		CostPerPerson:          floatPtr(c.CostPerPerson),
		Currency:               c.Currency,
		CostPerPersonNonMember: floatPtr(c.CostPerPersonNonMember),
		YachtFund:              floatPtr(c.YachtFund),
		Deposit:                floatPtr(c.Deposit),
		Included:               priceList(c.Included),
		NotIncluded:            priceList(c.NotIncluded),
		TravelNotes:            strPtr(c.TravelNotes),
		MinAge:                 intPtr(c.MinAge),
	}
	if member {
		out.GroupChatUrl = strPtr(c.GroupChatUrl)
	}
	return out
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
			Countries:              strPtr(t.Countries),
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
}

// SignUpNoteBody sets the note on the caller's own leg sign-up.
type SignUpNoteBody struct {
	Note *string `json:"note,omitempty" maxLength:"2000" doc:"Note to the organizer; omit or send an empty string to clear it"`
}

// CruiseTripEnrollmentsFromDB maps every leg enrollment of a cruise.
func CruiseTripEnrollmentsFromDB(rows []sqlcdb.ListCruiseTripEnrollmentsRow) []TripEnrollmentDetail {
	out := make([]TripEnrollmentDetail, len(rows))
	for i := range rows {
		r := rows[i]
		tid := r.TripID
		out[i] = TripEnrollmentDetail{
			ID:            r.ID,
			TripID:        &tid,
			UserID:        r.UserID,
			Note:          strPtr(r.Note),
			Status:        r.Status,
			PaymentStatus: r.PaymentStatus,
			CreatedAt:     timeVal(r.CreatedAt),
			UpdatedAt:     timeVal(r.UpdatedAt),
			UserName:      r.UserName,
			UserEmail:     r.UserEmail,
		}
	}
	return out
}
