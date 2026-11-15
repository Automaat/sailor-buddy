package dto

import (
	"cmp"
	"slices"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Me is the authenticated user's profile.
type Me struct {
	Suspended     bool    `json:"suspended"`
	EmailVerified bool    `json:"email_verified"`
	ID            int64   `json:"id"`
	Email         string  `json:"email"`
	Name          string  `json:"name"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Phone         *string `json:"phone,omitempty"`
	Onboarded     bool    `json:"onboarded" doc:"Whether the user completed the onboarding form"`
	AvatarURL     string  `json:"avatar_url"`
	Role          string  `json:"role"`
	IsCaptain     bool    `json:"is_captain"`
	Membership    string  `json:"membership" enum:"czlonek,sympatyk,gosc"`
	PatentType    *string `json:"patent_type,omitempty"`
	PatentNumber  *string `json:"patent_number,omitempty"`
	PersonID      *int64  `json:"person_id,omitempty" doc:"The account's person (crew record); a leg or voyage it captains gives the account captain rights"`
}

// MeFromUser maps a stored user row onto the profile DTO.
func MeFromUser(u sqlcdb.User) Me {
	return Me{
		Suspended:    u.Suspended,
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

// MyLeg is one of the caller's upcoming legs and how they take part in it.
type MyLeg struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	CruiseID      int64   `json:"cruise_id"`
	CruiseName    string  `json:"cruise_name" doc:"The edition's name; the label of a leg without a series"`
	SeriesID      *int64  `json:"series_id,omitempty"`
	SeriesName    *string `json:"series_name,omitempty"`
	Edition       *string `json:"edition,omitempty" doc:"The edition label within its series, such as 2026"`
	EmbarkDate    *string `json:"embark_date,omitempty"`
	DisembarkDate *string `json:"disembark_date,omitempty"`
	StartPort     *string `json:"start_port,omitempty"`
	EndPort       *string `json:"end_port,omitempty"`
	Relation      string  `json:"relation" enum:"captain,crew,sign_up" doc:"The caller's strongest link to the leg: captain, then crew, then sign-up"`
	SignUpStatus  *string `json:"sign_up_status,omitempty" enum:"accepted,pending,waitlisted" doc:"Status of the caller's sign-up; set only when relation is sign_up"`
}

// MyLegs lists the caller's upcoming legs.
type MyLegs struct {
	Legs []MyLeg `json:"legs" doc:"Sorted by embark date, undated legs last"`
}

// MyLegsFromDB maps the upcoming-leg rows. A row's relation is captain, crew
// or the sign-up status, which becomes relation sign_up with that status.
func MyLegsFromDB(rows []sqlcdb.ListUserUpcomingLegsRow) MyLegs {
	out := MyLegs{Legs: make([]MyLeg, len(rows))}
	for i := range rows {
		r := &rows[i]
		leg := MyLeg{
			ID:            r.ID,
			Name:          r.Name,
			CruiseID:      r.CruiseID,
			CruiseName:    r.CruiseName,
			SeriesID:      intPtr(r.SeriesID),
			SeriesName:    strPtr(r.SeriesName),
			Edition:       strPtr(r.Edition),
			EmbarkDate:    strPtr(r.EmbarkDate),
			DisembarkDate: strPtr(r.DisembarkDate),
			StartPort:     strPtr(r.StartPort),
			EndPort:       strPtr(r.EndPort),
			Relation:      r.Relation,
		}
		if r.Relation != "captain" && r.Relation != "crew" {
			status := r.Relation
			leg.Relation = "sign_up"
			leg.SignUpStatus = &status
		}
		out.Legs[i] = leg
	}
	return out
}

// VoyagesByYear is one row of the per-year sailing breakdown.
type VoyagesByYear struct {
	Year        *int64  `json:"year,omitempty"`
	VoyageCount int64   `json:"voyage_count"`
	TotalHours  float64 `json:"total_hours"`
	TotalMiles  float64 `json:"total_miles"`
	TotalDays   int64   `json:"total_days"`
}

// SeaTime sums the logged sailing of a set of voyages.
type SeaTime struct {
	VoyageCount      int64   `json:"voyage_count"`
	TotalHours       float64 `json:"total_hours"`
	TotalHoursSail   float64 `json:"total_hours_sail"`
	TotalHoursEngine float64 `json:"total_hours_engine"`
	TotalMiles       float64 `json:"total_miles"`
	TotalDays        int64   `json:"total_days"`
}

func (s *SeaTime) add(r *sqlcdb.ListUserSeaTimeVoyagesRow) {
	s.VoyageCount++
	s.TotalHours += r.HoursTotal
	s.TotalHoursSail += r.HoursSail
	s.TotalHoursEngine += r.HoursEngine
	s.TotalMiles += r.Miles
	s.TotalDays += r.Days
}

// SeaTimeVoyage is one voyage in the caller's sea-time log.
type SeaTimeVoyage struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Year          *int64  `json:"year,omitempty"`
	EmbarkDate    *string `json:"embark_date,omitempty"`
	DisembarkDate *string `json:"disembark_date,omitempty"`
	YachtName     *string `json:"yacht_name,omitempty"`
	Role          string  `json:"role" enum:"captain,crew" doc:"captain when the user is the voyage's assigned captain, else crew"`
	Miles         float64 `json:"miles"`
	HoursTotal    float64 `json:"hours_total"`
	Days          int64   `json:"days"`
}

// PersonalSeaTime is the caller's own club sea time.
type PersonalSeaTime struct {
	Total     SeaTime         `json:"total"`
	AsCaptain SeaTime         `json:"as_captain"`
	AsCrew    SeaTime         `json:"as_crew"`
	ByYear    []VoyagesByYear `json:"by_year"`
	Voyages   []SeaTimeVoyage `json:"voyages"`
}

// ClubSeaTime is the club-wide sailing summary.
type ClubSeaTime struct {
	Total  SeaTime         `json:"total"`
	ByYear []VoyagesByYear `json:"by_year"`
}

// Dashboard is the caller's sea time, plus club totals for admins.
type Dashboard struct {
	Personal PersonalSeaTime `json:"personal"`
	Club     *ClubSeaTime    `json:"club,omitempty" doc:"Club-wide totals; present for admins only"`
}

// PersonalSeaTimeFromDB aggregates the caller's voyages. Rows are one per
// voyage, so a voyage the user both captained and crewed counts once.
func PersonalSeaTimeFromDB(rows []sqlcdb.ListUserSeaTimeVoyagesRow) PersonalSeaTime {
	out := PersonalSeaTime{
		ByYear:  []VoyagesByYear{},
		Voyages: make([]SeaTimeVoyage, len(rows)),
	}
	yearIdx := map[types.NullInt64]int{}
	for i := range rows {
		r := &rows[i]
		out.Total.add(r)
		role := "crew"
		if r.AsCaptain {
			role = "captain"
			out.AsCaptain.add(r)
		} else {
			out.AsCrew.add(r)
		}
		out.Voyages[i] = SeaTimeVoyage{
			ID:            r.ID,
			Name:          r.Name,
			Year:          intPtr(r.Year),
			EmbarkDate:    strPtr(r.EmbarkDate),
			DisembarkDate: strPtr(r.DisembarkDate),
			YachtName:     strPtr(r.YachtName),
			Role:          role,
			Miles:         r.Miles,
			HoursTotal:    r.HoursTotal,
			Days:          r.Days,
		}
		j, ok := yearIdx[r.Year]
		if !ok {
			j = len(out.ByYear)
			yearIdx[r.Year] = j
			out.ByYear = append(out.ByYear, VoyagesByYear{Year: intPtr(r.Year)})
		}
		out.ByYear[j].VoyageCount++
		out.ByYear[j].TotalHours += r.HoursTotal
		out.ByYear[j].TotalMiles += r.Miles
		out.ByYear[j].TotalDays += r.Days
	}
	// Ascending with unknown year last, matching the club breakdown.
	slices.SortFunc(out.ByYear, func(a, b VoyagesByYear) int {
		switch {
		case a.Year == nil && b.Year == nil:
			return 0
		case a.Year == nil:
			return 1
		case b.Year == nil:
			return -1
		}
		return cmp.Compare(*a.Year, *b.Year)
	})
	return out
}

// ClubSeaTimeFromDB maps the club-wide totals and per-year rows.
func ClubSeaTimeFromDB(stats sqlcdb.GetDashboardStatsRow, byYear []sqlcdb.GetVoyagesByYearRow) *ClubSeaTime {
	return &ClubSeaTime{
		Total: SeaTime{
			VoyageCount:      stats.VoyageCount,
			TotalHours:       stats.TotalHours,
			TotalHoursSail:   stats.TotalHoursSail,
			TotalHoursEngine: stats.TotalHoursEngine,
			TotalMiles:       stats.TotalMiles,
			TotalDays:        stats.TotalDays,
		},
		ByYear: VoyagesByYearFromDB(byYear),
	}
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
