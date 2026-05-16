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
	format := opinionFormat(in.Body.Format)

	voyage, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.VoyageID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("get voyage for opinion", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get voyage")
	}

	assignment, err := getOpinionCrewAssignment(ctx, h.q, in.VoyageID, in.Body.CrewMemberID)
	if err != nil {
		return nil, err
	}

	yachtName, yachtType := "", ""
	if voyage.YachtID.Valid {
		if yacht, err := h.q.GetYacht(ctx, sqlcdb.GetYachtParams{ID: voyage.YachtID.Int64, OwnerID: user.UserID}); err == nil {
			yachtName, yachtType = yacht.Name, yacht.YachtType.String
		}
	}

	data := opinionDocData(voyage, assignment, yachtName, yachtType)
	filePath, err := storeOpinionFile(h.uploadDir, opinionScopeKey("user", user.UserID), in.VoyageID, in.Body.CrewMemberID, format, data)
	if err != nil {
		slog.Error("store opinion file", "voyage_id", in.VoyageID, "crew_member_id", in.Body.CrewMemberID, "format", format, "err", err)
		return nil, huma.Error500InternalServerError("failed to generate opinion document")
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
	opinion, err := loadVoyageOpinion(ctx, h.q, in.OpinionID, in.VoyageID)
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
	opinion, err := loadVoyageOpinion(ctx, h.q, in.OpinionID, in.VoyageID)
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

// loadVoyageOpinion loads an opinion and confirms it belongs to the voyage.
func loadVoyageOpinion(ctx context.Context, q sqlcdb.Querier, opinionID, voyageID int64) (sqlcdb.VoyageOpinion, error) {
	opinion, err := q.GetVoyageOpinion(ctx, opinionID)
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

// opinionFormat normalizes the requested document format, defaulting to pdf.
func opinionFormat(format string) string {
	if format == "" {
		return "pdf"
	}
	return format
}

// opinionScopeKey builds the upload-directory segment that segregates
// owner-scoped and org-scoped opinion files so their paths never collide.
func opinionScopeKey(kind string, id int64) string {
	return kind + "_" + strconv.FormatInt(id, 10)
}

// getOpinionCrewAssignment loads the voyage crew assignment for a member,
// mapping a missing row to a 404 so callers return it directly.
func getOpinionCrewAssignment(ctx context.Context, q sqlcdb.Querier, voyageID, crewMemberID int64) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
	assignment, err := q.GetVoyageCrewAssignmentByMember(ctx, sqlcdb.GetVoyageCrewAssignmentByMemberParams{
		VoyageID:     types.NullInt64{Int64: voyageID, Valid: true},
		CrewMemberID: crewMemberID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return assignment, huma.Error404NotFound("crew member not assigned to this voyage")
		}
		slog.Error("get voyage crew assignment for opinion", "voyage_id", voyageID, "crew_member_id", crewMemberID, "err", err)
		return assignment, huma.Error500InternalServerError("failed to get crew assignment")
	}
	return assignment, nil
}

// storeOpinionFile renders the opinion document and writes it under the upload
// directory, removing any prior file in a different format.
func storeOpinionFile(uploadDir, scopeKey string, voyageID, crewMemberID int64, format string, data docgen.OpinionData) (string, error) {
	fileBytes, err := renderOpinionFile(format, data)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(uploadDir, scopeKey, "opinions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for _, oldFmt := range []string{"pdf", "docx"} {
		if oldFmt != format {
			_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%d_%d.%s", voyageID, crewMemberID, oldFmt)))
		}
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%d_%d.%s", voyageID, crewMemberID, format))
	if err := os.WriteFile(filePath, fileBytes, 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

// opinionDocData assembles the document payload from the voyage and crew
// assignment, taking the already-resolved yacht details and the effective
// patent number (the assignment's, falling back to the member's).
func opinionDocData(voyage sqlcdb.Voyage, assignment sqlcdb.GetVoyageCrewAssignmentByMemberRow, yachtName, yachtType string) docgen.OpinionData {
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
