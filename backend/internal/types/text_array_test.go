package types

import (
	"slices"
	"testing"
)

func TestTextArrayScan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   any
		want []string
	}{
		{"list", "{NO,DK}", []string{"NO", "DK"}},
		{"bytes", []byte("{HR}"), []string{"HR"}},
		{"quoted item", `{"a b",c}`, []string{"a b", "c"}},
		{"empty", "{}", []string{}},
		{"null", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var a TextArray
			if err := a.Scan(tt.in); err != nil {
				t.Fatalf("Scan(%v): %v", tt.in, err)
			}
			if a == nil || !slices.Equal(a, tt.want) {
				t.Errorf("Scan(%v) = %#v, want %q", tt.in, a, tt.want)
			}
		})
	}
	var a TextArray
	if err := a.Scan("not an array"); err == nil {
		t.Error("Scan of a non-array = nil error")
	}
}

func TestTextArrayValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   TextArray
		want string
	}{
		{"list", TextArray{"NO", "DK"}, "{NO,DK}"},
		{"quoted item", TextArray{"a b"}, `{"a b"}`},
		{"empty", TextArray{}, "{}"},
		{"nil", nil, "{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.in.Value()
			if err != nil {
				t.Fatalf("Value(): %v", err)
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %q", got, tt.want)
			}
		})
	}
}
