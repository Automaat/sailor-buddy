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

type OrgCrewAssignmentHandler struct {
	q sqlcdb.Querier
}

func NewOrgCrewAssignmentHandler(q sqlcdb.Querier) *OrgCrewAssignmentHandler {
	return &OrgCrewAssignmentHandler{q: q}
}

type orgTripCrewListInput struct {
	Slug   string `path:"slug" doc:"Organization slug"`
	TripID int64  `path:"tripID" doc:"Trip ID"`
}

type assignOrgTripCrewInput struct {
	Slug   string `path:"slug" doc:"Organization slug"`
	TripID int64  `path:"tripID" doc:"Trip ID"`
	Body   dto.CrewAssignmentBody
}

type removeOrgTripCrewInput struct {
	Slug         string `path:"slug" doc:"Organization slug"`
	TripID       int64  `path:"tripID" doc:"Trip ID"`
	AssignmentID int64  `path:"assignmentID" doc:"Assignment ID"`
}

type orgVoyageCrewListInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
}

type assignOrgVoyageCrewInput struct {
	Slug     string `path:"slug" doc:"Organization slug"`
	VoyageID int64  `path:"voyageID" doc:"Voyage ID"`
	Body     dto.CrewAssignmentBody
}

type removeOrgVoyageCrewInput struct {
	Slug         string `path:"slug" doc:"Organization slug"`
	VoyageID     int64  `path:"voyageID" doc:"Voyage ID"`
	AssignmentID int64  `path:"assignmentID" doc:"Assignment ID"`
}

// RegisterOrgCrewAssignmentRoutes wires the org-scoped trip and voyage crew
// assignment operations onto the API.
func RegisterOrgCrewAssignmentRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewOrgCrewAssignmentHandler(q)
	tag := []string{"Org crew assignments"}

	huma.Register(api, huma.Operation{
		OperationID: "list-org-trip-crew", Method: http.MethodGet,
		Path:    "/orgs/{slug}/trips/{tripID}/crew",
		Summary: "List an org trip's crew", Tags: tag,
	}, h.listTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-org-trip-crew", Method: http.MethodPost,
		Path:    "/orgs/{slug}/trips/{tripID}/crew",
		Summary: "Assign a crew member to an org trip (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.assignTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-org-trip-crew", Method: http.MethodDelete,
		Path:    "/orgs/{slug}/trips/{tripID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from an org trip (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.removeTripCrew)

	huma.Register(api, huma.Operation{
		OperationID: "list-org-voyage-crew", Method: http.MethodGet,
		Path:    "/orgs/{slug}/voyages/{voyageID}/crew",
		Summary: "List an org voyage's crew", Tags: tag,
	}, h.listVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-org-voyage-crew", Method: http.MethodPost,
		Path:    "/orgs/{slug}/voyages/{voyageID}/crew",
		Summary: "Assign a crew member to an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusCreated,
	}, h.assignVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-org-voyage-crew", Method: http.MethodDelete,
		Path:    "/orgs/{slug}/voyages/{voyageID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from an org voyage (admin)", Tags: tag, DefaultStatus: http.StatusNoContent,
	}, h.removeVoyageCrew)
}

// verifyOrgCrewMember confirms the crew member belongs to the organization,
// so org routes cannot attach (and later expose) crew from another scope.
func (h *OrgCrewAssignmentHandler) verifyOrgCrewMember(ctx context.Context, crewMemberID int64, octx *middleware.OrgContext) error {
	if _, err := h.q.GetOrgCrewMember(ctx, sqlcdb.GetOrgCrewMemberParams{ID: crewMemberID, OrgID: orgID(octx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("crew member not found")
		}
		slog.Error("verify org crew member for assignment", "crew_member_id", crewMemberID, "org_id", octx.OrgID, "err", err)
		return huma.Error500InternalServerError("failed to verify crew member")
	}
	return nil
}

func (h *OrgCrewAssignmentHandler) listTripCrew(ctx context.Context, in *orgTripCrewListInput) (*crewAssignmentListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	assignments, err := h.q.ListOrgTripCrewAssignments(ctx, sqlcdb.ListOrgTripCrewAssignmentsParams{
		TripID: types.NullInt64{Int64: in.TripID, Valid: true},
		OrgID:  orgID(octx),
	})
	if err != nil {
		slog.Error("list org trip crew", "trip_id", in.TripID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trip crew")
	}
	return &crewAssignmentListOutput{Body: dto.OrgTripCrewFromDB(assignments)}, nil
}

func (h *OrgCrewAssignmentHandler) assignTripCrew(ctx context.Context, in *assignOrgTripCrewInput) (*crewAssignmentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.q.GetOrgTrip(ctx, sqlcdb.GetOrgTripParams{ID: in.TripID, OrgID: orgID(octx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("verify org trip for crew assignment", "trip_id", in.TripID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify trip")
	}
	if err := h.verifyOrgCrewMember(ctx, in.Body.CrewMemberID, octx); err != nil {
		return nil, err
	}
	assignment, err := h.q.CreateTripCrewAssignment(ctx, sqlcdb.CreateTripCrewAssignmentParams{
		TripID:       types.NullInt64{Int64: in.TripID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		slog.Error("assign org trip crew", "trip_id", in.TripID, "org_id", octx.OrgID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *OrgCrewAssignmentHandler) removeTripCrew(ctx context.Context, in *removeOrgTripCrewInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgTripCrewAssignment(ctx, sqlcdb.DeleteOrgTripCrewAssignmentParams{
		ID:     in.AssignmentID,
		TripID: types.NullInt64{Int64: in.TripID, Valid: true},
		OrgID:  orgID(octx),
	}); err != nil {
		slog.Error("remove org trip crew", "assignment_id", in.AssignmentID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}

func (h *OrgCrewAssignmentHandler) listVoyageCrew(ctx context.Context, in *orgVoyageCrewListInput) (*crewAssignmentListOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, false)
	if err != nil {
		return nil, err
	}
	assignments, err := h.q.ListOrgVoyageCrewAssignments(ctx, sqlcdb.ListOrgVoyageCrewAssignmentsParams{
		VoyageID: types.NullInt64{Int64: in.VoyageID, Valid: true},
		OrgID:    orgID(octx),
	})
	if err != nil {
		slog.Error("list org voyage crew", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage crew")
	}
	return &crewAssignmentListOutput{Body: dto.OrgVoyageCrewFromDB(assignments)}, nil
}

func (h *OrgCrewAssignmentHandler) assignVoyageCrew(ctx context.Context, in *assignOrgVoyageCrewInput) (*crewAssignmentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if _, err := h.q.GetOrgVoyage(ctx, sqlcdb.GetOrgVoyageParams{ID: in.VoyageID, OrgID: orgID(octx)}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify org voyage for crew assignment", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	if err := h.verifyOrgCrewMember(ctx, in.Body.CrewMemberID, octx); err != nil {
		return nil, err
	}
	assignment, err := h.q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID:     types.NullInt64{Int64: in.VoyageID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		slog.Error("assign org voyage crew", "voyage_id", in.VoyageID, "org_id", octx.OrgID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *OrgCrewAssignmentHandler) removeVoyageCrew(ctx context.Context, in *removeOrgVoyageCrewInput) (*noContentOutput, error) {
	octx, err := resolveOrg(ctx, h.q, in.Slug, true)
	if err != nil {
		return nil, err
	}
	if err := h.q.DeleteOrgVoyageCrewAssignment(ctx, sqlcdb.DeleteOrgVoyageCrewAssignmentParams{
		ID:       in.AssignmentID,
		VoyageID: types.NullInt64{Int64: in.VoyageID, Valid: true},
		OrgID:    orgID(octx),
	}); err != nil {
		slog.Error("remove org voyage crew", "assignment_id", in.AssignmentID, "org_id", octx.OrgID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}
