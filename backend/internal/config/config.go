package config

import "os"

type Config struct {
	DatabaseURL       string
	ListenAddr        string
	UploadDir         string
	FirebaseProjectID string
}

func Load() *Config {
	return &Config{
		DatabaseURL:       getenv("SAILOR_DATABASE_URL", "postgres://sailor:sailor@localhost:5432/sailor?sslmode=disable"),
		ListenAddr:        getenv("SAILOR_LISTEN_ADDR", ":8080"),
		UploadDir:         getenv("SAILOR_UPLOAD_DIR", "uploads"),
		FirebaseProjectID: getenv("SAILOR_FIREBASE_PROJECT_ID", "sailor-buddy-dev"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
