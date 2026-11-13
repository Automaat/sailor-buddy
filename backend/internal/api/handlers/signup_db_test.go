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

// TestSignUp_CruiseEnrollmentAssignedToLegAgainstDatabase checks that a
// cruise enrollment made before per-leg sign-up, once assigned to a leg, is
// the user's sign-up for it: shown on both reads, and a new sign-up returns
// it instead of adding a trip enrollment.
func TestSignUp_CruiseEnrollmentAssignedToLegAgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	leg := seedLeg(t, conn)
	if _, err := conn.ExecContext(ctx, "DELETE FROM trip_enrollments; UPDATE trips SET enroll_token = 'leg-tok'"); err != nil {
		t.Fatalf("reset leg: %v", err)
	}
	if _, err := conn.ExecContext(ctx,
		"INSERT INTO cruise_enrollments (cruise_id, user_id, trip_id, status) VALUES ($1, $2, $3, 'accepted')",
		leg.cruiseID, leg.userID, leg.tripID); err != nil {
		t.Fatalf("insert cruise enrollment: %v", err)
	}
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})
	caller := guestCtx(leg.userID)
	path := "/trips/" + strconv.FormatInt(leg.tripID, 10) + "/sign-up"

	var one dto.LegSignUp
	if err := json.Unmarshal(api.GetCtx(caller, path).Body.Bytes(), &one); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if one.Enrollment == nil || one.Enrollment.CruiseID == nil || one.Enrollment.Status != "accepted" {
		t.Fatalf("get = %+v, want the accepted cruise enrollment", one)
	}
	var legs []dto.LegSignUp
	if err := json.Unmarshal(api.GetCtx(caller, "/cruises/"+strconv.FormatInt(leg.cruiseID, 10)+"/leg-sign-ups").Body.Bytes(), &legs); err != nil {
		t.Fatalf("decode legs: %v", err)
	}
	if len(legs) != 1 || legs[0].Enrollment == nil || legs[0].Enrollment.CruiseID == nil {
		t.Fatalf("legs = %+v, want the cruise enrollment on the leg", legs)
	}
	if resp := api.PostCtx(caller, path); resp.Code != http.StatusOK {
		t.Fatalf("sign up: got %d, want 200; body=%s", resp.Code, resp.Body)
	}
	var n int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM trip_enrollments").Scan(&n); err != nil || n != 0 {
		t.Fatalf("trip enrollments = %d (err %v), want 0", n, err)
	}
	if resp := api.DeleteCtx(caller, path); resp.Code != http.StatusConflict {
		t.Fatalf("withdraw: got %d, want 409; body=%s", resp.Code, resp.Body)
	}
}
