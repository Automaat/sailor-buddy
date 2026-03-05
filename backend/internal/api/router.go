package api

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/handlers"
	"github.com/marcinskalski/sailor-buddy/backend/internal/api/middleware"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db/sqlcdb"
)

func NewRouter(db *sql.DB, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	q := sqlcdb.New(db)

	authH := handlers.NewAuthHandler(q, cfg.JWTSecret)
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		dashH := handlers.NewDashboardHandler(q)
		r.Get("/dashboard", dashH.Get)

		cruiseH := handlers.NewCruiseHandler(q)
		r.Route("/cruises", func(r chi.Router) {
			r.Get("/", cruiseH.List)
			r.Post("/", cruiseH.Create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cruiseH.Get)
				r.Put("/", cruiseH.Update)
				r.Delete("/", cruiseH.Delete)
			})
		})

		yachtH := handlers.NewYachtHandler(q)
		r.Route("/yachts", func(r chi.Router) {
			r.Get("/", yachtH.List)
			r.Post("/", yachtH.Create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", yachtH.Get)
				r.Put("/", yachtH.Update)
				r.Delete("/", yachtH.Delete)
			})
		})

		crewH := handlers.NewCrewHandler(q)
		r.Route("/crew", func(r chi.Router) {
			r.Get("/", crewH.List)
			r.Post("/", crewH.Create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", crewH.Get)
				r.Put("/", crewH.Update)
				r.Delete("/", crewH.Delete)
			})
		})

		r.Route("/cruises/{cruiseID}/crew", func(r chi.Router) {
			r.Get("/", crewH.ListCruiseCrew)
			r.Post("/", crewH.AssignCrew)
			r.Delete("/{assignmentID}", crewH.RemoveCruiseCrew)
		})

		trainingH := handlers.NewTrainingHandler(q)
		r.Route("/trainings", func(r chi.Router) {
			r.Get("/", trainingH.List)
			r.Post("/", trainingH.Create)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", trainingH.Get)
				r.Put("/", trainingH.Update)
				r.Delete("/", trainingH.Delete)
			})
		})

		importH := handlers.NewImportHandler(q)
		r.Post("/import/xlsx", importH.Upload)
		r.Post("/import/confirm", importH.Confirm)
	})

	return r
}
