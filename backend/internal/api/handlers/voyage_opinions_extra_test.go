package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
	"github.com/marcinskalski/sailor-buddy/backend/internal/docgen"
	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

func TestVoyageOpinion_List_DBError(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
		listVoyageVoyageOpinionsFn: func(context.Context, int64) ([]sqlcdb.ListVoyageVoyageOpinionsRow, error) {
			return nil, errors.New("fail")
		},
	}
	resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestVoyageOpinion_Generate(t *testing.T) {
	assignmentRow := sqlcdb.GetVoyageCrewAssignmentByMemberRow{
		ID: 1, CrewMemberID: 4, Role: "skipper", FullName: "Jan Kowalski",
		PatentNumber: types.NullString{String: "KJ-9", Valid: true},
	}

	t.Run("docx success", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: id, Name: "Baltic", YachtID: types.NullInt64{Int64: 2, Valid: true}}, nil
			},
			getVoyageCrewByMemberFn: func(context.Context, sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
				return assignmentRow, nil
			},
			getYachtFn: func(_ context.Context, id int64) (sqlcdb.Yacht, error) {
				return sqlcdb.Yacht{ID: id, Name: "Bavaria"}, nil
			},
			upsertVoyageOpinionFn: func(_ context.Context, arg sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: 7, VoyageID: arg.VoyageID, FileFormat: arg.FileFormat}, nil
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("voyage not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("get voyage db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, errors.New("fail")
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("crew member not assigned", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageCrewByMemberFn: func(context.Context, sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
				return sqlcdb.GetVoyageCrewAssignmentByMemberRow{}, sql.ErrNoRows
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("crew assignment db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageCrewByMemberFn: func(context.Context, sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
				return sqlcdb.GetVoyageCrewAssignmentByMemberRow{}, errors.New("fail")
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("upsert db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageCrewByMemberFn: func(context.Context, sqlcdb.GetVoyageCrewAssignmentByMemberParams) (sqlcdb.GetVoyageCrewAssignmentByMemberRow, error) {
				return assignmentRow, nil
			},
			upsertVoyageOpinionFn: func(context.Context, sqlcdb.UpsertVoyageOpinionParams) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{}, errors.New("fail")
			},
		}
		resp := opinionTestAPI(t, m).PostCtx(userCtx(context.Background()), "/voyages/3/opinions",
			map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("member forbidden", func(t *testing.T) {
		resp := opinionTestAPI(t, &mockQuerier{}).PostCtx(userCtxRole(context.Background(), "member"),
			"/voyages/3/opinions", map[string]any{"crew_member_id": 4, "format": "docx"})
		if resp.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", resp.Code)
		}
	})
}

func TestVoyageOpinion_Download(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "op.docx")
		if err := os.WriteFile(file, []byte("body"), 0o644); err != nil {
			t.Fatal(err)
		}
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: id, VoyageID: 3, FilePath: file, FileFormat: "docx"}, nil
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions/1/download")
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("voyage not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(context.Context, int64) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{}, sql.ErrNoRows
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions/1/download")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("opinion not found", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{}, sql.ErrNoRows
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions/1/download")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", resp.Code)
		}
	})

	t.Run("opinion lookup db error", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageOpinionFn: func(context.Context, int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{}, errors.New("fail")
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions/1/download")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})

	t.Run("file missing on disk", func(t *testing.T) {
		m := &mockQuerier{
			getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
			getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
				return sqlcdb.VoyageOpinion{ID: id, VoyageID: 3, FilePath: "/nonexistent/x.pdf", FileFormat: "pdf"}, nil
			},
		}
		resp := opinionTestAPI(t, m).GetCtx(userCtx(context.Background()), "/voyages/3/opinions/1/download")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500", resp.Code)
		}
	})
}

func TestVoyageOpinion_Delete_DBError(t *testing.T) {
	m := &mockQuerier{
		getVoyageFn: func(_ context.Context, id int64) (sqlcdb.Voyage, error) { return sqlcdb.Voyage{ID: id}, nil },
		getVoyageOpinionFn: func(_ context.Context, id int64) (sqlcdb.VoyageOpinion, error) {
			return sqlcdb.VoyageOpinion{ID: id, VoyageID: 3, FilePath: "/nonexistent/x.pdf"}, nil
		},
		deleteVoyageOpinionFn: func(context.Context, int64) error { return errors.New("fail") },
	}
	resp := opinionTestAPI(t, m).DeleteCtx(userCtx(context.Background()), "/voyages/3/opinions/1")
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", resp.Code)
	}
}

func TestOpinionContentType(t *testing.T) {
	if opinionContentType("docx") != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("docx content type wrong: %q", opinionContentType("docx"))
	}
	if opinionContentType("pdf") != "application/pdf" {
		t.Fatalf("pdf content type wrong: %q", opinionContentType("pdf"))
	}
}

func TestOpinionFormat(t *testing.T) {
	if opinionFormat("") != "pdf" {
		t.Fatalf("empty format must default to pdf, got %q", opinionFormat(""))
	}
	if opinionFormat("docx") != "docx" {
		t.Fatalf("explicit format must pass through, got %q", opinionFormat("docx"))
	}
}

func TestRenderOpinionFile(t *testing.T) {
	t.Run("docx renders bytes", func(t *testing.T) {
		out, err := renderOpinionFile("docx", docgen.OpinionData{CrewMemberName: "Jan"})
		if err != nil {
			t.Fatalf("render docx: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("docx output is empty")
		}
	})

	t.Run("unsupported format errors", func(t *testing.T) {
		if _, err := renderOpinionFile("rtf", docgen.OpinionData{}); err == nil {
			t.Fatal("expected error for unsupported format")
		}
	})
}

func TestNewVoyageOpinionHandler(t *testing.T) {
	if NewVoyageOpinionHandler(&mockQuerier{}, "/tmp") == nil {
		t.Fatal("want non-nil handler")
	}
}
