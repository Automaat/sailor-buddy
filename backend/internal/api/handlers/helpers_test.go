package handlers

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- null helpers (cruises.go) ---

func TestNullString(t *testing.T) {
	t.Parallel()
	s := "hello"
	tests := []struct {
		name string
		in   *string
		want sql.NullString
	}{
		{"nil", nil, sql.NullString{}},
		{"valid", &s, sql.NullString{String: "hello", Valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullString(tt.in)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNullInt64(t *testing.T) {
	t.Parallel()
	v := int64(42)
	tests := []struct {
		name string
		in   *int64
		want sql.NullInt64
	}{
		{"nil", nil, sql.NullInt64{}},
		{"valid", &v, sql.NullInt64{Int64: 42, Valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullInt64(tt.in)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNullFloat64(t *testing.T) {
	t.Parallel()
	v := 3.14
	tests := []struct {
		name string
		in   *float64
		want sql.NullFloat64
	}{
		{"nil", nil, sql.NullFloat64{}},
		{"valid", &v, sql.NullFloat64{Float64: 3.14, Valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullFloat64(tt.in)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// --- import helpers (import.go) ---

func TestCellAt(t *testing.T) {
	t.Parallel()
	row := []string{"a", "b", "c"}
	tests := []struct {
		name string
		row  []string
		idx  int
		want string
	}{
		{"first", row, 0, "a"},
		{"last", row, 2, "c"},
		{"out of bounds", row, 5, ""},
		{"empty row", nil, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cellAt(tt.row, tt.idx); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOptString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantVal string
	}{
		{"empty", "", true, ""},
		{"whitespace only", "   ", true, ""},
		{"value", "hello", false, "hello"},
		{"trimmed", "  hi  ", false, "hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optString(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %q", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if *got != tt.wantVal {
				t.Errorf("got %q, want %q", *got, tt.wantVal)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantVal int64
	}{
		{"empty", "", true, 0},
		{"whitespace", "  ", true, 0},
		{"invalid", "abc", true, 0},
		{"integer", "42", false, 42},
		{"float truncated", "3.9", false, 3},
		{"negative", "-5", false, -5},
		{"with spaces", " 10 ", false, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInt64(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %d", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if *got != tt.wantVal {
				t.Errorf("got %d, want %d", *got, tt.wantVal)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantVal float64
	}{
		{"empty", "", true, 0},
		{"whitespace", "  ", true, 0},
		{"invalid", "xyz", true, 0},
		{"dot decimal", "3.14", false, 3.14},
		{"comma decimal", "3,14", false, 3.14},
		{"integer", "7", false, 7.0},
		{"negative", "-2.5", false, -2.5},
		{"with spaces", " 1.5 ", false, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFloat(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %f", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if math.Abs(*got-tt.wantVal) > 1e-9 {
				t.Errorf("got %f, want %f", *got, tt.wantVal)
			}
		})
	}
}

func TestParseTidalWaters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      string
		wantNil bool
		wantVal int64
	}{
		{"empty", "", true, 0},
		{"whitespace", "  ", true, 0},
		{"tak", "tak", false, 1},
		{"TAK uppercase", "TAK", false, 1},
		{"Tak mixed", " Tak ", false, 1},
		{"nie", "nie", false, 0},
		{"other", "maybe", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTidalWaters(tt.in)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %d", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if *got != tt.wantVal {
				t.Errorf("got %d, want %d", *got, tt.wantVal)
			}
		})
	}
}

func TestExcelSerialToTime(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		serial float64
		want   time.Time
	}{
		{"epoch", 1, time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"2024-01-01", 45292, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"with time 12:00", 45292.5, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
		{"zero", 0, time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := excelSerialToTime(tt.serial)
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// --- respond helpers (respond.go) ---

func TestFlattenNulls(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   any
		want any
	}{
		{
			"valid NullString flattened to string value",
			map[string]any{"String": "x", "Valid": true},
			"x",
		},
		{
			"invalid NullString flattened to nil",
			map[string]any{"String": "", "Valid": false},
			nil,
		},
		{
			"valid NullInt64 flattened to int value",
			map[string]any{"Int64": float64(42), "Valid": true},
			float64(42),
		},
		{
			"invalid NullInt64 flattened to nil",
			map[string]any{"Int64": float64(0), "Valid": false},
			nil,
		},
		{
			"struct with NullString field",
			map[string]any{
				"name": map[string]any{"String": "Alice", "Valid": true},
				"note": map[string]any{"String": "", "Valid": false},
			},
			map[string]any{"name": "Alice", "note": nil},
		},
		{
			"non-null-struct map unchanged",
			map[string]any{"foo": "bar", "baz": float64(1)},
			map[string]any{"foo": "bar", "baz": float64(1)},
		},
		{
			"slice with null structs",
			[]any{
				map[string]any{"String": "a", "Valid": true},
				map[string]any{"String": "", "Valid": false},
			},
			[]any{"a", nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenNulls(tt.in)
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("got %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestRespondJSONFlattenNulls(t *testing.T) {
	t.Parallel()
	type row struct {
		Name sql.NullString
		Age  sql.NullInt64
	}
	tests := []struct {
		name     string
		data     row
		wantBody string
	}{
		{
			"valid fields flattened",
			row{
				Name: sql.NullString{String: "Alice", Valid: true},
				Age:  sql.NullInt64{Int64: 30, Valid: true},
			},
			`{"Age":30,"Name":"Alice"}` + "\n",
		},
		{
			"invalid fields become null",
			row{
				Name: sql.NullString{},
				Age:  sql.NullInt64{},
			},
			`{"Age":null,"Name":null}` + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondJSON(w, http.StatusOK, tt.data)
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body: got %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestRespondJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantBody   string
	}{
		{
			"map data",
			http.StatusOK,
			map[string]string{"key": "val"},
			200,
			`{"key":"val"}` + "\n",
		},
		{
			"nil data",
			http.StatusNoContent,
			nil,
			204,
			"",
		},
		{
			"nil slice returns empty array",
			http.StatusOK,
			[]string(nil),
			200,
			"[]\n",
		},
		{
			"non-nil empty slice",
			http.StatusOK,
			[]string{},
			200,
			"[]\n",
		},
		{
			"slice with items",
			http.StatusOK,
			[]int{1, 2},
			200,
			"[1,2]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondJSON(w, tt.status, tt.data)
			if w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type: got %q", ct)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body: got %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestRespondError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	respondError(w, http.StatusBadRequest, "bad input")
	if w.Code != 400 {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["error"] != "bad input" {
		t.Errorf("error: got %q, want %q", body["error"], "bad input")
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		wantErr bool
		wantVal string
	}{
		{"valid", `{"name":"test"}`, false, "test"},
		{"invalid json", `{bad`, true, ""},
		{"empty body", "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			var v struct{ Name string }
			err := decodeJSON(r, &v)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Name != tt.wantVal {
				t.Errorf("got %q, want %q", v.Name, tt.wantVal)
			}
		})
	}
}
