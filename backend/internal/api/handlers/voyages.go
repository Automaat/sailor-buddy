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

type VoyageHandler struct {
	q sqlcdb.Querier
}

func NewVoyageHandler(q sqlcdb.Querier) *VoyageHandler {
	return &VoyageHandler{q: q}
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

func (h *VoyageHandler) list(ctx context.Context, _ *struct{}) (*voyageListOutput, error) {
	user := middleware.GetUser(ctx)
	voyages, err := h.q.ListVoyages(ctx, user.UserID)
	if err != nil {
		slog.Error("list voyages", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyages")
	}
	return &voyageListOutput{Body: dto.VoyagesFromDB(voyages)}, nil
}

func (h *VoyageHandler) get(ctx context.Context, in *voyageIDParam) (*voyageOutput, error) {
	user := middleware.GetUser(ctx)
	voyage, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.ID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get voyage", "voyage_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *VoyageHandler) create(ctx context.Context, in *createVoyageInput) (*voyageOutput, error) {
	user := middleware.GetUser(ctx)
	voyage, err := h.q.CreateVoyage(ctx, sqlcdb.CreateVoyageParams{
		OwnerID:       user.UserID,
		Name:          in.Body.Name,
		Year:          nullInt64(in.Body.Year),
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		HoursTotal:    valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:     valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:   valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:         valOrZeroFloat(in.Body.Miles),
		Days:          valOrZeroInt(in.Body.Days),
		TidalWaters:   valOrZeroInt(in.Body.TidalWaters),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
	})
	if err != nil {
		slog.Error("create voyage", "user_id", user.UserID, "name", in.Body.Name, "err", err)
		return nil, huma.Error500InternalServerError("failed to create voyage")
	}
	return &voyageOutput{Body: dto.VoyageFromDB(voyage)}, nil
}

func (h *VoyageHandler) update(ctx context.Context, in *updateVoyageInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateVoyage(ctx, sqlcdb.UpdateVoyageParams{
		Name:          in.Body.Name,
		Year:          nullInt64(in.Body.Year),
		EmbarkDate:    nullString(in.Body.EmbarkDate),
		DisembarkDate: nullString(in.Body.DisembarkDate),
		Countries:     nullString(in.Body.Countries),
		StartPort:     nullString(in.Body.StartPort),
		EndPort:       nullString(in.Body.EndPort),
		CaptainName:   nullString(in.Body.CaptainName),
		YachtID:       nullInt64(in.Body.YachtID),
		HoursTotal:    valOrZeroFloat(in.Body.HoursTotal),
		HoursSail:     valOrZeroFloat(in.Body.HoursSail),
		HoursEngine:   valOrZeroFloat(in.Body.HoursEngine),
		HoursOver6bf:  valOrZeroFloat(in.Body.HoursOver6bf),
		Miles:         valOrZeroFloat(in.Body.Miles),
		Days:          valOrZeroInt(in.Body.Days),
		TidalWaters:   valOrZeroInt(in.Body.TidalWaters),
		CostTotal:     nullFloat64(in.Body.CostTotal),
		CostPerPerson: nullFloat64(in.Body.CostPerPerson),
		ImageLogoUrl:  nullString(in.Body.ImageLogoUrl),
		ImagePhotoUrl: nullString(in.Body.ImagePhotoUrl),
		ImageRouteUrl: nullString(in.Body.ImageRouteUrl),
		Description:   nullString(in.Body.Description),
		ID:            in.ID,
		OwnerID:       user.UserID,
	}); err != nil {
		slog.Error("update voyage", "voyage_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update voyage")
	}
	return &noContentOutput{}, nil
}

func (h *VoyageHandler) delete(ctx context.Context, in *voyageIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteVoyage(ctx, sqlcdb.DeleteVoyageParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("delete voyage", "voyage_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete voyage")
	}
	return &noContentOutput{}, nil
}
