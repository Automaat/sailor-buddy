package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type TrainingHandler struct {
	*crudHandlers[pageParams, trainingIDParam, createTrainingInput, updateTrainingInput, trainingIDParam, sqlcdb.Training, trainingListOutput, trainingOutput]
}

func NewTrainingHandler(q sqlcdb.Querier) *TrainingHandler {
	return &TrainingHandler{crudHandlers: newCRUDHandlers(trainingCRUDConfig(q))}
}

type trainingIDParam struct {
	ID int64 `path:"trainingID" doc:"Training ID"`
}

type createTrainingInput struct {
	Body dto.TrainingBody
}

type updateTrainingInput struct {
	ID   int64 `path:"trainingID" doc:"Training ID"`
	Body dto.TrainingBody
}

type trainingOutput struct {
	Body dto.Training
}

type trainingListOutput struct {
	Body dto.Page[dto.Training]
}

// RegisterTrainingRoutes wires the owner-scoped training operations onto the API.
func RegisterTrainingRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewTrainingHandler(q)
	tag := []string{"Trainings"}

	huma.Register(api, huma.Operation{
		OperationID: "list-trainings", Method: http.MethodGet, Path: "/trainings",
		Summary: "List trainings", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-training", Method: http.MethodGet, Path: "/trainings/{trainingID}",
		Summary: "Get a training", Tags: tag,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-training", Method: http.MethodPost, Path: "/trainings",
		Summary: "Create a training", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-training", Method: http.MethodPut, Path: "/trainings/{trainingID}",
		Summary: "Update a training", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-training", Method: http.MethodDelete, Path: "/trainings/{trainingID}",
		Summary: "Delete a training", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func trainingCRUDConfig(q sqlcdb.Querier) crudConfig[pageParams, trainingIDParam, createTrainingInput, updateTrainingInput, trainingIDParam, sqlcdb.Training, trainingListOutput, trainingOutput] {
	ownerScope := func(ctx context.Context, _ any) (crudScope, error) {
		return ownerCRUDScope(ctx), nil
	}
	return crudConfig[pageParams, trainingIDParam, createTrainingInput, updateTrainingInput, trainingIDParam, sqlcdb.Training, trainingListOutput, trainingOutput]{
		listScope: func(ctx context.Context, in *pageParams) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		getScope: func(ctx context.Context, in *trainingIDParam) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		createScope: func(ctx context.Context, in *createTrainingInput) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		updateScope: func(ctx context.Context, in *updateTrainingInput) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		deleteScope: func(ctx context.Context, in *trainingIDParam) (crudScope, error) {
			return ownerScope(ctx, in)
		},
		list: func(ctx context.Context, scope crudScope, in *pageParams) ([]sqlcdb.Training, error) {
			return q.ListTrainings(ctx, sqlcdb.ListTrainingsParams{
				UserID: scope.userID,
				Limit:  in.Limit,
				Offset: in.Offset,
			})
		},
		count: func(ctx context.Context, scope crudScope, _ *pageParams) (int64, error) {
			return q.CountTrainings(ctx, scope.userID)
		},
		get: func(ctx context.Context, scope crudScope, in *trainingIDParam) (sqlcdb.Training, error) {
			return q.GetTraining(ctx, sqlcdb.GetTrainingParams{ID: in.ID, UserID: scope.userID})
		},
		create: func(ctx context.Context, scope crudScope, in *createTrainingInput) (sqlcdb.Training, error) {
			return q.CreateTraining(ctx, sqlcdb.CreateTrainingParams{
				UserID:    scope.userID,
				Date:      nullString(in.Body.Date),
				Name:      in.Body.Name,
				Organizer: nullString(in.Body.Organizer),
				Cost:      nullFloat64(in.Body.Cost),
				Url:       nullString(in.Body.Url),
			})
		},
		update: func(ctx context.Context, scope crudScope, in *updateTrainingInput) error {
			return q.UpdateTraining(ctx, sqlcdb.UpdateTrainingParams{
				Date:      nullString(in.Body.Date),
				Name:      in.Body.Name,
				Organizer: nullString(in.Body.Organizer),
				Cost:      nullFloat64(in.Body.Cost),
				Url:       nullString(in.Body.Url),
				ID:        in.ID,
				UserID:    scope.userID,
			})
		},
		delete: func(ctx context.Context, scope crudScope, in *trainingIDParam) error {
			return q.DeleteTraining(ctx, sqlcdb.DeleteTrainingParams{ID: in.ID, UserID: scope.userID})
		},
		listOutput: func(in *pageParams, rows []sqlcdb.Training, total int64) *trainingListOutput {
			return &trainingListOutput{Body: dto.NewPage(dto.TrainingsFromDB(rows), total, in.Limit, in.Offset)}
		},
		itemOutput: func(row sqlcdb.Training) *trainingOutput {
			return &trainingOutput{Body: dto.TrainingFromDB(row)}
		},
		listLogAttrs: func(scope crudScope, _ *pageParams) []any {
			return scopeAttrs(scope)
		},
		getLogAttrs: func(scope crudScope, in *trainingIDParam) []any {
			return scopeAttrs(scope, "training_id", in.ID)
		},
		createLogAttrs: func(scope crudScope, in *createTrainingInput) []any {
			return scopeAttrs(scope, "name", in.Body.Name)
		},
		updateLogAttrs: func(scope crudScope, in *updateTrainingInput) []any {
			return scopeAttrs(scope, "training_id", in.ID)
		},
		deleteLogAttrs: func(scope crudScope, in *trainingIDParam) []any {
			return scopeAttrs(scope, "training_id", in.ID)
		},
		listLogMsg:      "list trainings",
		getLogMsg:       "get training",
		createLogMsg:    "create training",
		updateLogMsg:    "update training",
		deleteLogMsg:    "delete training",
		listClientMsg:   "failed to list trainings",
		getClientMsg:    "failed to get training",
		createClientMsg: "failed to create training",
		updateClientMsg: "failed to update training",
		deleteClientMsg: "failed to delete training",
		notFoundMsg:     "training not found",
	}
}
