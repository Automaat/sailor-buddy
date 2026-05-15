package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// CrewMember is the API representation of a crew member.
type CrewMember struct {
	ID                    int64     `json:"id"`
	OwnerID               int64     `json:"owner_id"`
	OrgID                 *int64    `json:"org_id,omitempty"`
	UserID                *int64    `json:"user_id,omitempty"`
	FullName              string    `json:"full_name"`
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

// CrewMemberBody is the owner-scoped create/update payload for a crew member.
type CrewMemberBody struct {
	FullName     string  `json:"full_name" minLength:"1" doc:"Crew member full name"`
	Email        *string `json:"email,omitempty"`
	PatentNumber *string `json:"patent_number,omitempty"`
}

// OrgCrewBody is the org-scoped create/update payload for a crew member,
// covering the extended PZŻ licence and emergency-contact fields.
type OrgCrewBody struct {
	FullName              string  `json:"full_name" minLength:"1" doc:"Crew member full name"`
	Email                 *string `json:"email,omitempty"`
	PatentNumber          *string `json:"patent_number,omitempty"`
	Phone                 *string `json:"phone,omitempty"`
	PzzLicenseType        *string `json:"pzz_license_type,omitempty"`
	PzzLicenseNumber      *string `json:"pzz_license_number,omitempty"`
	EmergencyContactName  *string `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone *string `json:"emergency_contact_phone,omitempty"`
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
		OwnerID:               m.OwnerID,
		OrgID:                 intPtr(m.OrgID),
		UserID:                intPtr(m.UserID),
		FullName:              m.FullName,
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
