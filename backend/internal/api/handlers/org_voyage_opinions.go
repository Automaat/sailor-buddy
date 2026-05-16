package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgVoyageOpinionHandler struct {
	q         sqlcdb.Querier
	uploadDir string
}

func NewOrgVoyageOpinionHandler(q sqlcdb.Querier, uploadDir string) *OrgVoyageOpinionHandler {
	return &OrgVoyageOpinionHandler{q: q, uploadDir: uploadDir}
}

type orgVoyageOpinionsParam struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
}

type generateOrgOpinionInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
	Body     dto.GenerateOpinionBody
}

type orgOpinionParam struct {
	Slug      string `path:"slug" doc:"Organization slug"`
	VoyageID  int64  `path:"voyageID" doc:"Voyage ID"`
	OpinionID int64  `path:"opinionID" doc:"Opinion ID"`
}

// RegisterOrgVoyageOpinionRoutes wires the org-scoped crew opinion document
// operations onto the API.
func RegisterOrgVoyageOpinionRoutes(api huma.API, q sqlcdb.Querier, uploadDir string) {
	h := NewOrgVoyageOpinionHandler(q, uploadDir)
	tag := []string{"Org voyage opinions"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-voyage-opinions", Method: http.MethodGet,
		Path:    "/orgs/{slug}/voyages/{voyageID}/opinions",
		Summary: "List an org voyage's generated opinions", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "generate-org-voyage-opinion", Method: http.MethodPost,
		Path:    "/orgs/{slug}/voyages/{voyageID}/opinions",
		Summary: "Generate a crew opinion document for an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.generate)
	huma.Register(api, huma.Operation{
		OperationID: "download-org-voyage-opinion", Method: http.MethodGet,
		Path:    "/orgs/{slug}/voyages/{voyageID}/opinions/{opinionID}/download",
		Summary: "Download an org voyage opinion document", Tags: tag,
	}, h.download)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-voyage-opinion", Method: http.MethodDelete,
		Path:    "/orgs/{slug}/voyages/{voyageID}/opinions/{opinionID}",
		Summary: "Delete an org voyage opinion (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

// verifyOrgVoyage confirms the voyage exists within the organization.
func (h *OrgVoyageOpinionHandler) verifyOrgVoyage(ctx context.Context, voyageID int64, octx *middleware.OrgContext) (sqlcdb.Voyage, error) {
	voyage, err := h.q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: voyageID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return voyage, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify org voyage for opinion", "voyage_id", voyageID, "org_id", octx.OrgID, "err", err)
		return voyage, huma.Error500InternalServerError("failed to verify voyage")
	}
	return voyage, nil
}

func (h *OrgVoyageOpinionHandler) list(ctx context.Context, in *orgVoyageOpinionsParam) (*opinionListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	if _, err := h.verifyOrgVoyage(ctx, in.VoyageID, octx); err != nil {
		return nil, err
	}
	opinions, err := h.q.ListVoyageVoyageOpinions(ctx, in.VoyageID)
	if err != nil {
		slog.Error("list org voyage opinions", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list opinions")
	}
	return &opinionListOutput{Body: dto.VoyageOpinionsFromDB(opinions)}, nil
}

func (h *OrgVoyageOpinionHandler) generate(ctx context.Context, in *generateOrgOpinionInput) (*opinionOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	voyage, err := h.verifyOrgVoyage(ctx, in.VoyageID, octx)
	if err != nil {
		return nil, err
	}
	assignment, err := getOpinionCrewAssignment(ctx, h.q, in.VoyageID, in.Body.CrewMemberID)
	if err != nil {
		return nil, err
	}

	yachtName, yachtType := "", ""
	if voyage.YachtID.Valid {
		if yacht, err := h.q.GetOrgYacht(ctx, sqlcdb.GetOrgYachtParams{ID: voyage.YachtID.Int64, OrgID: orgID(octx)}); err == nil {
			yachtName, yachtType = yacht.Name, yacht.YachtType.String
		}
	}

	format := opinionFormat(in.Body.Format)
	data := opinionDocData(voyage, assignment, yachtName, yachtType)
	filePath, err := storeOpinionFile(h.uploadDir, opinionScopeKey("org", octx.OrgID), in.VoyageID, in.Body.CrewMemberID, format, data)
	if err != nil {
		slog.Error("store org opinion file", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "format", format, "err", err)
		return nil, huma.Error500InternalServerError("failed to generate opinion document")
	}

	opinion, err := h.q.UpsertVoyageOpinion(ctx, sqlcdb.UpsertVoyageOpinionParams{
		VoyageID:     in.VoyageID,
		CrewMemberID: in.Body.CrewMemberID,
		FilePath:     filePath,
		FileFormat:   format,
	})
	if err != nil {
		slog.Error("upsert org voyage opinion", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to save opinion record")
	}
	return &opinionOutput{Body: dto.VoyageOpinionFromDB(opinion)}, nil
}

func (h *OrgVoyageOpinionHandler) download(ctx context.Context, in *orgOpinionParam) (*opinionFileOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	if _, err := h.verifyOrgVoyage(ctx, in.VoyageID, octx); err != nil {
		return nil, err
	}
	opinion, err := loadVoyageOpinion(ctx, h.q, in.OpinionID, in.VoyageID)
	if err != nil {
		return nil, err
	}
	fileBytes, err := os.ReadFile(opinion.FilePath)
	if err != nil {
		slog.Error("read org opinion file", "opinion_id", in.OpinionID, "path", opinion.FilePath, "err", err)
		return nil, huma.Error500InternalServerError("failed to read opinion file")
	}
	return &opinionFileOutput{
		ContentType:        opinionContentType(opinion.FileFormat),
		ContentDisposition: fmt.Sprintf(`attachment; filename="opinion_%d.%s"`, opinion.ID, opinion.FileFormat),
		Body:               fileBytes,
	}, nil
}

func (h *OrgVoyageOpinionHandler) delete(ctx context.Context, in *orgOpinionParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.verifyOrgVoyage(ctx, in.VoyageID, octx); err != nil {
		return nil, err
	}
	opinion, err := loadVoyageOpinion(ctx, h.q, in.OpinionID, in.VoyageID)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(opinion.FilePath)
	if err := h.q.DeleteVoyageOpinion(ctx, in.OpinionID); err != nil {
		slog.Error("delete org voyage opinion", "opinion_id", in.OpinionID, "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete opinion")
	}
	return &noContentOutput{}, nil
}
