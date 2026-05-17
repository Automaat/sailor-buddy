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
)

type OrgVoyagePortHandler struct {
	q  sqlcdb.Querier
	db *sql.DB
}

func NewOrgVoyagePortHandler(q sqlcdb.Querier, db *sql.DB) *OrgVoyagePortHandler {
	return &OrgVoyagePortHandler{q: q, db: db}
}

type orgVoyagePortListInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
}

type addOrgVoyagePortInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
	Body     dto.VoyagePortBody
}

type removeOrgVoyagePortInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
	PortID   int64  `path:"portID" doc:"Port ID"`
}

type reorderOrgVoyagePortsInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
	Body     dto.VoyagePortOrderBody
}

// RegisterOrgVoyagePortRoutes wires the org-scoped voyage port operations.
func RegisterOrgVoyagePortRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	h := NewOrgVoyagePortHandler(q, db)
	tag := []string{"Org voyage ports"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-voyage-ports", Method: http.MethodGet,
		Path:    "/orgs/{slug}/voyages/{voyageID}/ports",
		Summary: "List an org voyage's visited ports", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "add-org-voyage-port", Method: http.MethodPost,
		Path:    "/orgs/{slug}/voyages/{voyageID}/ports",
		Summary: "Add a visited port to an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.add)
	huma.Register(api, huma.Operation{
		OperationID: "remove-org-voyage-port", Method: http.MethodDelete,
		Path:    "/orgs/{slug}/voyages/{voyageID}/ports/{portID}",
		Summary: "Remove a visited port from an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.remove)
	huma.Register(api, huma.Operation{
		OperationID: "reorder-org-voyage-ports", Method: http.MethodPut,
		Path:    "/orgs/{slug}/voyages/{voyageID}/ports/order",
		Summary: "Reorder an org voyage's visited ports (admin)", Tags: tag,
	}, h.reorder)
}

func (h *OrgVoyagePortHandler) list(ctx context.Context, in *orgVoyagePortListInput) (*voyagePortListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	ports, err := h.q.ListOrgVoyagePorts(ctx, sqlcdb.ListOrgVoyagePortsParams{
		VoyageID: in.VoyageID,
		OrgID:    orgID(octx),
	})
	if err != nil {
		slog.Error("list org voyage ports", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage ports")
	}
	return &voyagePortListOutput{Body: dto.VoyagePortsFromDB(ports)}, nil
}

func (h *OrgVoyagePortHandler) add(ctx context.Context, in *addOrgVoyagePortInput) (*voyagePortOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: in.VoyageID, OrgID: orgID(octx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify org voyage for port", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
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
		slog.Error("add org voyage port", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to add voyage port")
	}
	return &voyagePortOutput{Body: dto.VoyagePortFromDB(port)}, nil
}

func (h *OrgVoyagePortHandler) remove(ctx context.Context, in *removeOrgVoyagePortInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgVoyagePort(ctx, sqlcdb.DeleteOrgVoyagePortParams{
		ID:       in.PortID,
		VoyageID: in.VoyageID,
		OrgID:    orgID(octx),
	}); err != nil {
		slog.Error("remove org voyage port", "port_id", in.PortID, "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove voyage port")
	}
	return &noContentOutput{}, nil
}

func (h *OrgVoyagePortHandler) reorder(ctx context.Context, in *reorderOrgVoyagePortsInput) (*voyagePortListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: in.VoyageID, OrgID: orgID(octx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify org voyage for reorder", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	// One transaction for every position write keeps the list from being
	// left half-renumbered if a write fails partway through.
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("reorder org voyage ports begin", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to reorder voyage ports")
	}
	defer func() { _ = tx.Rollback() }()
	qtx := sqlcdb.New(tx)
	for i, portID := range in.Body.PortIDs {
		if err := qtx.SetOrgVoyagePortPosition(ctx, sqlcdb.SetOrgVoyagePortPositionParams{
			ID:       portID,
			VoyageID: in.VoyageID,
			OrgID:    orgID(octx),
			Position: int64(i),
		}); err != nil {
			slog.Error("reorder org voyage port", "port_id", portID, "voyage_id", in.VoyageID, "err", err)
			return nil, huma.Error500InternalServerError("failed to reorder voyage ports")
		}
	}
	if err := tx.Commit(); err != nil {
		slog.Error("reorder org voyage ports commit", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to reorder voyage ports")
	}
	ports, err := h.q.ListOrgVoyagePorts(ctx, sqlcdb.ListOrgVoyagePortsParams{
		VoyageID: in.VoyageID,
		OrgID:    orgID(octx),
	})
	if err != nil {
		slog.Error("list org voyage ports after reorder", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage ports")
	}
	return &voyagePortListOutput{Body: dto.VoyagePortsFromDB(ports)}, nil
}
