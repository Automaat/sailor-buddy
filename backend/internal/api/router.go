package api

import (
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func NewRouter(db *sql.DB, cfg *config.Config, fbClient *fbauth.Client) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	allowCredentials := false
	for _, o := range cfg.CORSAllowedOrigins {
		if allowCredentials && o == "*" {
			panic("CORS misconfiguration: AllowCredentials=true with wildcard origin is forbidden")
		}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: allowCredentials,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	q := sqlcdb.New(db)

	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(fbClient, q))
			registerHumaRoutes(humachi.New(r, humaConfig()), q, db, cfg.UploadDir)
			mountStaticRoutes(r, cfg)
		})
	})

	return r
}

// mountStaticRoutes registers chi routes for static asset delivery, which is
// not modelled as an API operation in the OpenAPI spec.
func mountStaticRoutes(r chi.Router, cfg *config.Config) {
	uploadH := handlers.NewUploadHandler(cfg.UploadDir)
	r.Get("/uploads/*", uploadH.ServeFile)
}
