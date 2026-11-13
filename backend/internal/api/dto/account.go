package dto

import "github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"

// Me is the authenticated user's profile.
type Me struct {
	ID           int64   `json:"id"`
	Email        string  `json:"email"`
	Name         string  `json:"name"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Phone        *string `json:"phone,omitempty"`
	Onboarded    bool    `json:"onboarded" doc:"Whether the user completed the onboarding form"`
	AvatarURL    string  `json:"avatar_url"`
	Role         string  `json:"role"`
	IsCaptain    bool    `json:"is_captain"`
	Membership   string  `json:"membership" enum:"czlonek,sympatyk,gosc"`
	PatentType   *string `json:"patent_type,omitempty"`
	PatentNumber *string `json:"patent_number,omitempty"`
}

// MeFromUser maps a stored user row onto the profile DTO.
func MeFromUser(u sqlcdb.User) Me {
	return Me{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		Phone:        strPtr(u.Phone),
		Onboarded:    u.OnboardedAt.Valid,
		AvatarURL:    u.AvatarUrl.String,
		Role:         u.Role,
		IsCaptain:    u.IsCaptain,
		Membership:   u.Membership,
		PatentType:   strPtr(u.PatentType),
		PatentNumber: strPtr(u.PatentNumber),
	}
}

// UpdateProfileBody is the onboarding and profile form. Saving it marks the
// user as onboarded. PatentType is one of the three Polish patent grades, or
// omitted for none; a patent type requires a patent number.
type UpdateProfileBody struct {
	FirstName    string  `json:"first_name" minLength:"1" maxLength:"100" doc:"Given name"`
	LastName     string  `json:"last_name" minLength:"1" maxLength:"100" doc:"Family name"`
	Phone        *string `json:"phone,omitempty" maxLength:"30" pattern:"^\\+?[0-9 ()-]{6,29}$" doc:"Contact phone number"`
	PatentType   *string `json:"patent_type,omitempty" enum:"zeglarz_jachtowy,jachtowy_sternik_morski,kapitan_jachtowy" doc:"Polish sailing patent grade"`
	PatentNumber *string `json:"patent_number,omitempty" doc:"Patent registration number"`
}

// MyCruises lists the cruises the current user took part in.
type MyCruises struct {
	CruiseIDs []int64 `json:"cruise_ids" doc:"Cruises with the user's accepted enrollment, captained leg or crew-list entry"`
}

// VoyagesByYear is one row of the per-year sailing breakdown.
type VoyagesByYear struct {
	Year        *int64  `json:"year,omitempty"`
	VoyageCount int64   `json:"voyage_count"`
	TotalHours  float64 `json:"total_hours"`
	TotalMiles  float64 `json:"total_miles"`
	TotalDays   int64   `json:"total_days"`
}

// Dashboard is the club-wide sailing summary.
type Dashboard struct {
	VoyageCount      int64           `json:"voyage_count"`
	TotalHours       float64         `json:"total_hours"`
	TotalMiles       float64         `json:"total_miles"`
	TotalDays        int64           `json:"total_days"`
	TotalHoursSail   float64         `json:"total_hours_sail"`
	TotalHoursEngine float64         `json:"total_hours_engine"`
	ByYear           []VoyagesByYear `json:"by_year"`
}

// VoyagesByYearFromDB maps the per-year rows, returning a non-nil slice.
func VoyagesByYearFromDB(rows []sqlcdb.GetVoyagesByYearRow) []VoyagesByYear {
	out := make([]VoyagesByYear, len(rows))
	for i := range rows {
		out[i] = VoyagesByYear{
			Year:        intPtr(rows[i].Year),
			VoyageCount: rows[i].VoyageCount,
			TotalHours:  rows[i].TotalHours,
			TotalMiles:  rows[i].TotalMiles,
			TotalDays:   rows[i].TotalDays,
		}
	}
	return out
}
