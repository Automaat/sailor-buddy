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

type crudScope struct {
	userID   int64
	orgID    types.NullInt64
	logAttrs []any
}

func ownerCRUDScope(ctx context.Context) crudScope {
	user := middleware.GetUser(ctx)
	return crudScope{userID: user.UserID, logAttrs: []any{"user_id", user.UserID}}
}

func orgCRUDScope(ctx context.Context, q sqlcdb.Querier, slug string, requireAdmin bool) (crudScope, error) {
	octx, err := resolveOrg(ctx, q, slug, requireAdmin)
	if err != nil {
		return crudScope{}, err
	}
	user := middleware.GetUser(ctx)
	return crudScope{
		userID:   user.UserID,
		orgID:    orgID(octx),
		logAttrs: []any{"org_id", octx.OrgID},
	}, nil
}

// orgID wraps an org context's ID as the nullable column the org queries take.
func orgID(octx *middleware.OrgContext) types.NullInt64 {
	return types.NullInt64{Int64: octx.OrgID, Valid: true}
}

type crudConfig[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut any] struct {
	listScope   func(context.Context, *ListIn) (crudScope, error)
	getScope    func(context.Context, *GetIn) (crudScope, error)
	createScope func(context.Context, *CreateIn) (crudScope, error)
	updateScope func(context.Context, *UpdateIn) (crudScope, error)
	deleteScope func(context.Context, *DeleteIn) (crudScope, error)

	list   func(context.Context, crudScope, *ListIn) ([]Row, error)
	get    func(context.Context, crudScope, *GetIn) (Row, error)
	create func(context.Context, crudScope, *CreateIn) (Row, error)
	update func(context.Context, crudScope, *UpdateIn) error
	delete func(context.Context, crudScope, *DeleteIn) error

	listOutput func([]Row) *ListOut
	itemOutput func(Row) *ItemOut

	listLogAttrs   func(crudScope, *ListIn) []any
	getLogAttrs    func(crudScope, *GetIn) []any
	createLogAttrs func(crudScope, *CreateIn) []any
	updateLogAttrs func(crudScope, *UpdateIn) []any
	deleteLogAttrs func(crudScope, *DeleteIn) []any

	listLogMsg   string
	getLogMsg    string
	createLogMsg string
	updateLogMsg string
	deleteLogMsg string

	listClientMsg   string
	getClientMsg    string
	createClientMsg string
	updateClientMsg string
	deleteClientMsg string
	notFoundMsg     string
}

type crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut any] struct {
	cfg crudConfig[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]
}

func newCRUDHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut any](
	cfg crudConfig[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut],
) *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut] {
	return &crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]{cfg: cfg}
}

func (h *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]) list(
	ctx context.Context,
	in *ListIn,
) (*ListOut, error) {
	scope, err := h.cfg.listScope(ctx, in)
	if err != nil {
		return nil, err
	}
	rows, err := h.cfg.list(ctx, scope, in)
	if err != nil {
		logCRUD(h.cfg.listLogMsg, h.cfg.listLogAttrs(scope, in), err)
		return nil, huma.Error500InternalServerError(h.cfg.listClientMsg)
	}
	return h.cfg.listOutput(rows), nil
}

func (h *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]) get(
	ctx context.Context,
	in *GetIn,
) (*ItemOut, error) {
	scope, err := h.cfg.getScope(ctx, in)
	if err != nil {
		return nil, err
	}
	row, err := h.cfg.get(ctx, scope, in)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound(h.cfg.notFoundMsg)
		}
		logCRUD(h.cfg.getLogMsg, h.cfg.getLogAttrs(scope, in), err)
		return nil, huma.Error500InternalServerError(h.cfg.getClientMsg)
	}
	return h.cfg.itemOutput(row), nil
}

func (h *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]) create(
	ctx context.Context,
	in *CreateIn,
) (*ItemOut, error) {
	scope, err := h.cfg.createScope(ctx, in)
	if err != nil {
		return nil, err
	}
	row, err := h.cfg.create(ctx, scope, in)
	if err != nil {
		logCRUD(h.cfg.createLogMsg, h.cfg.createLogAttrs(scope, in), err)
		return nil, huma.Error500InternalServerError(h.cfg.createClientMsg)
	}
	return h.cfg.itemOutput(row), nil
}

func (h *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]) update(
	ctx context.Context,
	in *UpdateIn,
) (*noContentOutput, error) {
	scope, err := h.cfg.updateScope(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := h.cfg.update(ctx, scope, in); err != nil {
		logCRUD(h.cfg.updateLogMsg, h.cfg.updateLogAttrs(scope, in), err)
		return nil, huma.Error500InternalServerError(h.cfg.updateClientMsg)
	}
	return &noContentOutput{}, nil
}

func (h *crudHandlers[ListIn, GetIn, CreateIn, UpdateIn, DeleteIn, Row, ListOut, ItemOut]) delete(
	ctx context.Context,
	in *DeleteIn,
) (*noContentOutput, error) {
	scope, err := h.cfg.deleteScope(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := h.cfg.delete(ctx, scope, in); err != nil {
		logCRUD(h.cfg.deleteLogMsg, h.cfg.deleteLogAttrs(scope, in), err)
		return nil, huma.Error500InternalServerError(h.cfg.deleteClientMsg)
	}
	return &noContentOutput{}, nil
}

func logCRUD(msg string, attrs []any, err error) {
	slog.Error(msg, append(attrs, "err", err)...)
}

func scopeAttrs(scope crudScope, attrs ...any) []any {
	out := make([]any, 0, len(scope.logAttrs)+len(attrs))
	out = append(out, scope.logAttrs...)
	out = append(out, attrs...)
	return out
}
