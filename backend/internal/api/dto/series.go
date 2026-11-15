package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// CruiseSeries is the API representation of a recurring cruise series, such
// as a yearly expedition whose editions are separate cruises.
type CruiseSeries struct {
	ID          int64     `json:"id"`
	CreatedBy   *int64    `json:"created_by,omitempty"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Kind        string    `json:"kind" enum:"sea,inland" doc:"Kind of every edition: sea (morski) or inland (śródlądowy)"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CruiseSeriesBody is the create/update request payload for a series.
type CruiseSeriesBody struct {
	Name        string  `json:"name" minLength:"1" doc:"Series name"`
	Description *string `json:"description,omitempty"`
	Kind        string  `json:"kind,omitempty" enum:"sea,inland" default:"sea" doc:"Kind of every edition: sea (morski) or inland (śródlądowy)"`
}

// CruiseSeriesEdition is one cruise of a series with the miles and hours
// summed over its voyages.
type CruiseSeriesEdition struct {
	CruiseID    int64      `json:"cruise_id"`
	Name        string     `json:"name"`
	Edition     *string    `json:"edition,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty" doc:"When the edition was published; absent while it is a hidden draft"`
	VoyageCount int64      `json:"voyage_count"`
	TotalMiles  float64    `json:"total_miles"`
	TotalHours  float64    `json:"total_hours"`
}

// CruiseSeriesFromDB maps a database row to the API model.
func CruiseSeriesFromDB(s sqlcdb.CruiseSeries) CruiseSeries {
	return CruiseSeries{
		ID:          s.ID,
		CreatedBy:   intPtr(s.CreatedBy),
		Name:        s.Name,
		Description: strPtr(s.Description),
		Kind:        s.Kind,
		CreatedAt:   timeVal(s.CreatedAt),
		UpdatedAt:   timeVal(s.UpdatedAt),
	}
}

// CruiseSeriesListFromDB maps a slice of database rows, returning a non-nil
// slice.
func CruiseSeriesListFromDB(ss []sqlcdb.CruiseSeries) []CruiseSeries {
	out := make([]CruiseSeries, len(ss))
	for i := range ss {
		out[i] = CruiseSeriesFromDB(ss[i])
	}
	return out
}

// CruiseSeriesEditionsFromDB maps edition rows, returning a non-nil slice.
func CruiseSeriesEditionsFromDB(rows []sqlcdb.ListCruiseSeriesEditionsRow) []CruiseSeriesEdition {
	out := make([]CruiseSeriesEdition, len(rows))
	for i := range rows {
		r := &rows[i]
		out[i] = CruiseSeriesEdition{
			CruiseID:    r.ID,
			Name:        r.Name,
			Edition:     strPtr(r.Edition),
			PublishedAt: timePtr(r.PublishedAt),
			VoyageCount: r.VoyageCount,
			TotalMiles:  r.TotalMiles,
			TotalHours:  r.TotalHours,
		}
	}
	return out
}
