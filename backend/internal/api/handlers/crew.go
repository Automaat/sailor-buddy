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

type CrewHandler struct {
	q sqlcdb.Querier
}

func NewCrewHandler(q sqlcdb.Querier) *CrewHandler {
	return &CrewHandler{q: q}
}

type crewIDParam struct {
	ID int64 `path:"crewID" doc:"Crew member ID"`
}

type createCrewInput struct {
	Body dto.CrewMemberBody
}

type updateCrewInput struct {
	ID   int64 `path:"crewID" doc:"Crew member ID"`
	Body dto.CrewMemberBody
}

type crewOutput struct {
	Body dto.CrewMember
}

type crewListOutput struct {
	Body []dto.CrewMember
}

type assignTripCrewInput struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
	Body   dto.CrewAssignmentBody
}

type tripCrewListInput struct {
	TripID int64 `path:"tripID" doc:"Trip ID"`
}

type removeTripCrewInput struct {
	TripID       int64 `path:"tripID" doc:"Trip ID"`
	AssignmentID int64 `path:"assignmentID" doc:"Assignment ID"`
}

type assignVoyageCrewInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
	Body     dto.CrewAssignmentBody
}

type voyageCrewListInput struct {
	VoyageID int64 `path:"voyageID" doc:"Voyage ID"`
}

type removeVoyageCrewInput struct {
	VoyageID     int64 `path:"voyageID" doc:"Voyage ID"`
	AssignmentID int64 `path:"assignmentID" doc:"Assignment ID"`
}

type crewAssignmentOutput struct {
	Body dto.CrewAssignment
}

type crewAssignmentListOutput struct {
	Body []dto.CrewAssignment
}

// RegisterCrewRoutes wires the owner-scoped crew member and crew assignment
// operations onto the API.
func RegisterCrewRoutes(api huma.API, q sqlcdb.Querier) {
	h := NewCrewHandler(q)
	crew := []string{"Crew"}

	huma.Register(api, huma.Operation{
		OperationID: "list-crew", Method: http.MethodGet, Path: "/crew",
		Summary: "List crew members", Tags: crew,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-crew-member", Method: http.MethodGet, Path: "/crew/{crewID}",
		Summary: "Get a crew member", Tags: crew,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-crew-member", Method: http.MethodPost, Path: "/crew",
		Summary: "Create a crew member", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-crew-member", Method: http.MethodPut, Path: "/crew/{crewID}",
		Summary: "Update a crew member", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-crew-member", Method: http.MethodDelete, Path: "/crew/{crewID}",
		Summary: "Delete a crew member", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.delete)

	huma.Register(api, huma.Operation{
		OperationID: "list-trip-crew", Method: http.MethodGet, Path: "/trips/{tripID}/crew",
		Summary: "List a trip's crew", Tags: crew,
	}, h.listTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-trip-crew", Method: http.MethodPost, Path: "/trips/{tripID}/crew",
		Summary: "Assign a crew member to a trip", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.assignTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-trip-crew", Method: http.MethodDelete, Path: "/trips/{tripID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from a trip", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.removeTripCrew)

	huma.Register(api, huma.Operation{
		OperationID: "list-voyage-crew", Method: http.MethodGet, Path: "/voyages/{voyageID}/crew",
		Summary: "List a voyage's crew", Tags: crew,
	}, h.listVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-voyage-crew", Method: http.MethodPost, Path: "/voyages/{voyageID}/crew",
		Summary: "Assign a crew member to a voyage", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.assignVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-voyage-crew", Method: http.MethodDelete, Path: "/voyages/{voyageID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from a voyage", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.removeVoyageCrew)
}

func (h *CrewHandler) list(ctx context.Context, _ *struct{}) (*crewListOutput, error) {
	user := middleware.GetUser(ctx)
	members, err := h.q.ListCrewMembers(ctx, user.UserID)
	if err != nil {
		slog.Error("list crew members", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list crew members")
	}
	return &crewListOutput{Body: dto.CrewMembersFromDB(members)}, nil
}

func (h *CrewHandler) get(ctx context.Context, in *crewIDParam) (*crewOutput, error) {
	user := middleware.GetUser(ctx)
	member, err := h.q.GetCrewMember(ctx, sqlcdb.GetCrewMemberParams{ID: in.ID, OwnerID: user.UserID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("crew member not found")
		}
		slog.Error("get crew member", "crew_member_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *CrewHandler) create(ctx context.Context, in *createCrewInput) (*crewOutput, error) {
	user := middleware.GetUser(ctx)
	member, err := h.q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{
		OwnerID:      user.UserID,
		FullName:     in.Body.FullName,
		Email:        nullString(in.Body.Email),
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		slog.Error("create crew member", "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to create crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *CrewHandler) update(ctx context.Context, in *updateCrewInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.UpdateCrewMember(ctx, sqlcdb.UpdateCrewMemberParams{
		FullName:     in.Body.FullName,
		Email:        nullString(in.Body.Email),
		PatentNumber: nullString(in.Body.PatentNumber),
		ID:           in.ID,
		OwnerID:      user.UserID,
	}); err != nil {
		slog.Error("update crew member", "crew_member_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update crew member")
	}
	return &noContentOutput{}, nil
}

func (h *CrewHandler) delete(ctx context.Context, in *crewIDParam) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteCrewMember(ctx, sqlcdb.DeleteCrewMemberParams{ID: in.ID, OwnerID: user.UserID}); err != nil {
		slog.Error("delete crew member", "crew_member_id", in.ID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete crew member")
	}
	return &noContentOutput{}, nil
}

func (h *CrewHandler) listTripCrew(ctx context.Context, in *tripCrewListInput) (*crewAssignmentListOutput, error) {
	user := middleware.GetUser(ctx)
	assignments, err := h.q.ListTripCrewAssignments(ctx, sqlcdb.ListTripCrewAssignmentsParams{
		TripID:  types.NullInt64{Int64: in.TripID, Valid: true},
		OwnerID: user.UserID,
	})
	if err != nil {
		slog.Error("list trip crew", "trip_id", in.TripID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trip crew")
	}
	return &crewAssignmentListOutput{Body: dto.TripCrewFromDB(assignments)}, nil
}

func (h *CrewHandler) assignTripCrew(ctx context.Context, in *assignTripCrewInput) (*crewAssignmentOutput, error) {
	user := middleware.GetUser(ctx)
	if _, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.TripID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("verify trip for crew assignment", "trip_id", in.TripID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify trip")
	}
	assignment, err := h.q.CreateTripCrewAssignment(ctx, sqlcdb.CreateTripCrewAssignmentParams{
		TripID:       types.NullInt64{Int64: in.TripID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		slog.Error("assign trip crew", "trip_id", in.TripID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *CrewHandler) removeTripCrew(ctx context.Context, in *removeTripCrewInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteTripCrewAssignment(ctx, sqlcdb.DeleteTripCrewAssignmentParams{
		ID:      in.AssignmentID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("remove trip crew assignment", "assignment_id", in.AssignmentID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}

func (h *CrewHandler) listVoyageCrew(ctx context.Context, in *voyageCrewListInput) (*crewAssignmentListOutput, error) {
	user := middleware.GetUser(ctx)
	assignments, err := h.q.ListVoyageCrewAssignments(ctx, sqlcdb.ListVoyageCrewAssignmentsParams{
		VoyageID: types.NullInt64{Int64: in.VoyageID, Valid: true},
		OwnerID:  user.UserID,
	})
	if err != nil {
		slog.Error("list voyage crew", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage crew")
	}
	return &crewAssignmentListOutput{Body: dto.VoyageCrewFromDB(assignments)}, nil
}

func (h *CrewHandler) assignVoyageCrew(ctx context.Context, in *assignVoyageCrewInput) (*crewAssignmentOutput, error) {
	user := middleware.GetUser(ctx)
	if _, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.VoyageID, OwnerID: user.UserID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage for crew assignment", "voyage_id", in.VoyageID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	assignment, err := h.q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID:     types.NullInt64{Int64: in.VoyageID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		slog.Error("assign voyage crew", "voyage_id", in.VoyageID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *CrewHandler) removeVoyageCrew(ctx context.Context, in *removeVoyageCrewInput) (*noContentOutput, error) {
	user := middleware.GetUser(ctx)
	if err := h.q.DeleteVoyageCrewAssignment(ctx, sqlcdb.DeleteVoyageCrewAssignmentParams{
		ID:      in.AssignmentID,
		OwnerID: user.UserID,
	}); err != nil {
		slog.Error("remove voyage crew assignment", "assignment_id", in.AssignmentID, "user_id", user.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}
