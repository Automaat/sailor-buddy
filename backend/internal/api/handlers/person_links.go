package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// Request statuses of person_link_requests.
const (
	linkPending  = "pending"
	linkApproved = "approved"
	linkRejected = "rejected"
)

// PersonLinkHandler serves "this person is me" requests and the admin's
// approval, direct link and unlink of an account and a person. Linking an
// account to an existing person always goes through an admin.
type PersonLinkHandler struct {
	q     sqlcdb.Querier
	runTx txRunner
}

// RegisterPersonLinkRoutes wires the person link operations onto the API.
func RegisterPersonLinkRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB) {
	registerPersonLinkRoutes(api, &PersonLinkHandler{q: q, runTx: sqlTxRunner(db)})
}

type claimCandidatesInput struct {
	Query string `query:"q" maxLength:"200" doc:"Case-insensitive part of a name or other name; blank lists only people with the account's e-mail"`
}

type claimCandidatesOutput struct {
	Body []dto.PersonClaimCandidate
}

type myLinkRequestOutput struct {
	Body dto.MyPersonLinkRequest
}

type createLinkRequestInput struct {
	Body dto.PersonLinkRequestBody
}

type linkRequestOutput struct {
	Body dto.PersonLinkRequest
}

type linkRequestListOutput struct {
	Body []dto.PersonLinkRequest
}

type pendingCountOutput struct {
	Body dto.PendingCount
}

type linkRequestIDParam struct {
	ID int64 `path:"requestID" doc:"Person link request ID"`
}

type linkPersonInput struct {
	ID   int64 `path:"crewID" doc:"Person ID"`
	Body dto.PersonLinkBody
}

func registerPersonLinkRoutes(api huma.API, h *PersonLinkHandler) {
	self := []string{"Auth"}
	admin := []string{"Crew"}

	huma.Register(api, huma.Operation{
		OperationID: "list-person-claim-candidates", Method: http.MethodGet, Path: "/auth/me/person-candidates",
		Summary: "People without an account the current user may claim", Tags: self,
		Description: "At most 20 people, those with the account's e-mail first.",
	}, h.candidates)
	huma.Register(api, huma.Operation{
		OperationID: "get-my-person-link-request", Method: http.MethodGet, Path: "/auth/me/person-link-request",
		Summary: "The current user's latest person link request", Tags: self,
	}, h.mine)
	huma.Register(api, huma.Operation{
		OperationID: "create-person-link-request", Method: http.MethodPost, Path: "/auth/me/person-link-request",
		Summary: "Ask an admin to link the current account to a person", Tags: self, DefaultStatus: http.StatusCreated,
		Description: "404 when the person does not exist; 409 when it already has an account or the user " +
			"already has a pending request.",
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "cancel-person-link-request", Method: http.MethodDelete, Path: "/auth/me/person-link-request",
		Summary: "Cancel the current user's pending person link request", Tags: self, DefaultStatus: http.StatusNoContent,
		Description: "404 when there is no pending request.",
	}, h.cancel)

	huma.Register(api, huma.Operation{
		OperationID: "list-person-link-requests", Method: http.MethodGet, Path: "/person-link-requests",
		Summary: "Pending person link requests (admin)", Tags: admin,
	}, h.listPending)
	huma.Register(api, huma.Operation{
		OperationID: "count-person-link-requests", Method: http.MethodGet, Path: "/person-link-requests/pending-count",
		Summary: "Number of pending person link requests (admin)", Tags: admin,
	}, h.countPending)
	huma.Register(api, huma.Operation{
		OperationID: "approve-person-link-request", Method: http.MethodPost, Path: "/person-link-requests/{requestID}/approve",
		Summary: "Approve a person link request (admin)", Tags: admin,
		Description: "Merges the account's own person into the requested one, which then carries the account. " +
			"409 when the request is no longer pending or the person got an account meanwhile.",
	}, h.approve)
	huma.Register(api, huma.Operation{
		OperationID: "reject-person-link-request", Method: http.MethodPost, Path: "/person-link-requests/{requestID}/reject",
		Summary: "Reject a person link request (admin)", Tags: admin,
		Description: "409 when the request is no longer pending.",
	}, h.reject)

	huma.Register(api, huma.Operation{
		OperationID: "link-crew-member", Method: http.MethodPost, Path: "/crew/{crewID}/link",
		Summary: "Link an account to this person (admin)", Tags: admin,
		Description: "Merges the account's current person into this one. The account's pending request is " +
			"approved when it asked for this person and rejected otherwise. 409 when this person has " +
			"another account.",
	}, h.link)
	huma.Register(api, huma.Operation{
		OperationID: "unlink-crew-member", Method: http.MethodPost, Path: "/crew/{crewID}/unlink",
		Summary: "Unlink this person from its account (admin)", Tags: admin,
		Description: "The person stays without an account and loses the account's e-mail; the account gets " +
			"a new person without history. 409 when the person has no account.",
	}, h.unlink)
}

func (h *PersonLinkHandler) candidates(ctx context.Context, in *claimCandidatesInput) (*claimCandidatesOutput, error) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	rows, err := h.q.ListPersonClaimCandidates(ctx, sqlcdb.ListPersonClaimCandidatesParams{
		UserID: u.UserID, Search: nullString(trimmedPtr(&in.Query)),
	})
	if err != nil {
		slog.Error("list person claim candidates", "user_id", u.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to list people")
	}
	return &claimCandidatesOutput{Body: dto.PersonClaimCandidatesFromDB(rows)}, nil
}

func (h *PersonLinkHandler) mine(ctx context.Context, _ *struct{}) (*myLinkRequestOutput, error) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	row, err := h.q.GetLatestPersonLinkRequest(ctx, u.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return &myLinkRequestOutput{}, nil
	}
	if err != nil {
		slog.Error("get person link request", "user_id", u.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to load your request")
	}
	req := dto.MyPersonLinkRequestFromDB(row)
	return &myLinkRequestOutput{Body: dto.MyPersonLinkRequest{Request: &req}}, nil
}

func (h *PersonLinkHandler) create(ctx context.Context, in *createLinkRequestInput) (*linkRequestOutput, error) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	person, err := h.q.GetCrewMember(ctx, in.Body.PersonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, huma.Error404NotFound("person not found")
	}
	if err != nil {
		slog.Error("get requested person", "person_id", in.Body.PersonID, "err", err)
		return nil, huma.Error500InternalServerError("failed to send the request")
	}
	if person.UserID.Valid {
		return nil, huma.Error409Conflict("person already has an account")
	}
	req, err := h.q.CreatePersonLinkRequest(ctx, sqlcdb.CreatePersonLinkRequestParams{
		UserID: u.UserID, PersonID: person.ID,
	})
	switch {
	case isUniqueViolation(err):
		return nil, huma.Error409Conflict("you already have a pending request; cancel it first")
	case isForeignKeyViolation(err, "person_link_requests_person_id_fkey"):
		return nil, huma.Error404NotFound("person not found")
	case err != nil:
		slog.Error("create person link request", "user_id", u.UserID, "person_id", person.ID, "err", err)
		return nil, huma.Error500InternalServerError("failed to send the request")
	}
	return &linkRequestOutput{Body: dto.PersonLinkRequestFromDB(req, person.FullName)}, nil
}

func (h *PersonLinkHandler) cancel(ctx context.Context, _ *struct{}) (*noContentOutput, error) {
	u := middleware.GetUser(ctx)
	if u == nil {
		return nil, huma.Error401Unauthorized("not authenticated")
	}
	n, err := h.q.CancelPendingPersonLinkRequest(ctx, u.UserID)
	if err != nil {
		slog.Error("cancel person link request", "user_id", u.UserID, "err", err)
		return nil, huma.Error500InternalServerError("failed to cancel the request")
	}
	if n == 0 {
		return nil, huma.Error404NotFound("no pending request")
	}
	return &noContentOutput{}, nil
}

func (h *PersonLinkHandler) listPending(ctx context.Context, _ *struct{}) (*linkRequestListOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	rows, err := h.q.ListPendingPersonLinkRequests(ctx)
	if err != nil {
		slog.Error("list person link requests", "err", err)
		return nil, huma.Error500InternalServerError("failed to list requests")
	}
	return &linkRequestListOutput{Body: dto.PendingPersonLinkRequestsFromDB(rows)}, nil
}

func (h *PersonLinkHandler) countPending(ctx context.Context, _ *struct{}) (*pendingCountOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	n, err := h.q.CountPendingPersonLinkRequests(ctx)
	if err != nil {
		slog.Error("count person link requests", "err", err)
		return nil, huma.Error500InternalServerError("failed to count requests")
	}
	return &pendingCountOutput{Body: dto.PendingCount{Count: n}}, nil
}

// linkConflictError carries a 409 message out of a link transaction.
type linkConflictError string

func (e linkConflictError) Error() string { return string(e) }

func (h *PersonLinkHandler) approve(ctx context.Context, in *linkRequestIDParam) (*linkRequestOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	adminID := middleware.GetUser(ctx).UserID
	var req sqlcdb.PersonLinkRequest
	var result PersonMergeResult
	err := h.retryMerge(ctx, "approve person link request", func(q sqlcdb.Querier) error {
		var err error
		req, err = lockPendingRequest(ctx, q, in.ID)
		if err != nil {
			return err
		}
		result, err = linkAccount(ctx, q, req.UserID, req.PersonID)
		if err != nil {
			return err
		}
		return decideRequest(ctx, q, req.ID, linkApproved, adminID)
	})
	if err != nil {
		return nil, linkHTTPError(err, "approve person link request", in.ID)
	}
	removeOpinionFiles(result.RemovedOpinionFiles)
	req.Status = linkApproved
	return &linkRequestOutput{Body: dto.PersonLinkRequestFromDB(req, result.Kept.FullName)}, nil
}

func (h *PersonLinkHandler) reject(ctx context.Context, in *linkRequestIDParam) (*linkRequestOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	adminID := middleware.GetUser(ctx).UserID
	var req sqlcdb.PersonLinkRequest
	var person sqlcdb.CrewMember
	err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		var err error
		req, err = lockPendingRequest(ctx, q, in.ID)
		if err != nil {
			return err
		}
		if person, err = q.GetCrewMember(ctx, req.PersonID); err != nil {
			return &QueryError{Op: "GetCrewMember", Err: err}
		}
		return decideRequest(ctx, q, req.ID, linkRejected, adminID)
	})
	if err != nil {
		return nil, linkHTTPError(err, "reject person link request", in.ID)
	}
	req.Status = linkRejected
	return &linkRequestOutput{Body: dto.PersonLinkRequestFromDB(req, person.FullName)}, nil
}

func (h *PersonLinkHandler) link(ctx context.Context, in *linkPersonInput) (*crewOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	adminID := middleware.GetUser(ctx).UserID
	var result PersonMergeResult
	err := h.retryMerge(ctx, "link person", func(q sqlcdb.Querier) error {
		if _, err := q.GetUserByID(ctx, in.Body.UserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return linkNotFoundError("account not found")
			}
			return &QueryError{Op: "GetUserByID", Err: err}
		}
		var err error
		result, err = linkAccount(ctx, q, in.Body.UserID, in.ID)
		if err != nil {
			return err
		}
		if err := q.CloseUserPendingLinkRequests(ctx, sqlcdb.CloseUserPendingLinkRequestsParams{
			PersonID: in.ID, DecidedBy: types.NullInt64{Int64: adminID, Valid: true}, UserID: in.Body.UserID,
		}); err != nil {
			return &QueryError{Op: "CloseUserPendingLinkRequests", Err: err}
		}
		return nil
	})
	if err != nil {
		return nil, linkHTTPError(err, "link person", in.ID)
	}
	removeOpinionFiles(result.RemovedOpinionFiles)
	return &crewOutput{Body: dto.CrewMemberFromDB(result.Kept)}, nil
}

func (h *PersonLinkHandler) unlink(ctx context.Context, in *crewIDParam) (*crewOutput, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var person sqlcdb.CrewMember
	err := h.runTx(ctx, func(q sqlcdb.Querier) error {
		p, err := q.GetCrewMember(ctx, in.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return linkNotFoundError("person not found")
			}
			return &QueryError{Op: "GetCrewMember", Err: err}
		}
		if !p.UserID.Valid {
			return linkConflictError("person has no account")
		}
		if _, err := q.LockUserForUpdate(ctx, p.UserID.Int64); err != nil {
			return &QueryError{Op: "LockUserForUpdate", Err: err}
		}
		user, err := q.GetUserByID(ctx, p.UserID.Int64)
		if err != nil {
			return &QueryError{Op: "GetUserByID", Err: err}
		}
		n, err := q.UnlinkCrewMember(ctx, sqlcdb.UnlinkCrewMemberParams{
			AccountEmail: user.Email, ID: p.ID, UserID: p.UserID,
		})
		if err != nil {
			return &QueryError{Op: "UnlinkCrewMember", Err: err}
		}
		if n == 0 {
			return linkConflictError("the person's account changed meanwhile; try again")
		}
		if err := q.EnsureUserPerson(ctx, user.ID); err != nil {
			return &QueryError{Op: "EnsureUserPerson", Err: err}
		}
		if person, err = q.GetCrewMember(ctx, p.ID); err != nil {
			return &QueryError{Op: "GetCrewMember", Err: err}
		}
		return nil
	})
	if err != nil {
		return nil, linkHTTPError(err, "unlink person", in.ID)
	}
	return &crewOutput{Body: dto.CrewMemberFromDB(person)}, nil
}

// linkNotFoundError carries a 404 message out of a link transaction.
type linkNotFoundError string

func (e linkNotFoundError) Error() string { return string(e) }

// lockPendingRequest locks the request and requires it to be pending.
func lockPendingRequest(ctx context.Context, q sqlcdb.Querier, id int64) (sqlcdb.PersonLinkRequest, error) {
	req, err := q.LockPersonLinkRequest(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return req, linkNotFoundError("request not found")
		}
		return req, &QueryError{Op: "LockPersonLinkRequest", Err: err}
	}
	if req.Status != linkPending {
		return req, linkConflictError("request is already " + req.Status)
	}
	return req, nil
}

func decideRequest(ctx context.Context, q sqlcdb.Querier, id int64, status string, adminID int64) error {
	if err := q.DecidePersonLinkRequest(ctx, sqlcdb.DecidePersonLinkRequestParams{
		Status: status, DecidedBy: types.NullInt64{Int64: adminID, Valid: true}, ID: id,
	}); err != nil {
		return &QueryError{Op: "DecidePersonLinkRequest", Err: err}
	}
	return nil
}

// linkAccount links the account to the person by merging the account's own
// person into it. The person must not have an account; ErrMergeChanged and
// ErrMergeAccountConflict report one that got an account meanwhile.
func linkAccount(ctx context.Context, q sqlcdb.Querier, userID, personID int64) (PersonMergeResult, error) {
	target, err := q.GetCrewMember(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PersonMergeResult{}, linkNotFoundError("person not found")
		}
		return PersonMergeResult{}, &QueryError{Op: "GetCrewMember", Err: err}
	}
	if target.UserID.Valid {
		if target.UserID.Int64 == userID {
			return PersonMergeResult{}, linkConflictError("person is already linked to this account")
		}
		return PersonMergeResult{}, linkConflictError("person already has an account")
	}
	if err := q.EnsureUserPerson(ctx, userID); err != nil {
		return PersonMergeResult{}, &QueryError{Op: "EnsureUserPerson", Err: err}
	}
	own, err := q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: userID, Valid: true})
	if err != nil {
		return PersonMergeResult{}, &QueryError{Op: "GetCrewMemberByUserID", Err: err}
	}
	return MergePeople(ctx, q, personID, own.ID)
}

// retryMerge runs fn in a transaction, rerunning it when it is chosen as a
// deadlock victim, as the merge handler does.
func (h *PersonLinkHandler) retryMerge(ctx context.Context, op string, fn func(sqlcdb.Querier) error) error {
	var err error
	for attempt := 1; ; attempt++ {
		err = h.runTx(ctx, fn)
		if !isDeadlock(err) || attempt == mergeAttempts {
			return err
		}
		slog.Warn(op+" deadlocked; retrying", "attempt", attempt)
	}
}

func removeOpinionFiles(paths []string) {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("remove merged opinion file", "path", path, "err", err)
		}
	}
}

// linkHTTPError maps a link transaction error to its API status.
func linkHTTPError(err error, op string, id int64) error {
	var conflict linkConflictError
	var notFound linkNotFoundError
	switch {
	case errors.As(err, &conflict):
		return huma.Error409Conflict(string(conflict))
	case errors.As(err, &notFound):
		return huma.Error404NotFound(string(notFound))
	case errors.Is(err, ErrMergeAccountConflict), errors.Is(err, ErrMergeChanged):
		return huma.Error409Conflict("an account link changed meanwhile; reload and try again")
	case errors.Is(err, ErrMergeOtherNames):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, ErrMergePersonNotFound):
		return huma.Error404NotFound("person not found")
	}
	slog.Error(op, "id", id, "err", err)
	return huma.Error500InternalServerError("failed to " + op)
}
