package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"
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

// ErrWildcardCORSOrigin rejects a CORS_ALLOWED_ORIGINS entry whose wildcard
// can match origins the club does not control. go-chi/cors lowercases each
// entry and reads its first "*" as any text, so "*", "https://*" and
// "https://*.github.io" all allow origins anyone can serve.
var ErrWildcardCORSOrigin = errors.New(`CORS_ALLOWED_ORIGINS: a wildcard is allowed only in the first label of a host under a registrable domain (https://*.example.com, https://app-*.example.com) or as the port of a named host (http://localhost:*)`)

// ErrNullCORSOrigin rejects a CORS_ALLOWED_ORIGINS entry of "null". Browsers
// send "Origin: null" from sandboxed iframes, data: URLs and file: pages, so
// any site can claim it.
var ErrNullCORSOrigin = errors.New(`CORS_ALLOWED_ORIGINS: "null" is not an allowed origin; any site can send it from a sandboxed iframe or a data: URL`)

// Validate reports settings the API server must not start with.
func (c *Config) Validate() error {
	for _, o := range c.CORSAllowedOrigins {
		if strings.EqualFold(strings.TrimSpace(o), "null") {
			return fmt.Errorf("%w: %q", ErrNullCORSOrigin, o)
		}
		if strings.Contains(o, "*") && !isSafeWildcard(strings.ToLower(o)) {
			return fmt.Errorf("%w: %q", ErrWildcardCORSOrigin, o)
		}
	}
	return nil
}

// isSafeWildcard reports whether the lowercase origin o, an http(s) origin
// with exactly one "*", matches only origins of one owner. It accepts two
// forms:
//   - scheme://host:* matches any port of a named host;
//   - scheme://a*b.domain[:port] holds the "*" in the first host label, and
//     domain must be a registrable domain, not a public suffix, so
//     "https://*.co.uk" and "https://*.github.io" are refused.
func isSafeWildcard(o string) bool {
	scheme, hostport, ok := strings.Cut(o, "://")
	if !ok || (scheme != "http" && scheme != "https") ||
		strings.Count(hostport, "*") != 1 || strings.ContainsAny(hostport, `/\?#@`) {
		return false
	}
	if host, ok := strings.CutSuffix(hostport, ":*"); ok {
		return isNamedHost(host)
	}
	host, port, hasPort := strings.Cut(hostport, ":")
	if hasPort {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return false
		}
	}
	first, domain, ok := strings.Cut(host, ".")
	if !ok || !strings.Contains(first, "*") {
		return false
	}
	return isRegistrableDomain(domain)
}

// isNamedHost reports whether host is a host name, an IPv4 address or a
// bracketed IPv6 address, holding no wildcard.
func isNamedHost(host string) bool {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return len(host) > 2 && !strings.Contains(host, "*")
	}
	return host != "" && !strings.ContainsAny(host, ":*")
}

// isRegistrableDomain reports whether domain has a public suffix and at
// least one label of its own, and does not end in a numeric label, so an
// IP address tail such as "0.0.1" is refused.
func isRegistrableDomain(domain string) bool {
	if _, err := publicsuffix.EffectiveTLDPlusOne(domain); err != nil {
		return false
	}
	tld := domain[strings.LastIndexByte(domain, '.')+1:]
	_, err := strconv.Atoi(tld)
	return err != nil
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
