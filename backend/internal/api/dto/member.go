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
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	Role         string    `json:"role"`
	PatentType   *string   `json:"patent_type,omitempty"`
	PatentNumber *string   `json:"patent_number,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RoleBody is the request payload for changing a member's role.
type RoleBody struct {
	Role string `json:"role" enum:"admin,member" doc:"New role"`
}

// MemberFromUser maps a stored user row onto the member DTO.
func MemberFromUser(u sqlcdb.User) Member {
	return Member{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		AvatarURL:    strPtr(u.AvatarUrl),
		Role:         u.Role,
		PatentType:   strPtr(u.PatentType),
		PatentNumber: strPtr(u.PatentNumber),
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
