package dto

// Page is the API representation of a paginated list response: the items in
// the requested window plus the metadata a client needs to fetch the rest.
type Page[T any] struct {
	Items      []T    `json:"items"`
	Total      int64  `json:"total" doc:"Total number of items matching the query"`
	Limit      int    `json:"limit" doc:"Maximum items returned in this window"`
	Offset     int    `json:"offset" doc:"Number of items skipped before this window"`
	HasMore    bool   `json:"has_more" doc:"Whether more items exist beyond this window"`
	NextCursor string `json:"next_cursor,omitempty" doc:"Pass as after to fetch the next window; set when has_more is true"`
}

// NewPage assembles a Page from a window of items, the total match count,
// whether more items follow and the cursor of the next window. The items
// slice is normalised to non-nil so it serialises as [], never null.
func NewPage[T any](items []T, total int64, limit, offset int32, hasMore bool, nextCursor string) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{
		Items:      items,
		Total:      total,
		Limit:      int(limit),
		Offset:     int(offset),
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}
}
