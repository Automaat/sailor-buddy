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
	ID           int64     `json:"id"`
	TripID       *int64    `json:"trip_id,omitempty"`
	VoyageID     *int64    `json:"voyage_id,omitempty"`
	CrewMemberID int64     `json:"crew_member_id"`
	Role         string    `json:"role"`
	PatentNumber *string   `json:"patent_number,omitempty"`
	FullName     string    `json:"full_name"`
	Email        *string   `json:"email,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
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

// CrewMembersFromDB maps a slice of database rows, returning a non-nil slice.
func CrewMembersFromDB(ms []sqlcdb.CrewMember) []CrewMember {
	out := make([]CrewMember, len(ms))
	for i := range ms {
		out[i] = CrewMemberFromDB(ms[i])
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

// TripCrewFromDB maps the joined trip-crew rows, returning a non-nil slice.
func TripCrewFromDB(rows []sqlcdb.ListTripCrewAssignmentsRow) []CrewAssignment {
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
// how many crew assignments and voyage opinions move. A shared assignment or
// opinion is one both people already had; the kept person's copy stays.
type PersonMerge struct {
	Person            CrewMember `json:"person" doc:"Kept person after the merge"`
	Duplicate         CrewMember `json:"duplicate" doc:"Person removed by the merge"`
	TripAssignments   int64      `json:"trip_assignments" doc:"Leg crew assignments of the duplicate"`
	VoyageAssignments int64      `json:"voyage_assignments" doc:"Voyage crew assignments of the duplicate"`
	SharedAssignments int64      `json:"shared_assignments" doc:"Duplicate's assignments on legs and voyages the kept person is already on; they collapse into the kept person's"`
	Opinions          int64      `json:"opinions" doc:"Voyage opinions of the duplicate"`
	SharedOpinions    int64      `json:"shared_opinions" doc:"Duplicate's opinions for voyages the kept person already has one for; the kept person's stays"`
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
	}
}
