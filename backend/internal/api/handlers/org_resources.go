package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type OrgYachtHandler struct {
	q sqlcdb.Querier
}

func NewOrgYachtHandler(q sqlcdb.Querier) *OrgYachtHandler {
	return &OrgYachtHandler{q: q}
}

func (h *OrgYachtHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	yachts, err := h.q.ListOrgYachts(r.Context(), types.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
		slog.Error("list org yachts", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list yachts")
		return
	}
	respondJSON(w, http.StatusOK, yachts)
}

func (h *OrgYachtHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	yacht, err := h.q.GetOrgYacht(r.Context(), sqlcdb.GetOrgYachtParams{
		ID:    id,
		OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "yacht not found")
			return
		}
		slog.Error("get org yacht", "yacht_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get yacht")
		return
	}
	respondJSON(w, http.StatusOK, yacht)
}

func (h *OrgYachtHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	var req struct {
		Name           string  `json:"name"`
		RegistrationNo *string `json:"registration_no"`
		YachtType      *string `json:"yacht_type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	yacht, err := h.q.CreateOrgYacht(r.Context(), sqlcdb.CreateOrgYachtParams{
		OwnerID:        user.UserID,
		OrgID:          types.NullInt64{Int64: octx.OrgID, Valid: true},
		Name:           req.Name,
		RegistrationNo: nullString(req.RegistrationNo),
		YachtType:      nullString(req.YachtType),
	})
	if err != nil {
		slog.Error("create org yacht", "org_id", octx.OrgID, "user_id", user.UserID, "name", req.Name, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create yacht")
		return
	}
	respondJSON(w, http.StatusCreated, yacht)
}

func (h *OrgYachtHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	var req struct {
		Name           string  `json:"name"`
		RegistrationNo *string `json:"registration_no"`
		YachtType      *string `json:"yacht_type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateOrgYacht(r.Context(), sqlcdb.UpdateOrgYachtParams{
		Name:           req.Name,
		RegistrationNo: nullString(req.RegistrationNo),
		YachtType:      nullString(req.YachtType),
		ID:             id,
		OrgID:          types.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("update org yacht", "yacht_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update yacht")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgYachtHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid yacht id")
		return
	}
	if err := h.q.DeleteOrgYacht(r.Context(), sqlcdb.DeleteOrgYachtParams{
		ID:    id,
		OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("delete org yacht", "yacht_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete yacht")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

type OrgCrewHandler struct {
	q sqlcdb.Querier
}

func NewOrgCrewHandler(q sqlcdb.Querier) *OrgCrewHandler {
	return &OrgCrewHandler{q: q}
}

func (h *OrgCrewHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	crew, err := h.q.ListOrgCrewMembers(r.Context(), types.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
		slog.Error("list org crew members", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to list crew members")
		return
	}
	respondJSON(w, http.StatusOK, crew)
}

func (h *OrgCrewHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	member, err := h.q.GetOrgCrewMember(r.Context(), sqlcdb.GetOrgCrewMemberParams{
		ID:    id,
		OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "crew member not found")
			return
		}
		slog.Error("get org crew member", "crew_member_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get crew member")
		return
	}
	respondJSON(w, http.StatusOK, member)
}

type orgCrewRequest struct {
	FullName              string  `json:"full_name"`
	Email                 *string `json:"email"`
	PatentNumber          *string `json:"patent_number"`
	Phone                 *string `json:"phone"`
	PzzLicenseType        *string `json:"pzz_license_type"`
	PzzLicenseNumber      *string `json:"pzz_license_number"`
	EmergencyContactName  *string `json:"emergency_contact_name"`
	EmergencyContactPhone *string `json:"emergency_contact_phone"`
}

func (h *OrgCrewHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	var req orgCrewRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FullName == "" {
		respondError(w, http.StatusBadRequest, "full_name is required")
		return
	}
	member, err := h.q.CreateOrgCrewMember(r.Context(), sqlcdb.CreateOrgCrewMemberParams{
		OwnerID:               user.UserID,
		OrgID:                 types.NullInt64{Int64: octx.OrgID, Valid: true},
		UserID:                types.NullInt64{},
		FullName:              req.FullName,
		Email:                 nullString(req.Email),
		PatentNumber:          nullString(req.PatentNumber),
		Phone:                 nullString(req.Phone),
		PzzLicenseType:        nullString(req.PzzLicenseType),
		PzzLicenseNumber:      nullString(req.PzzLicenseNumber),
		EmergencyContactName:  nullString(req.EmergencyContactName),
		EmergencyContactPhone: nullString(req.EmergencyContactPhone),
	})
	if err != nil {
		slog.Error("create org crew member", "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to create crew member")
		return
	}
	respondJSON(w, http.StatusCreated, member)
}

func (h *OrgCrewHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	var req orgCrewRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FullName == "" {
		respondError(w, http.StatusBadRequest, "full_name is required")
		return
	}
	if err := h.q.UpdateOrgCrewMember(r.Context(), sqlcdb.UpdateOrgCrewMemberParams{
		FullName:              req.FullName,
		Email:                 nullString(req.Email),
		PatentNumber:          nullString(req.PatentNumber),
		Phone:                 nullString(req.Phone),
		PzzLicenseType:        nullString(req.PzzLicenseType),
		PzzLicenseNumber:      nullString(req.PzzLicenseNumber),
		EmergencyContactName:  nullString(req.EmergencyContactName),
		EmergencyContactPhone: nullString(req.EmergencyContactPhone),
		ID:                    id,
		OrgID:                 types.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("update org crew member", "crew_member_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to update crew member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgCrewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid crew member id")
		return
	}
	if err := h.q.DeleteOrgCrewMember(r.Context(), sqlcdb.DeleteOrgCrewMemberParams{
		ID:    id,
		OrgID: types.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		slog.Error("delete org crew member", "crew_member_id", id, "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to delete crew member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

type OrgDashboardHandler struct {
	q sqlcdb.Querier
}

func NewOrgDashboardHandler(q sqlcdb.Querier) *OrgDashboardHandler {
	return &OrgDashboardHandler{q: q}
}

func (h *OrgDashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	orgID := types.NullInt64{Int64: octx.OrgID, Valid: true}

	stats, err := h.q.GetOrgDashboardStats(r.Context(), orgID)
	if err != nil {
		slog.Error("org dashboard stats", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}

	byYear, err := h.q.GetOrgVoyagesByYear(r.Context(), orgID)
	if err != nil {
		slog.Error("org dashboard voyages by year", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to get voyages by year")
		return
	}

	members, err := h.q.ListOrgMembers(r.Context(), octx.OrgID)
	if err != nil {
		slog.Error("org dashboard list members", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to count members")
		return
	}

	yachts, err := h.q.ListOrgYachts(r.Context(), orgID)
	if err != nil {
		slog.Error("org dashboard list yachts", "org_id", octx.OrgID, "err", err)
		respondError(w, http.StatusInternalServerError, "failed to count yachts")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"voyage_count":       stats.VoyageCount,
		"total_hours":        stats.TotalHours,
		"total_miles":        stats.TotalMiles,
		"total_days":         stats.TotalDays,
		"total_hours_sail":   stats.TotalHoursSail,
		"total_hours_engine": stats.TotalHoursEngine,
		"member_count":       len(members),
		"yacht_count":        len(yachts),
		"by_year":            byYear,
	})
}
