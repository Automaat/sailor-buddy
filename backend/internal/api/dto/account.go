package dto

import "github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"

// Me is the authenticated user's profile.
type Me struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// VoyagesByYear is one row of the per-year sailing breakdown.
type VoyagesByYear struct {
	Year        *int64  `json:"year,omitempty"`
	VoyageCount int64   `json:"voyage_count"`
	TotalHours  float64 `json:"total_hours"`
	TotalMiles  float64 `json:"total_miles"`
	TotalDays   int64   `json:"total_days"`
}

// Dashboard is the owner-scoped sailing summary.
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
