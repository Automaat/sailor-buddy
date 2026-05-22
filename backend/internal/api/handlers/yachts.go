package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type YachtHandler struct {
	*crudHandlers[pageParams, yachtIDParam, createYachtInput, updateYachtInput, yachtIDParam, sqlcdb.Yacht, yachtListOutput, yachtOutput]
}

func NewYachtHandler(q sqlcdb.Querier) *YachtHandler {
	return &YachtHandler{crudHandlers: newCRUDHandlers(yachtCRUDConfig(q))}
}

type yachtIDParam struct {
	ID int64 `path:"yachtID" doc:"Yacht ID"`
}

type createYachtInput struct {
	Body dto.YachtBody
}

type updateYachtInput struct {
	ID   int64 `path:"yachtID" doc:"Yacht ID"`
	Body dto.YachtBody
}

type yachtOutput struct {
	Body dto.Yacht
}

type yachtListOutput struct {
	Body dto.Page[dto.Yacht]
}

// RegisterYachtRoutes wires the club yacht operations onto the API. Reads are
// open to any member; mutations require an admin.
func RegisterYachtRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewYachtHandler(q)
	tag := []string{"Yachts"}

	huma.Register(api, huma.Operation{
		OperationID: "list-yachts", Method: http.MethodGet, Path: "/yachts",
		Summary: "List yachts", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-yacht", Method: http.MethodGet, Path: "/yachts/{yachtID}",
		Summary: "Get a yacht", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-yacht", Method: http.MethodPost, Path: "/yachts",
		Summary: "Create a yacht (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-yacht", Method: http.MethodPut, Path: "/yachts/{yachtID}",
		Summary: "Update a yacht (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-yacht", Method: http.MethodDelete, Path: "/yachts/{yachtID}",
		Summary: "Delete a yacht (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func yachtCRUDConfig(q sqlcdb.Querier) crudConfig[pageParams, yachtIDParam, createYachtInput, updateYachtInput, yachtIDParam, sqlcdb.Yacht, yachtListOutput, yachtOutput] {
	return crudConfig[pageParams, yachtIDParam, createYachtInput, updateYachtInput, yachtIDParam, sqlcdb.Yacht, yachtListOutput, yachtOutput]{
		listScope: func(ctx context.Context, _ *pageParams) (crudScope, error) {
			return memberScope(ctx)
		},
		getScope: func(ctx context.Context, _ *yachtIDParam) (crudScope, error) {
			return memberScope(ctx)
		},
		createScope: func(ctx context.Context, _ *createYachtInput) (crudScope, error) {
			return adminScope(ctx)
		},
		updateScope: func(ctx context.Context, _ *updateYachtInput) (crudScope, error) {
			return adminScope(ctx)
		},
		deleteScope: func(ctx context.Context, _ *yachtIDParam) (crudScope, error) {
			return adminScope(ctx)
		},
		list: func(ctx context.Context, _ crudScope, in *pageParams) ([]sqlcdb.Yacht, error) {
			return q.ListYachts(ctx, sqlcdb.ListYachtsParams{Limit: in.Limit, Offset: in.Offset})
		},
		count: func(ctx context.Context, _ crudScope, _ *pageParams) (int64, error) {
			return q.CountYachts(ctx)
		},
		get: func(ctx context.Context, _ crudScope, in *yachtIDParam) (sqlcdb.Yacht, error) {
			return q.GetYacht(ctx, in.ID)
		},
		create: func(ctx context.Context, scope crudScope, in *createYachtInput) (sqlcdb.Yacht, error) {
			return q.CreateYacht(ctx, sqlcdb.CreateYachtParams{
				CreatedBy:      types.NullInt64{Int64: scope.userID, Valid: true},
				Name:           in.Body.Name,
				RegistrationNo: nullString(in.Body.RegistrationNo),
				YachtType:      nullString(in.Body.YachtType),
			})
		},
		update: func(ctx context.Context, _ crudScope, in *updateYachtInput) error {
			return q.UpdateYacht(ctx, sqlcdb.UpdateYachtParams{
				Name:           in.Body.Name,
				RegistrationNo: nullString(in.Body.RegistrationNo),
				YachtType:      nullString(in.Body.YachtType),
				ID:             in.ID,
			})
		},
		delete: func(ctx context.Context, _ crudScope, in *yachtIDParam) error {
			return q.DeleteYacht(ctx, in.ID)
		},
		listOutput: func(in *pageParams, rows []sqlcdb.Yacht, total int64) *yachtListOutput {
			return &yachtListOutput{Body: dto.NewPage(dto.YachtsFromDB(rows), total, in.Limit, in.Offset)}
		},
		itemOutput: func(row sqlcdb.Yacht) *yachtOutput {
			return &yachtOutput{Body: dto.YachtFromDB(row)}
		},
		listLogAttrs: func(scope crudScope, _ *pageParams) []any {
			return scopeAttrs(scope)
		},
		getLogAttrs: func(scope crudScope, in *yachtIDParam) []any {
			return scopeAttrs(scope, "yacht_id", in.ID)
		},
		createLogAttrs: func(scope crudScope, in *createYachtInput) []any {
			return scopeAttrs(scope, "name", in.Body.Name)
		},
		updateLogAttrs: func(scope crudScope, in *updateYachtInput) []any {
			return scopeAttrs(scope, "yacht_id", in.ID)
		},
		deleteLogAttrs: func(scope crudScope, in *yachtIDParam) []any {
			return scopeAttrs(scope, "yacht_id", in.ID)
		},
		listLogMsg:      "list yachts",
		getLogMsg:       "get yacht",
		createLogMsg:    "create yacht",
		updateLogMsg:    "update yacht",
		deleteLogMsg:    "delete yacht",
		listClientMsg:   "failed to list yachts",
		getClientMsg:    "failed to get yacht",
		createClientMsg: "failed to create yacht",
		updateClientMsg: "failed to update yacht",
		deleteClientMsg: "failed to delete yacht",
		notFoundMsg:     "yacht not found",
	}
}
