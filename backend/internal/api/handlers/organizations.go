package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// txRunner runs fn inside a database transaction, passing it a Querier bound
// to that transaction. It lets the membership write paths stay testable with
// a fake querier while running atomically in production.
type txRunner func(ctx context.Context, fn func(sqlcdb.Querier) error) error

// sqlTxRunner returns a txRunner backed by a real database connection.
func sqlTxRunner(db *sql.DB) txRunner {
	return func(ctx context.Context, fn func(sqlcdb.Querier) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return &QueryError{Op: "BeginTx", Err: err}
		}
		defer func() { _ = tx.Rollback() }()
		if err := fn(sqlcdb.New(tx)); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return &QueryError{Op: "Commit", Err: err}
		}
		return nil
	}
}

// errInviteExhausted signals that an invite's max-use count was reached while
// claiming it, so the membership transaction must roll back.
var errInviteExhausted = errors.New("invite exhausted")

type OrgHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

func NewOrgHandler(q sqlcdb.Querier, db *sql.DB) *OrgHandler {
	return &OrgHandler{q: q, runTx: sqlTxRunner(db)}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// --- huma I/O types ---

type orgListOutput struct {
	Body []dto.UserOrganization
}

type orgOutput struct {
	Body dto.Organization
}

type createOrgInput struct {
	Body dto.OrgBody
}

type updateOrgInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.OrgBody
}

type orgMembersOutput struct {
	Body []dto.OrgMember
}

type memberParam struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	MemberID int64  `path:"memberID" doc:"Org member ID"`
}

type memberRoleInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	MemberID int64  `path:"memberID" doc:"Org member ID"`
	Body     dto.MemberRoleBody
}

type createInviteInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.InviteRequestBody
}

type orgInviteOutput struct {
	Body dto.OrgInvite
}

type orgInvitesOutput struct {
	Body []dto.OrgInvite
}

type inviteParam struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	InviteID int64  `path:"inviteID" doc:"Invite ID"`
}

type joinTokenParam struct {
	Token string `path:"token" doc:"Invite token"`
}

type inviteInfoOutput struct {
	Body dto.InviteInfo
}

type inviteAcceptOutput struct {
	Body dto.InviteAcceptResult
}

// RegisterOrgRoutes wires the organization, membership and invite operations.
func RegisterOrgRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	NewOrgHandler(q, db).registerRoutes(api)
}

func (h *OrgHandler) registerRoutes(api huma.API) {
	tag := []string{"Organizations"}

	huma.Register(api, huma.Operation{
		OperationID: "list-orgs", Method: http.MethodGet, Path: "/orgs",
		Summary: "List the caller's organizations", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "create-org", Method: http.MethodPost, Path: "/orgs",
		Summary: "Create an organization", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "get-org", Method: http.MethodGet, Path: "/orgs/{slug}",
		Summary: "Get an organization", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "update-org", Method: http.MethodPut, Path: "/orgs/{slug}",
		Summary: "Update an organization (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org", Method: http.MethodDelete, Path: "/orgs/{slug}",
		Summary: "Delete an organization (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)

	huma.Register(api, huma.Operation{
		OperationID: "list-org-members", Method: http.MethodGet, Path: "/orgs/{slug}/members",
		Summary: "List organization members", Tags: tag,
	}, h.listMembers)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-member-role", Method: http.MethodPut, Path: "/orgs/{slug}/members/{memberID}/role",
		Summary: "Update a member's role (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateMemberRole)
	huma.Register(api, huma.Operation{
		OperationID: "remove-org-member", Method: http.MethodDelete, Path: "/orgs/{slug}/members/{memberID}",
		Summary: "Remove a member (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.removeMember)

	huma.Register(api, huma.Operation{
		OperationID: "create-org-invite", Method: http.MethodPost, Path: "/orgs/{slug}/invites",
		Summary: "Create an invite link (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.createInvite)
	huma.Register(api, huma.Operation{
		OperationID: "list-org-invites", Method: http.MethodGet, Path: "/orgs/{slug}/invites",
		Summary: "List invite links (admin)", Tags: tag,
	}, h.listInvites)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-invite", Method: http.MethodDelete, Path: "/orgs/{slug}/invites/{inviteID}",
		Summary: "Delete an invite link (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.deleteInvite)

	huma.Register(api, huma.Operation{
		OperationID: "get-invite-info", Method: http.MethodGet, Path: "/join/{token}",
		Summary: "Resolve an invite token", Tags: tag,
	}, h.getInviteInfo)
	huma.Register(api, huma.Operation{
		OperationID: "accept-invite", Method: http.MethodPost, Path: "/join/{token}",
		Summary: "Accept an invite", Tags: tag,
	}, h.acceptInvite)
}

func (h *OrgHandler) list(ctx context.Context, _ *struct{}) (*orgListOutput, error) {
	user := middleware.GetUser(ctx)
	orgs, err := h.q.ListUserOrganizations(ctx, user.UserID)
	if err != nil {
		slog.Error("list organizations", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list organizations")
	}
	return &orgListOutput{Body: dto.UserOrganizationsFromDB(orgs)}, nil
}

func (h *OrgHandler) create(ctx context.Context, in *createOrgInput) (*orgOutput, error) {
	user := middleware.GetUser(ctx)
	if in.Body.Slug == nil || *in.Body.Slug == "" {
		return nil, huma.Error422UnprocessableEntity("slug is required")
	}
	slug := strings.ToLower(*in.Body.Slug)
	if !slugRe.MatchString(slug) {
		return nil, huma.Error422UnprocessableEntity("slug must contain only lowercase letters, numbers, and hyphens")
	}

	var org sqlcdb.Organization
	err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		var txErr error
		org, txErr = q.CreateOrganization(ctx, sqlcdb.CreateOrganizationParams{
			Name:          in.Body.Name,
			Slug:          slug,
			Description:   nullString(in.Body.Description),
			LogoUrl:       nullString(in.Body.LogoUrl),
			PzzClubNumber: nullString(in.Body.PzzClubNumber),
			City:          nullString(in.Body.City),
			Website:       nullString(in.Body.Website),
		})
		if txErr != nil {
			return txErr
		}
		_, txErr = q.AddOrgMember(ctx, sqlcdb.AddOrgMemberParams{
			OrgID:  org.ID,
			UserID: user.UserID,
			Role:   "admin",
		})
		return txErr
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("slug already taken")
		}
		slog.Error("create organization", "user_id", user.UserID, "slug", slug, "err", err)
		return nil, huma.Error500InternalServerError("failed to create organization")
	}
	return &orgOutput{Body: dto.OrganizationFromDB(org)}, nil
}

func (h *OrgHandler) get(ctx context.Context, in *orgSlugParam) (*orgOutput, error) {
	if _, err := resolveOrg(ctx, h.q, in.Slug, false); err != nil {
		return nil, err
	}
	org, err := h.q.GetOrganizationBySlug(ctx, in.Slug)
	if err != nil {
		slog.Error("get organization", "slug", in.Slug, "err", err)
		return nil, huma.Error500InternalServerError("failed to get organization")
	}
	return &orgOutput{Body: dto.OrganizationFromDB(org)}, nil
}

func (h *OrgHandler) update(ctx context.Context, in *updateOrgInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateOrganization(ctx, sqlcdb.UpdateOrganizationParams{
		Name:          in.Body.Name,
		Description:   nullString(in.Body.Description),
		LogoUrl:       nullString(in.Body.LogoUrl),
		PzzClubNumber: nullString(in.Body.PzzClubNumber),
		City:          nullString(in.Body.City),
		Website:       nullString(in.Body.Website),
		ID:            octx.OrgID,
	}); err != nil {
		slog.Error("update organization", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update organization")
	}
	return &noContentOutput{}, nil
}

func (h *OrgHandler) delete(ctx context.Context, in *orgSlugParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrganization(ctx, octx.OrgID); err != nil {
		slog.Error("delete organization", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete organization")
	}
	return &noContentOutput{}, nil
}

func (h *OrgHandler) listMembers(ctx context.Context, in *orgSlugParam) (*orgMembersOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	members, err := h.q.ListOrgMembers(ctx, octx.OrgID)
	if err != nil {
		slog.Error("list org members", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list members")
	}
	return &orgMembersOutput{Body: dto.OrgMembersFromDB(members)}, nil
}

func (h *OrgHandler) updateMemberRole(ctx context.Context, in *memberRoleInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if in.Body.Role != "admin" {
		members, err := h.q.ListOrgMembers(ctx, octx.OrgID)
		if err != nil {
			slog.Error("list org members for role check", "org_id", octx.OrgID, "err", err)
			return nil, huma.Error500InternalServerError("failed to check members")
		}
		var isAdmin bool
		for i := range members {
			if members[i].ID == in.MemberID && members[i].Role == "admin" {
				isAdmin = true
				break
			}
		}
		if isAdmin {
			count, err := h.q.CountOrgAdmins(ctx, octx.OrgID)
			if err != nil {
				slog.Error("count org admins", "org_id", octx.OrgID, "err", err)
				return nil, huma.Error500InternalServerError("failed to check admins")
			}
			if count <= 1 {
				return nil, huma.Error422UnprocessableEntity("cannot demote the last admin")
			}
		}
	}
	if err := h.q.UpdateOrgMemberRole(ctx, sqlcdb.UpdateOrgMemberRoleParams{
		Role:  in.Body.Role,
		ID:    in.MemberID,
		OrgID: octx.OrgID,
	}); err != nil {
		slog.Error("update org member role", "org_id", octx.OrgID, "member_id", in.MemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update role")
	}
	return &noContentOutput{}, nil
}

func (h *OrgHandler) removeMember(ctx context.Context, in *memberParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	count, err := h.q.CountOrgAdmins(ctx, octx.OrgID)
	if err != nil {
		slog.Error("count org admins", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to check admins")
	}
	members, err := h.q.ListOrgMembers(ctx, octx.OrgID)
	if err != nil {
		slog.Error("list org members", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list members")
	}
	for i := range members {
		if members[i].ID == in.MemberID && members[i].Role == "admin" && count <= 1 {
			return nil, huma.Error422UnprocessableEntity("cannot remove the last admin")
		}
	}
	if err := h.q.RemoveOrgMember(ctx, sqlcdb.RemoveOrgMemberParams{ID: in.MemberID, OrgID: octx.OrgID}); err != nil {
		slog.Error("remove org member", "org_id", octx.OrgID, "member_id", in.MemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove member")
	}
	return &noContentOutput{}, nil
}

func (h *OrgHandler) createInvite(ctx context.Context, in *createInviteInput) (*orgInviteOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)

	role := in.Body.Role
	switch role {
	case "admin", "captain", "crew":
	default:
		role = "crew"
	}
	if in.Body.ExpiresInHours != nil && *in.Body.ExpiresInHours <= 0 {
		return nil, huma.Error422UnprocessableEntity("expires_in_hours must be greater than 0")
	}
	if in.Body.MaxUses != nil && *in.Body.MaxUses <= 0 {
		return nil, huma.Error422UnprocessableEntity("max_uses must be greater than 0")
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, huma.Error500InternalServerError("failed to generate token")
	}

	var expiresAt types.NullTime
	if in.Body.ExpiresInHours != nil {
		expiresAt = types.NullTime{Time: time.Now().Add(time.Duration(*in.Body.ExpiresInHours) * time.Hour), Valid: true}
	}

	invite, err := h.q.CreateOrgInvite(ctx, sqlcdb.CreateOrgInviteParams{
		OrgID:     octx.OrgID,
		Token:     hex.EncodeToString(b),
		Role:      role,
		CreatedBy: user.UserID,
		ExpiresAt: expiresAt,
		MaxUses:   nullInt64(in.Body.MaxUses),
	})
	if err != nil {
		slog.Error("create org invite", "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to create invite")
	}
	return &orgInviteOutput{Body: dto.OrgInviteFromDB(invite)}, nil
}

func (h *OrgHandler) listInvites(ctx context.Context, in *orgSlugParam) (*orgInvitesOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	invites, err := h.q.ListOrgInvites(ctx, octx.OrgID)
	if err != nil {
		slog.Error("list org invites", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list invites")
	}
	return &orgInvitesOutput{Body: dto.OrgInvitesFromDB(invites)}, nil
}

func (h *OrgHandler) deleteInvite(ctx context.Context, in *inviteParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgInvite(ctx, sqlcdb.DeleteOrgInviteParams{ID: in.InviteID, OrgID: octx.OrgID}); err != nil {
		slog.Error("delete org invite", "org_id", octx.OrgID, "invite_id", in.InviteID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete invite")
	}
	return &noContentOutput{}, nil
}

func (h *OrgHandler) getInviteInfo(ctx context.Context, in *joinTokenParam) (*inviteInfoOutput, error) {
	invite, err := h.q.GetOrgInviteByToken(ctx, in.Token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid invite link")
		}
		slog.Error("get org invite info", "err", err)
		return nil, huma.Error500InternalServerError("failed to get invite")
	}
	if invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()) {
		return nil, huma.Error410Gone("invite has expired")
	}
	if invite.MaxUses.Valid && invite.UseCount >= invite.MaxUses.Int64 {
		return nil, huma.Error410Gone("invite has reached maximum uses")
	}

	user := middleware.GetUser(ctx)
	_, memberErr := h.q.GetOrgMembership(ctx, sqlcdb.GetOrgMembershipParams{OrgID: invite.OrgID, UserID: user.UserID})
	return &inviteInfoOutput{Body: dto.InviteInfo{
		OrgName:       invite.OrgName,
		OrgSlug:       invite.OrgSlug,
		Role:          invite.Role,
		AlreadyMember: memberErr == nil,
	}}, nil
}

func (h *OrgHandler) acceptInvite(ctx context.Context, in *joinTokenParam) (*inviteAcceptOutput, error) {
	user := middleware.GetUser(ctx)
	invite, err := h.q.GetOrgInviteByToken(ctx, in.Token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("invalid invite link")
		}
		slog.Error("get org invite by token", "err", err)
		return nil, huma.Error500InternalServerError("failed to get invite")
	}
	if invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()) {
		return nil, huma.Error410Gone("invite has expired")
	}

	// Reject an existing member before claiming a use, so a repeated accept
	// does not burn the invite's use count.
	if _, err := h.q.GetOrgMembership(ctx, sqlcdb.GetOrgMembershipParams{
		OrgID:  invite.OrgID,
		UserID: user.UserID,
	}); err == nil {
		return nil, huma.Error409Conflict("already a member")
	} else if !errors.Is(err, sql.ErrNoRows) {
		slog.Error("check existing membership", "org_id", invite.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to join organization")
	}

	// Claim the invite use and add the member in one transaction: a failed
	// member write rolls back the consumed use count.
	err = h.runTx(ctx, func(q sqlcdb.Querier) error {
		if invite.MaxUses.Valid {
			rows, txErr := q.IncrementInviteUseCount(ctx, invite.ID)
			if txErr != nil {
				return txErr
			}
			if rows == 0 {
				return errInviteExhausted
			}
		}
		_, txErr := q.AddOrgMember(ctx, sqlcdb.AddOrgMemberParams{
			OrgID:  invite.OrgID,
			UserID: user.UserID,
			Role:   invite.Role,
		})
		return txErr
	})
	if err != nil {
		if errors.Is(err, errInviteExhausted) {
			return nil, huma.Error410Gone("invite has reached maximum uses")
		}
		if isUniqueViolation(err) {
			return nil, huma.Error409Conflict("already a member")
		}
		slog.Error("accept org invite", "org_id", invite.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to join organization")
	}
	return &inviteAcceptOutput{Body: dto.InviteAcceptResult{
		OrgName: invite.OrgName,
		OrgSlug: invite.OrgSlug,
		Role:    invite.Role,
	}}, nil
}
