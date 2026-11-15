package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// CrewHandler serves the people list. uploads holds the opinion files a
// merge removes.
type CrewHandler struct {
	q       sqlcdb.Querier
	runTx   txRunner
	uploads *os.Root
}

func NewCrewHandler(q sqlcdb.Querier, db *sql.DB, uploads *os.Root) *CrewHandler {
	return &CrewHandler{q: q, runTx: sqlTxRunner(db), uploads: uploads}
}

// listCrewInput repeats the pageParams fields: huma does not apply the
// defaults of an embedded struct.
type listCrewInput struct {
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32  `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
	After  string `query:"after" doc:"A previous page's next_cursor: the window starts after that page's last item, so rows deleted meanwhile shift nothing"`
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

type updateTripCrewRoleInput struct {
	TripID       int64 `path:"tripID" doc:"Trip ID"`
	AssignmentID int64 `path:"assignmentID" doc:"Assignment ID"`
	Body         dto.CrewRoleBody
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
func RegisterCrewRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB, uploads *os.Root) {
	registerCrewRoutes(api, NewCrewHandler(q, db, uploads))
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
		Description: "Shows the kept person as it would be after the merge and how many crew assignments, " +
			"voyage opinions and leg sign-ups would move. Changes nothing. Fails like the merge itself.",
	}, h.previewMerge)
	huma.Register(api, huma.Operation{
		OperationID: "merge-crew-member", Method: http.MethodPost, Path: "/crew/{crewID}/merge",
		Summary: "Merge a duplicate person into this one (admin)", Tags: crew,
		Description: "Moves the duplicate's crew assignments, voyage opinions and leg sign-ups to this " +
			"person, adds its names to the other names, fills empty contact fields, moves its account link " +
			"and deletes it, all in one transaction. Where both are on the same leg or voyage, or both have " +
			"an opinion for the same voyage, this person's stays. Where both signed up for the same leg, the " +
			"sign-up with the stronger status stays (accepted, pending, waitlisted, rejected; this person's " +
			"on a tie) with the more advanced payment status of the two. 409 when both are linked to " +
			"different accounts; 422 when both IDs are the same or the merged other names exceed the limit.",
	}, h.merge)

	huma.Register(api, huma.Operation{
		OperationID: "list-trip-crew", Method: http.MethodGet, Path: "/trips/{tripID}/crew",
		Summary: "List a trip's crew", Tags: crew,
		Description: "Any member who can see the leg reads names, roles and e-mails. Phone and emergency " +
			"contact come only to admins and the leg's captain.",
	}, h.listTripCrew)
	huma.Register(api, huma.Operation{
		OperationID: "update-trip-crew-role", Method: http.MethodPatch, Path: "/trips/{tripID}/crew/{assignmentID}",
		Summary: "Change a trip crew member's role (admin)", Tags: crew,
		Description: "Sets only the role of one crew entry of the trip. The crew of a planned leg comes " +
			"from accepted sign-ups, each a zalogant at first. 422 when the trimmed role is empty or " +
			"too long; 404 when the trip has no such entry.",
	}, h.updateTripCrewRole)
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
	var key nameKey
	ok, err := decodeCursor(in.After, &key)
	if err != nil {
		return nil, err
	}
	search := nullString(trimmedPtr(&in.Query))
	members, err := h.q.ListCrewMembers(ctx, sqlcdb.ListCrewMembersParams{
		Search: search, AfterID: afterID(ok, key.ID), AfterName: afterText(ok, key.Name),
		RowLimit: fetchLimit(in.Limit), RowOffset: in.Offset,
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
	page := buildPage(members, total, in.Limit, in.Offset, dto.CrewMembersFromDB,
		func(r sqlcdb.CrewMember) any { return nameKey{Name: r.FullName, ID: r.ID} })
	if err := h.redactEmergency(ctx, page.Items); err != nil {
		return nil, err
	}
	return &crewListOutput{Body: page}, nil
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
	people := []dto.CrewMember{dto.CrewMemberFromDB(member)}
	if err := h.redactEmergency(ctx, people); err != nil {
		return nil, err
	}
	return &crewOutput{Body: people[0]}, nil
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
	// Deleting a voyage's captain nulls voyages.captain_person_id after locking
	// the person and its opinions, the reverse of a captain write's order.
	var n int64
	err := onDeadlockRetry(func() error {
		var delErr error
		n, delErr = h.q.DeleteUnlinkedCrewMember(ctx, in.ID)
		return delErr
	})
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
	trip, err := h.q.GetTrip(ctx, sqlcdb.GetTripParams{ID: in.TripID, IncludeHidden: canSeeHidden(ctx)})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("trip not found")
		}
		slog.Error("get trip for crew", "trip_id", in.TripID, "err", err)
		return nil, huma.Error500InternalServerError("failed to get trip")
	}
	// Admins and this leg's captain read the phone and emergency contact; the
	// query decides the captain part in the same statement as the crew read.
	// Captaining another leg the same people sail grants nothing here.
	params := sqlcdb.ListTripCrewAssignmentsParams{TripID: trip.ID}
	if u := middleware.GetUser(ctx); u != nil {
		params.IsAdmin = u.Role == "admin"
		params.CallerUserID = types.NullInt64{Int64: u.UserID, Valid: true}
	}
	assignments, err := h.q.ListTripCrewAssignments(ctx, params)
	if err != nil {
		slog.Error("list trip crew", "trip_id", in.TripID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list trip crew")
	}
	return &crewAssignmentListOutput{Body: dto.TripCrewFromDB(assignments)}, nil
}

// crewMemberFKey is violated when the person was deleted, or merged away,
// between verifyCrewMember and the crew insert.
const crewMemberFKey = "crew_assignments_crew_member_id_fkey"

// assignCrewError maps a hand-assignment insert failure caused by the
// person to its API error, or returns nil. A merge that moves the
// duplicate's entry onto the same leg or voyage makes the person already
// listed, or keeps deadlocking the insert.
func assignCrewError(err error) error {
	switch {
	case isForeignKeyViolation(err, crewMemberFKey):
		return huma.Error404NotFound("crew member not found")
	case isUniqueViolation(err):
		return huma.Error409Conflict("crew member is already on the crew list")
	case isDeadlock(err):
		return huma.Error409Conflict("the person changed meanwhile: reload and try again")
	}
	return nil
}

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

func (h *CrewHandler) updateTripCrewRole(ctx context.Context, in *updateTripCrewRoleInput) (*crewAssignmentOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	role := strings.TrimSpace(in.Body.Role)
	if role == "" {
		return nil, huma.Error422UnprocessableEntity("role must not be blank")
	}
	if utf8.RuneCountInString(role) > dto.MaxCrewRoleLen {
		return nil, huma.Error422UnprocessableEntity(
			fmt.Sprintf("role must be at most %d characters", dto.MaxCrewRoleLen))
	}
	row, err := h.q.UpdateTripCrewAssignmentRole(ctx, sqlcdb.UpdateTripCrewAssignmentRoleParams{
		Role:   role,
		ID:     in.AssignmentID,
		TripID: in.TripID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("crew assignment not found")
		}
		slog.Error("update trip crew role", "trip_id", in.TripID, "assignment_id", in.AssignmentID, "err", err)
		return nil, huma.Error500InternalServerError("failed to update crew role")
	}
	// Only admins edit roles, so the reply carries the contacts.
	return &crewAssignmentOutput{Body: dto.TripCrewMemberFromDB(sqlcdb.ListTripCrewAssignmentsRow{
		ID: row.ID, TripID: row.TripID, VoyageID: row.VoyageID, CrewMemberID: row.CrewMemberID,
		Role: row.Role, PatentNumber: row.PatentNumber, CreatedAt: row.CreatedAt,
		FullName: row.FullName, Email: row.Email, Phone: row.Phone,
		EmergencyContactName: row.EmergencyContactName, EmergencyContactPhone: row.EmergencyContactPhone,
		ContactsVisible: true,
	})}, nil
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
	if err := requireVisibleVoyage(ctx, h.q, in.VoyageID); err != nil {
		return nil, err
	}
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
	if _, err := h.q.GetVoyage(ctx, sqlcdb.GetVoyageParams{ID: in.VoyageID, IncludeHidden: true}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("voyage not found")
		}
		slog.Error("verify voyage for crew assignment", "voyage_id", in.VoyageID, "err", err)
		return nil, huma.Error500InternalServerError("failed to verify voyage")
	}
	if err := h.verifyCrewMember(ctx, in.Body.CrewMemberID); err != nil {
		return nil, err
	}
	var assignment sqlcdb.CrewAssignment
	err := onDeadlockRetry(func() error {
		var err error
		assignment, err = h.q.CreateVoyageCrewAssignment(ctx, sqlcdb.CreateVoyageCrewAssignmentParams{
			VoyageID:     types.NullInt64{Int64: in.VoyageID, Valid: true},
			CrewMemberID: in.Body.CrewMemberID,
			Role:         in.Body.Role,
			PatentNumber: nullString(in.Body.PatentNumber),
		})
		return err
	})
	if err != nil {
		if resp := assignCrewError(err); resp != nil {
			return nil, resp
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

func (h *CrewHandler) redactEmergency(ctx context.Context, people []dto.CrewMember) error {
	user := middleware.GetUser(ctx)
	if user == nil {
		return huma.Error401Unauthorized("not authenticated")
	}
	if user.Role == "admin" || len(people) == 0 {
		return nil
	}
	ids, err := h.q.ListCaptainCrewPersonIDs(ctx, types.NullInt64{Int64: user.UserID, Valid: true})
	if err != nil {
		slog.Error("load captain crew access", "user_id", user.UserID, "err", err)
		return huma.Error500InternalServerError("failed to check emergency contact access")
	}
	allowed := make(map[int64]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	for i := range people {
		if !allowed[people[i].ID] {
			people[i].EmergencyContactName = nil
			people[i].EmergencyContactPhone = nil
		}
	}
	return nil
}
