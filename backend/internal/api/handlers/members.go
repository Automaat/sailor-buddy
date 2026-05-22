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

type updateMemberRoleInput struct {
	UserID int64 `path:"userID" doc:"User ID"`
	Body   dto.RoleBody
}

// RegisterMembersRoutes wires the club member roster onto the API. The roster
// is visible to any member; role changes require an admin.
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
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	target, err := h.q.GetUserByID(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("member not found")
		}
		slog.Error("get member for role change", "user_id", in.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get member")
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
