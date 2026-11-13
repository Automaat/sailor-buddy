package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// --- null helpers (cruises.go) ---

func TestNullString(t *testing.T) {
	t.Parallel()
	s := "hello"
	tests := []struct {
		name string
		in   *string
		want types.NullString
	}{
		{"nil", nil, types.NullString{}},
		{"valid", &s, types.NullString{String: "hello", Valid: true}},
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
		want types.NullInt64
	}{
		{"nil", nil, types.NullInt64{}},
		{"valid", &v, types.NullInt64{Int64: 42, Valid: true}},
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
		want types.NullFloat64
	}{
		{"nil", nil, types.NullFloat64{}},
		{"valid", &v, types.NullFloat64{Float64: 3.14, Valid: true}},
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

// --- respond helpers (respond.go) ---

func TestRespondJSONNullFields(t *testing.T) {
	t.Parallel()
	type row struct {
		Name types.NullString
		Age  types.NullInt64
	}
	tests := []struct {
		name     string
		data     row
		wantBody string
	}{
		{
			"valid fields render as plain values",
			row{
				Name: types.NullString{String: "Alice", Valid: true},
				Age:  types.NullInt64{Int64: 30, Valid: true},
			},
			`{"Name":"Alice","Age":30}` + "\n",
		},
		{
			"invalid fields become null",
			row{
				Name: types.NullString{},
				Age:  types.NullInt64{},
			},
			`{"Name":null,"Age":null}` + "\n",
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
