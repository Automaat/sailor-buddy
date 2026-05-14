package api

import (
	"database/sql"
	"net/http"

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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	q := sqlcdb.New(db)

	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(fbClient, q))

			authH := handlers.NewAuthHandler()
			r.Get("/auth/me", authH.Me)

			dashH := handlers.NewDashboardHandler(q)
			r.Get("/dashboard", dashH.Get)

			tripH := handlers.NewTripHandler(q, db)
			voyH := handlers.NewVoyageHandler(q)
			crewH := handlers.NewCrewHandler(q)
			opinH := handlers.NewVoyageOpinionHandler(q, cfg.UploadDir)
			enrollH := handlers.NewEnrollmentHandler(q)
			r.Route("/enroll/{token}", func(r chi.Router) {
				r.Get("/", enrollH.GetByToken)
				r.Post("/", enrollH.Enroll)
			})

			r.Route("/trips", func(r chi.Router) {
				r.Get("/", tripH.List)
				r.Post("/", tripH.Create)
				r.Post("/{id}/complete", tripH.Complete)
				r.Post("/{id}/cancel", tripH.Cancel)
				r.Route("/{tripID}/crew", func(r chi.Router) {
					r.Get("/", crewH.ListTripCrew)
					r.Post("/", crewH.AssignTripCrew)
					r.Delete("/{assignmentID}", crewH.RemoveTripCrew)
				})
				r.Post("/{tripID}/enroll-token", enrollH.GenerateToken)
				r.Delete("/{tripID}/enroll-token", enrollH.ClearToken)
				r.Get("/{tripID}/enrollments", enrollH.ListEnrollments)
				r.Put("/{tripID}/enrollments/{id}/status", enrollH.UpdateStatus)
				r.Delete("/{tripID}/enrollments/{id}", enrollH.DeleteEnrollment)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", tripH.Get)
					r.Put("/", tripH.Update)
					r.Delete("/", tripH.Delete)
				})
			})

			r.Route("/voyages", func(r chi.Router) {
				r.Get("/", voyH.List)
				r.Post("/", voyH.Create)
				r.Route("/{voyageID}/crew", func(r chi.Router) {
					r.Get("/", crewH.ListVoyageCrew)
					r.Post("/", crewH.AssignVoyageCrew)
					r.Delete("/{assignmentID}", crewH.RemoveVoyageCrew)
				})
				r.Route("/{voyageID}/opinions", func(r chi.Router) {
					r.Get("/", opinH.List)
					r.Post("/", opinH.Generate)
					r.Get("/{id}/download", opinH.Download)
					r.Delete("/{id}", opinH.Delete)
				})
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", voyH.Get)
					r.Put("/", voyH.Update)
					r.Delete("/", voyH.Delete)
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

			r.Route("/crew", func(r chi.Router) {
				r.Get("/", crewH.List)
				r.Post("/", crewH.Create)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", crewH.Get)
					r.Put("/", crewH.Update)
					r.Delete("/", crewH.Delete)
				})
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

			uploadH := handlers.NewUploadHandler(cfg.UploadDir)
			r.Post("/upload/image", uploadH.UploadImage)
			r.Get("/uploads/*", uploadH.ServeFile)

			importH := handlers.NewImportHandler(q)
			r.Post("/import/xlsx", importH.Upload)
			r.Post("/import/confirm", importH.Confirm)

			orgH := handlers.NewOrgHandler(q)

			r.Route("/join/{token}", func(r chi.Router) {
				r.Get("/", orgH.GetInviteInfo)
				r.Post("/", orgH.AcceptInvite)
			})

			r.Route("/orgs", func(r chi.Router) {
				r.Get("/", orgH.List)
				r.Post("/", orgH.Create)

				r.Route("/{slug}", func(r chi.Router) {
					r.Use(middleware.OrgFromSlug(q))

					r.Get("/", orgH.Get)

					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireOrgRole("admin"))
						r.Put("/", orgH.Update)
						r.Delete("/", orgH.Delete)
					})

					r.Get("/members", orgH.ListMembers)
					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireOrgRole("admin"))
						r.Put("/members/{memberID}/role", orgH.UpdateMemberRole)
						r.Delete("/members/{memberID}", orgH.RemoveMember)
						r.Post("/invites", orgH.CreateInvite)
						r.Get("/invites", orgH.ListInvites)
						r.Delete("/invites/{inviteID}", orgH.DeleteInvite)
					})

					orgYachtH := handlers.NewOrgYachtHandler(q)
					r.Get("/yachts", orgYachtH.List)
					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireOrgRole("admin"))
						r.Post("/yachts", orgYachtH.Create)
						r.Put("/yachts/{id}", orgYachtH.Update)
						r.Delete("/yachts/{id}", orgYachtH.Delete)
					})
					r.Get("/yachts/{id}", orgYachtH.Get)

					orgTripH := handlers.NewOrgTripHandler(q, db)
					orgVoyH := handlers.NewOrgVoyageHandler(q)
					orgCruiseH := handlers.NewOrgCruiseHandler(q)
					cruiseEnrollH := handlers.NewCruiseEnrollmentHandler(q)
					r.Get("/trips", orgTripH.List)
					r.Get("/voyages", orgVoyH.List)
					r.Get("/cruises", orgCruiseH.List)
					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireOrgRole("admin"))
						r.Post("/trips", orgTripH.Create)
						r.Put("/trips/{id}", orgTripH.Update)
						r.Delete("/trips/{id}", orgTripH.Delete)
						r.Post("/trips/{id}/complete", orgTripH.Complete)
						r.Post("/trips/{id}/cancel", orgTripH.Cancel)
						r.Post("/voyages", orgVoyH.Create)
						r.Put("/voyages/{id}", orgVoyH.Update)
						r.Delete("/voyages/{id}", orgVoyH.Delete)
						r.Post("/cruises", orgCruiseH.Create)
						r.Put("/cruises/{id}", orgCruiseH.Update)
						r.Delete("/cruises/{id}", orgCruiseH.Delete)
						r.Post("/cruises/{id}/enroll-token", orgCruiseH.GenerateEnrollToken)
						r.Delete("/cruises/{id}/enroll-token", orgCruiseH.ClearEnrollToken)
						r.Put("/cruises/{id}/enrollments/{enrollmentID}/status", cruiseEnrollH.UpdateStatus)
						r.Put("/cruises/{id}/enrollments/{enrollmentID}/trip", cruiseEnrollH.AssignToTrip)
						r.Delete("/cruises/{id}/enrollments/{enrollmentID}", cruiseEnrollH.Delete)
					})
					r.Get("/trips/{id}", orgTripH.Get)
					r.Get("/voyages/{id}", orgVoyH.Get)
					r.Get("/cruises/{id}", orgCruiseH.Get)
					r.Get("/cruises/{id}/trips", orgCruiseH.ListChildTrips)
					r.Get("/cruises/{id}/voyages", orgCruiseH.ListChildVoyages)
					r.Get("/cruises/{id}/enrollments", cruiseEnrollH.List)

					orgCrewH := handlers.NewOrgCrewHandler(q)
					r.Get("/crew", orgCrewH.List)
					r.Group(func(r chi.Router) {
						r.Use(middleware.RequireOrgRole("admin"))
						r.Post("/crew", orgCrewH.Create)
						r.Put("/crew/{id}", orgCrewH.Update)
						r.Delete("/crew/{id}", orgCrewH.Delete)
					})
					r.Get("/crew/{id}", orgCrewH.Get)

					orgDashH := handlers.NewOrgDashboardHandler(q)
					r.Get("/dashboard", orgDashH.Get)
				})
			})
		})
	})

	return r
}
