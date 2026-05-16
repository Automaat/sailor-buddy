package handlers

// pageParams carries the limit/offset query parameters shared by every
// paginated list operation. huma derives the OpenAPI parameter definitions —
// including the defaults and bounds — from these struct tags. The fields are
// int32 to match the sqlc-generated LIMIT/OFFSET query parameters directly.
type pageParams struct {
	Limit  int32 `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32 `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
}

// orgListParams is the input for an org-scoped paginated list: the org slug
// path segment plus the shared pagination query parameters.
type orgListParams struct {
	Slug string `path:"slug" doc:"Organization slug"`
	pageParams
}

// sqlLimit and sqlOffset name the query params at the call site where they
// feed a sqlc LIMIT/OFFSET argument. huma has already validated the bounds.
func (p pageParams) sqlLimit() int32  { return p.Limit }
func (p pageParams) sqlOffset() int32 { return p.Offset }
