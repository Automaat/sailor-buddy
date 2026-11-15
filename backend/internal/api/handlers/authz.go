package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

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
// and write opinions for their own voyages, and complete their own legs once
// they end; a captain person without an account grants no rights, and
// everything else stays admin-only.
func requireAdminOrCaptain(ctx context.Context, q sqlcdb.Querier, captainPersonID types.NullInt64) error {
	u := middleware.GetUser(ctx)
	if u == nil {
		return huma.Error403Forbidden("admin or captain only")
	}
	if u.Role == "admin" {
		return nil
	}
	ok, err := isCaptain(ctx, q, captainPersonID)
	if err != nil {
		return err
	}
	if !ok {
		return huma.Error403Forbidden("admin or captain only")
	}
	return nil
}

// isCaptain reports whether the caller's account is linked to the captain
// person. A missing captain, a captain without an account or a person gone
// meanwhile is nobody's.
func isCaptain(ctx context.Context, q sqlcdb.Querier, captainPersonID types.NullInt64) (bool, error) {
	u := middleware.GetUser(ctx)
	if u == nil || !captainPersonID.Valid {
		return false, nil
	}
	captain, err := q.GetCrewMember(ctx, captainPersonID.Int64)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		slog.Error("get captain person", "person_id", captainPersonID.Int64, "err", err)
		return false, huma.Error500InternalServerError("failed to verify captain")
	}
	return captain.UserID.Valid && captain.UserID.Int64 == u.UserID, nil
}

// hideForbidden turns a 403 for a hidden record into 404 for a caller who
// cannot see it, so a refusal does not reveal the draft. visible answers
// whether the record is published; other errors pass through.
func hideForbidden(ctx context.Context, authErr error, visible func() error) error {
	status, ok := errors.AsType[huma.StatusError](authErr)
	if canSeeHidden(ctx) || !ok || status.GetStatus() != http.StatusForbidden {
		return authErr
	}
	if err := visible(); err != nil {
		return err
	}
	return authErr
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
