package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNullMarshalJSON(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"NullString valid", NullString{String: "x", Valid: true}, `"x"`},
		{"NullString invalid", NullString{}, "null"},
		{"NullString empty valid", NullString{String: "", Valid: true}, `""`},
		{"NullInt64 valid", NullInt64{Int64: 42, Valid: true}, "42"},
		{"NullInt64 invalid", NullInt64{}, "null"},
		{"NullInt64 zero valid", NullInt64{Int64: 0, Valid: true}, "0"},
		{"NullFloat64 valid", NullFloat64{Float64: 3.5, Valid: true}, "3.5"},
		{"NullFloat64 invalid", NullFloat64{}, "null"},
		{"NullTime valid", NullTime{Time: ts, Valid: true}, `"2026-05-15T09:30:00Z"`},
		{"NullTime invalid", NullTime{}, "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNullUnmarshalJSON(t *testing.T) {
	t.Parallel()
	t.Run("NullString", func(t *testing.T) {
		t.Parallel()
		var n NullString
		if err := json.Unmarshal([]byte(`"hi"`), &n); err != nil {
			t.Fatal(err)
		}
		if n != (NullString{String: "hi", Valid: true}) {
			t.Errorf("got %+v", n)
		}
		if err := json.Unmarshal([]byte(`null`), &n); err != nil {
			t.Fatal(err)
		}
		if n != (NullString{}) {
			t.Errorf("null: got %+v", n)
		}
	})
	t.Run("NullInt64", func(t *testing.T) {
		t.Parallel()
		var n NullInt64
		if err := json.Unmarshal([]byte(`7`), &n); err != nil {
			t.Fatal(err)
		}
		if n != (NullInt64{Int64: 7, Valid: true}) {
			t.Errorf("got %+v", n)
		}
		if err := json.Unmarshal([]byte(`null`), &n); err != nil {
			t.Fatal(err)
		}
		if n != (NullInt64{}) {
			t.Errorf("null: got %+v", n)
		}
	})
	t.Run("NullFloat64", func(t *testing.T) {
		t.Parallel()
		var n NullFloat64
		if err := json.Unmarshal([]byte(`1.25`), &n); err != nil {
			t.Fatal(err)
		}
		if n != (NullFloat64{Float64: 1.25, Valid: true}) {
			t.Errorf("got %+v", n)
		}
	})
	t.Run("NullTime", func(t *testing.T) {
		t.Parallel()
		var n NullTime
		if err := json.Unmarshal([]byte(`"2026-05-15T09:30:00Z"`), &n); err != nil {
			t.Fatal(err)
		}
		if !n.Valid || !n.Time.Equal(time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)) {
			t.Errorf("got %+v", n)
		}
	})
}

func TestNullRoundTrip(t *testing.T) {
	t.Parallel()
	var n NullString
	for _, raw := range []string{`"abc"`, `null`} {
		if err := json.Unmarshal([]byte(raw), &n); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != raw {
			t.Errorf("round trip %s -> %s", raw, out)
		}
	}
}

// TestNullNested verifies nulls inside slices and nested structs marshal to
// plain values with no "Valid" key leaking into the output.
func TestNullNested(t *testing.T) {
	t.Parallel()
	type inner struct {
		Note NullString `json:"note"`
	}
	payload := struct {
		Items []NullInt64 `json:"items"`
		Inner inner       `json:"inner"`
	}{
		Items: []NullInt64{{Int64: 1, Valid: true}, {}},
		Inner: inner{Note: NullString{}},
	}
	got, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"items":[1,null],"inner":{"note":null}}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestNullScanValue(t *testing.T) {
	t.Parallel()
	t.Run("NullString scan", func(t *testing.T) {
		t.Parallel()
		var n NullString
		if err := n.Scan("hello"); err != nil {
			t.Fatal(err)
		}
		if n != (NullString{String: "hello", Valid: true}) {
			t.Errorf("got %+v", n)
		}
		if err := n.Scan(nil); err != nil {
			t.Fatal(err)
		}
		if n.Valid {
			t.Errorf("nil scan should be invalid: %+v", n)
		}
	})
	t.Run("Value", func(t *testing.T) {
		t.Parallel()
		v, err := NullInt64{Int64: 9, Valid: true}.Value()
		if err != nil {
			t.Fatal(err)
		}
		if v != int64(9) {
			t.Errorf("got %v", v)
		}
		v, err = NullInt64{}.Value()
		if err != nil {
			t.Fatal(err)
		}
		if v != nil {
			t.Errorf("invalid Value should be nil, got %v", v)
		}
	})
}
