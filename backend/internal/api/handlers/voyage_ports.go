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

type VoyagePortHandler struct {
	q sqlcdb.Querier
}

func NewVoyagePortHandler(q sqlcdb.Querier) *VoyagePortHandler {
	return &VoyagePortHandler{q: q}
}

// --- huma operation input/output types ---

type voyagePortListInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
}

type addVoyagePortInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
	Body     dto.VoyagePortBody
}

type removeVoyagePortInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
	PortID   int64 `path:"portID" doc:"Port ID"`
}

type voyagePortOutput struct {
	Body dto.VoyagePort
}

type voyagePortListOutput struct {
	Body []dto.VoyagePort
}

// RegisterVoyagePortRoutes wires the owner-scoped voyage port operations.
func RegisterVoyagePortRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewVoyagePortHandler(q)
	tag := []string{"Voyage ports"}

	huma.Register(api, huma.Operation{
		OperationID: "list-voyage-ports", Method: http.MethodGet,
		Path:    "/voyages/{voyageID}/ports",
		Summary: "List a voyage's visited ports", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "add-voyage-port", Method: http.MethodPost,
		Path:    "/voyages/{voyageID}/ports",
		Summary: "Add a visited port to a voyage", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.add)
	huma.Register(api, huma.Operation{
		OperationID: "remove-voyage-port", Method: http.MethodDelete,
		Path:    "/voyages/{voyageID}/ports/{portID}",
		Summary: "Remove a visited port from a voyage", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.remove)
}

func (h *VoyagePortHandler) list(ctx context.Context, in *voyagePortListInput) (*voyagePortListOutput, error) {
	user := middleware.GetUser(ctx)
	ports, err := h.q.ListVoyagePorts(ctx, sqlcdb.ListVoyagePortsParams{
		VoyageID: in.VoyageID,
		OwnerID:  user.UserID,
	})
	if err != nil {
		slog.Error("list voyage ports", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage ports")
	}
	return &voyagePortListOutput{Body: dto.VoyagePortsFromDB(ports)}, nil
}

func (h *VoyagePortHandler) add(ctx context.Context, in *addVoyagePortInput) (*voyagePortOutput, error) {
	user := middleware.GetUser(ctx)
	if _, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.VoyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage for port", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	port, err := h.q.CreateVoyagePort(ctx, sqlcdb.CreateVoyagePortParams{
		VoyageID:  in.VoyageID,
		Name:      in.Body.Name,
		Latitude:  in.Body.Latitude,
		Longitude: in.Body.Longitude,
		Position:  valOrZeroInt(in.Body.Position),
	})
	if err != nil {
		slog.Error("add voyage port", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to add voyage port")
	}
	return &voyagePortOutput{Body: dto.VoyagePortFromDB(port)}, nil
}

func (h *VoyagePortHandler) remove(ctx context.Context, in *removeVoyagePortInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteVoyagePort(ctx, sqlcdb.DeleteVoyagePortParams{
		ID:       in.PortID,
		VoyageID: in.VoyageID,
		OwnerID:  user.UserID,
	}); err != nil {
		slog.Error("remove voyage port", "port_id", in.PortID, "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove voyage port")
	}
	return &noContentOutput{}, nil
}
