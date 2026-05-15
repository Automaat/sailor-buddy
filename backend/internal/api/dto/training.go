package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// Training is the API representation of a training record.
type Training struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Date      *string   `json:"date,omitempty"`
	Name      string    `json:"name"`
	Organizer *string   `json:"organizer,omitempty"`
	Cost      *float64  `json:"cost,omitempty"`
	Url       *string   `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrainingBody is the create/update request payload for a training.
type TrainingBody struct {
	Name      string   `json:"name" minLength:"1" doc:"Training name"`
	Date      *string  `json:"date,omitempty"`
	Organizer *string  `json:"organizer,omitempty"`
	Cost      *float64 `json:"cost,omitempty"`
	Url       *string  `json:"url,omitempty"`
}

// TrainingFromDB maps a database row to the API model.
func TrainingFromDB(t sqlcdb.Training) Training {
	return Training{
		ID:        t.ID,
		UserID:    t.UserID,
		Date:      strPtr(t.Date),
		Name:      t.Name,
		Organizer: strPtr(t.Organizer),
		Cost:      floatPtr(t.Cost),
		Url:       strPtr(t.Url),
		CreatedAt: timeVal(t.CreatedAt),
		UpdatedAt: timeVal(t.UpdatedAt),
	}
}

// TrainingsFromDB maps a slice of database rows, returning a non-nil slice.
func TrainingsFromDB(ts []sqlcdb.Training) []Training {
	out := make([]Training, len(ts))
	for i, t := range ts {
		out[i] = TrainingFromDB(t)
	}
	return out
}
