package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func opinionRouter(m *mockQuerier, uploadDir string) *chi.Mux {
	h := NewVoyageOpinionHandler(m, uploadDir)
	r := chi.NewRouter()
	r.Route("/cruises/{cruiseID}/opinions", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Generate)
		r.Get("/{id}/download", h.Download)
		r.Delete("/{id}", h.Delete)
	})
	return r
}

func testCruiseMock() func(context.Context, sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
	return func(_ context.Context, arg sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
		if arg.ID == 1 && arg.OwnerID == 1 {
			return sqlcdb.Cruise{
				ID:      1,
				OwnerID: 1,
				Name:    "Test Cruise",
				EmbarkDate: sql.NullString{
					String: "2025-07-01",
					Valid:  true,
				},
				DisembarkDate: sql.NullString{
					String: "2025-07-14",
					Valid:  true,
				},
			}, nil
		}
		return sqlcdb.Cruise{}, sql.ErrNoRows
	}
}

func TestVoyageOpinion_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: testCruiseMock(),
			listCruiseVoyageOpinionsFn: func(_ context.Context, cruiseID int64) ([]sqlcdb.ListCruiseVoyageOpinionsRow, error) {
				return []sqlcdb.ListCruiseVoyageOpinionsRow{
					{ID: 1, CruiseID: 1, CrewMemberID: 1, FileFormat: "pdf", FullName: "Jan"},
				}, nil
			},
		}
		r := opinionRouter(m, t.TempDir())
		req := httptest.NewRequest(http.MethodGet, "/cruises/1/opinions", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("cruise not found", func(t *testing.T) {
		m := &mockQuerier{
			getCruiseFn: func(_ context.Context, _ sqlcdb.GetCruiseParams) (sqlcdb.Cruise, error) {
				return sqlcdb.Cruise{}, sql.ErrNoRows
			},
		}
		r := opinionRouter(m, t.TempDir())
		req := httptest.NewRequest(http.MethodGet, "/cruises/999/opinions", nil)
		req = req.WithContext(userCtx(req.Context()))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", w.Code)
		}
	})
}

func TestVoyageOpinion_Generate_InvalidFormat(t *testing.T) {
	m := &mockQuerier{
		getCruiseFn: testCruiseMock(),
	}
	r := opinionRouter(m, t.TempDir())
	body := strings.NewReader(`{"crew_member_id": 1, "format": "txt"}`)
	req := httptest.NewRequest(http.MethodPost, "/cruises/1/opinions", body)
	req = req.WithContext(userCtx(req.Context()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestVoyageOpinion_Generate_MissingCrewMember(t *testing.T) {
	m := &mockQuerier{}
	r := opinionRouter(m, t.TempDir())
	body := strings.NewReader(`{"format": "pdf"}`)
	req := httptest.NewRequest(http.MethodPost, "/cruises/1/opinions", body)
	req = req.WithContext(userCtx(req.Context()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestVoyageOpinion_Generate_DOCX(t *testing.T) {
	dir := t.TempDir()
	m := &mockQuerier{
		getCruiseFn: testCruiseMock(),
		getCrewAssignmentByCruiseAndMemberFn: func(_ context.Context, _ sqlcdb.GetCrewAssignmentByCruiseAndMemberParams) (sqlcdb.GetCrewAssignmentByCruiseAndMemberRow, error) {
			return sqlcdb.GetCrewAssignmentByCruiseAndMemberRow{
				ID: 1, CruiseID: 1, CrewMemberID: 1,
				Role: "Sternik", FullName: "Jan Kowalski",
			}, nil
		},
		getYachtFn: func(_ context.Context, _ sqlcdb.GetYachtParams) (sqlcdb.Yacht, error) {
			return sqlcdb.Yacht{}, sql.ErrNoRows
		},
		upsertVoyageOpinionFn: func(_ context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
			return sqlcdb.VoyageOpinion{
				ID: 1, CruiseID: arg.CruiseID, CrewMemberID: arg.CrewMemberID,
				FilePath: arg.FilePath, FileFormat: arg.FileFormat,
			}, nil
		},
	}
	r := opinionRouter(m, dir)
	body := strings.NewReader(`{"crew_member_id": 1, "format": "docx"}`)
	req := httptest.NewRequest(http.MethodPost, "/cruises/1/opinions", body)
	req = req.WithContext(userCtx(req.Context()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201: %s", w.Code, w.Body.String())
	}
}

func TestVoyageOpinion_Delete(t *testing.T) {
	dir := t.TempDir()
	tmpFile := dir + "/test.pdf"
	_ = os.WriteFile(tmpFile, []byte("fake"), 0o644)

	m := &mockQuerier{
		getCruiseFn: testCruiseMock(),
		getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
			return sqlcdb.VoyageOpinion{ID: id, CruiseID: 1, FilePath: tmpFile, FileFormat: "pdf"}, nil
		},
		deleteVoyageOpinionFn: func(_ context.Context, _ int64) error {
			return nil
		},
	}
	r := opinionRouter(m, dir)
	req := httptest.NewRequest(http.MethodDelete, "/cruises/1/opinions/1", nil)
	req = req.WithContext(userCtx(req.Context()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204: %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatal("expected file to be deleted")
	}
}

func TestVoyageOpinion_Download(t *testing.T) {
	dir := t.TempDir()
	tmpFile := dir + "/test.pdf"
	_ = os.WriteFile(tmpFile, []byte("PDF-content"), 0o644)

	m := &mockQuerier{
		getCruiseFn: testCruiseMock(),
		getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
			return sqlcdb.VoyageOpinion{ID: id, CruiseID: 1, FilePath: tmpFile, FileFormat: "pdf"}, nil
		},
	}
	r := opinionRouter(m, dir)
	req := httptest.NewRequest(http.MethodGet, "/cruises/1/opinions/1/download", nil)
	req = req.WithContext(userCtx(req.Context()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("expected Content-Disposition attachment header")
	}
}
