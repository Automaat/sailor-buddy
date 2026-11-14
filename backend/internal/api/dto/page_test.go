package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewPage(t *testing.T) {
	t.Run("carries the window metadata", func(t *testing.T) {
		page := NewPage([]int{1, 2, 3}, 50, 10, 20, true, "cur")
		if page.Total != 50 || page.Limit != 10 || page.Offset != 20 || !page.HasMore || page.NextCursor != "cur" {
			t.Fatalf("unexpected meta: %+v", page)
		}
		if len(page.Items) != 3 {
			t.Fatalf("items = %v", page.Items)
		}
	})

	t.Run("nil items serialise as empty slice", func(t *testing.T) {
		page := NewPage[int](nil, 0, 50, 0, false, "")
		if page.Items == nil {
			t.Fatalf("items must be non-nil")
		}
		if len(page.Items) != 0 || page.HasMore {
			t.Fatalf("unexpected empty page: %+v", page)
		}
	})

	t.Run("last window omits next_cursor", func(t *testing.T) {
		b, err := json.Marshal(NewPage([]int{9}, 1, 5, 0, false, ""))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "next_cursor") {
			t.Fatalf("last page JSON %s must omit next_cursor", b)
		}
	})
}
