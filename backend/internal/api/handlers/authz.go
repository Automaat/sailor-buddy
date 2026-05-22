package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
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
