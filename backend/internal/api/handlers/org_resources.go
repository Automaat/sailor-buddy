package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgYachtHandler struct {
	q sqlcdb.Querier
}

func NewOrgYachtHandler(q sqlcdb.Querier) *OrgYachtHandler {
	return &OrgYachtHandler{q: q}
}

func (h *OrgYachtHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	yachts, err := h.q.ListOrgYachts(r.Context(), sql.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
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
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "yacht not found")
			return
		}
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
		OrgID:          sql.NullInt64{Int64: octx.OrgID, Valid: true},
		Name:           req.Name,
		RegistrationNo: nullString(req.RegistrationNo),
		YachtType:      nullString(req.YachtType),
	})
	if err != nil {
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
		OrgID:          sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
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
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete yacht")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

type OrgCruiseHandler struct {
	q sqlcdb.Querier
}

func NewOrgCruiseHandler(q sqlcdb.Querier) *OrgCruiseHandler {
	return &OrgCruiseHandler{q: q}
}

func (h *OrgCruiseHandler) List(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	cruises, err := h.q.ListOrgCruises(r.Context(), sql.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list cruises")
		return
	}
	respondJSON(w, http.StatusOK, cruises)
}

func (h *OrgCruiseHandler) Get(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	cruise, err := h.q.GetOrgCruise(r.Context(), sqlcdb.GetOrgCruiseParams{
		ID:    id,
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "cruise not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get cruise")
		return
	}
	respondJSON(w, http.StatusOK, cruise)
}

func (h *OrgCruiseHandler) Create(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	var req cruiseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	cruise, err := h.q.CreateOrgCruise(r.Context(), sqlcdb.CreateOrgCruiseParams{
		OwnerID:       user.UserID,
		OrgID:         sql.NullInt64{Int64: octx.OrgID, Valid: true},
		Name:          req.Name,
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		HoursTotal:    nullFloat64(req.HoursTotal),
		HoursSail:     nullFloat64(req.HoursSail),
		HoursEngine:   nullFloat64(req.HoursEngine),
		HoursOver6bf:  nullFloat64(req.HoursOver6bf),
		Miles:         nullFloat64(req.Miles),
		Days:          nullInt64(req.Days),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		TidalWaters:   nullInt64(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		MaxCrew:       nullInt64(req.MaxCrew),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create cruise")
		return
	}
	respondJSON(w, http.StatusCreated, cruise)
}

func (h *OrgCruiseHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	var req cruiseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateOrgCruise(r.Context(), sqlcdb.UpdateOrgCruiseParams{
		Name:          req.Name,
		Year:          nullInt64(req.Year),
		EmbarkDate:    nullString(req.EmbarkDate),
		DisembarkDate: nullString(req.DisembarkDate),
		Countries:     nullString(req.Countries),
		StartPort:     nullString(req.StartPort),
		EndPort:       nullString(req.EndPort),
		HoursTotal:    nullFloat64(req.HoursTotal),
		HoursSail:     nullFloat64(req.HoursSail),
		HoursEngine:   nullFloat64(req.HoursEngine),
		HoursOver6bf:  nullFloat64(req.HoursOver6bf),
		Miles:         nullFloat64(req.Miles),
		Days:          nullInt64(req.Days),
		CaptainName:   nullString(req.CaptainName),
		YachtID:       nullInt64(req.YachtID),
		TidalWaters:   nullInt64(req.TidalWaters),
		CostTotal:     nullFloat64(req.CostTotal),
		CostPerPerson: nullFloat64(req.CostPerPerson),
		ImageLogoUrl:  nullString(req.ImageLogoUrl),
		ImagePhotoUrl: nullString(req.ImagePhotoUrl),
		ImageRouteUrl: nullString(req.ImageRouteUrl),
		Description:   nullString(req.Description),
		MaxCrew:       nullInt64(req.MaxCrew),
		ID:            id,
		OrgID:         sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update cruise")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgCruiseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cruise id")
		return
	}
	if err := h.q.DeleteOrgCruise(r.Context(), sqlcdb.DeleteOrgCruiseParams{
		ID:    id,
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete cruise")
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
	crew, err := h.q.ListOrgCrewMembers(r.Context(), sql.NullInt64{Int64: octx.OrgID, Valid: true})
	if err != nil {
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
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "crew member not found")
			return
		}
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
		OrgID:                 sql.NullInt64{Int64: octx.OrgID, Valid: true},
		UserID:                sql.NullInt64{},
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
		OrgID:                 sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
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
		OrgID: sql.NullInt64{Int64: octx.OrgID, Valid: true},
	}); err != nil {
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
	orgID := sql.NullInt64{Int64: octx.OrgID, Valid: true}

	stats, err := h.q.GetOrgDashboardStats(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}

	byYear, err := h.q.GetOrgCruisesByYear(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get cruises by year")
		return
	}

	members, err := h.q.ListOrgMembers(r.Context(), octx.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to count members")
		return
	}

	yachts, err := h.q.ListOrgYachts(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to count yachts")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"cruise_count":       stats.CruiseCount,
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
