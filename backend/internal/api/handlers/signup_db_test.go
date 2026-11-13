package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// TestSignUp_AgainstDatabase runs the one-click sign-up operations against
// real tables, covering the SQL the mocks stand in for: the open check, the
// idempotent insert, the note update, the pending-only withdraw and the
// cruise-wide listings.
func TestSignUp_AgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	q := sqlcdb.New(conn)
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: q, runTx: sqlTxRunner(conn)})

	user, err := q.CreateUser(ctx, sqlcdb.CreateUserParams{Email: "guest@example.com", Name: ""})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cruise, err := q.CreateCruise(ctx, sqlcdb.CreateCruiseParams{
		Name: "WSEX", Kind: "sea", Currency: "PLN",
		Included: json.RawMessage("[]"), NotIncluded: json.RawMessage("[]"),
	})
	if err != nil {
		t.Fatalf("create cruise: %v", err)
	}
	trip, err := q.CreateTrip(ctx, sqlcdb.CreateTripParams{
		CruiseID: cruise.ID, Name: "Leg 1", Status: sqlcdb.TripStatusPlanned, Currency: "PLN",
	})
	if err != nil {
		t.Fatalf("create trip: %v", err)
	}
	caller := guestCtx(user.ID)
	path := "/trips/" + strconv.FormatInt(trip.ID, 10) + "/sign-up"

	if resp := api.PostCtx(caller, path); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("closed leg: got %d, want 422; body=%s", resp.Code, resp.Body)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE trips SET enroll_token = 'leg-tok' WHERE id = $1", trip.ID); err != nil {
		t.Fatalf("open leg: %v", err)
	}
	if resp := api.PostCtx(caller, path); resp.Code != http.StatusCreated {
		t.Fatalf("sign up: got %d, want 201; body=%s", resp.Code, resp.Body)
	}
	if resp := api.PostCtx(caller, path); resp.Code != http.StatusOK {
		t.Fatalf("sign up again: got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	if resp := api.PutCtx(caller, path+"/note", map[string]any{"note": "late by a day"}); resp.Code != http.StatusOK {
		t.Fatalf("note: got %d, want 200; body=%s", resp.Code, resp.Body)
	}

	resp := api.GetCtx(caller, "/cruises/"+strconv.FormatInt(cruise.ID, 10)+"/leg-sign-ups")
	var legs []dto.LegSignUp
	if err := json.Unmarshal(resp.Body.Bytes(), &legs); err != nil {
		t.Fatalf("decode legs: %v; body=%s", err, resp.Body)
	}
	if len(legs) != 1 || !legs[0].Open || legs[0].Enrollment == nil ||
		legs[0].Enrollment.Note == nil || *legs[0].Enrollment.Note != "late by a day" {
		t.Fatalf("legs = %+v, want one open leg with the noted pending sign-up", legs)
	}

	resp = api.GetCtx(userCtx(ctx), "/cruises/"+strconv.FormatInt(cruise.ID, 10)+"/trip-enrollments")
	var all []dto.TripEnrollmentDetail
	if err := json.Unmarshal(resp.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode enrollments: %v; body=%s", err, resp.Body)
	}
	if len(all) != 1 || all[0].UserEmail != "guest@example.com" {
		t.Fatalf("enrollments = %+v, want the guest's sign-up", all)
	}

	if _, err := conn.ExecContext(ctx, "UPDATE trip_enrollments SET status = 'accepted' WHERE trip_id = $1", trip.ID); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if resp := api.DeleteCtx(caller, path); resp.Code != http.StatusConflict {
		t.Fatalf("withdraw accepted: got %d, want 409; body=%s", resp.Code, resp.Body)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE trip_enrollments SET status = 'pending' WHERE trip_id = $1", trip.ID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if resp := api.DeleteCtx(caller, path); resp.Code != http.StatusNoContent {
		t.Fatalf("withdraw: got %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if resp := api.DeleteCtx(caller, path); resp.Code != http.StatusNotFound {
		t.Fatalf("withdraw again: got %d, want 404; body=%s", resp.Code, resp.Body)
	}

	if _, err := conn.ExecContext(ctx, "UPDATE trips SET status = 'cancelled' WHERE id = $1", trip.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if resp := api.PostCtx(caller, path); resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cancelled leg: got %d, want 422; body=%s", resp.Code, resp.Body)
	}
	var n int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM trip_enrollments").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows = %d (err %v), want 0", n, err)
	}
}
