package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Organization is the API representation of an organization.
type Organization struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   *string   `json:"description,omitempty"`
	LogoUrl       *string   `json:"logo_url,omitempty"`
	PzzClubNumber *string   `json:"pzz_club_number,omitempty"`
	City          *string   `json:"city,omitempty"`
	Website       *string   `json:"website,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// OrgBody is the create/update request payload for an organization. Slug is
// required on create and ignored on update.
type OrgBody struct {
	Name          string  `json:"name" minLength:"1" doc:"Organization name"`
	Slug          *string `json:"slug,omitempty" doc:"URL slug (lowercase letters, digits, hyphens); required on create"`
	Description   *string `json:"description,omitempty"`
	LogoUrl       *string `json:"logo_url,omitempty"`
	PzzClubNumber *string `json:"pzz_club_number,omitempty"`
	City          *string `json:"city,omitempty"`
	Website       *string `json:"website,omitempty"`
}

// OrgMember is an organization membership joined with the member's user.
type OrgMember struct {
	ID            int64     `json:"id"`
	OrgID         int64     `json:"org_id"`
	UserID        int64     `json:"user_id"`
	Role          string    `json:"role"`
	JoinedAt      time.Time `json:"joined_at"`
	UserName      string    `json:"user_name"`
	UserEmail     string    `json:"user_email"`
	UserAvatarURL *string   `json:"user_avatar_url,omitempty"`
}

// OrgInvite is an organization invite link.
type OrgInvite struct {
	ID          int64      `json:"id"`
	OrgID       int64      `json:"org_id"`
	Token       string     `json:"token"`
	Role        string     `json:"role"`
	CreatedBy   int64      `json:"created_by"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	MaxUses     *int64     `json:"max_uses,omitempty"`
	UseCount    int64      `json:"use_count"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatorName string     `json:"creator_name,omitempty"`
}

// InviteRequestBody creates an organization invite link.
type InviteRequestBody struct {
	Role           string `json:"role,omitempty" enum:"admin,captain,crew" default:"crew" doc:"Role granted to invitees"`
	ExpiresInHours *int64 `json:"expires_in_hours,omitempty" doc:"Invite lifetime in hours"`
	MaxUses        *int64 `json:"max_uses,omitempty" doc:"Maximum number of uses"`
}

// MemberRoleBody updates an organization member's role.
type MemberRoleBody struct {
	Role string `json:"role" enum:"admin,captain,crew" doc:"New role"`
}

// InviteInfo describes the organization behind an invite token.
type InviteInfo struct {
	OrgName       string `json:"org_name"`
	OrgSlug       string `json:"org_slug"`
	Role          string `json:"role"`
	AlreadyMember bool   `json:"already_member"`
}

// InviteAcceptResult is returned after accepting an invite.
type InviteAcceptResult struct {
	OrgName string `json:"org_name"`
	OrgSlug string `json:"org_slug"`
	Role    string `json:"role"`
}

// OrgDashboard is the org-scoped sailing and membership summary.
type OrgDashboard struct {
	VoyageCount      int64           `json:"voyage_count"`
	TotalHours       float64         `json:"total_hours"`
	TotalMiles       float64         `json:"total_miles"`
	TotalDays        int64           `json:"total_days"`
	TotalHoursSail   float64         `json:"total_hours_sail"`
	TotalHoursEngine float64         `json:"total_hours_engine"`
	MemberCount      int             `json:"member_count"`
	YachtCount       int             `json:"yacht_count"`
	ByYear           []VoyagesByYear `json:"by_year"`
}

// OrganizationFromDB maps a database row to the API model.
func OrganizationFromDB(o sqlcdb.Organization) Organization {
	return Organization{
		ID:            o.ID,
		Name:          o.Name,
		Slug:          o.Slug,
		Description:   strPtr(o.Description),
		LogoUrl:       strPtr(o.LogoUrl),
		PzzClubNumber: strPtr(o.PzzClubNumber),
		City:          strPtr(o.City),
		Website:       strPtr(o.Website),
		CreatedAt:     timeVal(o.CreatedAt),
		UpdatedAt:     timeVal(o.UpdatedAt),
	}
}

// OrganizationsFromDB maps a slice of database rows, returning a non-nil slice.
func OrganizationsFromDB(os []sqlcdb.Organization) []Organization {
	out := make([]Organization, len(os))
	for i := range os {
		out[i] = OrganizationFromDB(os[i])
	}
	return out
}

// UserOrganization is an organization plus the caller's role within it.
type UserOrganization struct {
	Organization
	Role string `json:"role"`
}

// UserOrganizationsFromDB maps the caller's org rows, returning a non-nil slice.
func UserOrganizationsFromDB(rows []sqlcdb.ListUserOrganizationsRow) []UserOrganization {
	out := make([]UserOrganization, len(rows))
	for i := range rows {
		r := rows[i]
		out[i] = UserOrganization{
			Organization: Organization{
				ID:            r.ID,
				Name:          r.Name,
				Slug:          r.Slug,
				Description:   strPtr(r.Description),
				LogoUrl:       strPtr(r.LogoUrl),
				PzzClubNumber: strPtr(r.PzzClubNumber),
				City:          strPtr(r.City),
				Website:       strPtr(r.Website),
				CreatedAt:     timeVal(r.CreatedAt),
				UpdatedAt:     timeVal(r.UpdatedAt),
			},
			Role: r.Role,
		}
	}
	return out
}

// OrgMembersFromDB maps the joined member rows, returning a non-nil slice.
func OrgMembersFromDB(rows []sqlcdb.ListOrgMembersRow) []OrgMember {
	out := make([]OrgMember, len(rows))
	for i := range rows {
		out[i] = OrgMember{
			ID:            rows[i].ID,
			OrgID:         rows[i].OrgID,
			UserID:        rows[i].UserID,
			Role:          rows[i].Role,
			JoinedAt:      timeVal(rows[i].JoinedAt),
			UserName:      rows[i].UserName,
			UserEmail:     rows[i].UserEmail,
			UserAvatarURL: strPtr(rows[i].UserAvatarUrl),
		}
	}
	return out
}

// OrgInviteFromDB maps an invite row (without creator name) to the API model.
func OrgInviteFromDB(i sqlcdb.OrgInvite) OrgInvite {
	return OrgInvite{
		ID:        i.ID,
		OrgID:     i.OrgID,
		Token:     i.Token,
		Role:      i.Role,
		CreatedBy: i.CreatedBy,
		ExpiresAt: timePtr(i.ExpiresAt),
		MaxUses:   intPtr(i.MaxUses),
		UseCount:  i.UseCount,
		CreatedAt: timeVal(i.CreatedAt),
	}
}

// OrgInvitesFromDB maps the joined invite rows, returning a non-nil slice.
func OrgInvitesFromDB(rows []sqlcdb.ListOrgInvitesRow) []OrgInvite {
	out := make([]OrgInvite, len(rows))
	for i := range rows {
		out[i] = OrgInvite{
			ID:          rows[i].ID,
			OrgID:       rows[i].OrgID,
			Token:       rows[i].Token,
			Role:        rows[i].Role,
			CreatedBy:   rows[i].CreatedBy,
			ExpiresAt:   timePtr(rows[i].ExpiresAt),
			MaxUses:     intPtr(rows[i].MaxUses),
			UseCount:    rows[i].UseCount,
			CreatedAt:   timeVal(rows[i].CreatedAt),
			CreatorName: rows[i].CreatorName,
		}
	}
	return out
}
