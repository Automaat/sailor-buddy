package config

import (
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	ListenAddr         string
	UploadDir          string
	FirebaseProjectID  string
	CORSAllowedOrigins []string
}

func Load() *Config {
	return &Config{
		DatabaseURL:        getenv("SAILOR_DATABASE_URL", "postgres://sailor:sailor@localhost:5432/sailor?sslmode=disable"),
		ListenAddr:         getenv("SAILOR_LISTEN_ADDR", ":8080"),
		UploadDir:          getenv("SAILOR_UPLOAD_DIR", "uploads"),
		FirebaseProjectID:  getenv("SAILOR_FIREBASE_PROJECT_ID", "sailor-buddy-dev"),
		CORSAllowedOrigins: loadCORSOrigins(),
	}
}

func loadCORSOrigins() []string {
	v := os.Getenv("CORS_ALLOWED_ORIGINS")
	if v == "" {
		return []string{"http://localhost:5173"}
	}
	parts := strings.Split(v, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if o := strings.TrimSpace(p); o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:5173"}
	}
	return origins
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
