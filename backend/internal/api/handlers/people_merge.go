package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Errors returned by PlanPersonMerge and MergePeople.
var (
	ErrMergeSamePerson      = errors.New("a person cannot be merged into itself")
	ErrMergePersonNotFound  = errors.New("person not found")
	ErrMergeAccountConflict = errors.New("both people are linked to different accounts")
	ErrMergeOtherNames      = fmt.Errorf("merged other names exceed %d names of at most %d characters",
		dto.MaxOtherNames, dto.MaxOtherNameLen)
)

// PersonMergeResult is a planned or applied merge of Duplicate into Kept.
// Kept holds the person as it is after the merge. RemovedOpinionFiles lists
// opinion documents of deleted duplicate opinions; remove them only after
// the merge's transaction commits.
type PersonMergeResult struct {
	Kept                sqlcdb.CrewMember
	Duplicate           sqlcdb.CrewMember
	Counts              sqlcdb.CountCrewMergeRowsRow
	RemovedOpinionFiles []string
}

// PlanPersonMerge works out a merge of duplicateID into keptID without
// changing anything. It fails with the same errors as MergePeople.
func PlanPersonMerge(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) (PersonMergeResult, error) {
	if keptID == duplicateID {
		return PersonMergeResult{}, ErrMergeSamePerson
	}
	kept, err := getMergePerson(ctx, q, keptID)
	if err != nil {
		return PersonMergeResult{}, err
	}
	duplicate, err := getMergePerson(ctx, q, duplicateID)
	if err != nil {
		return PersonMergeResult{}, err
	}
	return planMerge(ctx, q, kept, duplicate)
}

// MergePeople merges the duplicate person into the kept one using q, which
// must be bound to a transaction the caller commits or rolls back. Crew
// assignments and voyage opinions move to the kept person; where both people
// have one for the same leg, voyage or opinion, the kept person's stays. The
// duplicate's name and other names join the kept person's other names, empty
// contact fields are filled from the duplicate, and the duplicate's account
// link moves over. Two different accounts fail with ErrMergeAccountConflict.
// The duplicate is deleted before the kept person is updated, which frees its
// account link under the unique user_id. users rows are never touched.
func MergePeople(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) (PersonMergeResult, error) {
	if keptID == duplicateID {
		return PersonMergeResult{}, ErrMergeSamePerson
	}
	locked, err := q.LockCrewMembersForMerge(ctx, sqlcdb.LockCrewMembersForMergeParams{
		FirstID: keptID, SecondID: duplicateID,
	})
	if err != nil {
		return PersonMergeResult{}, &QueryError{Op: "LockCrewMembersForMerge", Err: err}
	}
	var kept, duplicate *sqlcdb.CrewMember
	for i := range locked {
		switch locked[i].ID {
		case keptID:
			kept = &locked[i]
		case duplicateID:
			duplicate = &locked[i]
		}
	}
	if kept == nil || duplicate == nil {
		return PersonMergeResult{}, ErrMergePersonNotFound
	}
	plan, err := planMerge(ctx, q, *kept, *duplicate)
	if err != nil {
		return PersonMergeResult{}, err
	}

	if err := q.FillSharedCrewAssignmentPatents(ctx, sqlcdb.FillSharedCrewAssignmentPatentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "FillSharedCrewAssignmentPatents", Err: err}
	}
	if _, err := q.DeleteSharedCrewAssignments(ctx, sqlcdb.DeleteSharedCrewAssignmentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "DeleteSharedCrewAssignments", Err: err}
	}
	if _, err := q.MoveCrewAssignments(ctx, sqlcdb.MoveCrewAssignmentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "MoveCrewAssignments", Err: err}
	}
	files, err := q.DeleteSharedVoyageOpinions(ctx, sqlcdb.DeleteSharedVoyageOpinionsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	})
	if err != nil {
		return PersonMergeResult{}, &QueryError{Op: "DeleteSharedVoyageOpinions", Err: err}
	}
	if _, err := q.MoveVoyageOpinions(ctx, sqlcdb.MoveVoyageOpinionsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "MoveVoyageOpinions", Err: err}
	}
	if _, err := q.DeleteMergedCrewMember(ctx, duplicateID); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "DeleteMergedCrewMember", Err: err}
	}
	k := plan.Kept
	updated, err := q.UpdateMergedCrewMember(ctx, sqlcdb.UpdateMergedCrewMemberParams{
		UserID:                k.UserID,
		Email:                 k.Email,
		PatentNumber:          k.PatentNumber,
		Phone:                 k.Phone,
		PzzLicenseType:        k.PzzLicenseType,
		PzzLicenseNumber:      k.PzzLicenseNumber,
		EmergencyContactName:  k.EmergencyContactName,
		EmergencyContactPhone: k.EmergencyContactPhone,
		OtherNames:            k.OtherNames,
		ID:                    keptID,
	})
	if err != nil {
		return PersonMergeResult{}, &QueryError{Op: "UpdateMergedCrewMember", Err: err}
	}
	plan.Kept = updated
	plan.RemovedOpinionFiles = files
	return plan, nil
}

func getMergePerson(ctx context.Context, q sqlcdb.Querier, id int64) (sqlcdb.CrewMember, error) {
	p, err := q.GetCrewMember(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlcdb.CrewMember{}, ErrMergePersonNotFound
		}
		return sqlcdb.CrewMember{}, &QueryError{Op: "GetCrewMember", Err: err}
	}
	return p, nil
}

// planMerge builds the kept person's merged fields and counts the rows that
// move.
func planMerge(ctx context.Context, q sqlcdb.Querier, kept, duplicate sqlcdb.CrewMember) (PersonMergeResult, error) {
	if kept.UserID.Valid && duplicate.UserID.Valid {
		return PersonMergeResult{}, ErrMergeAccountConflict
	}
	names, err := mergeOtherNames(kept, duplicate)
	if err != nil {
		return PersonMergeResult{}, err
	}
	counts, err := q.CountCrewMergeRows(ctx, sqlcdb.CountCrewMergeRowsParams{
		KeptID: kept.ID, DuplicateID: duplicate.ID,
	})
	if err != nil {
		return PersonMergeResult{}, &QueryError{Op: "CountCrewMergeRows", Err: err}
	}
	merged := kept
	merged.OtherNames = names
	if !merged.UserID.Valid {
		merged.UserID = duplicate.UserID
	}
	merged.Email = orNullString(kept.Email, duplicate.Email)
	merged.PatentNumber = orNullString(kept.PatentNumber, duplicate.PatentNumber)
	merged.Phone = orNullString(kept.Phone, duplicate.Phone)
	merged.PzzLicenseType = orNullString(kept.PzzLicenseType, duplicate.PzzLicenseType)
	merged.PzzLicenseNumber = orNullString(kept.PzzLicenseNumber, duplicate.PzzLicenseNumber)
	merged.EmergencyContactName = orNullString(kept.EmergencyContactName, duplicate.EmergencyContactName)
	merged.EmergencyContactPhone = orNullString(kept.EmergencyContactPhone, duplicate.EmergencyContactPhone)
	return PersonMergeResult{Kept: merged, Duplicate: duplicate, Counts: counts}, nil
}

func orNullString(a, b types.NullString) types.NullString {
	if a.Valid && strings.TrimSpace(a.String) != "" {
		return a
	}
	if b.Valid && strings.TrimSpace(b.String) != "" {
		return b
	}
	return a
}

// mergeOtherNames appends the duplicate's name and other names to the kept
// person's other names, skipping blanks, repeats and the kept person's own
// name (case-insensitive).
func mergeOtherNames(kept, duplicate sqlcdb.CrewMember) (json.RawMessage, error) {
	var keptNames, dupNames []string
	if err := decodeNames(kept.OtherNames, &keptNames); err != nil {
		return nil, err
	}
	if err := decodeNames(duplicate.OtherNames, &dupNames); err != nil {
		return nil, err
	}
	candidates := make([]string, 0, len(keptNames)+len(dupNames)+1)
	candidates = append(candidates, keptNames...)
	candidates = append(candidates, duplicate.FullName)
	candidates = append(candidates, dupNames...)

	seen := map[string]bool{strings.ToLower(strings.TrimSpace(kept.FullName)): true}
	out := make([]string, 0, len(candidates))
	for _, name := range candidates {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		if utf8.RuneCountInString(name) > dto.MaxOtherNameLen {
			return nil, ErrMergeOtherNames
		}
		seen[key] = true
		out = append(out, name)
	}
	if len(out) > dto.MaxOtherNames {
		return nil, ErrMergeOtherNames
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode merged other names: %w", err)
	}
	return b, nil
}

func decodeNames(raw json.RawMessage, out *[]string) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode other names: %w", err)
	}
	return nil
}

// mergeHTTPError maps a merge error to its API status.
func mergeHTTPError(err error, keptID, duplicateID int64) error {
	switch {
	case errors.Is(err, ErrMergeSamePerson), errors.Is(err, ErrMergeOtherNames):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, ErrMergePersonNotFound):
		return huma.Error404NotFound("crew member not found")
	case errors.Is(err, ErrMergeAccountConflict):
		return huma.Error409Conflict("both people are linked to different accounts and cannot be merged")
	}
	slog.Error("merge people", "kept_id", keptID, "duplicate_id", duplicateID, "err", err)
	return huma.Error500InternalServerError("failed to merge people")
}

type mergeCrewInput struct {
	ID   int64 `path:"crewID" doc:"Kept person ID"`
	Body dto.PersonMergeBody
}

type previewCrewMergeInput struct {
	ID          int64 `path:"crewID" doc:"Kept person ID"`
	DuplicateID int64 `query:"duplicate_id" required:"true" minimum:"1" doc:"Person that would be merged in and removed"`
}

type personMergeOutput struct {
	Body dto.PersonMerge
}

func (h *CrewHandler) previewMerge(ctx context.Context, in *previewCrewMergeInput) (*personMergeOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	plan, err := PlanPersonMerge(ctx, h.q, in.ID, in.DuplicateID)
	if err != nil {
		return nil, mergeHTTPError(err, in.ID, in.DuplicateID)
	}
	return &personMergeOutput{Body: dto.PersonMergeFromDB(plan.Kept, plan.Duplicate, plan.Counts)}, nil
}

func (h *CrewHandler) merge(ctx context.Context, in *mergeCrewInput) (*personMergeOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var result PersonMergeResult
	if err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		var err error
		result, err = MergePeople(ctx, q, in.ID, in.Body.DuplicateID)
		return err
	}); err != nil {
		return nil, mergeHTTPError(err, in.ID, in.Body.DuplicateID)
	}
	for _, path := range result.RemovedOpinionFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("remove merged opinion file", "path", path, "err", err)
		}
	}
	return &personMergeOutput{Body: dto.PersonMergeFromDB(result.Kept, result.Duplicate, result.Counts)}, nil
}
