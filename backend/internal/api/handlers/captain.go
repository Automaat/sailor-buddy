package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// captainPersonNotFound answers a captain person that is unknown, or was
// deleted between the lookup and the write.
func captainPersonNotFound() error {
	return huma.Error422UnprocessableEntity("captain person not found")
}

// captainAssignment resolves a trip/voyage captain. With a person ID it
// snapshots the person's name into captain_name; without one the free-text
// name is kept (imported history). Any person may captain: the captain group
// (users.is_captain) only orders and marks the picker.
func captainAssignment(ctx context.Context, q sqlcdb.Querier, personID *int64, name *string) (types.NullInt64, types.NullString, error) {
	if personID == nil {
		return types.NullInt64{}, nullString(name), nil
	}
	p, err := q.GetCrewMember(ctx, *personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.NullInt64{}, types.NullString{}, captainPersonNotFound()
		}
		slog.Error("get captain person", "person_id", *personID, "err", err)
		return types.NullInt64{}, types.NullString{}, huma.Error500InternalServerError("failed to verify captain")
	}
	return types.NullInt64{Int64: p.ID, Valid: true}, types.NullString{String: p.FullName, Valid: true}, nil
}

// captainPrerequisites gates the captain group: club members (czlonek) with
// a patent number on their account.
func captainPrerequisites(u sqlcdb.User) error {
	if u.Membership != "czlonek" {
		return huma.Error422UnprocessableEntity("captains must be club members (czlonek)")
	}
	if !u.PatentNumber.Valid || u.PatentNumber.String == "" {
		return huma.Error422UnprocessableEntity("captain has no patent number on their account")
	}
	return nil
}
