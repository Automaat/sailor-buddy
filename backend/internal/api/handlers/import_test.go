package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func TestImportHandler_Confirm(t *testing.T) {
	t.Run("empty request", func(t *testing.T) {
		m := &mockQuerier{}
		h := NewImportHandler(m)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"cruises":[],"trainings":[]}`))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewImportHandler(&mockQuerier{})
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("creates cruise", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("creates training", func(t *testing.T) {
		m := &mockQuerier{
			createTrainingFn: func(_ context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[],"trainings":[{"name":"RYA"}]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("creates yacht from cruise", func(t *testing.T) {
		yachtName := "SY Odyssey"
		yachtType := "sloop"
		m := &mockQuerier{
			getYachtByNameFn: func(context.Context, sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, sql.ErrNoRows
			},
			createYachtFn: func(_ context.Context, arg sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: 10, Name: arg.Name}, nil
			},
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","yacht_name":"` + yachtName + `","yacht_type":"` + yachtType + `"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("reuses existing yacht", func(t *testing.T) {
		m := &mockQuerier{
			getYachtByNameFn: func(context.Context, sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: 5, Name: "Existing"}, nil
			},
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","yacht_name":"Existing"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("creates captain crew member", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberByNameFn: func(context.Context, sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
			createCrewMemberFn: func(_ context.Context, arg sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: 10, FullName: arg.FullName}, nil
			},
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","captain_name":"John"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("reuses existing captain", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberByNameFn: func(context.Context, sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{ID: 5, FullName: "John"}, nil
			},
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","captain_name":"John"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("yacht create error", func(t *testing.T) {
		m := &mockQuerier{
			getYachtByNameFn: func(context.Context, sqlcdb.GetYachtByNameParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, sql.ErrNoRows
			},
			createYachtFn: func(context.Context, sqlcdb.CreateYachtParams) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{}, errors.New("fail")
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","yacht_name":"New"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("crew create error", func(t *testing.T) {
		m := &mockQuerier{
			getCrewMemberByNameFn: func(context.Context, sqlcdb.GetCrewMemberByNameParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, sql.ErrNoRows
			},
			createCrewMemberFn: func(context.Context, sqlcdb.CreateCrewMemberParams) (sqlcdb.CrewMember, error) {
				return sqlcdb.CrewMember{}, errors.New("fail")
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","captain_name":"John"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("cruise create error", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(context.Context, sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, errors.New("fail")
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip"}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("training create error", func(t *testing.T) {
		m := &mockQuerier{
			createTrainingFn: func(context.Context, sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("fail")
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[],"trainings":[{"name":"RYA"}]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("skips empty cruise name", func(t *testing.T) {
		m := &mockQuerier{}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":""}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("skips empty training name", func(t *testing.T) {
		m := &mockQuerier{}
		h := NewImportHandler(m)
		body := `{"cruises":[],"trainings":[{"name":""}]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})

	t.Run("tidal waters passed through", func(t *testing.T) {
		m := &mockQuerier{
			createCruiseFn: func(_ context.Context, arg sqlcdb.CreateCruiseParams) (sqlcdb.Cruise, error) {
				if !arg.TidalWaters.Valid || arg.TidalWaters.Int64 != 1 {
					t.Errorf("expected tidal_waters=1, got %v", arg.TidalWaters)
				}
				return sqlcdb.Cruise{ID: 1, Name: arg.Name}, nil
			},
		}
		h := NewImportHandler(m)
		body := `{"cruises":[{"name":"Trip","tidal_waters":1}],"trainings":[]}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		h.Confirm(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want %d", w.Code, http.StatusCreated)
		}
	})
}

func TestNewImportHandler(t *testing.T) {
	m := &mockQuerier{}
	h := NewImportHandler(m)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
