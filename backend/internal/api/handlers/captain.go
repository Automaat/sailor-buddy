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

// captainAssignment resolves a trip/voyage captain. With a user ID it checks
// the user is in the captain group with a patent number on file and
// snapshots their name; without one the free-text name is kept (imported
// history, captains without an account). Eligibility is checked only when the
// captain changes from current, so a captain who later left the group does
// not block unrelated edits.
func captainAssignment(ctx context.Context, q sqlcdb.Querier, current types.NullInt64, userID *int64, name *string) (types.NullInt64, types.NullString, error) {
	if userID == nil {
		return types.NullInt64{}, nullString(name), nil
	}
	u, err := q.GetUserByID(ctx, *userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.NullInt64{}, types.NullString{}, huma.Error422UnprocessableEntity("captain user not found")
		}
		slog.Error("get captain user", "user_id", *userID, "err", err)
		return types.NullInt64{}, types.NullString{}, huma.Error500InternalServerError("failed to verify captain")
	}
	if !current.Valid || current.Int64 != u.ID {
		if err := captainEligible(u); err != nil {
			return types.NullInt64{}, types.NullString{}, err
		}
	}
	return types.NullInt64{Int64: u.ID, Valid: true}, types.NullString{String: u.Name, Valid: true}, nil
}

// captainEligible gates the captain group: club members (czlonek) with a
// patent number on their account.
func captainEligible(u sqlcdb.User) error {
	if !u.IsCaptain {
		return huma.Error422UnprocessableEntity("user is not in the captain group")
	}
	return captainPrerequisites(u)
}

func captainPrerequisites(u sqlcdb.User) error {
	if u.Membership != "czlonek" {
		return huma.Error422UnprocessableEntity("captains must be club members (czlonek)")
	}
	if !u.PatentNumber.Valid || u.PatentNumber.String == "" {
		return huma.Error422UnprocessableEntity("captain has no patent number on their account")
	}
	return nil
}
