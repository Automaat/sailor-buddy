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
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestCrewHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCrewMembersFn: func(context.Context, int64) ([]sqlcdb.CrewMember, error) {
				return []sqlcdb.CrewMember{{ID: 1, FullName: "John"}}, nil
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listCrewMembersFn: func(context.Context, int64) ([]sqlcdb.CrewMember, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_Get(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(_ context.Context, arg sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: arg.ID, FullName: "John"}, nil
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("not found", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(context.Context, sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberFn: func(context.Context, sqlcdb.GetCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Get(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_Create(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createCrewMemberFn: func(_ context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: 1, FullName: arg.FullName}, nil
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"full_name":"John Doe"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("missing full_name", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			createCrewMemberFn: func(context.Context, sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"full_name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Create(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_Update(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			updateCrewMemberFn: func(context.Context, sqlcdb.UpdateCrewMemberParams) error { return nil },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"full_name":"Updated"}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/abc", strings.NewReader(`{"full_name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing full_name", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			updateCrewMemberFn: func(context.Context, sqlcdb.UpdateCrewMemberParams) error { return errors.New("fail") },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodPut, "/1", strings.NewReader(`{"full_name":"X"}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_Delete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteCrewMemberFn: func(context.Context, sqlcdb.DeleteCrewMemberParams) error { return nil },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteCrewMemberFn: func(context.Context, sqlcdb.DeleteCrewMemberParams) error { return errors.New("fail") },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.Delete(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_AssignCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1}, nil
			},
			createCrewAssignmentFn: func(_ context.Context, arg sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{ID: 1, CruiseID: arg.CruiseID, Role: arg.Role}, nil
			},
		}
		h := NewCrewHandler(m)
		body := `{"crew_member_id":1,"role":"skipper"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid cruise id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("cruise not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "99")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("missing role", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1}, nil
			},
		}
		h := NewCrewHandler(m)
		body := `{"crew_member_id":1}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error on assign", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1}, nil
			},
			createCrewAssignmentFn: func(context.Context, sqlcdb.CreateCrewAssignmentParams) (sqlcdb.CrewAssignment, error) {
				return sqlcdb.CrewAssignment{}, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		body := `{"crew_member_id":1,"role":"skipper"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("cruise db error", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		body := `{"crew_member_id":1,"role":"skipper"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.AssignCrew(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_ListCruiseCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseCrewFn: func(context.Context, sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error) {
				return []sqlcdb.ListCruiseCrewAssignmentsRow{{ID: 1, Role: "skipper"}}, nil
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.ListCruiseCrew(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid cruise id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.ListCruiseCrew(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			listCruiseCrewFn: func(context.Context, sqlcdb.ListCruiseCrewAssignmentsParams) ([]sqlcdb.ListCruiseCrewAssignmentsRow, error) {
				return nil, errors.New("fail")
			},
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("cruiseID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.ListCruiseCrew(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestCrewHandler_RemoveCruiseCrew(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			deleteCrewAssignmentFn: func(context.Context, sqlcdb.DeleteCrewAssignmentParams) error { return nil },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("assignmentID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveCruiseCrew(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("invalid assignment id", func(t *testing.T) {
		h := NewCrewHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("assignmentID", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveCruiseCrew(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("db error", func(t *testing.T) {
		m := &mockQuerier{
			deleteCrewAssignmentFn: func(context.Context, sqlcdb.DeleteCrewAssignmentParams) error { return errors.New("fail") },
		}
		h := NewCrewHandler(m)
		req := httptest.NewRequest(http.MethodDelete, "/1", nil)
		req = req.WithContext(userCtx(req.Context()))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("assignmentID", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.RemoveCruiseCrew(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}
