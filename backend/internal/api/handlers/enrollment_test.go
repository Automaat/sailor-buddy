package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

var errDBEnroll = errors.New("db error")

func enrollmentRouter(m *mockQuerier) *chi.Mux {
	h := NewEnrollmentHandler(m)
	r := chi.NewRouter()
	r.Get("/enroll/{token}", h.GetCruiseByToken)
	r.Post("/enroll/{token}", h.Enroll)
	r.Post("/cruises/{cruiseID}/enroll-token", h.GenerateToken)
	r.Delete("/cruises/{cruiseID}/enroll-token", h.ClearToken)
	r.Get("/cruises/{cruiseID}/enrollments", h.ListEnrollments)
	r.Put("/cruises/{cruiseID}/enrollments/{id}/status", h.UpdateStatus)
	r.Delete("/cruises/{cruiseID}/enrollments/{id}", h.DeleteEnrollment)
	return r
}

func testEnrollCruise() sqlcdb.GetCruiseByEnrollTokenRow {
	return sqlcdb.GetCruiseByEnrollTokenRow{ID: 1, Name: "Test Cruise"}
}

func TestEnrollment_GetCruiseByToken(t *testing.T) {
	t.Run("success not enrolled", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getUserEnrollmentFn: func(_ context.Context, _ sqlcdb.GetUserEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, sql.ErrNoRows
			},
			countCruiseEnrollmentsFn: func(_ context.Context, _ int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{Accepted: 2, Total: 5}, nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/enroll/abc123", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("success already enrolled", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getUserEnrollmentFn: func(_ context.Context, _ sqlcdb.GetUserEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{ID: 10, CruiseID: 1, UserID: 1, Status: "pending"}, nil
			},
			countCruiseEnrollmentsFn: func(_ context.Context, _ int64) (sqlcdb.CountCruiseEnrollmentsRow, error) {
				return sqlcdb.CountCruiseEnrollmentsRow{Accepted: 1, Total: 1}, nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/enroll/abc123", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("token not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, sql.ErrNoRows
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/enroll/bad", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", w.Code)
		}
	})

	t.Run("db error on token lookup", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/enroll/abc123", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_Enroll(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getCruiseStatusFn: func(_ context.Context, _ int64) (sqlcdb.CruiseStatus, error) {
				return sqlcdb.CruiseStatusPlanned, nil
			},
			createCruiseEnrollmentFn: func(_ context.Context, _ sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{ID: 1, CruiseID: 1, UserID: 1, Status: "pending"}, nil
			},
		}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"note": "Looking forward to it"}`)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201: %s", w.Code, w.Body.String())
		}
	})

	t.Run("rejected on completed cruise", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getCruiseStatusFn: func(_ context.Context, _ int64) (sqlcdb.CruiseStatus, error) {
				return sqlcdb.CruiseStatusCompleted, nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409: %s", w.Code, w.Body.String())
		}
	})

	t.Run("token not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, sql.ErrNoRows
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/bad", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", w.Code)
		}
	})

	t.Run("db error on token lookup", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return sqlcdb.GetCruiseByEnrollTokenRow{}, errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getCruiseStatusFn: func(_ context.Context, _ int64) (sqlcdb.CruiseStatus, error) {
				return sqlcdb.CruiseStatusPlanned, nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", strings.NewReader(`not json`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("already enrolled conflict", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getCruiseStatusFn: func(_ context.Context, _ int64) (sqlcdb.CruiseStatus, error) {
				return sqlcdb.CruiseStatusPlanned, nil
			},
			createCruiseEnrollmentFn: func(_ context.Context, _ sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, &pgconn.PgError{Code: "23505"}
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("got %d, want 409", w.Code)
		}
	})

	t.Run("generic db error on enroll", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseByEnrollTokenFn: func(_ context.Context, _ sql.NullString) (sqlcdb.GetCruiseByEnrollTokenRow, error) {
				return testEnrollCruise(), nil
			},
			getCruiseStatusFn: func(_ context.Context, _ int64) (sqlcdb.CruiseStatus, error) {
				return sqlcdb.CruiseStatusPlanned, nil
			},
			createCruiseEnrollmentFn: func(_ context.Context, _ sqlcdb.CreateCruiseEnrollmentParams) (sqlcdb.CruiseEnrollment, error) {
				return sqlcdb.CruiseEnrollment{}, errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/enroll/abc123", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_GenerateToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, _ sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1}, nil
			},
			setCruiseEnrollTokenFn: func(_ context.Context, _ sqlcdb.SetCruiseEnrollTokenParams) error {
				return nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid cruise id", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/cruises/abc/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("cruise not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, _ sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", w.Code)
		}
	})

	t.Run("db error on cruise lookup", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, _ sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})

	t.Run("db error on set token", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, _ sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1}, nil
			},
			setCruiseEnrollTokenFn: func(_ context.Context, _ sqlcdb.SetCruiseEnrollTokenParams) error {
				return errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPost, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_ClearToken(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			clearCruiseEnrollTokenFn: func(_ context.Context, _ sqlcdb.ClearCruiseEnrollTokenParams) error {
				return nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid cruise id", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/abc/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			clearCruiseEnrollTokenFn: func(_ context.Context, _ sqlcdb.ClearCruiseEnrollTokenParams) error {
				return errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/1/enroll-token", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_ListEnrollments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseEnrollmentsFn: func(_ context.Context, _ sqlcdb.ListCruiseEnrollmentsParams) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
				return []sqlcdb.ListCruiseEnrollmentsRow{
					{ID: 1, CruiseID: 1, UserID: 1, Status: "pending", UserName: "Alice", UserEmail: "alice@example.com"},
				}, nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/cruises/1/enrollments", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid cruise id", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/cruises/abc/enrollments", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseEnrollmentsFn: func(_ context.Context, _ sqlcdb.ListCruiseEnrollmentsParams) ([]sqlcdb.ListCruiseEnrollmentsRow, error) {
				return nil, errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodGet, "/cruises/1/enrollments", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_UpdateStatus(t *testing.T) {
	t.Run("success accepted", func(t *testing.T) {
		m := &mockQuerier{
			updateEnrollmentStatusFn: func(_ context.Context, _ sqlcdb.UpdateEnrollmentStatusParams) error {
				return nil
			},
		}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"status": "accepted"}`)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/5/status", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204: %s", w.Code, w.Body.String())
		}
	})

	t.Run("success rejected", func(t *testing.T) {
		m := &mockQuerier{
			updateEnrollmentStatusFn: func(_ context.Context, _ sqlcdb.UpdateEnrollmentStatusParams) error {
				return nil
			},
		}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"status": "rejected"}`)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/5/status", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204", w.Code)
		}
	})

	t.Run("invalid enrollment id", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"status": "accepted"}`)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/abc/status", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/5/status", strings.NewReader(`not json`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("invalid status value", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"status": "unknown"}`)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/5/status", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateEnrollmentStatusFn: func(_ context.Context, _ sqlcdb.UpdateEnrollmentStatusParams) error {
				return errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		body := strings.NewReader(`{"status": "waitlisted"}`)
		req := httptest.NewRequest(http.MethodPut, "/cruises/1/enrollments/5/status", body)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}

func TestEnrollment_DeleteEnrollment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteCruiseEnrollmentFn: func(_ context.Context, _ sqlcdb.DeleteCruiseEnrollmentParams) error {
				return nil
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/1/enrollments/5", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid enrollment id", func(t *testing.T) {
		m := &mockQuerier{}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/1/enrollments/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteCruiseEnrollmentFn: func(_ context.Context, _ sqlcdb.DeleteCruiseEnrollmentParams) error {
				return errDBEnroll
			},
		}
		r := enrollmentRouter(m)
		req := httptest.NewRequest(http.MethodDelete, "/cruises/1/enrollments/5", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", w.Code)
		}
	})
}
