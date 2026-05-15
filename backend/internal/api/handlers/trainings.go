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
)

type TrainingHandler struct {
	q sqlcdb.Querier
}

func NewTrainingHandler(q sqlcdb.Querier) *TrainingHandler {
	return &TrainingHandler{q: q}
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
	Body []dto.Training
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

func (h *TrainingHandler) list(ctx context.Context, _ *struct{}) (*trainingListOutput, error) {
	user := middleware.GetUser(ctx)
	trainings, err := h.q.ListTrainings(ctx, user.UserID)
	if err != nil {
		slog.Error("list trainings", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trainings")
	}
	return &trainingListOutput{Body: dto.TrainingsFromDB(trainings)}, nil
}

func (h *TrainingHandler) get(ctx context.Context, in *trainingIDParam) (*trainingOutput, error) {
	user := middleware.GetUser(ctx)
	training, err := h.q.GetTraining(ctx, sqlcdb.GetTrainingParams{ID: in.ID, UserID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("training not found")
		}
		slog.Error("get training", "training_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get training")
	}
	return &trainingOutput{Body: dto.TrainingFromDB(training)}, nil
}

func (h *TrainingHandler) create(ctx context.Context, in *createTrainingInput) (*trainingOutput, error) {
	user := middleware.GetUser(ctx)
	training, err := h.q.CreateTraining(ctx, sqlcdb.CreateTrainingParams{
		UserID:    user.UserID,
		Date:      nullString(in.Body.Date),
		Name:      in.Body.Name,
		Organizer: nullString(in.Body.Organizer),
		Cost:      nullFloat64(in.Body.Cost),
		Url:       nullString(in.Body.Url),
	})
	if err != nil {
		slog.Error("create training", "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create training")
	}
	return &trainingOutput{Body: dto.TrainingFromDB(training)}, nil
}

func (h *TrainingHandler) update(ctx context.Context, in *updateTrainingInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateTraining(ctx, sqlcdb.UpdateTrainingParams{
		Date:      nullString(in.Body.Date),
		Name:      in.Body.Name,
		Organizer: nullString(in.Body.Organizer),
		Cost:      nullFloat64(in.Body.Cost),
		Url:       nullString(in.Body.Url),
		ID:        in.ID,
		UserID:    user.UserID,
	}); err != nil {
		slog.Error("update training", "training_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update training")
	}
	return &noContentOutput{}, nil
}

func (h *TrainingHandler) delete(ctx context.Context, in *trainingIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteTraining(ctx, sqlcdb.DeleteTrainingParams{ID: in.ID, UserID: user.UserID}); err != nil {
		slog.Error("delete training", "training_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete training")
	}
	return &noContentOutput{}, nil
}
