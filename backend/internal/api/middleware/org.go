package middleware

// OrgContext carries the resolved organization scope for a request: the org's
// ID, slug and the caller's role within it. It is populated by the handlers'
// resolveOrg helper, which replaced the former OrgFromSlug/RequireOrgRole
// chi middleware when org routes moved to huma.
type OrgContext struct {
	OrgID int64
	Slug  string
	Role  string
}
