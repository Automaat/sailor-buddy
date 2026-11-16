package countries

import (
	"slices"
	"strings"
	"testing"
)

func TestAll_ListsAssignedCodesInOrder(t *testing.T) {
	list := All()
	if len(list) != 249 {
		t.Fatalf("len(All()) = %d, want the 249 assigned ISO 3166-1 codes", len(list))
	}
	if !slices.IsSortedFunc(list, func(a, b Country) int { return strings.Compare(a.Code, b.Code) }) {
		t.Error("countries.json is not ordered by code")
	}
	for _, code := range []string{"EU", "UK", "XK", "ZZ"} {
		if Known(code) {
			t.Errorf("Known(%q) = true, want only assigned ISO codes", code)
		}
	}
	list[0].Name = "changed"
	if All()[0].Name == "changed" {
		t.Error("All() returned the shared slice")
	}
}

func TestName(t *testing.T) {
	tests := []struct{ code, want string }{
		{"NO", "Norwegia"},
		{"DK", "Dania"},
		{"HR", "Chorwacja"},
		{"GB", "Wielka Brytania"},
		{"QQ", "QQ"},
	}
	for _, tt := range tests {
		if got := Name(tt.code); got != tt.want {
			t.Errorf("Name(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name  string
		codes []string
		want  string
	}{
		{"none", nil, ""},
		{"one", []string{"NO"}, "Norwegia"},
		{"keeps order", []string{"SE", "NO", "DK"}, "Szwecja, Norwegia, Dania"},
		{"unknown code kept", []string{"NO", "QQ"}, "Norwegia, QQ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.codes); got != tt.want {
				t.Errorf("Format(%v) = %q, want %q", tt.codes, got, tt.want)
			}
		})
	}
}

func TestParse_RejectsBadData(t *testing.T) {
	tests := []struct{ name, raw, want string }{
		{"not json", `{`, "countries.json"},
		{"lower-case code", `[{"code":"no","name":"Norwegia"}]`, "two upper-case letters"},
		{"blank name", `[{"code":"NO","name":" "}]`, "has no name"},
		{"repeated code", `[{"code":"NO","name":"Norwegia"},{"code":"NO","name":"Norwegia"}]`, "listed twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}
