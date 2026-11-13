package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// requireParentCruise rejects a trip or voyage body whose cruise does not
// exist. Every trip and voyage is a leg of a club event.
func requireParentCruise(ctx context.Context, q sqlcdb.Querier, cruiseID int64) error {
	if _, err := q.GetCruise(ctx, cruiseID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error422UnprocessableEntity("cruise not found")
		}
		slog.Error("verify parent cruise", "cruise_id", cruiseID, "err", err)
		return huma.Error500InternalServerError("failed to verify cruise")
	}
	return nil
}
