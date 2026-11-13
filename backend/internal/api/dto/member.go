package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Member is a club member: a registered user account and their role.
type Member struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        *string   `json:"phone,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	Role         string    `json:"role"`
	IsCaptain    bool      `json:"is_captain"`
	Membership   string    `json:"membership" enum:"czlonek,sympatyk,gosc"`
	PatentType   *string   `json:"patent_type,omitempty"`
	PatentNumber *string   `json:"patent_number,omitempty"`
	Onboarded    bool      `json:"onboarded" doc:"Whether the user completed the onboarding form"`
	CreatedAt    time.Time `json:"created_at"`
}

// RoleBody is the request payload for changing a member's role.
type RoleBody struct {
	Role string `json:"role" enum:"admin,member" doc:"New role"`
}

// CaptainBody is the request payload for adding or removing a member from
// the club's captain group.
type CaptainBody struct {
	IsCaptain bool `json:"is_captain" doc:"Whether the member belongs to the captain group"`
}

// MembershipBody is the request payload for changing a member's status:
// czlonek (AGH student member), sympatyk (non-AGH student), gosc (guest).
type MembershipBody struct {
	Membership string `json:"membership" enum:"czlonek,sympatyk,gosc" doc:"Membership status"`
}

// MemberFromUser maps a stored user row onto the member DTO.
func MemberFromUser(u sqlcdb.User) Member {
	return Member{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		Phone:        strPtr(u.Phone),
		AvatarURL:    strPtr(u.AvatarUrl),
		Role:         u.Role,
		IsCaptain:    u.IsCaptain,
		Membership:   u.Membership,
		PatentType:   strPtr(u.PatentType),
		PatentNumber: strPtr(u.PatentNumber),
		Onboarded:    u.OnboardedAt.Valid,
		CreatedAt:    timeVal(u.CreatedAt),
	}
}

// MembersFromDB maps user rows to member DTOs, returning a non-nil slice.
func MembersFromDB(users []sqlcdb.User) []Member {
	out := make([]Member, len(users))
	for i := range users {
		out[i] = MemberFromUser(users[i])
	}
	return out
}
