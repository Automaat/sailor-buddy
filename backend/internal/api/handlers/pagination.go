package handlers

// pageParams carries the limit/offset query parameters shared by every
// paginated owner-scoped list operation. huma derives the OpenAPI parameter
// definitions — including the defaults and bounds — from these struct tags.
// The fields are int32 to match the sqlc-generated LIMIT/OFFSET parameters.
type pageParams struct {
	Limit  int32 `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32 `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
}

// orgListParams is the input for an org-scoped paginated list: the org slug
// path segment plus the shared pagination query parameters. The pagination
// fields are declared inline rather than embedded because huma does not
// promote query parameters from an anonymous embedded struct.
type orgListParams struct {
	Slug   string `path:"slug" doc:"Organization slug"`
	Limit  int32  `query:"limit" minimum:"1" maximum:"100" default:"50" doc:"Maximum number of items to return"`
	Offset int32  `query:"offset" minimum:"0" default:"0" doc:"Number of items to skip"`
}
