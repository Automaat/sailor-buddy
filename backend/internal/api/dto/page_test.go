package dto

import "testing"

func TestNewPage(t *testing.T) {
	t.Run("computes has_more from total and window", func(t *testing.T) {
		page := NewPage([]int{1, 2, 3}, 50, 10, 20)
		if page.Total != 50 || page.Limit != 10 || page.Offset != 20 {
			t.Fatalf("unexpected meta: %+v", page)
		}
		if !page.HasMore {
			t.Fatalf("want has_more true (23 of 50 seen)")
		}
		if len(page.Items) != 3 {
			t.Fatalf("items = %v", page.Items)
		}
	})

	t.Run("last window has no more", func(t *testing.T) {
		page := NewPage([]int{9, 10}, 10, 5, 8)
		if page.HasMore {
			t.Fatalf("want has_more false (all 10 seen)")
		}
	})

	t.Run("nil items serialise as empty slice", func(t *testing.T) {
		page := NewPage[int](nil, 0, 50, 0)
		if page.Items == nil {
			t.Fatalf("items must be non-nil")
		}
		if len(page.Items) != 0 || page.HasMore {
			t.Fatalf("unexpected empty page: %+v", page)
		}
	})
}
