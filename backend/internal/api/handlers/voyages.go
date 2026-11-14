package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type VoyageHandler struct {
	*crudHandlers[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]
	q sqlcdb.Querier
}

func NewVoyageHandler(q sqlcdb.Querier) *VoyageHandler {
	return &VoyageHandler{crudHandlers: newCRUDHandlers(voyageCRUDConfig(q)), q: q}
}

func (h *VoyageHandler) updateLog(ctx context.Context, in *updateVoyageLogInput) (*noContentOutput, error) {
	voyage, err := getVoyageForCaptain(ctx, h.q, in.ID)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateVoyageLog(ctx, sqlcdb.UpdateVoyageLogParams{
		HoursTotal:   valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:    valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:  valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf: valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:        valOrZeroFloat(in.Body.Miles),
		Days:         valOrZeroInt(in.Body.Days),
		TidalWaters:  valOrZeroInt(in.Body.TidalWaters),
		CaptainNotes: nullString(in.Body.CaptainNotes),
		YachtID:      nullInt64(in.Body.YachtID),
		TrackUrl:     nullString(trimmedPtr(in.Body.TrackUrl)),
		VoyageCard:   nullString(trimmedPtr(in.Body.VoyageCard)),
		ID:           voyage.ID,
	}); err != nil {
		if isForeignKeyViolation(err, "voyages_yacht_id_fkey") {
			return nil, yachtNotFound()
		}
		slog.Error("update voyage log", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update voyage log")
	}
	return &noContentOutput{}, nil
}

// addYacht creates a yacht from a voyage form so a charter yacht used once
// can be entered without leaving the form. The voyage's captain may add one
// here even though the club yacht list is otherwise admin-managed; picking it
// for the voyage is a separate log or voyage update.
func (h *VoyageHandler) addYacht(ctx context.Context, in *addVoyageYachtInput) (*yachtOutput, error) {
	if _, err := getVoyageForCaptain(ctx, h.q, in.ID); err != nil {
		return nil, err
	}
	yacht, err := h.q.CreateYacht(ctx, sqlcdb.CreateYachtParams{
		CreatedBy:      types.NullInt64{Int64: middleware.GetUser(ctx).UserID, Valid: true},
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
		Berths:         nullInt64(in.Body.Berths),
		LengthM:        nullFloat64(in.Body.LengthM),
		BuildYear:      nullInt64(in.Body.BuildYear),
		Engine:         nullString(in.Body.Engine),
		OwnerName:      nullString(in.Body.OwnerName),
	})
	if err != nil {
		slog.Error("add voyage yacht", "voyage_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to add yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

// getVoyageForCaptain loads a voyage and authorizes the caller as an admin or
// its assigned captain.
func getVoyageForCaptain(ctx context.Context, q sqlcdb.Querier, id int64) (sqlcdb.Voyage, error) {
	voyage, err := q.GetVoyage(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlcdb.Voyage{}, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get voyage", "voyage_id", id, "err", err)
		return sqlcdb.Voyage{}, huma.Error500InternalServerError("failed to get voyage")
	}
	if err := requireAdminOrCaptain(ctx, q, voyage.CaptainPersonID); err != nil {
		return sqlcdb.Voyage{}, err
	}
	return voyage, nil
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

type addVoyageYachtInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.YachtBody
}

type updateVoyageLogInput struct {
	ID   int64 `path:"voyageID" doc:"Voyage ID"`
	Body dto.VoyageLogBody
}

// RegisterVoyageRoutes wires the club voyage operations onto the API. Reads are
// open to any member; mutations require an admin, except the post-cruise log,
// which the voyage's captain may also write.
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
		Summary: "Create a voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-voyage", Method: http.MethodPut, Path: "/voyages/{voyageID}",
		Summary: "Update a voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "update-voyage-log", Method: http.MethodPut, Path: "/voyages/{voyageID}/log",
		Summary: "Log post-cruise statistics and notes (admin or voyage captain)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.updateLog)
	huma.Register(api, huma.Operation{
		OperationID: "add-voyage-yacht", Method: http.MethodPost, Path: "/voyages/{voyageID}/yachts",
		Summary: "Add a yacht to the club list from a voyage (admin or voyage captain)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.addYacht)
	huma.Register(api, huma.Operation{
		OperationID: "delete-voyage", Method: http.MethodDelete, Path: "/voyages/{voyageID}",
		Summary: "Delete a voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

// voyageWriteErr maps a voyage insert or update that references a captain
// person or yacht deleted underneath it to a client error.
func voyageWriteErr(err error) error {
	switch {
	case isForeignKeyViolation(err, "voyages_captain_person_id_fkey"):
		return captainPersonNotFound()
	case isForeignKeyViolation(err, "voyages_yacht_id_fkey"):
		return yachtNotFound()
	}
	return err
}

func voyageCRUDConfig(q sqlcdb.Querier) crudConfig[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput] {
	return crudConfig[pageParams, voyageIDParam, createVoyageInput, updateVoyageInput, voyageIDParam, sqlcdb.Voyage, voyageListOutput, voyageOutput]{
		listScope: func(ctx context.Context, _ *pageParams) (crudScope, error) {
			return memberScope(ctx)
		},
		getScope: func(ctx context.Context, _ *voyageIDParam) (crudScope, error) {
			return memberScope(ctx)
		},
		createScope: func(ctx context.Context, _ *createVoyageInput) (crudScope, error) {
			return adminScope(ctx)
		},
		updateScope: func(ctx context.Context, _ *updateVoyageInput) (crudScope, error) {
			return adminScope(ctx)
		},
		deleteScope: func(ctx context.Context, _ *voyageIDParam) (crudScope, error) {
			return adminScope(ctx)
		},
		list: func(ctx context.Context, _ crudScope, in *pageParams) ([]sqlcdb.Voyage, error) {
			return q.ListVoyages(ctx, sqlcdb.ListVoyagesParams{Limit: in.Limit, Offset: in.Offset})
		},
		count: func(ctx context.Context, _ crudScope, _ *pageParams) (int64, error) {
			return q.CountVoyages(ctx)
		},
		get: func(ctx context.Context, _ crudScope, in *voyageIDParam) (sqlcdb.Voyage, error) {
			return q.GetVoyage(ctx, in.ID)
		},
		create: func(ctx context.Context, scope crudScope, in *createVoyageInput) (sqlcdb.Voyage, error) {
			if err := requireParentCruise(ctx, q, in.Body.CruiseID); err != nil {
				return sqlcdb.Voyage{}, err
			}
			params := createVoyageParams(scope, in.Body)
			var err error
			params.CaptainPersonID, params.CaptainName, err = captainAssignment(ctx, q, in.Body.CaptainPersonID, in.Body.CaptainName)
			if err != nil {
				return sqlcdb.Voyage{}, err
			}
			voyage, err := q.CreateVoyage(ctx, params)
			return voyage, voyageWriteErr(err)
		},
		update: func(ctx context.Context, _ crudScope, in *updateVoyageInput) error {
			if err := requireParentCruise(ctx, q, in.Body.CruiseID); err != nil {
				return err
			}
			params := updateVoyageParams(in.Body)
			params.ID = in.ID
			if _, err := q.GetVoyage(ctx, in.ID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return huma.Error404NotFound("voyage not found")
				}
				return err
			}
			var err error
			params.CaptainPersonID, params.CaptainName, err = captainAssignment(ctx, q, in.Body.CaptainPersonID, in.Body.CaptainName)
			if err != nil {
				return err
			}
			return voyageWriteErr(q.UpdateVoyage(ctx, params))
		},
		delete: func(ctx context.Context, _ crudScope, in *voyageIDParam) error {
			return q.DeleteVoyage(ctx, in.ID)
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
		CreatedBy:     types.NullInt64{Int64: scope.userID, Valid: true},
		CruiseID:      body.CruiseID,
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
		TrackUrl:      nullString(trimmedPtr(body.TrackUrl)),
		VoyageCard:    nullString(trimmedPtr(body.VoyageCard)),
	}
}

func updateVoyageParams(body dto.VoyageBody) sqlcdb.UpdateVoyageParams {
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
		TrackUrl:      nullString(trimmedPtr(body.TrackUrl)),
		VoyageCard:    nullString(trimmedPtr(body.VoyageCard)),
		CruiseID:      body.CruiseID,
	}
}
