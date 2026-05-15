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
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// orgID wraps an org context's ID as the nullable column the org queries take.
func orgID(octx *middleware.OrgContext) types.NullInt64 {
	return types.NullInt64{Int64: octx.OrgID, Valid: true}
}

// --- org yachts ---

type OrgYachtHandler struct {
	q sqlcdb.Querier
}

func NewOrgYachtHandler(q sqlcdb.Querier) *OrgYachtHandler {
	return &OrgYachtHandler{q: q}
}

type orgYachtParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"yachtID" doc:"Yacht ID"`
}

type createOrgYachtInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.YachtBody
}

type updateOrgYachtInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"yachtID" doc:"Yacht ID"`
	Body dto.YachtBody
}

// --- org crew ---

type OrgCrewHandler struct {
	q sqlcdb.Querier
}

func NewOrgCrewHandler(q sqlcdb.Querier) *OrgCrewHandler {
	return &OrgCrewHandler{q: q}
}

type orgCrewParam struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"crewID" doc:"Crew member ID"`
}

type createOrgCrewInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	Body dto.OrgCrewBody
}

type updateOrgCrewInput struct {
	Slug string `path:"slug" doc:"Organization slug"`
	ID   int64  `path:"crewID" doc:"Crew member ID"`
	Body dto.OrgCrewBody
}

// --- org dashboard ---

type OrgDashboardHandler struct {
	q sqlcdb.Querier
}

func NewOrgDashboardHandler(q sqlcdb.Querier) *OrgDashboardHandler {
	return &OrgDashboardHandler{q: q}
}

type orgDashboardOutput struct {
	Body dto.OrgDashboard
}

// RegisterOrgResourceRoutes wires the org yacht, crew and dashboard operations.
func RegisterOrgResourceRoutes(api huma.API, q sqlcdb.Querier) {
	yh := NewOrgYachtHandler(q)
	ch := NewOrgCrewHandler(q)
	dh := NewOrgDashboardHandler(q)
	tag := []string{"Org resources"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-yachts", Method: http.MethodGet, Path: "/orgs/{slug}/yachts",
		Summary: "List org yachts", Tags: tag,
	}, yh.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-yacht", Method: http.MethodGet, Path: "/orgs/{slug}/yachts/{yachtID}",
		Summary: "Get an org yacht", Tags: tag,
	}, yh.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-yacht", Method: http.MethodPost, Path: "/orgs/{slug}/yachts",
		Summary: "Create an org yacht (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, yh.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-yacht", Method: http.MethodPut, Path: "/orgs/{slug}/yachts/{yachtID}",
		Summary: "Update an org yacht (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, yh.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-yacht", Method: http.MethodDelete, Path: "/orgs/{slug}/yachts/{yachtID}",
		Summary: "Delete an org yacht (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, yh.delete)

	huma.Register(api, huma.Operation{
		OperationID: "list-org-crew", Method: http.MethodGet, Path: "/orgs/{slug}/crew",
		Summary: "List org crew members", Tags: tag,
	}, ch.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-org-crew-member", Method: http.MethodGet, Path: "/orgs/{slug}/crew/{crewID}",
		Summary: "Get an org crew member", Tags: tag,
	}, ch.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-org-crew-member", Method: http.MethodPost, Path: "/orgs/{slug}/crew",
		Summary: "Create an org crew member (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, ch.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-org-crew-member", Method: http.MethodPut, Path: "/orgs/{slug}/crew/{crewID}",
		Summary: "Update an org crew member (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, ch.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-org-crew-member", Method: http.MethodDelete, Path: "/orgs/{slug}/crew/{crewID}",
		Summary: "Delete an org crew member (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, ch.delete)

	huma.Register(api, huma.Operation{
		OperationID: "get-org-dashboard", Method: http.MethodGet, Path: "/orgs/{slug}/dashboard",
		Summary: "Org sailing and membership summary", Tags: tag,
	}, dh.get)
}

func (h *OrgYachtHandler) list(ctx context.Context, in *orgSlugParam) (*yachtListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	yachts, err := h.q.ListOrgYachts(ctx, orgID(octx))
	if err != nil {
		slog.Error("list org yachts", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list yachts")
	}
	return &yachtListOutput{Body: dto.YachtsFromDB(yachts)}, nil
}

func (h *OrgYachtHandler) get(ctx context.Context, in *orgYachtParam) (*yachtOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	yacht, err := h.q.GetOrgYacht(ctx, sqlcdb.GetOrgYachtParams{ID: in.ID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("yacht not found")
		}
		slog.Error("get org yacht", "yacht_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

func (h *OrgYachtHandler) create(ctx context.Context, in *createOrgYachtInput) (*yachtOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	yacht, err := h.q.CreateOrgYacht(ctx, sqlcdb.CreateOrgYachtParams{
		OwnerID:        user.UserID,
		OrgID:          orgID(octx),
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
	})
	if err != nil {
		slog.Error("create org yacht", "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to create yacht")
	}
	return &yachtOutput{Body: dto.YachtFromDB(yacht)}, nil
}

func (h *OrgYachtHandler) update(ctx context.Context, in *updateOrgYachtInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateOrgYacht(ctx, sqlcdb.UpdateOrgYachtParams{
		Name:           in.Body.Name,
		RegistrationNo: nullString(in.Body.RegistrationNo),
		YachtType:      nullString(in.Body.YachtType),
		ID:             in.ID,
		OrgID:          orgID(octx),
	}); err != nil {
		slog.Error("update org yacht", "yacht_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update yacht")
	}
	return &noContentOutput{}, nil
}

func (h *OrgYachtHandler) delete(ctx context.Context, in *orgYachtParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgYacht(ctx, sqlcdb.DeleteOrgYachtParams{ID: in.ID, OrgID: orgID(octx)}); err != nil {
		slog.Error("delete org yacht", "yacht_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete yacht")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCrewHandler) list(ctx context.Context, in *orgSlugParam) (*crewListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	crew, err := h.q.ListOrgCrewMembers(ctx, orgID(octx))
	if err != nil {
		slog.Error("list org crew members", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list crew members")
	}
	return &crewListOutput{Body: dto.CrewMembersFromDB(crew)}, nil
}

func (h *OrgCrewHandler) get(ctx context.Context, in *orgCrewParam) (*crewOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	member, err := h.q.GetOrgCrewMember(ctx, sqlcdb.GetOrgCrewMemberParams{ID: in.ID, OrgID: orgID(octx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("crew member not found")
		}
		slog.Error("get org crew member", "crew_member_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *OrgCrewHandler) create(ctx context.Context, in *createOrgCrewInput) (*crewOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	member, err := h.q.CreateOrgCrewMember(ctx, sqlcdb.CreateOrgCrewMemberParams{
		OwnerID:               user.UserID,
		OrgID:                 orgID(octx),
		UserID:                types.NullInt64{},
		FullName:              in.Body.FullName,
		Email:                 nullString(in.Body.Email),
		PatentNumber:          nullString(in.Body.PatentNumber),
		Phone:                 nullString(in.Body.Phone),
		PzzLicenseType:        nullString(in.Body.PzzLicenseType),
		PzzLicenseNumber:      nullString(in.Body.PzzLicenseNumber),
		EmergencyContactName:  nullString(in.Body.EmergencyContactName),
		EmergencyContactPhone: nullString(in.Body.EmergencyContactPhone),
	})
	if err != nil {
		slog.Error("create org crew member", "org_id", octx.OrgID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to create crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *OrgCrewHandler) update(ctx context.Context, in *updateOrgCrewInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateOrgCrewMember(ctx, sqlcdb.UpdateOrgCrewMemberParams{
		FullName:              in.Body.FullName,
		Email:                 nullString(in.Body.Email),
		PatentNumber:          nullString(in.Body.PatentNumber),
		Phone:                 nullString(in.Body.Phone),
		PzzLicenseType:        nullString(in.Body.PzzLicenseType),
		PzzLicenseNumber:      nullString(in.Body.PzzLicenseNumber),
		EmergencyContactName:  nullString(in.Body.EmergencyContactName),
		EmergencyContactPhone: nullString(in.Body.EmergencyContactPhone),
		ID:                    in.ID,
		OrgID:                 orgID(octx),
	}); err != nil {
		slog.Error("update org crew member", "crew_member_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update crew member")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCrewHandler) delete(ctx context.Context, in *orgCrewParam) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgCrewMember(ctx, sqlcdb.DeleteOrgCrewMemberParams{ID: in.ID, OrgID: orgID(octx)}); err != nil {
		slog.Error("delete org crew member", "crew_member_id", in.ID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete crew member")
	}
	return &noContentOutput{}, nil
}

func (h *OrgDashboardHandler) get(ctx context.Context, in *orgSlugParam) (*orgDashboardOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	id := orgID(octx)

	stats, err := h.q.GetOrgDashboardStats(ctx, id)
	if err != nil {
		slog.Error("org dashboard stats", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get dashboard stats")
	}
	byYear, err := h.q.GetOrgVoyagesByYear(ctx, id)
	if err != nil {
		slog.Error("org dashboard voyages by year", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get voyages by year")
	}
	members, err := h.q.ListOrgMembers(ctx, octx.OrgID)
	if err != nil {
		slog.Error("org dashboard list members", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to count members")
	}
	yachts, err := h.q.ListOrgYachts(ctx, id)
	if err != nil {
		slog.Error("org dashboard list yachts", "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to count yachts")
	}

	return &orgDashboardOutput{Body: dto.OrgDashboard{
		VoyageCount:      stats.VoyageCount,
		TotalHours:       stats.TotalHours,
		TotalMiles:       stats.TotalMiles,
		TotalDays:        stats.TotalDays,
		TotalHoursSail:   stats.TotalHoursSail,
		TotalHoursEngine: stats.TotalHoursEngine,
		MemberCount:      len(members),
		YachtCount:       len(yachts),
		ByYear:           dto.OrgVoyagesByYearFromDB(byYear),
	}}, nil
}
