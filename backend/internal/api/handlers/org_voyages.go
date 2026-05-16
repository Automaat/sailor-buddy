package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgVoyageHandler struct {
	*crudHandlers[orgListParams, orgVoyageParam, createOrgVoyageInput, updateOrgVoyageInput, orgVoyageParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]
}

func NewOrgVoyageHandler(q sqlcdb.Querier) *OrgVoyageHandler {
	return &OrgVoyageHandler{crudHandlers: newCRUDHandlers(orgVoyageCRUDConfig(q))}
}

type orgVoyageParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"voyageID" doc:"Voyage ID"`
}

type createOrgVoyageInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.VoyageBody
}

type updateOrgVoyageInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageBody
}

// RegisterOrgVoyageRoutes wires the org-scoped voyage operations onto the API.
func RegisterOrgVoyageRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewOrgVoyageHandler(q)
	tag := []string{"Org voyages"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-voyages", Method: http.MethodGet, Path: "/orgs/{slug}/voyages",
		Summary: "List org voyages", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-voyage", Method: http.MethodGet, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Get an org voyage", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-voyage", Method: http.MethodPost, Path: "/orgs/{slug}/voyages",
		Summary: "Create an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-voyage", Method: http.MethodPut, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Update an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-voyage", Method: http.MethodDelete, Path: "/orgs/{slug}/voyages/{voyageID}",
		Summary: "Delete an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func orgVoyageCRUDConfig(q sqlcdb.Querier) crudConfig[orgListParams, orgVoyageParam, createOrgVoyageInput, updateOrgVoyageInput, orgVoyageParam, sqlcdb.Voyage, voyageListOutput, voyageOutput] {
	readScope := func(ctx context.Context, slug string) (crudScope, error) {
		return orgCRUDScope(ctx, q, slug, false)
	}
	writeScope := func(ctx context.Context, slug string) (crudScope, error) {
		return orgCRUDScope(ctx, q, slug, true)
	}
	return crudConfig[orgListParams, orgVoyageParam, createOrgVoyageInput, updateOrgVoyageInput, orgVoyageParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]{
		listScope: func(ctx context.Context, in *orgListParams) (crudScope, error) {
			return readScope(ctx, in.Slug)
		},
		getScope: func(ctx context.Context, in *orgVoyageParam) (crudScope, error) {
			return readScope(ctx, in.Slug)
		},
		createScope: func(ctx context.Context, in *createOrgVoyageInput) (crudScope, error) {
			return writeScope(ctx, in.Slug)
		},
		updateScope: func(ctx context.Context, in *updateOrgVoyageInput) (crudScope, error) {
			return writeScope(ctx, in.Slug)
		},
		deleteScope: func(ctx context.Context, in *orgVoyageParam) (crudScope, error) {
			return writeScope(ctx, in.Slug)
		},
		list: func(ctx context.Context, scope crudScope, in *orgListParams) ([]sqlcdb.Voyage, error) {
			return q.ListOrgVoyages(ctx, sqlcdb.ListOrgVoyagesParams{
				OrgID:  scope.orgID,
				Limit:  in.Limit,
				Offset: in.Offset,
			})
		},
		count: func(ctx context.Context, scope crudScope, _ *orgListParams) (int64, error) {
			return q.CountOrgVoyages(ctx, scope.orgID)
		},
		get: func(ctx context.Context, scope crudScope, in *orgVoyageParam) (sqlcdb.Voyage, error) {
			return q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: in.ID, OrgID: scope.orgID})
		},
		create: func(ctx context.Context, scope crudScope, in *createOrgVoyageInput) (sqlcdb.Voyage, error) {
			return q.CreateOrgVoyage(ctx, createOrgVoyageParams(scope, in.Body))
		},
		update: func(ctx context.Context, scope crudScope, in *updateOrgVoyageInput) error {
			params := updateOrgVoyageParams(scope, in.Body)
			params.ID = in.ID
			return q.UpdateOrgVoyage(ctx, params)
		},
		delete: func(ctx context.Context, scope crudScope, in *orgVoyageParam) error {
			return q.DeleteOrgVoyage(ctx, sqlcdb.DeleteOrgVoyageParams{ID: in.ID, OrgID: scope.orgID})
		},
		listOutput: func(in *orgListParams, rows []sqlcdb.Voyage, total int64) *voyageListOutput {
			return &voyageListOutput{Body: dto.NewPage(dto.VoyagesFromDB(rows), total, in.Limit, in.Offset)}
		},
		itemOutput: func(row sqlcdb.Voyage) *voyageOutput {
			return &voyageOutput{Body: dto.VoyageFromDB(row)}
		},
		listLogAttrs: func(scope crudScope, _ *orgListParams) []any {
			return scopeAttrs(scope)
		},
		getLogAttrs: func(scope crudScope, in *orgVoyageParam) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		createLogAttrs: func(scope crudScope, in *createOrgVoyageInput) []any {
			return scopeAttrs(scope, "user_id", scope.userID, "name", in.Body.Name)
		},
		updateLogAttrs: func(scope crudScope, in *updateOrgVoyageInput) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		deleteLogAttrs: func(scope crudScope, in *orgVoyageParam) []any {
			return scopeAttrs(scope, "voyage_id", in.ID)
		},
		listLogMsg:      "list org voyages",
		getLogMsg:       "get org voyage",
		createLogMsg:    "create org voyage",
		updateLogMsg:    "update org voyage",
		deleteLogMsg:    "delete org voyage",
		listClientMsg:   "failed to list voyages",
		getClientMsg:    "failed to get voyage",
		createClientMsg: "failed to create voyage",
		updateClientMsg: "failed to update voyage",
		deleteClientMsg: "failed to delete voyage",
		notFoundMsg:     "voyage not found",
	}
}

func createOrgVoyageParams(scope crudScope, body dto.VoyageBody) sqlcdb.CreateOrgVoyageParams {
	return sqlcdb.CreateOrgVoyageParams{
		OwnerID:       scope.userID,
		OrgID:         scope.orgID,
		CruiseID:      nullInt64(body.CruiseID),
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

func updateOrgVoyageParams(scope crudScope, body dto.VoyageBody) sqlcdb.UpdateOrgVoyageParams {
	return sqlcdb.UpdateOrgVoyageParams{
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
		CruiseID:      nullInt64(body.CruiseID),
		OrgID:         scope.orgID,
	}
}
