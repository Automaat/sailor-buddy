package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

type OrgHandler struct {
	q sqlcdb.Querier
}

func NewOrgHandler(q sqlcdb.Querier) *OrgHandler {
	return &OrgHandler{q: q}
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type orgRequest struct {
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	Description   *string `json:"description"`
	LogoUrl       *string `json:"logo_url"`
	PzzClubNumber *string `json:"pzz_club_number"`
	City          *string `json:"city"`
	Website       *string `json:"website"`
}

func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	orgs, err := h.q.ListUserOrganizations(r.Context(), user.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}
	respondJSON(w, http.StatusOK, orgs)
}

func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	var req orgRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Slug == "" {
		respondError(w, http.StatusBadRequest, "slug is required")
		return
	}
	req.Slug = strings.ToLower(req.Slug)
	if !slugRe.MatchString(req.Slug) {
		respondError(w, http.StatusBadRequest, "slug must contain only lowercase letters, numbers, and hyphens")
		return
	}

	org, err := h.q.CreateOrganization(r.Context(), sqlcdb.CreateOrganizationParams{
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   nullString(req.Description),
		LogoUrl:       nullString(req.LogoUrl),
		PzzClubNumber: nullString(req.PzzClubNumber),
		City:          nullString(req.City),
		Website:       nullString(req.Website),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, http.StatusConflict, "slug already taken")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	_, err = h.q.AddOrgMember(r.Context(), sqlcdb.AddOrgMemberParams{
		OrgID:  org.ID,
		UserID: user.UserID,
		Role:   "admin",
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add creator as admin")
		return
	}

	respondJSON(w, http.StatusCreated, org)
}

func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, err := h.q.GetOrganizationBySlug(r.Context(), slug)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "organization not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get organization")
		return
	}
	respondJSON(w, http.StatusOK, org)
}

func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	var req orgRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.q.UpdateOrganization(r.Context(), sqlcdb.UpdateOrganizationParams{
		Name:          req.Name,
		Description:   nullString(req.Description),
		LogoUrl:       nullString(req.LogoUrl),
		PzzClubNumber: nullString(req.PzzClubNumber),
		City:          nullString(req.City),
		Website:       nullString(req.Website),
		ID:            octx.OrgID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update organization")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgHandler) Delete(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	if err := h.q.DeleteOrganization(r.Context(), octx.OrgID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete organization")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	members, err := h.q.ListOrgMembers(r.Context(), octx.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	respondJSON(w, http.StatusOK, members)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

func (h *OrgHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	memberID, err := strconv.ParseInt(chi.URLParam(r, "memberID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid member id")
		return
	}
	var req updateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.Role {
	case "admin", "captain", "crew":
	default:
		respondError(w, http.StatusBadRequest, "invalid role")
		return
	}
	if req.Role != "admin" {
		members, err := h.q.ListOrgMembers(r.Context(), octx.OrgID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check members")
			return
		}
		var isAdmin bool
		for _, m := range members {
			if m.ID == memberID && m.Role == "admin" {
				isAdmin = true
				break
			}
		}
		if isAdmin {
			count, err := h.q.CountOrgAdmins(r.Context(), octx.OrgID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to check admins")
				return
			}
			if count <= 1 {
				respondError(w, http.StatusBadRequest, "cannot demote the last admin")
				return
			}
		}
	}
	if err := h.q.UpdateOrgMemberRole(r.Context(), sqlcdb.UpdateOrgMemberRoleParams{
		Role:  req.Role,
		ID:    memberID,
		OrgID: octx.OrgID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update role")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	memberID, err := strconv.ParseInt(chi.URLParam(r, "memberID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid member id")
		return
	}

	count, err := h.q.CountOrgAdmins(r.Context(), octx.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check admins")
		return
	}

	members, err := h.q.ListOrgMembers(r.Context(), octx.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	for _, m := range members {
		if m.ID == memberID && m.Role == "admin" && count <= 1 {
			respondError(w, http.StatusBadRequest, "cannot remove the last admin")
			return
		}
	}

	if err := h.q.RemoveOrgMember(r.Context(), sqlcdb.RemoveOrgMemberParams{
		ID:    memberID,
		OrgID: octx.OrgID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

type createInviteRequest struct {
	Role      string `json:"role"`
	ExpiresIn *int64 `json:"expires_in_hours"`
	MaxUses   *int64 `json:"max_uses"`
}

func (h *OrgHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	user := middleware.GetUser(r.Context())
	var req createInviteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.Role {
	case "admin", "captain", "crew":
	default:
		req.Role = "crew"
	}
	if req.ExpiresIn != nil && *req.ExpiresIn <= 0 {
		respondError(w, http.StatusBadRequest, "expires_in_hours must be greater than 0")
		return
	}
	if req.MaxUses != nil && *req.MaxUses <= 0 {
		respondError(w, http.StatusBadRequest, "max_uses must be greater than 0")
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	token := hex.EncodeToString(b)

	var expiresAt sql.NullTime
	if req.ExpiresIn != nil {
		expiresAt = sql.NullTime{
			Time:  time.Now().Add(time.Duration(*req.ExpiresIn) * time.Hour),
			Valid: true,
		}
	}

	invite, err := h.q.CreateOrgInvite(r.Context(), sqlcdb.CreateOrgInviteParams{
		OrgID:     octx.OrgID,
		Token:     token,
		Role:      req.Role,
		CreatedBy: user.UserID,
		ExpiresAt: expiresAt,
		MaxUses:   nullInt64(req.MaxUses),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}
	respondJSON(w, http.StatusCreated, invite)
}

func (h *OrgHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	invites, err := h.q.ListOrgInvites(r.Context(), octx.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}
	respondJSON(w, http.StatusOK, invites)
}

func (h *OrgHandler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	octx := middleware.GetOrg(r.Context())
	inviteID, err := strconv.ParseInt(chi.URLParam(r, "inviteID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid invite id")
		return
	}
	if err := h.q.DeleteOrgInvite(r.Context(), sqlcdb.DeleteOrgInviteParams{
		ID:    inviteID,
		OrgID: octx.OrgID,
	}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete invite")
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *OrgHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	user := middleware.GetUser(r.Context())

	invite, err := h.q.GetOrgInviteByToken(r.Context(), token)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "invalid invite link")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get invite")
		return
	}

	if invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()) {
		respondError(w, http.StatusGone, "invite has expired")
		return
	}

	if invite.MaxUses.Valid {
		rows, err := h.q.IncrementInviteUseCount(r.Context(), invite.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to claim invite")
			return
		}
		if rows == 0 {
			respondError(w, http.StatusGone, "invite has reached maximum uses")
			return
		}
	}

	_, err = h.q.AddOrgMember(r.Context(), sqlcdb.AddOrgMemberParams{
		OrgID:  invite.OrgID,
		UserID: user.UserID,
		Role:   invite.Role,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondError(w, http.StatusConflict, "already a member")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to join organization")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"org_name": invite.OrgName,
		"org_slug": invite.OrgSlug,
		"role":     invite.Role,
	})
}

func (h *OrgHandler) GetInviteInfo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	invite, err := h.q.GetOrgInviteByToken(r.Context(), token)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "invalid invite link")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get invite")
		return
	}

	if invite.ExpiresAt.Valid && invite.ExpiresAt.Time.Before(time.Now()) {
		respondError(w, http.StatusGone, "invite has expired")
		return
	}
	if invite.MaxUses.Valid && invite.UseCount >= invite.MaxUses.Int64 {
		respondError(w, http.StatusGone, "invite has reached maximum uses")
		return
	}

	user := middleware.GetUser(r.Context())
	_, memberErr := h.q.GetOrgMembership(r.Context(), sqlcdb.GetOrgMembershipParams{
		OrgID:  invite.OrgID,
		UserID: user.UserID,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"org_name":       invite.OrgName,
		"org_slug":       invite.OrgSlug,
		"role":           invite.Role,
		"already_member": memberErr == nil,
	})
}
