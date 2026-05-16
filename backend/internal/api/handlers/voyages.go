package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type VoyageHandler struct {
	*crudHandlers[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]
}

func NewVoyageHandler(q sqlcdb.Querier) *VoyageHandler {
	return &VoyageHandler{crudHandlers: newCRUDHandlers(voyageCRUDConfig(q))}
}

type voyageIDParam struct {
	ID int64 `path:"voyageID" doc:"Voyage ID"`
}

type createVoyageInput struct {
	Body dto.VoyageBody
}

type updateVoyageInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageBody
}

type voyageListOutput struct {
	Body dto.Page[dto.Voyage]
}

// cruiseVoyagesOutput is the unpaginated array body for a cruise's child voyages.
type cruiseVoyagesOutput struct {
	Body []dto.Voyage
}

// RegisterVoyageRoutes wires the owner-scoped voyage operations onto the API.
func RegisterVoyageRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewVoyageHandler(q)
	tag := []string{"Voyages"}

	huma.Register(api, huma.Operation{
		OperationID: "list-voyages", Method: http.MethodGet, Path: "/voyages",
		Summary: "List voyages", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-voyage", Method: http.MethodGet, Path: "/voyages/{voyageID}",
		Summary: "Get a voyage", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-voyage", Method: http.MethodPost, Path: "/voyages",
		Summary: "Create a voyage", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-voyage", Method: http.MethodPut, Path: "/voyages/{voyageID}",
		Summary: "Update a voyage", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-voyage", Method: http.MethodDelete, Path: "/voyages/{voyageID}",
		Summary: "Delete a voyage", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func voyageCRUDConfig(q sqlcdb.Querier) crudConfig[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput] {
	ownerScope := func(ctx context.Context, _ any) (crudScope, error) {
		return ownerCRUDScope(ctx), nil
	}
	return crudConfig[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]{
		listScope: func(ctx context.Context, in *pageParams) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		getScope: func(ctx context.Context, in *voyageIDParam) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		createScope: func(ctx context.Context, in *createVoyageInput) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		updateScope: func(ctx context.Context, in *updateVoyageInput) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		deleteScope: func(ctx context.Context, in *voyageIDParam) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		list: func(ctx context.Context, scope crudScope, in *pageParams) ([]sqlcdb.Voyage, error) {
			return q.ListVoyages(ctx, sqlcdb.ListVoyagesParams{
				OwnerID: scope.userID,
				Limit:   in.Limit,
				Offset:  in.Offset,
			})
		},
		count: func(ctx context.Context, scope crudScope, _ *pageParams) (int64, error) {
			return q.CountVoyages(ctx, scope.userID)
		},
		get: func(ctx context.Context, scope crudScope, in *voyageIDParam) (sqlcdb.Voyage, error) {
			return q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.ID, OwnerID: scope.userID})
		},
		create: func(ctx context.Context, scope crudScope, in *createVoyageInput) (sqlcdb.Voyage, error) {
			return q.CreateVoyage(ctx, createVoyageParams(scope, in.Body))
		},
		update: func(ctx context.Context, scope crudScope, in *updateVoyageInput) error {
			params := updateVoyageParams(scope, in.Body)
			params.ID = in.ID
			return q.UpdateVoyage(ctx, params)
		},
		delete: func(ctx context.Context, scope crudScope, in *voyageIDParam) error {
			return q.DeleteVoyage(ctx, sqlcdb.DeleteVoyageParams{ID: in.ID, OwnerID: scope.userID})
		},
		listOutput: func(in *pageParams, rows []sqlcdb.Voyage, total int64) *voyageListOutput {
			return &voyageListOutput{Body: dto.NewPage(dto.VoyagesFromDB(rows), total, in.Limit, in.Offset)}
		},
		itemOutput: func(row sqlcdb.Voyage) *voyageOutput {
			return &voyageOutput{Body: dto.VoyageFromDB(row)}
		},
		listLogAttrs: func(scope crudScope, _ *pageParams) []any {
			return scopeAttrs(scope)
		},
		getLogAttrs: func(scope crudScope, in *voyageIDParam) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		createLogAttrs: func(scope crudScope, in *createVoyageInput) []any {
			return scopeAttrs(scope, "name", in.Body.Name)
		},
		updateLogAttrs: func(scope crudScope, in *updateVoyageInput) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		deleteLogAttrs: func(scope crudScope, in *voyageIDParam) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		listLogMsg:      "list voyages",
		getLogMsg:       "get voyage",
		createLogMsg:    "create voyage",
		updateLogMsg:    "update voyage",
		deleteLogMsg:    "delete voyage",
		listClientMsg:   "failed to list voyages",
		getClientMsg:    "failed to get voyage",
		createClientMsg: "failed to create voyage",
		updateClientMsg: "failed to update voyage",
		deleteClientMsg: "failed to delete voyage",
		notFoundMsg:     "voyage not found",
	}
}

func createVoyageParams(scope crudScope, body dto.VoyageBody) sqlcdb.CreateVoyageParams {
	return sqlcdb.CreateVoyageParams{
		OwnerID:       scope.userID,
		Name:          body.Name,
		Year:          nullInt64(body.Year),
		EmbarkDate:    nullString(body.EmbarkDate),
		DisembarkDate: nullString(body.DisembarkDate),
		Countries:     nullString(body.Countries),
		StartPort:     nullString(body.StartPort),
		EndPort:       nullString(body.EndPort),
		CaptainName:   nullString(body.CaptainName),
		YachtID:       nullInt64(body.YachtID),
		HoursTotal:    valOrZeroFloat(body.HoursTotal),
		HoursSail:     valOrZeroFloat(body.HoursSail),
		HoursEngine:   valOrZeroFloat(body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(body.HoursOver6bf),
		Miles:         valOrZeroFloat(body.Miles),
		Days:          valOrZeroInt(body.Days),
		TidalWaters:   valOrZeroInt(body.TidalWaters),
		CostTotal:     nullFloat64(body.CostTotal),
		CostPerPerson: nullFloat64(body.CostPerPerson),
		ImageLogoUrl:  nullString(body.ImageLogoUrl),
		ImagePhotoUrl: nullString(body.ImagePhotoUrl),
		ImageRouteUrl: nullString(body.ImageRouteUrl),
		Description:   nullString(body.Description),
	}
}

func updateVoyageParams(scope crudScope, body dto.VoyageBody) sqlcdb.UpdateVoyageParams {
	return sqlcdb.UpdateVoyageParams{
		Name:          body.Name,
		Year:          nullInt64(body.Year),
		EmbarkDate:    nullString(body.EmbarkDate),
		DisembarkDate: nullString(body.DisembarkDate),
		Countries:     nullString(body.Countries),
		StartPort:     nullString(body.StartPort),
		EndPort:       nullString(body.EndPort),
		CaptainName:   nullString(body.CaptainName),
		YachtID:       nullInt64(body.YachtID),
		HoursTotal:    valOrZeroFloat(body.HoursTotal),
		HoursSail:     valOrZeroFloat(body.HoursSail),
		HoursEngine:   valOrZeroFloat(body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(body.HoursOver6bf),
		Miles:         valOrZeroFloat(body.Miles),
		Days:          valOrZeroInt(body.Days),
		TidalWaters:   valOrZeroInt(body.TidalWaters),
		CostTotal:     nullFloat64(body.CostTotal),
		CostPerPerson: nullFloat64(body.CostPerPerson),
		ImageLogoUrl:  nullString(body.ImageLogoUrl),
		ImagePhotoUrl: nullString(body.ImagePhotoUrl),
		ImageRouteUrl: nullString(body.ImageRouteUrl),
		Description:   nullString(body.Description),
		OwnerID:       scope.userID,
	}
}
