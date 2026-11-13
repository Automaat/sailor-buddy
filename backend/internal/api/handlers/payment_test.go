package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/dto"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestUpdatePaymentStatus(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		body     map[string]any
		update   func(context.Context, sqlcdb.UpdateTripEnrollmentPaymentStatusParams) (sqlcdb.TripEnrollment, error)
		wantCode int
	}{
		{
			name: "admin sets deposit",
			ctx:  userCtx(context.Background()),
			body: map[string]any{"payment_status": "deposit"},
			update: func(_ context.Context, arg sqlcdb.UpdateTripEnrollmentPaymentStatusParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{ID: arg.ID, TripID: arg.TripID, Status: "pending", PaymentStatus: arg.PaymentStatus}, nil
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "member is forbidden",
			ctx:      userCtxRole(context.Background(), "member"),
			body:     map[string]any{"payment_status": "paid"},
			wantCode: http.StatusForbidden,
		},
		{
			name:     "unknown status is rejected",
			ctx:      userCtx(context.Background()),
			body:     map[string]any{"payment_status": "refunded"},
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name: "enrollment of another trip is not found",
			ctx:  userCtx(context.Background()),
			body: map[string]any{"payment_status": "paid"},
			update: func(context.Context, sqlcdb.UpdateTripEnrollmentPaymentStatusParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{}, sql.ErrNoRows
			},
			wantCode: http.StatusNotFound,
		},
		{
			name: "database error",
			ctx:  userCtx(context.Background()),
			body: map[string]any{"payment_status": "paid"},
			update: func(context.Context, sqlcdb.UpdateTripEnrollmentPaymentStatusParams) (sqlcdb.TripEnrollment, error) {
				return sqlcdb.TripEnrollment{}, errors.New("boom")
			},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *sqlcdb.UpdateTripEnrollmentPaymentStatusParams
			m := &mockQuerier{}
			if tt.update != nil {
				m.updateTripEnrollmentPaymentFn = func(ctx context.Context, arg sqlcdb.UpdateTripEnrollmentPaymentStatusParams) (sqlcdb.TripEnrollment, error) {
					got = &arg
					return tt.update(ctx, arg)
				}
			}
			resp := enrollTestAPI(t, m).PutCtx(tt.ctx, "/trips/9/enrollments/3/payment-status", tt.body)
			if resp.Code != tt.wantCode {
				t.Fatalf("got %d, want %d; body=%s", resp.Code, tt.wantCode, resp.Body)
			}
			if tt.update == nil {
				return
			}
			if got.ID != 3 || got.TripID != 9 {
				t.Fatalf("update args = %+v, want enrollment 3 of trip 9", *got)
			}
			if tt.wantCode != http.StatusOK {
				return
			}
			var e dto.Enrollment
			if err := json.Unmarshal(resp.Body.Bytes(), &e); err != nil {
				t.Fatalf("decode: %v; body=%s", err, resp.Body)
			}
			if e.PaymentStatus != "deposit" || e.Status != "pending" {
				t.Fatalf("enrollment = %+v, want pending with deposit", e)
			}
		})
	}
}

// TestPaymentStatus_AgainstDatabase checks the payment status SQL: a new
// sign-up starts unpaid, an admin change persists and leaves the review
// status alone, a review change leaves the payment status alone, a member
// cannot withdraw a sign-up with a recorded payment, and an enrollment id
// under another trip is not updated.
func TestPaymentStatus_AgainstDatabase(t *testing.T) {
	conn := lockTestDB(t)
	ctx := context.Background()
	leg := seedLeg(t, conn)
	if _, err := conn.ExecContext(ctx, "DELETE FROM trip_enrollments; UPDATE trips SET enroll_token = 'leg-tok'"); err != nil {
		t.Fatalf("reset leg: %v", err)
	}
	_, api := humatest.New(t)
	registerEnrollmentRoutes(api, &EnrollmentHandler{q: sqlcdb.New(conn), runTx: sqlTxRunner(conn)})
	trip := "/trips/" + strconv.FormatInt(leg.tripID, 10)

	resp := api.PostCtx(guestCtx(leg.userID), trip+"/sign-up")
	if resp.Code != http.StatusCreated {
		t.Fatalf("sign up: got %d, want 201; body=%s", resp.Code, resp.Body)
	}
	signUp := decodeSignUp(t, resp.Body.Bytes())
	if signUp.Enrollment.PaymentStatus != "unpaid" {
		t.Fatalf("new sign-up payment_status = %q, want unpaid", signUp.Enrollment.PaymentStatus)
	}
	enrollment := trip + "/enrollments/" + strconv.FormatInt(signUp.Enrollment.ID, 10)

	if resp := api.PutCtx(guestCtx(leg.userID), enrollment+"/payment-status",
		map[string]any{"payment_status": "paid"}); resp.Code != http.StatusForbidden {
		t.Fatalf("member update: got %d, want 403; body=%s", resp.Code, resp.Body)
	}
	if resp := api.PutCtx(userCtx(ctx), enrollment+"/payment-status",
		map[string]any{"payment_status": "deposit"}); resp.Code != http.StatusOK {
		t.Fatalf("admin update: got %d, want 200; body=%s", resp.Code, resp.Body)
	}

	listed := func() dto.TripEnrollmentDetail {
		t.Helper()
		var rows []dto.TripEnrollmentDetail
		resp := api.GetCtx(userCtx(ctx), trip+"/enrollments")
		if err := json.Unmarshal(resp.Body.Bytes(), &rows); err != nil || len(rows) != 1 {
			t.Fatalf("list: %v, rows=%d; body=%s", err, len(rows), resp.Body)
		}
		return rows[0]
	}
	if got := listed(); got.PaymentStatus != "deposit" || got.Status != "pending" {
		t.Fatalf("after payment update: status=%q payment=%q, want pending/deposit", got.Status, got.PaymentStatus)
	}

	if resp := api.PutCtx(userCtx(ctx), enrollment+"/status",
		map[string]any{"status": "accepted"}); resp.Code != http.StatusNoContent {
		t.Fatalf("accept: got %d, want 204; body=%s", resp.Code, resp.Body)
	}
	if got := listed(); got.PaymentStatus != "deposit" || got.Status != "accepted" {
		t.Fatalf("after accept: status=%q payment=%q, want accepted/deposit", got.Status, got.PaymentStatus)
	}

	var mine dto.LegSignUp
	if err := json.Unmarshal(api.GetCtx(guestCtx(leg.userID), trip+"/sign-up").Body.Bytes(), &mine); err != nil {
		t.Fatalf("decode sign-up: %v", err)
	}
	if mine.Enrollment == nil || mine.Enrollment.PaymentStatus != "deposit" {
		t.Fatalf("member sign-up = %+v, want deposit", mine.Enrollment)
	}

	if _, err := conn.ExecContext(ctx, "UPDATE trip_enrollments SET status = 'pending' WHERE trip_id = $1", leg.tripID); err != nil {
		t.Fatalf("reset to pending: %v", err)
	}
	if resp := api.DeleteCtx(guestCtx(leg.userID), trip+"/sign-up"); resp.Code != http.StatusConflict {
		t.Fatalf("withdraw with deposit: got %d, want 409; body=%s", resp.Code, resp.Body)
	}
	if got := listed(); got.PaymentStatus != "deposit" {
		t.Fatalf("withdraw changed payment to %q", got.PaymentStatus)
	}

	other := "/trips/" + strconv.FormatInt(leg.tripID+1000, 10) + "/enrollments/" +
		strconv.FormatInt(signUp.Enrollment.ID, 10) + "/payment-status"
	if resp := api.PutCtx(userCtx(ctx), other, map[string]any{"payment_status": "paid"}); resp.Code != http.StatusNotFound {
		t.Fatalf("other trip: got %d, want 404; body=%s", resp.Code, resp.Body)
	}
	if got := listed(); got.PaymentStatus != "deposit" {
		t.Fatalf("other-trip update changed payment to %q", got.PaymentStatus)
	}
}
