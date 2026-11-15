package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// requireAdmin returns a 403 error unless the caller is a club admin. It gates
// every mutation of shared club data; reads stay open to any authenticated
// member. It replaced the per-organization resolveOrg membership check when
// the app collapsed to a single club.
func requireAdmin(ctx context.Context) error {
	u := middleware.GetUser(ctx)
	if u == nil || u.Role != "admin" {
		return huma.Error403Forbidden("admin only")
	}
	return nil
}

// canSeeHidden reports whether the caller may read hidden (unpublished)
// cruises, trips and voyages: admins and the captain group
// (users.is_captain). Everyone else sees a hidden record as missing.
func canSeeHidden(ctx context.Context) bool {
	u := middleware.GetUser(ctx)
	return u != nil && (u.Role == "admin" || u.IsCaptain)
}

// requireAdminOrCaptain allows admins and the account linked to the record's
// captain person. Captains log post-cruise data (hours, miles, ports, notes)
// for their own voyages; a captain person without an account grants no
// rights, and everything else stays admin-only.
func requireAdminOrCaptain(ctx context.Context, q sqlcdb.Querier, captainPersonID types.NullInt64) error {
	u := middleware.GetUser(ctx)
	if u == nil {
		return huma.Error403Forbidden("admin or voyage captain only")
	}
	if u.Role == "admin" {
		return nil
	}
	if !captainPersonID.Valid {
		return huma.Error403Forbidden("admin or voyage captain only")
	}
	captain, err := q.GetCrewMember(ctx, captainPersonID.Int64)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error403Forbidden("admin or voyage captain only")
		}
		slog.Error("get captain person", "person_id", captainPersonID.Int64, "err", err)
		return huma.Error500InternalServerError("failed to verify captain")
	}
	if captain.UserID.Valid && captain.UserID.Int64 == u.UserID {
		return nil
	}
	return huma.Error403Forbidden("admin or voyage captain only")
}

// requireVisibleTrip answers 404 unless the trip exists and the caller may
// see it. Nested trip routes use it so a hidden leg's crew and sign-ups stay
// hidden with the leg.
func requireVisibleTrip(ctx context.Context, q sqlcdb.Querier, id int64) error {
	if _, err := q.GetTrip(ctx, sqlcdb.GetTripParams{ID: id, IncludeHidden: canSeeHidden(ctx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("trip not found")
		}
		slog.Error("verify trip visibility", "trip_id", id, "err", err)
		return huma.Error500InternalServerError("failed to get trip")
	}
	return nil
}

// requireVisibleVoyage answers 404 unless the voyage exists and the caller
// may see it. Nested voyage routes use it so a hidden voyage's crew, ports
// and opinions stay hidden with it.
func requireVisibleVoyage(ctx context.Context, q sqlcdb.Querier, id int64) error {
	if _, err := q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: id, IncludeHidden: canSeeHidden(ctx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage visibility", "voyage_id", id, "err", err)
		return huma.Error500InternalServerError("failed to get voyage")
	}
	return nil
}
