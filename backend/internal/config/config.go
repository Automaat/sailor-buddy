package config

import "os"

type Config struct {
	DBPath     string
	ListenAddr string
	JWTSecret  string
	UploadDir  string
}

func Load() *Config {
	return &Config{
		DBPath:     getenv("SAILOR_DB_PATH", "sailor.db"),
		ListenAddr: getenv("SAILOR_LISTEN_ADDR", ":8080"),
		JWTSecret:  getenv("SAILOR_JWT_SECRET", "dev-secret-change-me"),
		UploadDir:  getenv("SAILOR_UPLOAD_DIR", "uploads"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
