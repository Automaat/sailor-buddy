package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

type CrewHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

func NewCrewHandler(q sqlcdb.Querier, db *sql.DB) *CrewHandler {
	return &CrewHandler{q: q, runTx: sqlTxRunner(db)}
}

// listCrewInput repeats the pageParams fields: huma does not apply the
// defaults of an embedded struct.
type listCrewInput struct {
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32  `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
	Query  string `query:"q" maxLength:"200" doc:"Case-insensitive part of a name, other name or e-mail; blank lists everyone"`
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
	Body dto.Page[dto.CrewMember]
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

// RegisterCrewRoutes wires the club crew member and crew assignment operations
// onto the API. Reads are open to any member; mutations require an admin.
func RegisterCrewRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	registerCrewRoutes(api, NewCrewHandler(q, db))
}

func registerCrewRoutes(api huma.API, h *CrewHandler) {
	crew := []string{"Crew"}

	huma.Register(api, huma.Operation{
		OperationID: "list-crew", Method: http.MethodGet, Path: "/crew",
		Summary: "List and search people", Tags: crew,
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID: "get-crew-member", Method: http.MethodGet, Path: "/crew/{crewID}",
		Summary: "Get a crew member", Tags: crew,
	}, h.get)
	huma.Register(api, huma.Operation{
		OperationID: "create-crew-member", Method: http.MethodPost, Path: "/crew",
		Summary: "Create a crew member (admin)", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "update-crew-member", Method: http.MethodPut, Path: "/crew/{crewID}",
		Summary: "Update a crew member (admin)", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID: "delete-crew-member", Method: http.MethodDelete, Path: "/crew/{crewID}",
		Summary: "Delete a person without an account (admin)", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.delete)
	huma.Register(api, huma.Operation{
		OperationID: "preview-crew-merge", Method: http.MethodGet, Path: "/crew/{crewID}/merge-preview",
		Summary: "Preview merging a duplicate person into this one (admin)", Tags: crew,
		Description: "Shows the kept person as it would be after the merge and how many crew assignments " +
			"and voyage opinions would move. Changes nothing. Fails like the merge itself.",
	}, h.previewMerge)
	huma.Register(api, huma.Operation{
		OperationID: "merge-crew-member", Method: http.MethodPost, Path: "/crew/{crewID}/merge",
		Summary: "Merge a duplicate person into this one (admin)", Tags: crew,
		Description: "Moves the duplicate's crew assignments and voyage opinions to this person, adds its " +
			"names to the other names, fills empty contact fields, moves its account link and deletes it, " +
			"all in one transaction. Where both are on the same leg or voyage, or both have an opinion for " +
			"the same voyage, this person's stays. 409 when both are linked to different accounts; " +
			"422 when both IDs are the same or the merged other names exceed the limit.",
	}, h.merge)

	huma.Register(api, huma.Operation{
		OperationID: "list-trip-crew", Method: http.MethodGet, Path: "/trips/{tripID}/crew",
		Summary: "List a trip's crew", Tags: crew,
	}, h.listTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-trip-crew", Method: http.MethodPost, Path: "/trips/{tripID}/crew",
		Summary: "Assign a crew member to a trip (admin)", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.assignTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-trip-crew", Method: http.MethodDelete, Path: "/trips/{tripID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from a trip (admin)", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.removeTripCrew)

	huma.Register(api, huma.Operation{
		OperationID: "list-voyage-crew", Method: http.MethodGet, Path: "/voyages/{voyageID}/crew",
		Summary: "List a voyage's crew", Tags: crew,
	}, h.listVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "assign-voyage-crew", Method: http.MethodPost, Path: "/voyages/{voyageID}/crew",
		Summary: "Assign a crew member to a voyage (admin)", Tags: crew, DefaultStatus: http.StatusCreated,
	}, h.assignVoyageCrew)
	huma.Register(api, huma.Operation{
		OperationID: "remove-voyage-crew", Method: http.MethodDelete, Path: "/voyages/{voyageID}/crew/{assignmentID}",
		Summary: "Remove a crew assignment from a voyage (admin)", Tags: crew, DefaultStatus: http.StatusNoContent,
	}, h.removeVoyageCrew)
}

func (h *CrewHandler) list(ctx context.Context, in *listCrewInput) (*crewListOutput, error) {
	search := nullString(trimmedPtr(&in.Query))
	members, err := h.q.ListCrewMembers(ctx, sqlcdb.ListCrewMembersParams{
		Search: search, RowLimit: in.Limit, RowOffset: in.Offset,
	})
	if err != nil {
		slog.Error("list crew members", "err", err)
		return nil, huma.Error500InternalServerError("failed to list crew members")
	}
	total, err := h.q.CountCrewMembers(ctx, search)
	if err != nil {
		slog.Error("count crew members", "err", err)
		return nil, huma.Error500InternalServerError("failed to list crew members")
	}
	return &crewListOutput{Body: dto.NewPage(dto.CrewMembersFromDB(members), total, in.Limit, in.Offset)}, nil
}

func (h *CrewHandler) get(ctx context.Context, in *crewIDParam) (*crewOutput, error) {
	member, err := h.q.GetCrewMember(ctx, in.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("crew member not found")
		}
		slog.Error("get crew member", "crew_member_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *CrewHandler) create(ctx context.Context, in *createCrewInput) (*crewOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	otherNames, err := encodeOtherNames(in.Body.OtherNames)
	if err != nil {
		return nil, err
	}
	user := middleware.GetUser(ctx)
	member, err := h.q.CreateCrewMember(ctx, sqlcdb.CreateCrewMemberParams{
		CreatedBy:             types.NullInt64{Int64: user.UserID, Valid: true},
		FullName:              in.Body.FullName,
		Email:                 nullString(in.Body.Email),
		PatentNumber:          nullString(in.Body.PatentNumber),
		Phone:                 nullString(in.Body.Phone),
		PzzLicenseType:        nullString(in.Body.PzzLicenseType),
		PzzLicenseNumber:      nullString(in.Body.PzzLicenseNumber),
		EmergencyContactName:  nullString(in.Body.EmergencyContactName),
		EmergencyContactPhone: nullString(in.Body.EmergencyContactPhone),
		OtherNames:            otherNames,
	})
	if err != nil {
		slog.Error("create crew member", "err", err)
		return nil, huma.Error500InternalServerError("failed to create crew member")
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(member)}, nil
}

func (h *CrewHandler) update(ctx context.Context, in *updateCrewInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	otherNames, err := encodeOtherNames(in.Body.OtherNames)
	if err != nil {
		return nil, err
	}
	if err := h.q.UpdateCrewMember(ctx, sqlcdb.UpdateCrewMemberParams{
		FullName:              in.Body.FullName,
		Email:                 nullString(in.Body.Email),
		PatentNumber:          nullString(in.Body.PatentNumber),
		Phone:                 nullString(in.Body.Phone),
		PzzLicenseType:        nullString(in.Body.PzzLicenseType),
		PzzLicenseNumber:      nullString(in.Body.PzzLicenseNumber),
		EmergencyContactName:  nullString(in.Body.EmergencyContactName),
		EmergencyContactPhone: nullString(in.Body.EmergencyContactPhone),
		OtherNames:            otherNames,
		ID:                    in.ID,
	}); err != nil {
		slog.Error("update crew member", "crew_member_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update crew member")
	}
	return &noContentOutput{}, nil
}

func (h *CrewHandler) delete(ctx context.Context, in *crewIDParam) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	n, err := h.q.DeleteUnlinkedCrewMember(ctx, in.ID)
	if err != nil {
		slog.Error("delete crew member", "crew_member_id", in.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to delete crew member")
	}
	if n == 0 {
		if _, err := h.q.GetCrewMember(ctx, in.ID); err == nil {
			return nil, huma.Error409Conflict("person is linked to an account and cannot be deleted")
		} else if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("check crew member after delete", "crew_member_id", in.ID, "err", err)
			return nil, huma.Error500InternalServerError("failed to delete crew member")
		}
	}
	return &noContentOutput{}, nil
}

// encodeOtherNames trims a person's other names, drops blank and repeated
// (case-insensitive) entries and encodes the rest for the JSONB column.
func encodeOtherNames(names []string) (json.RawMessage, error) {
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if utf8.RuneCountInString(name) > dto.MaxOtherNameLen {
			return nil, huma.Error422UnprocessableEntity(
				fmt.Sprintf("other names must be at most %d characters", dto.MaxOtherNameLen))
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to encode other names")
	}
	return b, nil
}

func (h *CrewHandler) listTripCrew(ctx context.Context, in *tripCrewListInput) (*crewAssignmentListOutput, error) {
	assignments, err := h.q.ListTripCrewAssignments(ctx, types.NullInt64{Int64: in.TripID, Valid: true})
	if err != nil {
		slog.Error("list trip crew", "trip_id", in.TripID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trip crew")
	}
	return &crewAssignmentListOutput{Body: dto.TripCrewFromDB(assignments)}, nil
}

// crewMemberFKey is violated when the person was deleted, or merged away,
// between verifyCrewMember and the crew insert.
const crewMemberFKey = "crew_assignments_crew_member_id_fkey"

// verifyCrewMember confirms the crew member exists.
func (h *CrewHandler) verifyCrewMember(ctx context.Context, crewMemberID int64) error {
	if _, err := h.q.GetCrewMember(ctx, crewMemberID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound("crew member not found")
		}
		slog.Error("verify crew member for assignment", "crew_member_id", crewMemberID, "err", err)
		return huma.Error500InternalServerError("failed to verify crew member")
	}
	return nil
}

func (h *CrewHandler) assignTripCrew(ctx context.Context, in *assignTripCrewInput) (*crewAssignmentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := h.q.GetTrip(ctx, in.TripID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("verify trip for crew assignment", "trip_id", in.TripID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify trip")
	}
	if err := h.verifyCrewMember(ctx, in.Body.CrewMemberID); err != nil {
		return nil, err
	}
	assignment, err := h.q.CreateTripCrewAssignment(ctx, sqlcdb.CreateTripCrewAssignmentParams{
		TripID:       types.NullInt64{Int64: in.TripID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		if isForeignKeyViolation(err, "crew_assignments_trip_id_fkey") {
			return nil, huma.Error409Conflict("trip is no longer open: it was completed or deleted")
		}
		if isForeignKeyViolation(err, crewMemberFKey) {
			return nil, huma.Error404NotFound("crew member not found")
		}
		slog.Error("assign trip crew", "trip_id", in.TripID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *CrewHandler) removeTripCrew(ctx context.Context, in *removeTripCrewInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteTripCrewAssignment(ctx, sqlcdb.DeleteTripCrewAssignmentParams{
		ID:     in.AssignmentID,
		TripID: types.NullInt64{Int64: in.TripID, Valid: true},
	}); err != nil {
		slog.Error("remove trip crew assignment", "assignment_id", in.AssignmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}

func (h *CrewHandler) listVoyageCrew(ctx context.Context, in *voyageCrewListInput) (*crewAssignmentListOutput, error) {
	assignments, err := h.q.ListVoyageCrewAssignments(ctx, types.NullInt64{Int64: in.VoyageID, Valid: true})
	if err != nil {
		slog.Error("list voyage crew", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list voyage crew")
	}
	return &crewAssignmentListOutput{Body: dto.VoyageCrewFromDB(assignments)}, nil
}

func (h *CrewHandler) assignVoyageCrew(ctx context.Context, in *assignVoyageCrewInput) (*crewAssignmentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := h.q.GetVoyage(ctx, in.VoyageID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage for crew assignment", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	if err := h.verifyCrewMember(ctx, in.Body.CrewMemberID); err != nil {
		return nil, err
	}
	assignment, err := h.q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
		VoyageID:     types.NullInt64{Int64: in.VoyageID, Valid: true},
		CrewMemberID: in.Body.CrewMemberID,
		Role:         in.Body.Role,
		PatentNumber: nullString(in.Body.PatentNumber),
	})
	if err != nil {
		if isForeignKeyViolation(err, crewMemberFKey) {
			return nil, huma.Error404NotFound("crew member not found")
		}
		slog.Error("assign voyage crew", "voyage_id", in.VoyageID, "crew_member_id", in.Body.CrewMemberID, "err", err)
		return nil, huma.Error500InternalServerError("failed to assign crew member")
	}
	return &crewAssignmentOutput{Body: dto.CrewAssignmentFromDB(assignment)}, nil
}

func (h *CrewHandler) removeVoyageCrew(ctx context.Context, in *removeVoyageCrewInput) (*noContentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.q.DeleteVoyageCrewAssignment(ctx, sqlcdb.DeleteVoyageCrewAssignmentParams{
		ID:       in.AssignmentID,
		VoyageID: types.NullInt64{Int64: in.VoyageID, Valid: true},
	}); err != nil {
		slog.Error("remove voyage crew assignment", "assignment_id", in.AssignmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to remove crew assignment")
	}
	return &noContentOutput{}, nil
}
