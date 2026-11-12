package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
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

// requireAdminOrCaptain allows admins and the captain assigned to the record.
// Captains log post-cruise data (hours, miles, ports, notes) for their own
// voyages; everything else stays admin-only.
func requireAdminOrCaptain(ctx context.Context, captainUserID types.NullInt64) error {
	u := middleware.GetUser(ctx)
	if u == nil {
		return huma.Error403Forbidden("admin or voyage captain only")
	}
	if u.Role == "admin" || (captainUserID.Valid && captainUserID.Int64 == u.UserID) {
		return nil
	}
	return huma.Error403Forbidden("admin or voyage captain only")
}
