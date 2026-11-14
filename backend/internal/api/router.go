package api

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

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
	r.Get("/readyz", readyz(db))

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

// readyTimeout bounds the database ping behind /readyz.
const readyTimeout = 2 * time.Second

// readyz reports 200 when db answers a ping within readyTimeout, else 503.
func readyz(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			slog.Warn("readyz: database ping failed", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// mountStaticRoutes registers chi routes for static asset delivery, which is
// not modelled as an API operation in the OpenAPI spec.
func mountStaticRoutes(r chi.Router, cfg *config.Config) {
	uploadH := handlers.NewUploadHandler(cfg.UploadDir)
	r.Get("/uploads/*", uploadH.ServeFile)
}
