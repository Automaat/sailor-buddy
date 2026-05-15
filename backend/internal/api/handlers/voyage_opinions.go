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

	"github.com/go-chi/chi/v5"
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

type generateRequest struct {
	CrewMemberID int64  `json:"crew_member_id"`
	Format       string `json:"format"`
}

func (h *VoyageOpinionHandler) Generate(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}

	var req generateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CrewMemberID == 0 {
		respondError(w, http.StatusBadRequest, "crew_member_id is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if req.Format != "pdf" && req.Format != "docx" {
		respondError(w, http.StatusBadRequest, "format must be pdf or docx")
		return
	}

	voyage, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("get voyage for opinion", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get voyage")
		return
	}

	assignment, err := h.q.GetVoyageCrewAssignmentByMember(r.Context(), sqlcdb.GetVoyageCrewAssignmentByMemberParams{
		VoyageID:     types.NullInt64{Int64: voyageID, Valid: true},
		CrewMemberID: req.CrewMemberID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "crew member not assigned to this voyage")
			return
		}
		slog.Error("get voyage crew assignment for opinion", "voyage_id", voyageID, "crew_member_id", req.CrewMemberID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get crew assignment")
		return
	}

	data := h.buildOpinionData(r.Context(), user.UserID, voyage, assignment)

	fileBytes, err := renderOpinionFile(req.Format, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate "+req.Format)
		return
	}

	dir := filepath.Join(h.uploadDir, strconv.FormatInt(user.UserID, 10), "opinions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}

	for _, oldFmt := range []string{"pdf", "docx"} {
		if oldFmt != req.Format {
			_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%d_%d.%s", voyageID, req.CrewMemberID, oldFmt)))
		}
	}

	filename := fmt.Sprintf("%d_%d.%s", voyageID, req.CrewMemberID, req.Format)
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, fileBytes, 0o644); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	opinion, err := h.q.UpsertVoyageOpinion(r.Context(), sqlcdb.UpsertVoyageOpinionParams{
		VoyageID:     voyageID,
		CrewMemberID: req.CrewMemberID,
		FilePath:     filePath,
		FileFormat:   req.Format,
	})
	if err != nil {
		slog.Error("upsert voyage opinion", "voyage_id", voyageID, "crew_member_id", req.CrewMemberID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to save opinion record")
		return
	}

	respondJSON(w, http.StatusCreated, opinion)
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

func (h *VoyageOpinionHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}

	if _, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("verify voyage for opinion list", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify voyage")
		return
	}

	opinions, err := h.q.ListVoyageVoyageOpinions(r.Context(), voyageID)
	if err != nil {
		slog.Error("list voyage opinions", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list opinions")
		return
	}

	respondJSON(w, http.StatusOK, opinions)
}

func (h *VoyageOpinionHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}

	if _, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("verify voyage for opinion download", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify voyage")
		return
	}

	opID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opinion id")
		return
	}

	opinion, err := h.q.GetVoyageOpinion(r.Context(), opID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "opinion not found")
			return
		}
		slog.Error("get voyage opinion for download", "opinion_id", opID, "voyage_id", voyageID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get opinion")
		return
	}

	if opinion.VoyageID != voyageID {
		respondError(w, http.StatusNotFound, "opinion not found")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="opinion_%d.%s"`, opinion.ID, opinion.FileFormat))
	http.ServeFile(w, r, opinion.FilePath)
}

func (h *VoyageOpinionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	voyageID, err := strconv.ParseInt(chi.URLParam(r, "voyageID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid voyage id")
		return
	}

	if _, err := h.q.GetVoyage(r.Context(), sqlcdb.GetVoyageParams{ID: voyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "voyage not found")
			return
		}
		slog.Error("verify voyage for opinion delete", "voyage_id", voyageID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to verify voyage")
		return
	}

	opID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opinion id")
		return
	}

	opinion, err := h.q.GetVoyageOpinion(r.Context(), opID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "opinion not found")
			return
		}
		slog.Error("get voyage opinion for delete", "opinion_id", opID, "voyage_id", voyageID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get opinion")
		return
	}

	if opinion.VoyageID != voyageID {
		respondError(w, http.StatusNotFound, "opinion not found")
		return
	}

	_ = os.Remove(opinion.FilePath)

	if err := h.q.DeleteVoyageOpinion(r.Context(), opID); err != nil {
		slog.Error("delete voyage opinion", "opinion_id", opID, "voyage_id", voyageID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete opinion")
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}
