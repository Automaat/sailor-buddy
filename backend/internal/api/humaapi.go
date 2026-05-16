package api

import (
	"database/sql"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

// humaConfig builds the shared OpenAPI config. The /api base path lives in the
// server URL so operation paths stay relative. The schema-link transformer is
// dropped so response bodies do not carry a $schema field. Doc and spec routes
// are disabled — the spec is produced offline by cmd/openapi.
func humaConfig() huma.Config {
	cfg := huma.DefaultConfig("Sailor Buddy API", "1.0.0")
	cfg.Servers = []*huma.Server{{URL: "/api"}}
	cfg.Transformers = nil
	cfg.DocsPath = ""
	cfg.OpenAPIPath = ""
	cfg.SchemasPath = ""
	return cfg
}

// registerHumaRoutes wires every huma-served operation onto the given API.
// It is shared by the live router and the offline spec generator, so the
// generated OpenAPI document always matches the routes the server serves.
func registerHumaRoutes(api huma.API, q sqlcdb.Querier, db *sql.DB, uploadDir string) {
	handlers.RegisterAuthRoutes(api)
	handlers.RegisterDashboardRoutes(api, q)
	handlers.RegisterTripRoutes(api, q, db)
	handlers.RegisterVoyageRoutes(api, q)
	handlers.RegisterYachtRoutes(api, q)
	handlers.RegisterTrainingRoutes(api, q)
	handlers.RegisterCrewRoutes(api, q)
	handlers.RegisterEnrollmentRoutes(api, q)
	handlers.RegisterVoyageOpinionRoutes(api, q, uploadDir)
	handlers.RegisterUploadRoutes(api, uploadDir)
	handlers.RegisterImportRoutes(api, q)
	handlers.RegisterOrgRoutes(api, q)
	handlers.RegisterOrgResourceRoutes(api, q)
	handlers.RegisterOrgTripRoutes(api, q, db)
	handlers.RegisterOrgVoyageRoutes(api, q)
	handlers.RegisterOrgCruiseRoutes(api, q)
	handlers.RegisterCruiseEnrollmentRoutes(api, q)
	handlers.RegisterOrgCrewAssignmentRoutes(api, q)
	handlers.RegisterOrgVoyageOpinionRoutes(api, q, uploadDir)
}

// OpenAPIYAML builds the OpenAPI document for all huma-served routes and
// returns it as YAML. Handlers are registered with nil dependencies — the
// document is derived from the operation types only, handlers are never run.
func OpenAPIYAML() ([]byte, error) {
	api := humachi.New(chi.NewRouter(), humaConfig())
	registerHumaRoutes(api, nil, nil, "")
	return api.OpenAPI().YAML()
}
