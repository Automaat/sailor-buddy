package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/docgen"
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
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
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

	cruise, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: cruiseID, OwnerID: user.UserID})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}

	assignment, err := h.q.GetCrewAssignmentByCruiseAndMember(r.Context(), sqlcdb.GetCrewAssignmentByCruiseAndMemberParams{
		CruiseID:     cruiseID,
		CrewMemberID: req.CrewMemberID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "crew member not assigned to this cruise")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get crew assignment")
		return
	}

	var yachtName, yachtType string
	if cruise.YachtID.Valid {
		yacht, err := h.q.GetYacht(r.Context(), sqlcdb.GetYachtParams{ID: cruise.YachtID.Int64, OwnerID: user.UserID})
		if err == nil {
			yachtName = yacht.Name
			yachtType = yacht.YachtType.String
		}
	}

	patent := assignment.PatentNumber.String
	if patent == "" {
		patent = assignment.MemberPatent.String
	}

	data := docgen.OpinionData{
		CrewMemberName: assignment.FullName,
		PatentNumber:   patent,
		CruiseName:     cruise.Name,
		EmbarkDate:     cruise.EmbarkDate.String,
		DisembarkDate:  cruise.DisembarkDate.String,
		YachtName:      yachtName,
		YachtType:      yachtType,
		StartPort:      cruise.StartPort.String,
		EndPort:        cruise.EndPort.String,
		Countries:      cruise.Countries.String,
		Miles:          cruise.Miles.Float64,
		HoursTotal:     cruise.HoursTotal.Float64,
		HoursSail:      cruise.HoursSail.Float64,
		HoursEngine:    cruise.HoursEngine.Float64,
		HoursOver6bf:   cruise.HoursOver6bf.Float64,
		Days:           cruise.Days.Int64,
		TidalWaters:    cruise.TidalWaters.Int64 > 0,
		CaptainName:    cruise.CaptainName.String,
		Role:           assignment.Role,
		GeneratedDate:  time.Now().Format("2006-01-02"),
	}

	var fileBytes []byte
	switch req.Format {
	case "pdf":
		html, err := docgen.RenderHTML(data)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to render template")
			return
		}
		fileBytes, err = docgen.GeneratePDF(html)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to generate PDF")
			return
		}
	case "docx":
		var err error
		fileBytes, err = docgen.GenerateDOCX(data)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to generate DOCX")
			return
		}
	}

	dir := filepath.Join(h.uploadDir, strconv.FormatInt(user.UserID, 10), "opinions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}

	for _, oldFmt := range []string{"pdf", "docx"} {
		if oldFmt != req.Format {
			_ = os.Remove(filepath.Join(dir, fmt.Sprintf("%d_%d.%s", cruiseID, req.CrewMemberID, oldFmt)))
		}
	}

	filename := fmt.Sprintf("%d_%d.%s", cruiseID, req.CrewMemberID, req.Format)
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, fileBytes, 0o644); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	opinion, err := h.q.UpsertVoyageOpinion(r.Context(), sqlcdb.UpsertVoyageOpinionParams{
		CruiseID:     cruiseID,
		CrewMemberID: req.CrewMemberID,
		FilePath:     filePath,
		FileFormat:   req.Format,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save opinion record")
		return
	}

	respondJSON(w, http.StatusCreated, opinion)
}

func (h *VoyageOpinionHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}

	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: cruiseID, OwnerID: user.UserID}); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
		return
	}

	opinions, err := h.q.ListCruiseVoyageOpinions(r.Context(), cruiseID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list opinions")
		return
	}

	respondJSON(w, http.StatusOK, opinions)
}

func (h *VoyageOpinionHandler) Download(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}

	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: cruiseID, OwnerID: user.UserID}); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
		return
	}

	opID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opinion id")
		return
	}

	opinion, err := h.q.GetVoyageOpinion(r.Context(), opID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "opinion not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get opinion")
		return
	}

	if opinion.CruiseID != cruiseID {
		respondError(w, http.StatusNotFound, "opinion not found")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="opinion_%d.%s"`, opinion.ID, opinion.FileFormat))
	http.ServeFile(w, r, opinion.FilePath)
}

func (h *VoyageOpinionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	cruiseID, err := strconv.ParseInt(chi.URLParam(r, "cruiseID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}

	if _, err := h.q.GetCruise(r.Context(), sqlcdb.GetCruiseParams{ID: cruiseID, OwnerID: user.UserID}); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to verify cruise")
		return
	}

	opID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opinion id")
		return
	}

	opinion, err := h.q.GetVoyageOpinion(r.Context(), opID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "opinion not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get opinion")
		return
	}

	if opinion.CruiseID != cruiseID {
		respondError(w, http.StatusNotFound, "opinion not found")
		return
	}

	_ = os.Remove(opinion.FilePath)

	if err := h.q.DeleteVoyageOpinion(r.Context(), opID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete opinion")
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}
