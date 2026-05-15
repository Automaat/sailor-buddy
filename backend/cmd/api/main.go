package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/marcinskalski/sailor-buddy/backend/internal/api"
	"github.com/marcinskalski/sailor-buddy/backend/internal/auth"
	"github.com/marcinskalski/sailor-buddy/backend/internal/config"
	"github.com/marcinskalski/sailor-buddy/backend/internal/db"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.Load()

	database, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.Migrate(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	ctx := context.Background()
	fbClient, err := auth.NewFirebaseAuth(ctx, cfg.FirebaseProjectID)
	if err != nil {
		return fmt.Errorf("init firebase auth: %w", err)
	}

	router := api.NewRouter(database, cfg, fbClient)

	log.Printf("listening on %s", cfg.ListenAddr)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}
