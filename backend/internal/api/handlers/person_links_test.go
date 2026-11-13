package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestPersonLinks_MembersForbidden(t *testing.T) {
	_, api := humatest.New(t)
	registerPersonLinkRoutes(api, &PersonLinkHandler{q: &mockQuerier{}})
	member := memberCtx(2)
	cases := []struct{ method, path string }{
		{http.MethodGet, "/person-link-requests"},
		{http.MethodGet, "/person-link-requests/pending-count"},
		{http.MethodPost, "/person-link-requests/1/approve"},
		{http.MethodPost, "/person-link-requests/1/reject"},
		{http.MethodPost, "/crew/1/link"},
		{http.MethodPost, "/crew/1/unlink"},
	}
	for _, c := range cases {
		var args []any
		if c.path == "/crew/1/link" {
			args = append(args, map[string]any{"user_id": 3})
		}
		resp := api.DoCtx(member, c.method, c.path, args...)
		if resp.Code != http.StatusForbidden {
			t.Errorf("%s %s: got %d, want 403; body=%s", c.method, c.path, resp.Code, resp.Body)
		}
	}
}

// linkFixture seeds accounts and people on a real database.
type linkFixture struct {
	t    *testing.T
	conn *sql.DB
	q    sqlcdb.Querier
	api  humatest.TestAPI
}

func newLinkFixture(t *testing.T) *linkFixture {
	t.Helper()
	conn := lockTestDB(t)
	q := sqlcdb.New(conn)
	_, api := humatest.New(t)
	registerPersonLinkRoutes(api, &PersonLinkHandler{q: q, runTx: sqlTxRunner(conn)})
	return &linkFixture{t: t, conn: conn, q: q, api: api}
}

func (f *linkFixture) scalar(query string, args ...any) int64 {
	f.t.Helper()
	var id int64
	if err := f.conn.QueryRowContext(context.Background(), query, args...).Scan(&id); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return id
}

// account creates a user with its own person and returns both IDs.
func (f *linkFixture) account(email, name string) (userID, personID int64) {
	f.t.Helper()
	ctx := context.Background()
	u, err := f.q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: email, Name: name})
	if err != nil {
		f.t.Fatalf("create user: %v", err)
	}
	if err := f.q.EnsureUserPerson(ctx, u.ID); err != nil {
		f.t.Fatalf("ensure person: %v", err)
	}
	p, err := f.q.GetCrewMemberByUserID(ctx, types.NullInt64{Int64: u.ID, Valid: true})
	if err != nil {
		f.t.Fatalf("account person: %v", err)
	}
	return u.ID, p.ID
}

func (f *linkFixture) person(name string, email *string) int64 {
	f.t.Helper()
	return f.scalar(`INSERT INTO crew_members (full_name, email) VALUES ($1, $2) RETURNING id`, name, email)
}

// voyageOf puts the person on a voyage of a new cruise and returns the
// cruise.
func (f *linkFixture) voyageOf(personID int64, name string) int64 {
	f.t.Helper()
	cruise := f.scalar(`INSERT INTO cruises (name, kind, currency) VALUES ($1, 'sea', 'PLN') RETURNING id`, name)
	voyage := f.scalar(`INSERT INTO voyages (cruise_id, name) VALUES ($1, $2) RETURNING id`, cruise, name)
	f.scalar(`INSERT INTO crew_assignments (voyage_id, crew_member_id, role) VALUES ($1, $2, 'zalogant') RETURNING id`, voyage, personID)
	return cruise
}

func (f *linkFixture) request(userID, personID int64, want int) dto.PersonLinkRequest {
	f.t.Helper()
	resp := f.api.PostCtx(callerCtx(userID, "member"), "/auth/me/person-link-request", map[string]any{"person_id": personID})
	if resp.Code != want {
		f.t.Fatalf("request person %d for user %d: got %d, want %d; body=%s", personID, userID, resp.Code, want, resp.Body)
	}
	var out dto.PersonLinkRequest
	if want == http.StatusCreated {
		decodeBody(f.t, resp.Body.Bytes(), &out)
	}
	return out
}

func (f *linkFixture) decide(adminID, requestID int64, action string, want int) {
	f.t.Helper()
	resp := f.api.PostCtx(callerCtx(adminID, "admin"), fmt.Sprintf("/person-link-requests/%d/%s", requestID, action))
	if resp.Code != want {
		f.t.Fatalf("%s request %d: got %d, want %d; body=%s", action, requestID, resp.Code, want, resp.Body)
	}
}

func (f *linkFixture) mine(userID int64) *dto.PersonLinkRequest {
	f.t.Helper()
	resp := f.api.GetCtx(callerCtx(userID, "member"), "/auth/me/person-link-request")
	if resp.Code != http.StatusOK {
		f.t.Fatalf("my request: got %d; body=%s", resp.Code, resp.Body)
	}
	var out dto.MyPersonLinkRequest
	decodeBody(f.t, resp.Body.Bytes(), &out)
	return out.Request
}

func (f *linkFixture) userIDOf(personID int64) types.NullInt64 {
	f.t.Helper()
	p, err := f.q.GetCrewMember(context.Background(), personID)
	if err != nil {
		f.t.Fatalf("get person %d: %v", personID, err)
	}
	return p.UserID
}

func (f *linkFixture) cruisesOf(userID int64) []int64 {
	f.t.Helper()
	ids, err := f.q.ListUserCruiseIDs(context.Background(), userID)
	if err != nil {
		f.t.Fatalf("list cruises: %v", err)
	}
	slices.Sort(ids)
	return ids
}

func decodeBody(t *testing.T, body []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, body)
	}
}

func TestPersonLinks_ApproveMergesTheAccountPersonIn(t *testing.T) {
	f := newLinkFixture(t)
	admin, _ := f.account("admin@example.dev", "Admin")
	user, own := f.account("anna@example.dev", "Anna Nowak")
	past := f.person("Anna Kowalska", new("anna.old@example.dev"))
	ownCruise := f.voyageOf(own, "2026")
	pastCruise := f.voyageOf(past, "2019")
	if got := f.cruisesOf(user); !slices.Equal(got, []int64{ownCruise}) {
		t.Fatalf("cruises before approval: got %v, want %v", got, []int64{ownCruise})
	}

	req := f.request(user, past, http.StatusCreated)
	if req.Status != "pending" || req.PersonName != "Anna Kowalska" {
		t.Fatalf("created request: %+v", req)
	}
	if got := f.mine(user); got == nil || got.ID != req.ID || got.Status != "pending" {
		t.Fatalf("my request while pending: %+v", got)
	}

	resp := f.api.GetCtx(callerCtx(admin, "admin"), "/person-link-requests")
	var pending []dto.PersonLinkRequest
	decodeBody(t, resp.Body.Bytes(), &pending)
	if len(pending) != 1 || pending[0].UserEmail != "anna@example.dev" || pending[0].PersonName != "Anna Kowalska" {
		t.Fatalf("pending list: %s", resp.Body)
	}
	resp = f.api.GetCtx(callerCtx(admin, "admin"), "/person-link-requests/pending-count")
	var count dto.PendingCount
	decodeBody(t, resp.Body.Bytes(), &count)
	if count.Count != 1 {
		t.Fatalf("pending count: %s", resp.Body)
	}

	if f.userIDOf(past).Valid {
		t.Fatal("person linked before approval")
	}
	f.decide(admin, req.ID, "approve", http.StatusOK)

	if got := f.userIDOf(past); !got.Valid || got.Int64 != user {
		t.Errorf("requested person account: got %+v, want %d", got, user)
	}
	if n := f.scalar(`SELECT COUNT(*) FROM crew_members WHERE id = $1`, own); n != 0 {
		t.Errorf("account's own person still exists")
	}
	if got := f.cruisesOf(user); !slices.Equal(got, []int64{ownCruise, pastCruise}) {
		t.Errorf("cruises after approval: got %v, want %v", got, []int64{ownCruise, pastCruise})
	}
	if got := f.mine(user); got == nil || got.Status != "approved" {
		t.Errorf("my request after approval: %+v", got)
	}
	f.decide(admin, req.ID, "approve", http.StatusConflict)
	f.decide(admin, req.ID, "reject", http.StatusConflict)
}

func TestPersonLinks_Refusals(t *testing.T) {
	f := newLinkFixture(t)
	admin, _ := f.account("admin@example.dev", "Admin")
	userA, ownA := f.account("a@example.dev", "A")
	userB, _ := f.account("b@example.dev", "B")
	shared := f.person("Jan Kowalski", nil)
	other := f.person("Jan Nowak", nil)

	f.request(userA, ownA, http.StatusConflict)
	f.request(userA, 999999, http.StatusNotFound)

	reqA := f.request(userA, shared, http.StatusCreated)
	f.request(userA, other, http.StatusConflict)
	reqB := f.request(userB, shared, http.StatusCreated)

	f.decide(admin, reqA.ID, "approve", http.StatusOK)
	f.decide(admin, reqB.ID, "approve", http.StatusConflict)
	if got := f.userIDOf(shared); got.Int64 != userA {
		t.Errorf("stale approval moved the person: user %+v", got)
	}
	f.request(userB, shared, http.StatusConflict)
	f.decide(admin, 999999, "approve", http.StatusNotFound)
}

func TestPersonLinks_CancelAndReject(t *testing.T) {
	f := newLinkFixture(t)
	admin, _ := f.account("admin@example.dev", "Admin")
	user, _ := f.account("u@example.dev", "U")
	p := f.person("Ewa Lis", nil)

	if got := f.mine(user); got != nil {
		t.Fatalf("my request before any: %+v", got)
	}
	ctx := callerCtx(user, "member")
	if resp := f.api.DeleteCtx(ctx, "/auth/me/person-link-request"); resp.Code != http.StatusNotFound {
		t.Fatalf("cancel without request: got %d", resp.Code)
	}
	req := f.request(user, p, http.StatusCreated)
	if resp := f.api.DeleteCtx(ctx, "/auth/me/person-link-request"); resp.Code != http.StatusNoContent {
		t.Fatalf("cancel: got %d; body=%s", resp.Code, resp.Body)
	}
	if got := f.mine(user); got == nil || got.Status != "cancelled" {
		t.Fatalf("my request after cancel: %+v", got)
	}
	f.decide(admin, req.ID, "approve", http.StatusConflict)

	req = f.request(user, p, http.StatusCreated)
	f.decide(admin, req.ID, "reject", http.StatusOK)
	if got := f.mine(user); got == nil || got.Status != "rejected" {
		t.Fatalf("my request after reject: %+v", got)
	}
	if f.userIDOf(p).Valid {
		t.Error("rejected request linked the person")
	}
	f.request(user, p, http.StatusCreated)
}

func TestPersonLinks_Candidates(t *testing.T) {
	f := newLinkFixture(t)
	user, _ := f.account("anna@example.dev", "Anna Nowak")
	_, linked := f.account("anna.k@example.dev", "Anna Kowalska")
	byEmail := f.person("Anna Zielińska", new("ANNA@example.dev"))
	byName := f.person("Anna Abramczyk", nil)
	f.person("Piotr Lis", nil)

	list := func(q string) []dto.PersonClaimCandidate {
		t.Helper()
		resp := f.api.GetCtx(callerCtx(user, "member"), "/auth/me/person-candidates?q="+q)
		if resp.Code != http.StatusOK {
			t.Fatalf("candidates %q: got %d; body=%s", q, resp.Code, resp.Body)
		}
		var out []dto.PersonClaimCandidate
		decodeBody(t, resp.Body.Bytes(), &out)
		return out
	}
	ids := func(cs []dto.PersonClaimCandidate) []int64 {
		out := make([]int64, len(cs))
		for i, c := range cs {
			out[i] = c.ID
		}
		return out
	}

	if got := list(""); !slices.Equal(ids(got), []int64{byEmail}) || !got[0].EmailMatch {
		t.Errorf("blank search: got %+v, want only the e-mail match", got)
	}
	got := list("anna")
	if !slices.Equal(ids(got), []int64{byEmail, byName}) {
		t.Errorf("search anna: got %v, want e-mail match first then %d, never the linked %d", ids(got), byName, linked)
	}
}

func TestPersonLinks_AdminLinkAndUnlink(t *testing.T) {
	f := newLinkFixture(t)
	admin, _ := f.account("admin@example.dev", "Admin")
	user, own := f.account("d@example.dev", "Dorota")
	other, _ := f.account("e@example.dev", "Edward")
	target := f.person("Dorota Wiśniewska", new("d@example.dev"))
	elsewhere := f.person("Dorota W.", nil)
	targetCruise := f.voyageOf(target, "2018")
	ownCruise := f.voyageOf(own, "2025")
	req := f.request(user, elsewhere, http.StatusCreated)

	adminCtx := callerCtx(admin, "admin")
	resp := f.api.PostCtx(adminCtx, fmt.Sprintf("/crew/%d/link", target), map[string]any{"user_id": user})
	if resp.Code != http.StatusOK {
		t.Fatalf("link: got %d; body=%s", resp.Code, resp.Body)
	}
	if got := f.userIDOf(target); got.Int64 != user {
		t.Fatalf("linked person account: %+v", got)
	}
	if got := f.cruisesOf(user); !slices.Equal(got, []int64{targetCruise, ownCruise}) {
		t.Errorf("cruises after link: got %v", got)
	}
	if got := f.mine(user); got == nil || got.ID != req.ID || got.Status != "rejected" {
		t.Errorf("request for another person after a direct link: %+v", got)
	}

	resp = f.api.PostCtx(adminCtx, fmt.Sprintf("/crew/%d/link", target), map[string]any{"user_id": other})
	if resp.Code != http.StatusConflict {
		t.Errorf("link a person with another account: got %d; body=%s", resp.Code, resp.Body)
	}
	resp = f.api.PostCtx(adminCtx, fmt.Sprintf("/crew/%d/link", elsewhere), map[string]any{"user_id": 999999})
	if resp.Code != http.StatusNotFound {
		t.Errorf("link a missing account: got %d; body=%s", resp.Code, resp.Body)
	}

	resp = f.api.PostCtx(adminCtx, fmt.Sprintf("/crew/%d/unlink", target))
	if resp.Code != http.StatusOK {
		t.Fatalf("unlink: got %d; body=%s", resp.Code, resp.Body)
	}
	var unlinked dto.CrewMember
	decodeBody(t, resp.Body.Bytes(), &unlinked)
	if unlinked.HasAccount || unlinked.Email != nil {
		t.Errorf("unlinked person: %+v", unlinked)
	}
	fresh, err := f.q.GetCrewMemberByUserID(context.Background(), types.NullInt64{Int64: user, Valid: true})
	if err != nil {
		t.Fatalf("account person after unlink: %v", err)
	}
	if fresh.ID == target || fresh.ID == own {
		t.Errorf("account kept an old person %d", fresh.ID)
	}
	if got := f.cruisesOf(user); len(got) != 0 {
		t.Errorf("cruises after unlink: got %v, want none", got)
	}
	resp = f.api.PostCtx(adminCtx, fmt.Sprintf("/crew/%d/unlink", target))
	if resp.Code != http.StatusConflict {
		t.Errorf("unlink a person without an account: got %d", resp.Code)
	}
}
