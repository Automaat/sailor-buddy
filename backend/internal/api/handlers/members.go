package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type MembersHandler struct {
	q sqlcdb.Querier
}

func NewMembersHandler(q sqlcdb.Querier) *MembersHandler {
	return &MembersHandler{q: q}
}

type membersListOutput struct {
	Body []dto.Member
}

type updateMemberCaptainInput struct {
	UserID int64 `path:"userID" doc:"User ID"`
	Body   dto.CaptainBody
}

type updateMemberMembershipInput struct {
	UserID int64 `path:"userID" doc:"User ID"`
	Body   dto.MembershipBody
}

type updateMemberRoleInput struct {
	UserID int64 `path:"userID" doc:"User ID"`
	Body   dto.RoleBody
}

// RegisterMembersRoutes wires the club member roster onto the API. The roster
// is visible to any member; role, captain-group and membership changes require
// an admin.
func RegisterMembersRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewMembersHandler(q)
	tag := []string{"Members"}

	huma.Register(api, huma.Operation{
		OperationID: "list-members", Method: http.MethodGet, Path: "/members",
		Summary: "List club members", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "update-member-role", Method: http.MethodPut, Path: "/members/{userID}/role",
		Summary: "Change a member's role (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateRole)
	huma.Register(api, huma.Operation{
		OperationID: "update-member-captain", Method: http.MethodPut, Path: "/members/{userID}/captain",
		Summary: "Add or remove a member from the captain group (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateCaptain)
	huma.Register(api, huma.Operation{
		OperationID: "update-member-membership", Method: http.MethodPut, Path: "/members/{userID}/membership",
		Summary: "Change a member's membership status (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateMembership)
}

func (h *MembersHandler) getTarget(ctx context.Context, userID int64) (sqlcdb.User, error) {
	if err := requireAdmin(ctx); err != nil {
		return sqlcdb.User{}, err
	}
	target, err := h.q.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlcdb.User{}, huma.Error404NotFound("member not found")
		}
		slog.Error("get member", "user_id", userID, "err", err)
		return sqlcdb.User{}, huma.Error500InternalServerError("failed to get member")
	}
	return target, nil
}

func (h *MembersHandler) updateCaptain(ctx context.Context, in *updateMemberCaptainInput) (*noContentOutput, error) {
	target, err := h.getTarget(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if in.Body.IsCaptain {
		if err := captainPrerequisites(target); err != nil {
			return nil, err
		}
	}
	if err := h.q.UpdateUserCaptain(ctx, sqlcdb.UpdateUserCaptainParams{IsCaptain: in.Body.IsCaptain, ID: in.UserID}); err != nil {
		slog.Error("update member captain", "user_id", in.UserID, "is_captain", in.Body.IsCaptain, "err", err)
		return nil, huma.Error500InternalServerError("failed to update captain status")
	}
	return &noContentOutput{}, nil
}

func (h *MembersHandler) updateMembership(ctx context.Context, in *updateMemberMembershipInput) (*noContentOutput, error) {
	target, err := h.getTarget(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if target.IsCaptain && in.Body.Membership != "czlonek" {
		return nil, huma.Error422UnprocessableEntity("remove the member from the captain group first")
	}
	if err := h.q.UpdateUserMembership(ctx, sqlcdb.UpdateUserMembershipParams{Membership: in.Body.Membership, ID: in.UserID}); err != nil {
		slog.Error("update member membership", "user_id", in.UserID, "membership", in.Body.Membership, "err", err)
		return nil, huma.Error500InternalServerError("failed to update membership")
	}
	return &noContentOutput{}, nil
}

func (h *MembersHandler) list(ctx context.Context, _ *struct{}) (*membersListOutput, error) {
	users, err := h.q.ListUsers(ctx)
	if err != nil {
		slog.Error("list members", "err", err)
		return nil, huma.Error500InternalServerError("failed to list members")
	}
	return &membersListOutput{Body: dto.MembersFromDB(users)}, nil
}

func (h *MembersHandler) updateRole(ctx context.Context, in *updateMemberRoleInput) (*noContentOutput, error) {
	target, err := h.getTarget(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	// Refuse to demote the last admin — the club must always have one.
	if target.Role == "admin" && in.Body.Role != "admin" {
		count, err := h.q.CountAdmins(ctx)
		if err != nil {
			slog.Error("count admins", "err", err)
			return nil, huma.Error500InternalServerError("failed to check admins")
		}
		if count <= 1 {
			return nil, huma.Error400BadRequest("cannot demote the last admin")
		}
	}
	if err := h.q.UpdateUserRole(ctx, sqlcdb.UpdateUserRoleParams{Role: in.Body.Role, ID: in.UserID}); err != nil {
		slog.Error("update member role", "user_id", in.UserID, "role", in.Body.Role, "err", err)
		return nil, huma.Error500InternalServerError("failed to update role")
	}
	return &noContentOutput{}, nil
}
