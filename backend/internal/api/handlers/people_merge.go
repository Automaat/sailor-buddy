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

// mergeAttempts bounds how often a merge chosen as a deadlock victim reruns.
const mergeAttempts = 3

// Errors returned by PlanPersonMerge and MergePeople.
var (
	ErrMergeSamePerson      = errors.New("a person cannot be merged into itself")
	ErrMergePersonNotFound  = errors.New("person not found")
	ErrMergeAccountConflict = errors.New("both people are linked to different accounts")
	ErrMergeChanged         = errors.New("a person's account link changed during the merge")
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
// must be bound to a transaction the caller commits or rolls back; a caller
// should retry it when the transaction fails with a deadlock (40P01). Crew
// assignments, voyage opinions and leg and voyage captaincies move to the
// kept person; where both people
// have one for the same leg, voyage or opinion, the kept person's stays. The
// duplicate's names join the kept person's other names, empty contact fields
// are filled from the duplicate, and the duplicate's account link moves over
// together with the account's name, since profile saves overwrite it anyway;
// the kept person's own name then joins its other names. Two different
// accounts fail with ErrMergeAccountConflict. The duplicate is deleted
// before the kept person is updated, which frees its account link under the
// unique user_id. users rows are never changed.
func MergePeople(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) (PersonMergeResult, error) {
	kept, duplicate, err := lockMergePeople(ctx, q, keptID, duplicateID)
	if err != nil {
		return PersonMergeResult{}, err
	}
	plan, err := planMerge(ctx, q, kept, duplicate)
	if err != nil {
		return PersonMergeResult{}, err
	}
	files, err := moveMergeRows(ctx, q, keptID, duplicateID)
	if err != nil {
		return PersonMergeResult{}, err
	}
	if _, err := q.DeleteMergedCrewMember(ctx, duplicateID); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "DeleteMergedCrewMember", Err: err}
	}
	k := plan.Kept
	updated, err := q.UpdateMergedCrewMember(ctx, sqlcdb.UpdateMergedCrewMemberParams{
		UserID:                k.UserID,
		FullName:              k.FullName,
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

// lockMergePeople locks, in order, the account whose link moves, both people
// and every leg and voyage either person is on or captains, and returns both
// people as locked. People come before legs, the order leg completion and
// person deletion use. Sign-up acceptance locks the leg, then the account's
// person; when it and a merge deadlock, the losing handler retries.
func lockMergePeople(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) (kept, duplicate sqlcdb.CrewMember, err error) {
	if keptID == duplicateID {
		return kept, duplicate, ErrMergeSamePerson
	}
	seenKept, err := getMergePerson(ctx, q, keptID)
	if err != nil {
		return kept, duplicate, err
	}
	seenDuplicate, err := getMergePerson(ctx, q, duplicateID)
	if err != nil {
		return kept, duplicate, err
	}
	if seenKept.UserID.Valid && seenDuplicate.UserID.Valid {
		return kept, duplicate, ErrMergeAccountConflict
	}
	if seenDuplicate.UserID.Valid {
		if _, err := q.LockUserForUpdate(ctx, seenDuplicate.UserID.Int64); err != nil {
			return kept, duplicate, &QueryError{Op: "LockUserForUpdate", Err: err}
		}
	}
	locked, err := q.LockCrewMembersForMerge(ctx, sqlcdb.LockCrewMembersForMergeParams{
		FirstID: keptID, SecondID: duplicateID,
	})
	if err != nil {
		return kept, duplicate, &QueryError{Op: "LockCrewMembersForMerge", Err: err}
	}
	found := 0
	for i := range locked {
		switch locked[i].ID {
		case keptID:
			kept = locked[i]
			found++
		case duplicateID:
			duplicate = locked[i]
			found++
		}
	}
	if found != 2 {
		return kept, duplicate, ErrMergePersonNotFound
	}
	if kept.UserID != seenKept.UserID || duplicate.UserID != seenDuplicate.UserID {
		return kept, duplicate, ErrMergeChanged
	}
	return kept, duplicate, lockMergeLegs(ctx, q, keptID, duplicateID)
}

func lockMergeLegs(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) error {
	legs := sqlcdb.LockCrewMergeTripsParams{FirstID: keptID, SecondID: duplicateID}
	if err := q.LockCrewMergeTrips(ctx, legs); err != nil {
		return &QueryError{Op: "LockCrewMergeTrips", Err: err}
	}
	if err := q.LockCrewMergeVoyages(ctx, sqlcdb.LockCrewMergeVoyagesParams(legs)); err != nil {
		return &QueryError{Op: "LockCrewMergeVoyages", Err: err}
	}
	return nil
}

// moveMergeRows moves the duplicate's crew assignments, voyage opinions,
// captaincies and person link requests to the kept person, dropping the entries the kept person
// already has. It returns the files of the dropped opinions.
func moveMergeRows(ctx context.Context, q sqlcdb.Querier, keptID, duplicateID int64) ([]string, error) {
	if err := q.MergeSharedCrewAssignments(ctx, sqlcdb.MergeSharedCrewAssignmentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MergeSharedCrewAssignments", Err: err}
	}
	if _, err := q.DeleteSharedCrewAssignments(ctx, sqlcdb.DeleteSharedCrewAssignmentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "DeleteSharedCrewAssignments", Err: err}
	}
	if _, err := q.MoveCrewAssignments(ctx, sqlcdb.MoveCrewAssignmentsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MoveCrewAssignments", Err: err}
	}
	files, err := q.DeleteSharedVoyageOpinions(ctx, sqlcdb.DeleteSharedVoyageOpinionsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	})
	if err != nil {
		return nil, &QueryError{Op: "DeleteSharedVoyageOpinions", Err: err}
	}
	if _, err := q.MoveVoyageOpinions(ctx, sqlcdb.MoveVoyageOpinionsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MoveVoyageOpinions", Err: err}
	}
	if _, err := q.MoveTripCaptain(ctx, sqlcdb.MoveTripCaptainParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MoveTripCaptain", Err: err}
	}
	if _, err := q.MoveVoyageCaptain(ctx, sqlcdb.MoveVoyageCaptainParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MoveVoyageCaptain", Err: err}
	}
	if _, err := q.MovePersonLinkRequests(ctx, sqlcdb.MovePersonLinkRequestsParams{
		KeptID: keptID, DuplicateID: duplicateID,
	}); err != nil {
		return nil, &QueryError{Op: "MovePersonLinkRequests", Err: err}
	}
	return files, nil
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
	fullName := kept.FullName
	if !kept.UserID.Valid && duplicate.UserID.Valid {
		fullName = duplicate.FullName
	}
	names, err := mergeOtherNames(fullName, kept, duplicate)
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
	merged.FullName = fullName
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

// mergeOtherNames collects the kept person's other names, both full names
// and the duplicate's other names, skipping blanks, repeats and the merged
// person's fullName (case-insensitive).
func mergeOtherNames(fullName string, kept, duplicate sqlcdb.CrewMember) (json.RawMessage, error) {
	var keptNames, dupNames []string
	if err := decodeNames(kept.OtherNames, &keptNames); err != nil {
		return nil, err
	}
	if err := decodeNames(duplicate.OtherNames, &dupNames); err != nil {
		return nil, err
	}
	candidates := make([]string, 0, len(keptNames)+len(dupNames)+2)
	candidates = append(candidates, keptNames...)
	candidates = append(candidates, kept.FullName, duplicate.FullName)
	candidates = append(candidates, dupNames...)

	seen := map[string]bool{strings.ToLower(strings.TrimSpace(fullName)): true}
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
	case errors.Is(err, ErrMergeChanged):
		return huma.Error409Conflict("a person's account link changed during the merge; try again")
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
	var err error
	for attempt := 1; ; attempt++ {
		err = h.runTx(ctx, func(q sqlcdb.Querier) error {
			var err error
			result, err = MergePeople(ctx, q, in.ID, in.Body.DuplicateID)
			return err
		})
		if !isDeadlock(err) || attempt == mergeAttempts {
			break
		}
		slog.Warn("merge people deadlocked; retrying", "kept_id", in.ID, "duplicate_id", in.Body.DuplicateID, "attempt", attempt)
	}
	if err != nil {
		return nil, mergeHTTPError(err, in.ID, in.Body.DuplicateID)
	}
	for _, path := range result.RemovedOpinionFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("remove merged opinion file", "path", path, "err", err)
		}
	}
	return &personMergeOutput{Body: dto.PersonMergeFromDB(result.Kept, result.Duplicate, result.Counts)}, nil
}
