package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/docgen"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type VoyageOpinionHandler struct {
	q         sqlcdb.Querier
	uploadDir string
}

func NewVoyageOpinionHandler(q sqlcdb.Querier, uploadDir string) *VoyageOpinionHandler {
	return &VoyageOpinionHandler{q: q, uploadDir: uploadDir}
}

type generateOpinionInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
	Body     dto.GenerateOpinionBody
}

type opinionParam struct {
	VoyageID  int64 `path:"voyageID" doc:"Voyage ID"`
	OpinionID int64 `path:"opinionID" doc:"Opinion ID"`
}

type opinionOutput struct {
	Body dto.VoyageOpinion
}

type opinionListOutput struct {
	Body []dto.VoyageOpinion
}

type opinionFileOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

// RegisterVoyageOpinionRoutes wires the crew opinion document operations.
func RegisterVoyageOpinionRoutes(api huma.API, q sqlcdb.Querier, uploadDir string) {
	h := NewVoyageOpinionHandler(q, uploadDir)
	tag := []string{"Voyage opinions"}

	huma.Register(api, huma.Operation{
		OperationID: "list-voyage-opinions", Method: http.MethodGet, Path: "/voyages/{voyageID}/opinions",
		Summary: "List a voyage's generated opinions", Tags: tag,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "generate-voyage-opinion", Method: http.MethodPost, Path: "/voyages/{voyageID}/opinions",
		Summary: "Generate a crew opinion document", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.generate)
	huma.Register(api, huma.Operation{
		OperationID: "download-voyage-opinion", Method: http.MethodGet, Path: "/voyages/{voyageID}/opinions/{opinionID}/download",
		Summary: "Download an opinion document", Tags: tag,
	}, h.download)
	huma.Register(api, huma.Operation{
		OperationID: "delete-voyage-opinion", Method: http.MethodDelete, Path: "/voyages/{voyageID}/opinions/{opinionID}",
		Summary: "Delete an opinion", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *VoyageOpinionHandler) list(ctx context.Context, in *voyageIDParam) (*opinionListOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.verifyVoyage(ctx, in.ID, user.UserID); err != nil {
		return nil, err
	}
	opinions, err := h.q.ListVoyageVoyageOpinions(ctx, in.ID)
	if err != nil {
		slog.Error("list voyage opinions", "voyage_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list opinions")
	}
	return &opinionListOutput{Body: dto.VoyageOpinionsFromDB(opinions)}, nil
}

func (h *VoyageOpinionHandler) generate(ctx context.Context, in *generateOpinionInput) (*opinionOutput, error) {
	user := middleware.GetUser(ctx)
	format := in.Body.Format
	if format == "" {
		format = "pdf"
	}

	voyage, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.VoyageID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get voyage for opinion", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}

	assignment, err := h.q.GetVoyageCrewAssignmentByMember(ctx, sqlcdb.GetVoyageCrewAssignmentByMemberParams{
		VoyageID:     types.NullInt64{Int64: in.VoyageID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("crew member not assigned to this voyage")
		}
		slog.Error("get voyage crew assignment for opinion", "voyage_id", in.VoyageID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get crew assignment")
	}

	data := h.buildOpinionData(ctx, user.UserID, voyage, assignment)
	fileBytes, err := renderOpinionFile(format, data)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to generate " + format)
	}

	dir := filepath.Join(h.uploadDir, strconv.FormatInt(user.UserID, 10), "opinions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, huma.Error500InternalServerError("failed to create directory")
	}
	for _, oldFmt := range []string{"pdf", "docx"} {
		if oldFmt != format {
			_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%d_%d.%s", in.VoyageID, in.Body.CrewMemberID, oldFmt)))
		}
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%d_%d.%s", in.VoyageID, in.Body.CrewMemberID, format))
	if err := os.WriteFile(filePath, fileBytes, 0o644); err != nil {
		return nil, huma.Error500InternalServerError("failed to save file")
	}

	opinion, err := h.q.UpsertVoyageOpinion(ctx, sqlcdb.UpsertVoyageOpinionParams{
		VoyageID:     in.VoyageID,
		CrewMemberID: in.Body.CrewMemberID,
		FilePath:     filePath,
		FileFormat:   format,
	})
	if err != nil {
		slog.Error("upsert voyage opinion", "voyage_id", in.VoyageID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to save opinion record")
	}
	return &opinionOutput{Body: dto.VoyageOpinionFromDB(opinion)}, nil
}

func (h *VoyageOpinionHandler) download(ctx context.Context, in *opinionParam) (*opinionFileOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.verifyVoyage(ctx, in.VoyageID, user.UserID); err != nil {
		return nil, err
	}
	opinion, err := h.opinionForVoyage(ctx, in.OpinionID, in.VoyageID)
	if err != nil {
		return nil, err
	}
	fileBytes, err := os.ReadFile(opinion.FilePath)
	if err != nil {
		slog.Error("read opinion file", "opinion_id", in.OpinionID, "path", opinion.FilePath, "err", err)
		return nil, huma.Error500InternalServerError("failed to read opinion file")
	}
	return &opinionFileOutput{
		ContentType:        opinionContentType(opinion.FileFormat),
		ContentDisposition: fmt.Sprintf(`attachment; filename="opinion_%d.%s"`, opinion.ID, opinion.FileFormat),
		Body:               fileBytes,
	}, nil
}

func (h *VoyageOpinionHandler) delete(ctx context.Context, in *opinionParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.verifyVoyage(ctx, in.VoyageID, user.UserID); err != nil {
		return nil, err
	}
	opinion, err := h.opinionForVoyage(ctx, in.OpinionID, in.VoyageID)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(opinion.FilePath)
	if err := h.q.DeleteVoyageOpinion(ctx, in.OpinionID); err != nil {
		slog.Error("delete voyage opinion", "opinion_id", in.OpinionID, "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete opinion")
	}
	return &noContentOutput{}, nil
}

// verifyVoyage confirms the voyage exists and belongs to the caller.
func (h *VoyageOpinionHandler) verifyVoyage(ctx context.Context, voyageID, userID int64) error {
	if _, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: userID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage for opinion", "voyage_id", voyageID, "user_id", userID, "err", err)
		return huma.Error500InternalServerError("failed to verify voyage")
	}
	return nil
}

// opinionForVoyage loads an opinion and confirms it belongs to the voyage.
func (h *VoyageOpinionHandler) opinionForVoyage(ctx context.Context, opinionID, voyageID int64) (sqlcdb.VoyageOpinion, error) {
	opinion, err := h.q.GetVoyageOpinion(ctx, opinionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlcdb.VoyageOpinion{}, huma.Error404NotFound("opinion not found")
		}
		slog.Error("get voyage opinion", "opinion_id", opinionID, "voyage_id", voyageID, "err", err)
		return sqlcdb.VoyageOpinion{}, huma.Error500InternalServerError("failed to get opinion")
	}
	if opinion.VoyageID != voyageID {
		return sqlcdb.VoyageOpinion{}, huma.Error404NotFound("opinion not found")
	}
	return opinion, nil
}

// opinionContentType maps an opinion file format to its MIME type.
func opinionContentType(format string) string {
	if format == "docx" {
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return "application/pdf"
}

// buildOpinionData assembles the document payload from the voyage and crew
// assignment, resolving the yacht name/type and the effective patent number.
func (h *VoyageOpinionHandler) buildOpinionData(ctx context.Context, userID int64, voyage sqlcdb.Voyage, assignment sqlcdb.GetVoyageCrewAssignmentByMemberRow) docgen.OpinionData {
	var yachtName, yachtType string
	if voyage.YachtID.Valid {
		if yacht, err := h.q.GetYacht(ctx, sqlcdb.GetYachtParams{ID: voyage.YachtID.Int64, OwnerID: userID}); err == nil {
			yachtName = yacht.Name
			yachtType = yacht.YachtType.String
		}
	}

	patent := assignment.PatentNumber.String
	if patent == "" {
		patent = assignment.MemberPatent.String
	}

	return docgen.OpinionData{
		CrewMemberName: assignment.FullName,
		PatentNumber:   patent,
		CruiseName:     voyage.Name,
		EmbarkDate:     voyage.EmbarkDate.String,
		DisembarkDate:  voyage.DisembarkDate.String,
		YachtName:      yachtName,
		YachtType:      yachtType,
		StartPort:      voyage.StartPort.String,
		EndPort:        voyage.EndPort.String,
		Countries:      voyage.Countries.String,
		Miles:          voyage.Miles,
		HoursTotal:     voyage.HoursTotal,
		HoursSail:      voyage.HoursSail,
		HoursEngine:    voyage.HoursEngine,
		HoursOver6bf:   voyage.HoursOver6bf,
		Days:           voyage.Days,
		TidalWaters:    voyage.TidalWaters > 0,
		CaptainName:    voyage.CaptainName.String,
		Role:           assignment.Role,
		GeneratedDate:  time.Now().Format("2006-01-02"),
	}
}

// renderOpinionFile produces the opinion document bytes for the requested
// format. PDF goes through the HTML template; docx is generated directly.
func renderOpinionFile(format string, data docgen.OpinionData) ([]byte, error) {
	switch format {
	case "pdf":
		html, err := docgen.RenderHTML(data)
		if err != nil {
			return nil, err
		}
		return docgen.GeneratePDF(html)
	case "docx":
		return docgen.GenerateDOCX(data)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
