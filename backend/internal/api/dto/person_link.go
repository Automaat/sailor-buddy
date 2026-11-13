package dto

import (
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// PersonClaimCandidate is a person without an account that the user may
// claim. Contact details stay hidden; EmailMatch tells that the person's
// e-mail is the account's.
type PersonClaimCandidate struct {
	ID         int64    `json:"id"`
	FullName   string   `json:"full_name"`
	OtherNames []string `json:"other_names"`
	EmailMatch bool     `json:"email_match" doc:"Whether the person's e-mail is the account's"`
}

// PersonLinkRequest is a user's request to be linked to an existing person.
// PersonName is the name the user claimed; the admin list shows the
// person's current name instead. The user and person e-mails are filled
// only on the admin list.
type PersonLinkRequest struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	UserName    string     `json:"user_name,omitempty"`
	UserEmail   string     `json:"user_email,omitempty"`
	PersonID    int64      `json:"person_id"`
	PersonName  string     `json:"person_name"`
	PersonEmail *string    `json:"person_email,omitempty"`
	Status      string     `json:"status" enum:"pending,approved,rejected,cancelled"`
	CreatedAt   time.Time  `json:"created_at"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
}

// MyPersonLinkRequest wraps the user's latest request, absent when there is
// none.
type MyPersonLinkRequest struct {
	Request *PersonLinkRequest `json:"request,omitempty" doc:"The latest request; absent when the user never sent one"`
}

// PersonLinkRequestBody is the payload of a "this person is me" request.
type PersonLinkRequestBody struct {
	PersonID int64 `json:"person_id" minimum:"1" doc:"Person without an account the user claims to be"`
}

// PendingCount is the number of pending person link requests.
type PendingCount struct {
	Count int64 `json:"count"`
}

// PersonLinkBody is the payload of an admin's direct link of an account to
// a person.
type PersonLinkBody struct {
	UserID int64 `json:"user_id" minimum:"1" doc:"Account to link to the person"`
}

// PersonClaimCandidatesFromDB maps candidate rows, returning a non-nil slice.
func PersonClaimCandidatesFromDB(rows []sqlcdb.ListPersonClaimCandidatesRow) []PersonClaimCandidate {
	out := make([]PersonClaimCandidate, len(rows))
	for i, r := range rows {
		out[i] = PersonClaimCandidate{
			ID: r.ID, FullName: r.FullName, OtherNames: stringList(r.OtherNames), EmailMatch: r.EmailMatch,
		}
	}
	return out
}

// PersonLinkRequestFromDB maps a request row.
func PersonLinkRequestFromDB(r sqlcdb.PersonLinkRequest) PersonLinkRequest {
	out := PersonLinkRequest{
		ID: r.ID, UserID: r.UserID, PersonID: r.PersonID, PersonName: r.PersonName,
		Status: r.Status, CreatedAt: r.CreatedAt,
	}
	if r.DecidedAt.Valid {
		t := r.DecidedAt.Time
		out.DecidedAt = &t
	}
	return out
}

// MyPersonLinkRequestFromDB maps the user's latest request row.
func MyPersonLinkRequestFromDB(r sqlcdb.GetLatestPersonLinkRequestRow) PersonLinkRequest {
	return PersonLinkRequestFromDB(sqlcdb.PersonLinkRequest{
		ID: r.ID, UserID: r.UserID, PersonID: r.PersonID, PersonName: r.PersonName, Status: r.Status,
		DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt,
	})
}

// PendingPersonLinkRequestsFromDB maps the admin list rows, returning a
// non-nil slice.
func PendingPersonLinkRequestsFromDB(rows []sqlcdb.ListPendingPersonLinkRequestsRow) []PersonLinkRequest {
	out := make([]PersonLinkRequest, len(rows))
	for i := range rows {
		r := &rows[i]
		req := PersonLinkRequestFromDB(sqlcdb.PersonLinkRequest{
			ID: r.ID, UserID: r.UserID, PersonID: r.PersonID, PersonName: r.CurrentPersonName, Status: r.Status,
			DecidedBy: r.DecidedBy, DecidedAt: r.DecidedAt, CreatedAt: r.CreatedAt,
		})
		req.UserName = r.UserName
		req.UserEmail = r.UserEmail
		req.PersonEmail = strPtr(r.PersonEmail)
		out[i] = req
	}
	return out
}
