package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// MaxOtherNames caps how many other names a person carries.
const MaxOtherNames = 20

// MaxOtherNameLen caps the length of one other name, in characters.
const MaxOtherNameLen = 200

// CrewMember is the API representation of a person (crew record). A person
// linked to an account carries its user ID; every account has exactly one.
type CrewMember struct {
	ID                    int64     `json:"id"`
	CreatedBy             *int64    `json:"created_by,omitempty"`
	UserID                *int64    `json:"user_id,omitempty"`
	HasAccount            bool      `json:"has_account" doc:"Whether the person is linked to an account"`
	FullName              string    `json:"full_name"`
	OtherNames            []string  `json:"other_names" doc:"Other spellings and maiden names that search matches"`
	Email                 *string   `json:"email,omitempty"`
	PatentNumber          *string   `json:"patent_number,omitempty"`
	Phone                 *string   `json:"phone,omitempty"`
	PzzLicenseType        *string   `json:"pzz_license_type,omitempty"`
	PzzLicenseNumber      *string   `json:"pzz_license_number,omitempty"`
	EmergencyContactName  *string   `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string   `json:"emergency_contact_phone,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// CrewMemberBody is the create/update payload for a crew member, covering the
// PZŻ licence and emergency-contact fields.
type CrewMemberBody struct {
	FullName              string   `json:"full_name" minLength:"1" doc:"Crew member full name"`
	OtherNames            []string `json:"other_names,omitempty" maxItems:"20" doc:"Other spellings and maiden names; blank and repeated entries are dropped, each at most 200 characters. An update replaces the list."`
	Email                 *string  `json:"email,omitempty"`
	PatentNumber          *string  `json:"patent_number,omitempty"`
	Phone                 *string  `json:"phone,omitempty"`
	PzzLicenseType        *string  `json:"pzz_license_type,omitempty"`
	PzzLicenseNumber      *string  `json:"pzz_license_number,omitempty"`
	EmergencyContactName  *string  `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string  `json:"emergency_contact_phone,omitempty"`
}

// CrewAssignment is a crew member's assignment to a trip or voyage.
type CrewAssignment struct {
	ID                    int64     `json:"id"`
	TripID                *int64    `json:"trip_id,omitempty"`
	VoyageID              *int64    `json:"voyage_id,omitempty"`
	CrewMemberID          int64     `json:"crew_member_id"`
	Role                  string    `json:"role"`
	PatentNumber          *string   `json:"patent_number,omitempty"`
	FullName              string    `json:"full_name"`
	Email                 *string   `json:"email,omitempty"`
	Phone                 *string   `json:"phone,omitempty" doc:"Leg crew list only, for admins and the leg's captain"`
	EmergencyContactName  *string   `json:"emergency_contact_name,omitempty" doc:"Leg crew list only, for admins and the leg's captain"`
	EmergencyContactPhone *string   `json:"emergency_contact_phone,omitempty" doc:"Leg crew list only, for admins and the leg's captain"`
	CreatedAt             time.Time `json:"created_at"`
}

// MaxCrewRoleLen caps a crew role, in characters.
const MaxCrewRoleLen = 100

// CrewRoleBody sets the role of one crew entry.
type CrewRoleBody struct {
	Role string `json:"role" minLength:"1" maxLength:"100" doc:"Crew role, such as zalogant; trimmed, must not be blank"`
}

// CrewAssignmentBody is the request payload for assigning a crew member.
type CrewAssignmentBody struct {
	CrewMemberID int64   `json:"crew_member_id" doc:"Crew member ID"`
	Role         string  `json:"role" minLength:"1" doc:"Assignment role"`
	PatentNumber *string `json:"patent_number,omitempty"`
}

// CrewMemberFromDB maps a database row to the API model.
func CrewMemberFromDB(m sqlcdb.CrewMember) CrewMember {
	return CrewMember{
		ID:                    m.ID,
		CreatedBy:             intPtr(m.CreatedBy),
		UserID:                intPtr(m.UserID),
		HasAccount:            m.UserID.Valid,
		FullName:              m.FullName,
		OtherNames:            stringList(m.OtherNames),
		Email:                 strPtr(m.Email),
		PatentNumber:          strPtr(m.PatentNumber),
		Phone:                 strPtr(m.Phone),
		PzzLicenseType:        strPtr(m.PzzLicenseType),
		PzzLicenseNumber:      strPtr(m.PzzLicenseNumber),
		EmergencyContactName:  strPtr(m.EmergencyContactName),
		EmergencyContactPhone: strPtr(m.EmergencyContactPhone),
		CreatedAt:             timeVal(m.CreatedAt),
		UpdatedAt:             timeVal(m.UpdatedAt),
	}
}

// CrewMemberForCaller maps a person read for the caller. The emergency
// contact stays only when the row says the caller may read it.
func CrewMemberForCaller(m sqlcdb.CrewMember, contactsVisible bool) CrewMember {
	c := CrewMemberFromDB(m)
	if !contactsVisible {
		c.EmergencyContactName, c.EmergencyContactPhone = nil, nil
	}
	return c
}

// CrewMembersForCaller maps the people list rows, returning a non-nil slice.
func CrewMembersForCaller(rows []sqlcdb.ListCrewMembersRow) []CrewMember {
	out := make([]CrewMember, len(rows))
	for i := range rows {
		out[i] = CrewMemberForCaller(rows[i].CrewMember, rows[i].ContactsVisible)
	}
	return out
}

// CrewAssignmentFromDB maps an assignment row without the joined member name.
func CrewAssignmentFromDB(a sqlcdb.CrewAssignment) CrewAssignment {
	return CrewAssignment{
		ID:           a.ID,
		TripID:       intPtr(a.TripID),
		VoyageID:     intPtr(a.VoyageID),
		CrewMemberID: a.CrewMemberID,
		Role:         a.Role,
		PatentNumber: strPtr(a.PatentNumber),
		CreatedAt:    timeVal(a.CreatedAt),
	}
}

// TripCrewMemberFromDB maps one joined trip-crew row. The phone and
// emergency contact stay only when the row says the caller may read them.
func TripCrewMemberFromDB(r sqlcdb.ListTripCrewAssignmentsRow) CrewAssignment {
	a := CrewAssignment{
		ID:                    r.ID,
		TripID:                intPtr(r.TripID),
		VoyageID:              intPtr(r.VoyageID),
		CrewMemberID:          r.CrewMemberID,
		Role:                  r.Role,
		PatentNumber:          strPtr(r.PatentNumber),
		FullName:              r.FullName,
		Email:                 strPtr(r.Email),
		Phone:                 strPtr(r.Phone),
		EmergencyContactName:  strPtr(r.EmergencyContactName),
		EmergencyContactPhone: strPtr(r.EmergencyContactPhone),
		CreatedAt:             timeVal(r.CreatedAt),
	}
	if !r.ContactsVisible {
		a.Phone, a.EmergencyContactName, a.EmergencyContactPhone = nil, nil, nil
	}
	return a
}

// TripCrewFromDB maps the joined trip-crew rows, returning a non-nil slice.
func TripCrewFromDB(rows []sqlcdb.ListTripCrewAssignmentsRow) []CrewAssignment {
	out := make([]CrewAssignment, len(rows))
	for i := range rows {
		out[i] = TripCrewMemberFromDB(rows[i])
	}
	return out
}

// VoyageCrewFromDB maps the joined voyage-crew rows, returning a non-nil slice.
func VoyageCrewFromDB(rows []sqlcdb.ListVoyageCrewAssignmentsRow) []CrewAssignment {
	out := make([]CrewAssignment, len(rows))
	for i := range rows {
		r := rows[i]
		out[i] = CrewAssignment{
			ID:           r.ID,
			TripID:       intPtr(r.TripID),
			VoyageID:     intPtr(r.VoyageID),
			CrewMemberID: r.CrewMemberID,
			Role:         r.Role,
			PatentNumber: strPtr(r.PatentNumber),
			FullName:     r.FullName,
			Email:        strPtr(r.Email),
			CreatedAt:    timeVal(r.CreatedAt),
		}
	}
	return out
}

// PersonMergeBody names the duplicate person merged into the path's person.
type PersonMergeBody struct {
	DuplicateID int64 `json:"duplicate_id" minimum:"1" doc:"Person merged into the path's person and then removed"`
}

// PersonMerge describes a merge of a duplicate person into a kept one: the
// kept person as it is (or would be) afterwards, the removed duplicate, and
// how many crew assignments, voyage opinions and leg sign-ups move. A shared
// assignment or opinion is one both people already had; the kept person's
// copy stays. Of two sign-ups for one leg, one stays.
type PersonMerge struct {
	Person            CrewMember `json:"person" doc:"Kept person after the merge"`
	Duplicate         CrewMember `json:"duplicate" doc:"Person removed by the merge"`
	TripAssignments   int64      `json:"trip_assignments" doc:"Leg crew assignments of the duplicate"`
	VoyageAssignments int64      `json:"voyage_assignments" doc:"Voyage crew assignments of the duplicate"`
	SharedAssignments int64      `json:"shared_assignments" doc:"Duplicate's assignments on legs and voyages the kept person is already on; they collapse into the kept person's"`
	Opinions          int64      `json:"opinions" doc:"Voyage opinions of the duplicate"`
	SharedOpinions    int64      `json:"shared_opinions" doc:"Duplicate's opinions for voyages the kept person already has one for; the kept person's stays"`
	CaptainedTrips    int64      `json:"captained_trips" doc:"Legs the duplicate captains"`
	CaptainedVoyages  int64      `json:"captained_voyages" doc:"Voyages the duplicate captains"`
	TripEnrollments   int64      `json:"trip_enrollments" doc:"Leg sign-ups of the duplicate"`
	SharedEnrollments int64      `json:"shared_trip_enrollments" doc:"Duplicate's sign-ups for legs the kept person also signed up for; one sign-up per leg stays"`
}

// PersonMergeFromDB maps merge counts and both people to the API model.
func PersonMergeFromDB(kept, duplicate sqlcdb.CrewMember, c sqlcdb.CountCrewMergeRowsRow) PersonMerge {
	return PersonMerge{
		Person:            CrewMemberFromDB(kept),
		Duplicate:         CrewMemberFromDB(duplicate),
		TripAssignments:   c.TripAssignments,
		VoyageAssignments: c.VoyageAssignments,
		SharedAssignments: c.SharedAssignments,
		Opinions:          c.Opinions,
		SharedOpinions:    c.SharedOpinions,
		CaptainedTrips:    c.CaptainedTrips,
		CaptainedVoyages:  c.CaptainedVoyages,
		TripEnrollments:   c.TripEnrollments,
		SharedEnrollments: c.SharedTripEnrollments,
	}
}
