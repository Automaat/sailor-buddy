package handlers

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/xuri/excelize/v2"

	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func createTestXLSX(t *testing.T, opinieRows, szkoleniaRows [][]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	idx, err := f.NewSheet("opinie")
	if err != nil {
		t.Fatalf("NewSheet opinie: %v", err)
	}
	f.SetActiveSheet(idx)
	for i, row := range opinieRows {
		for j, val := range row {
			cell, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName: %v", err)
			}
			if err := f.SetCellValue("opinie", cell, val); err != nil {
				t.Fatalf("SetCellValue opinie %s: %v", cell, err)
			}
		}
	}

	_, err = f.NewSheet("szkolenia")
	if err != nil {
		t.Fatalf("NewSheet szkolenia: %v", err)
	}
	for i, row := range szkoleniaRows {
		for j, val := range row {
			cell, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				t.Fatalf("CoordinatesToCellName: %v", err)
			}
			if err := f.SetCellValue("szkolenia", cell, val); err != nil {
				t.Fatalf("SetCellValue szkolenia %s: %v", cell, err)
			}
		}
	}

	_ = f.DeleteSheet("Sheet1")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("Write xlsx: %v", err)
	}
	return &buf
}

func TestParseOpinionSheet(t *testing.T) {
	t.Run("parses rows", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		idx, _ := f.NewSheet("opinie")
		f.SetActiveSheet(idx)

		header := []string{
			"name", "year", "embark", "disembark", "countries",
			"start", "end", "hours_total", "hours_sail", "hours_engine",
			"hours_6bf", "miles", "days", "captain", "yacht",
			"type", "tidal", "cost_total", "cost_pp", "desc",
		}
		for j, v := range header {
			cell, _ := excelize.CoordinatesToCellName(j+1, 1)
			_ = f.SetCellValue("opinie", cell, v)
		}

		row := []string{
			"Baltic Trip", "2024", "", "", "Poland",
			"Gdansk", "Hel", "48", "30", "18",
			"5", "120", "7", "John", "SY Test",
			"sloop", "tak", "5000", "1000", "Great trip",
		}
		for j, v := range row {
			cell, _ := excelize.CoordinatesToCellName(j+1, 2)
			_ = f.SetCellValue("opinie", cell, v)
		}

		cruises, err := parseOpinionSheet(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cruises) != 1 {
			t.Fatalf("got %d cruises, want 1", len(cruises))
		}
		if cruises[0].Name != "Baltic Trip" {
			t.Errorf("got name %q, want %q", cruises[0].Name, "Baltic Trip")
		}
		if cruises[0].TidalWaters == nil || *cruises[0].TidalWaters != 1 {
			t.Errorf("expected tidal_waters=1")
		}
	})

	t.Run("skips empty rows", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		idx, _ := f.NewSheet("opinie")
		f.SetActiveSheet(idx)
		_ = f.SetCellValue("opinie", "A1", "header")
		_ = f.SetCellValue("opinie", "A2", "")

		cruises, err := parseOpinionSheet(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cruises) != 0 {
			t.Fatalf("got %d cruises, want 0", len(cruises))
		}
	})

	t.Run("missing sheet", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		_, err := parseOpinionSheet(f)
		if err == nil {
			t.Fatal("expected error for missing sheet")
		}
	})
}

func TestParseTrainingSheet(t *testing.T) {
	t.Run("parses rows", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		idx, _ := f.NewSheet("szkolenia")
		f.SetActiveSheet(idx)

		_ = f.SetCellValue("szkolenia", "A1", "date")
		_ = f.SetCellValue("szkolenia", "B1", "name")
		_ = f.SetCellValue("szkolenia", "A2", "2024-01-15")
		_ = f.SetCellValue("szkolenia", "B2", "RYA Day Skipper")
		_ = f.SetCellValue("szkolenia", "C2", "RYA")
		_ = f.SetCellValue("szkolenia", "D2", "500")

		trainings, err := parseTrainingSheet(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(trainings) != 1 {
			t.Fatalf("got %d trainings, want 1", len(trainings))
		}
		if trainings[0].Name != "RYA Day Skipper" {
			t.Errorf("got name %q, want %q", trainings[0].Name, "RYA Day Skipper")
		}
	})

	t.Run("skips empty rows", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		idx, _ := f.NewSheet("szkolenia")
		f.SetActiveSheet(idx)
		_ = f.SetCellValue("szkolenia", "A1", "header")

		trainings, err := parseTrainingSheet(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(trainings) != 0 {
			t.Fatalf("got %d, want 0", len(trainings))
		}
	})

	t.Run("missing sheet", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		_, err := parseTrainingSheet(f)
		if err == nil {
			t.Fatal("expected error for missing sheet")
		}
	})
}

func TestParseExcelDate(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	idx, _ := f.NewSheet("dates")
	f.SetActiveSheet(idx)

	_ = f.SetCellValue("dates", "A1", 45658)
	_ = f.SetCellValue("dates", "B1", "2024-01-15")
	_ = f.SetCellValue("dates", "C1", "")

	t.Run("serial date", func(t *testing.T) {
		result := parseExcelDate(f, "dates", 1, 0)
		if result == nil {
			t.Fatal("expected non-nil date")
		}
	})

	t.Run("string date", func(t *testing.T) {
		result := parseExcelDate(f, "dates", 1, 1)
		if result == nil {
			t.Fatal("expected non-nil date")
		}
	})

	t.Run("empty cell", func(t *testing.T) {
		result := parseExcelDate(f, "dates", 1, 2)
		if result != nil {
			t.Fatalf("expected nil, got %v", *result)
		}
	})
}

func importTestAPI(t *testing.T, m *mockQuerier) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	h := &ImportHandler{runTx: directTxRunner(m)}
	h.registerRoutes(api)
	return api
}

// multipartFile builds a multipart body with one "file" part holding content.
func multipartFile(t *testing.T, filename string, content []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body, w.FormDataContentType()
}

func TestUpload(t *testing.T) {
	t.Run("valid xlsx", func(t *testing.T) {
		xlsxBuf := createTestXLSX(t,
			[][]string{{"name", "year"}, {"Baltic Trip", "2024"}},
			[][]string{{"date", "name"}, {"2024-01-15", "RYA"}},
		)
		body, ct := multipartFile(t, "test.xlsx", xlsxBuf.Bytes())
		resp := importTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/import/xlsx", "Content-Type: "+ct, body)
		if resp.Code != http.StatusOK {
			t.Fatalf("got %d, want 200: %s", resp.Code, resp.Body)
		}
	})

	t.Run("missing file field", func(t *testing.T) {
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		_ = w.Close()
		resp := importTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/import/xlsx", "Content-Type: "+w.FormDataContentType(), body)
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf("got %d, want 422", resp.Code)
		}
	})

	t.Run("invalid xlsx", func(t *testing.T) {
		body, ct := multipartFile(t, "test.xlsx", []byte("not an xlsx file"))
		resp := importTestAPI(t, &mockQuerier{}).PostCtx(userCtx(context.Background()), "/import/xlsx", "Content-Type: "+ct, body)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", resp.Code)
		}
	})
}

func TestConfirm(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := &mockQuerier{
			createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
			},
			createTrainingFn: func(_ context.Context, arg sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{ID: 1, Name: arg.Name}, nil
			},
		}
		resp := importTestAPI(t, m).PostCtx(userCtx(context.Background()), "/import/confirm",
			map[string]any{
				"voyages":   []map[string]any{{"name": "Baltic"}},
				"trainings": []map[string]any{{"name": "RYA Day Skipper"}},
			})
		if resp.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201; body=%s", resp.Code, resp.Body)
		}
	})

	t.Run("training failure rolls back the import", func(t *testing.T) {
		voyageCreated := false
		m := &mockQuerier{
			createVoyageFn: func(_ context.Context, arg sqlcdb.CreateVoyageParams) (sqlcdb.Voyage, error) {
				voyageCreated = true
				return sqlcdb.Voyage{ID: 1, Name: arg.Name}, nil
			},
			createTrainingFn: func(context.Context, sqlcdb.CreateTrainingParams) (sqlcdb.Training, error) {
				return sqlcdb.Training{}, errors.New("training write failed")
			},
		}
		resp := importTestAPI(t, m).PostCtx(userCtx(context.Background()), "/import/confirm",
			map[string]any{
				"voyages":   []map[string]any{{"name": "Baltic"}},
				"trainings": []map[string]any{{"name": "RYA Day Skipper"}},
			})
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("got %d, want 500; body=%s", resp.Code, resp.Body)
		}
		if !voyageCreated {
			t.Error("expected voyage create to run inside the same transaction")
		}
	})
}
