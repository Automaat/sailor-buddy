package dto

// Page is the API representation of a paginated list response: the items in
// the requested window plus the metadata a client needs to fetch the rest.
type Page[T any] struct {
	Items   []T   `json:"items"`
	Total   int64 `json:"total" doc:"Total number of items matching the query"`
	Limit   int   `json:"limit" doc:"Maximum items returned in this window"`
	Offset  int   `json:"offset" doc:"Number of items skipped before this window"`
	HasMore bool  `json:"has_more" doc:"Whether more items exist beyond this window"`
}

// NewPage assembles a Page from a window of items and the total match count.
// The items slice is normalised to non-nil so it serialises as [], never null.
func NewPage[T any](items []T, total int64, limit, offset int32) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{
		Items:   items,
		Total:   total,
		Limit:   int(limit),
		Offset:  int(offset),
		HasMore: int64(offset)+int64(len(items)) < total,
	}
}
