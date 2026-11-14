package config

import (
	"errors"
	"fmt"
	"log/slog"
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

// ErrWildcardCORSOrigin rejects a CORS_ALLOWED_ORIGINS entry that matches
// origins the API does not name. go-chi/cors reads any "*" as a wildcard, so
// "*", "https://*" and "http*" all allow arbitrary origins.
var ErrWildcardCORSOrigin = errors.New(`CORS_ALLOWED_ORIGINS: a wildcard is allowed only as a subdomain of a named domain, e.g. https://*.example.com`)

// Validate reports settings the API server must not start with.
func (c *Config) Validate() error {
	for _, o := range c.CORSAllowedOrigins {
		if !strings.Contains(o, "*") {
			continue
		}
		if !isSubdomainWildcard(o) {
			return fmt.Errorf("%w: %q", ErrWildcardCORSOrigin, o)
		}
	}
	return nil
}

// isSubdomainWildcard reports whether o has the form scheme://*.domain, where
// domain holds no other wildcard, path or port-only part and has at least two
// labels, so "https://*.com" is refused.
func isSubdomainWildcard(o string) bool {
	scheme, domain, ok := strings.Cut(o, "://*.")
	if !ok || (scheme != "http" && scheme != "https") {
		return false
	}
	host, _, _ := strings.Cut(domain, ":")
	if strings.ContainsAny(domain, "*/") || !strings.Contains(host, ".") {
		return false
	}
	for label := range strings.SplitSeq(host, ".") {
		if label == "" {
			return false
		}
	}
	return true
}

func loadCORSOrigins() []string {
	const fallback = "http://localhost:5173"
	v := os.Getenv("CORS_ALLOWED_ORIGINS")
	if v == "" {
		slog.Warn("CORS_ALLOWED_ORIGINS not set, defaulting to localhost", "origins", fallback)
		return []string{fallback}
	}
	parts := strings.Split(v, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if o := strings.TrimSpace(p); o != "" {
			origins = append(origins, o)
		}
	}
	if len(origins) == 0 {
		slog.Warn("CORS_ALLOWED_ORIGINS blank after trimming, defaulting to localhost", "origins", fallback)
		return []string{fallback}
	}
	return origins
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
