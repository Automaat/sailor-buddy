package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// orgSlugParam is the shared path parameter for org-scoped operations.
type orgSlugParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
}

// resolveOrg loads the organization for slug, verifies the caller is a member,
// and (when requireAdmin) that they are an admin. It replaces the chi
// OrgFromSlug + RequireOrgRole middleware for huma-served org routes.
func resolveOrg(ctx context.Context, q sqlcdb.Querier, slug string, requireAdmin bool) (*middleware.OrgContext, error) {
	org, err := q.GetOrganizationBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("organization not found")
		}
		slog.Error("resolve org", "slug", slug, "err", err)
		return nil, huma.Error500InternalServerError("failed to get organization")
	}

	user := middleware.GetUser(ctx)
	member, err := q.GetOrgMembership(ctx, sqlcdb.GetOrgMembershipParams{OrgID: org.ID, UserID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error403Forbidden("not a member of this organization")
		}
		slog.Error("resolve org membership", "slug", slug, "err", err)
		return nil, huma.Error500InternalServerError("failed to check membership")
	}
	if requireAdmin && member.Role != "admin" {
		return nil, huma.Error403Forbidden("insufficient permissions")
	}
	return &middleware.OrgContext{OrgID: org.ID, Slug: slug, Role: member.Role}, nil
}
