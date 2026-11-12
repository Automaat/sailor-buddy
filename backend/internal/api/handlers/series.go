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
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type SeriesHandler struct {
	*crudHandlers[pageParams, seriesIDParam, createSeriesInput, updateSeriesInput, seriesIDParam, sqlcdb.CruiseSeries, seriesListOutput, seriesOutput]
	q sqlcdb.Querier
}

func NewSeriesHandler(q sqlcdb.Querier) *SeriesHandler {
	return &SeriesHandler{crudHandlers: newCRUDHandlers(seriesCRUDConfig(q)), q: q}
}

type seriesIDParam struct {
	ID int64 `path:"seriesID" doc:"Series ID"`
}

type createSeriesInput struct {
	Body dto.CruiseSeriesBody
}

type updateSeriesInput struct {
	ID   int64 `path:"seriesID" doc:"Series ID"`
	Body dto.CruiseSeriesBody
}

type seriesOutput struct {
	Body dto.CruiseSeries
}

type seriesListOutput struct {
	Body dto.Page[dto.CruiseSeries]
}

type seriesEditionsOutput struct {
	Body []dto.CruiseSeriesEdition
}

// RegisterSeriesRoutes wires the cruise series operations onto the API.
// Reads are open to any member; mutations require an admin.
func RegisterSeriesRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewSeriesHandler(q)
	tag := []string{"Series"}

	huma.Register(api, huma.Operation{
		OperationID: "list-series", Method: http.MethodGet, Path: "/series",
		Summary: "List cruise series", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-series", Method: http.MethodGet, Path: "/series/{seriesID}",
		Summary: "Get a cruise series", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-series", Method: http.MethodPost, Path: "/series",
		Summary: "Create a cruise series (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-series", Method: http.MethodPut, Path: "/series/{seriesID}",
		Summary: "Update a cruise series (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-series", Method: http.MethodDelete, Path: "/series/{seriesID}",
		Summary: "Delete a cruise series (admin); its cruises are kept and unlinked", Tags: tag,
		DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "list-series-editions", Method: http.MethodGet, Path: "/series/{seriesID}/editions",
		Summary: "List a series' editions, newest first, with voyage totals", Tags: tag,
	}, h.listEditions)
}

func (h *SeriesHandler) listEditions(ctx context.Context, in *seriesIDParam) (*seriesEditionsOutput, error) {
	if _, err := h.q.GetCruiseSeries(ctx, in.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("series not found")
		}
		slog.Error("verify series", "series_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list editions")
	}
	rows, err := h.q.ListCruiseSeriesEditions(ctx, types.NullInt64{Int64: in.ID, Valid: true})
	if err != nil {
		slog.Error("list series editions", "series_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list editions")
	}
	return &seriesEditionsOutput{Body: dto.CruiseSeriesEditionsFromDB(rows)}, nil
}

func seriesCRUDConfig(q sqlcdb.Querier) crudConfig[pageParams, seriesIDParam, createSeriesInput, updateSeriesInput, seriesIDParam, sqlcdb.CruiseSeries, seriesListOutput, seriesOutput] {
	return crudConfig[pageParams, seriesIDParam, createSeriesInput, updateSeriesInput, seriesIDParam, sqlcdb.CruiseSeries, seriesListOutput, seriesOutput]{
		listScope: func(ctx context.Context, _ *pageParams) (crudScope, error) {
			return memberScope(ctx)
		},
		getScope: func(ctx context.Context, _ *seriesIDParam) (crudScope, error) {
			return memberScope(ctx)
		},
		createScope: func(ctx context.Context, _ *createSeriesInput) (crudScope, error) {
			return adminScope(ctx)
		},
		updateScope: func(ctx context.Context, _ *updateSeriesInput) (crudScope, error) {
			return adminScope(ctx)
		},
		deleteScope: func(ctx context.Context, _ *seriesIDParam) (crudScope, error) {
			return adminScope(ctx)
		},
		list: func(ctx context.Context, _ crudScope, in *pageParams) ([]sqlcdb.CruiseSeries, error) {
			return q.ListCruiseSeries(ctx, sqlcdb.ListCruiseSeriesParams{Limit: in.Limit, Offset: in.Offset})
		},
		count: func(ctx context.Context, _ crudScope, _ *pageParams) (int64, error) {
			return q.CountCruiseSeries(ctx)
		},
		get: func(ctx context.Context, _ crudScope, in *seriesIDParam) (sqlcdb.CruiseSeries, error) {
			return q.GetCruiseSeries(ctx, in.ID)
		},
		create: func(ctx context.Context, scope crudScope, in *createSeriesInput) (sqlcdb.CruiseSeries, error) {
			return q.CreateCruiseSeries(ctx, sqlcdb.CreateCruiseSeriesParams{
				CreatedBy:   types.NullInt64{Int64: scope.userID, Valid: true},
				Name:        in.Body.Name,
				Description: nullString(in.Body.Description),
			})
		},
		update: func(ctx context.Context, _ crudScope, in *updateSeriesInput) error {
			return q.UpdateCruiseSeries(ctx, sqlcdb.UpdateCruiseSeriesParams{
				Name:        in.Body.Name,
				Description: nullString(in.Body.Description),
				ID:          in.ID,
			})
		},
		delete: func(ctx context.Context, _ crudScope, in *seriesIDParam) error {
			return q.DeleteCruiseSeries(ctx, in.ID)
		},
		listOutput: func(in *pageParams, rows []sqlcdb.CruiseSeries, total int64) *seriesListOutput {
			return &seriesListOutput{Body: dto.NewPage(dto.CruiseSeriesListFromDB(rows), total, in.Limit, in.Offset)}
		},
		itemOutput: func(row sqlcdb.CruiseSeries) *seriesOutput {
			return &seriesOutput{Body: dto.CruiseSeriesFromDB(row)}
		},
		listLogAttrs: func(scope crudScope, _ *pageParams) []any {
			return scopeAttrs(scope)
		},
		getLogAttrs: func(scope crudScope, in *seriesIDParam) []any {
			return scopeAttrs(scope, "series_id", in.ID)
		},
		createLogAttrs: func(scope crudScope, in *createSeriesInput) []any {
			return scopeAttrs(scope, "name", in.Body.Name)
		},
		updateLogAttrs: func(scope crudScope, in *updateSeriesInput) []any {
			return scopeAttrs(scope, "series_id", in.ID)
		},
		deleteLogAttrs: func(scope crudScope, in *seriesIDParam) []any {
			return scopeAttrs(scope, "series_id", in.ID)
		},
		listLogMsg:      "list series",
		getLogMsg:       "get series",
		createLogMsg:    "create series",
		updateLogMsg:    "update series",
		deleteLogMsg:    "delete series",
		listClientMsg:   "failed to list series",
		getClientMsg:    "failed to get series",
		createClientMsg: "failed to create series",
		updateClientMsg: "failed to update series",
		deleteClientMsg: "failed to delete series",
		notFoundMsg:     "series not found",
	}
}
