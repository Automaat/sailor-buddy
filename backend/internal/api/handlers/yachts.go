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

type YachtHandler struct {
	q sqlcdb.Querier
}

func NewYachtHandler(q sqlcdb.Querier) *YachtHandler {
	return &YachtHandler{q: q}
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
	Body []dto.Yacht
}

// RegisterYachtRoutes wires the owner-scoped yacht operations onto the API.
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
		Summary: "Create a yacht", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-yacht", Method: http.MethodPut, Path: "/yachts/{yachtID}",
		Summary: "Update a yacht", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-yacht", Method: http.MethodDelete, Path: "/yachts/{yachtID}",
		Summary: "Delete a yacht", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *YachtHandler) list(ctx context.Context, _ *struct{}) (*yachtListOutput, error) {
	user := middleware.GetUser(ctx)
	yachts, err := h.q.ListYachts(ctx, user.UserID)
	if err != nil {
		slog.Error("list yachts", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list yachts")
	}
	return &yachtListOutput{Body: dto.YachtsFromDB(yachts)}, nil
}

func (h *YachtHandler) get(ctx context.Context, in *yachtIDParam) (*yachtOutput, error) {
	user := middleware.GetUser(ctx)
	yacht, err := h.q.GetYacht(ctx, sqlcdb.GetYachtParams{ID: in.ID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("yacht not found")
		}
		slog.Error("get yacht", "yacht_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

func (h *YachtHandler) create(ctx context.Context, in *createYachtInput) (*yachtOutput, error) {
	user := middleware.GetUser(ctx)
	yacht, err := h.q.CreateYacht(ctx, sqlcdb.CreateYachtParams{
		OwnerID:        user.UserID,
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
	})
	if err != nil {
		slog.Error("create yacht", "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

func (h *YachtHandler) update(ctx context.Context, in *updateYachtInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateYacht(ctx, sqlcdb.UpdateYachtParams{
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
		ID:             in.ID,
		OwnerID:        user.UserID,
	}); err != nil {
		slog.Error("update yacht", "yacht_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update yacht")
	}
	return &noContentOutput{}, nil
}

func (h *YachtHandler) delete(ctx context.Context, in *yachtIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteYacht(ctx, sqlcdb.DeleteYachtParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("delete yacht", "yacht_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete yacht")
	}
	return &noContentOutput{}, nil
}
