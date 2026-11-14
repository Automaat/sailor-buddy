package config

import (
	"errors"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("SAILOR_DATABASE_URL", "")
	t.Setenv("SAILOR_LISTEN_ADDR", "")
	t.Setenv("SAILOR_UPLOAD_DIR", "")
	t.Setenv("SAILOR_FIREBASE_PROJECT_ID", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	cfg := Load()

	defaults := map[string]struct {
		got  string
		want string
	}{
		"DatabaseURL":       {cfg.DatabaseURL, ""},
		"ListenAddr":        {cfg.ListenAddr, ":8080"},
		"UploadDir":         {cfg.UploadDir, "uploads"},
		"FirebaseProjectID": {cfg.FirebaseProjectID, "sailor-buddy-dev"},
	}

	for name, tc := range defaults {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got, tc.want)
		}
	}

	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSAllowedOrigins = %v, want [http://localhost:5173]", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	tests := []struct {
		envKey string
		value  string
		field  func(*Config) string
	}{
		{"SAILOR_DATABASE_URL", "postgres://custom:5432/db", func(c *Config) string { return c.DatabaseURL }},
		{"SAILOR_LISTEN_ADDR", ":9090", func(c *Config) string { return c.ListenAddr }},
		{"SAILOR_UPLOAD_DIR", "/tmp/uploads", func(c *Config) string { return c.UploadDir }},
		{"SAILOR_FIREBASE_PROJECT_ID", "my-project", func(c *Config) string { return c.FirebaseProjectID }},
	}

	for _, tc := range tests {
		t.Run(tc.envKey, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.value)
			cfg := Load()
			if got := tc.field(cfg); got != tc.value {
				t.Errorf("%s: got %q, want %q", tc.envKey, got, tc.value)
			}
		})
	}
}

func TestLoadCORSOrigins(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{"empty falls back to localhost", "", []string{"http://localhost:5173"}},
		{"single origin", "https://app.example.com", []string{"https://app.example.com"}},
		{"multiple origins", "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
		{"trims whitespace", " https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
		{"blank-only falls back to localhost", "   ", []string{"http://localhost:5173"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tc.env)
			cfg := Load()
			if len(cfg.CORSAllowedOrigins) != len(tc.want) {
				t.Fatalf("got %v, want %v", cfg.CORSAllowedOrigins, tc.want)
			}
			for i, o := range tc.want {
				if cfg.CORSAllowedOrigins[i] != o {
					t.Errorf("[%d] got %q, want %q", i, cfg.CORSAllowedOrigins[i], o)
				}
			}
		})
	}
}

func TestGetenv(t *testing.T) {
	t.Run("returns fallback when unset", func(t *testing.T) {
		got := getenv("SAILOR_TEST_UNSET_KEY", "fallback")
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("SAILOR_TEST_KEY", "override")
		got := getenv("SAILOR_TEST_KEY", "fallback")
		if got != "override" {
			t.Errorf("got %q, want %q", got, "override")
		}
	})

	t.Run("returns fallback when empty", func(t *testing.T) {
		t.Setenv("SAILOR_TEST_EMPTY", "")
		got := getenv("SAILOR_TEST_EMPTY", "fallback")
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})
}

func TestValidate_CORSOrigins(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr error
	}{
		{"default localhost", "", nil},
		{"listed origins", "https://a.com,https://b.com", nil},
		{"subdomain pattern", "https://*.example.com", nil},
		{"wildcard", "*", ErrWildcardCORSOrigin},
		{"wildcard among origins", "https://a.com, *", ErrWildcardCORSOrigin},
		{"scheme wildcard", "https://*", ErrWildcardCORSOrigin},
		{"prefix wildcard", "http*", ErrWildcardCORSOrigin},
		{"top-level domain wildcard", "https://*.com", ErrWildcardCORSOrigin},
		{"wildcard in domain", "https://*.*.example.com", ErrWildcardCORSOrigin},
		{"wildcard with path", "https://*.example.com/x", ErrWildcardCORSOrigin},
		{"wildcard after host", "https://example.com*", ErrWildcardCORSOrigin},
		{"non-http scheme", "ftp://*.example.com", ErrWildcardCORSOrigin},
		{"empty label", "https://*.example..com", ErrWildcardCORSOrigin},
		{"subdomain pattern with port", "https://*.example.com:8443", nil},
		{"upper-case subdomain pattern", "HTTPS://*.Example.COM", nil},
		{"deeper subdomain pattern", "https://*.app.example.com", nil},
		{"subdomain of a private suffix owner", "https://*.club.github.io", nil},
		{"wildcard inside first label", "https://app-*.example.com", nil},
		{"any port of localhost", "http://localhost:*", nil},
		{"any port of a named host", "https://api.example.com:*", nil},
		{"any port of an IPv4 host", "http://127.0.0.1:*", nil},
		{"any port of an IPv6 host", "http://[::1]:*", nil},
		{"public suffix wildcard", "https://*.co.uk", ErrWildcardCORSOrigin},
		{"private suffix wildcard", "https://*.github.io", ErrWildcardCORSOrigin},
		{"hosting suffix wildcard", "https://*.vercel.app", ErrWildcardCORSOrigin},
		{"upper-case public suffix wildcard", "HTTPS://*.GitHub.IO", ErrWildcardCORSOrigin},
		{"wildcard label under a public suffix", "https://app-*.github.io", ErrWildcardCORSOrigin},
		{"wildcard in second label", "https://app.*.example.com", ErrWildcardCORSOrigin},
		{"single-label domain", "https://*.localhost", ErrWildcardCORSOrigin},
		{"numeric tail", "https://*.0.0.1", ErrWildcardCORSOrigin},
		{"wildcard host with any port", "https://*.example.com:*", ErrWildcardCORSOrigin},
		{"any port without host", "http://:*", ErrWildcardCORSOrigin},
		{"any port after a port", "http://localhost:5173:*", ErrWildcardCORSOrigin},
		{"wildcard inside port", "http://localhost:80*", ErrWildcardCORSOrigin},
		{"non-numeric port", "https://*.example.com:https", ErrWildcardCORSOrigin},
		{"wildcard with userinfo", "https://user@*.example.com", ErrWildcardCORSOrigin},
		{"wildcard with query", "https://*.example.com?x", ErrWildcardCORSOrigin},
		{"null origin", "null", ErrNullCORSOrigin},
		{"upper-case null origin", "NULL", ErrNullCORSOrigin},
		{"null among origins", "https://a.com, Null", ErrNullCORSOrigin},
		{"null as a host", "https://null.example.com", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", "postgres://custom:5432/db")
			t.Setenv("CORS_ALLOWED_ORIGINS", tc.env)
			err := Load().Validate()
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRequireDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    string
		wantErr error
	}{
		{"unset", "", "", ErrDatabaseURLUnset},
		{"blank", "   ", "", ErrDatabaseURLUnset},
		{"set", "postgres://custom:5432/db", "postgres://custom:5432/db", nil},
		{"trims whitespace", " postgres://custom:5432/db ", "postgres://custom:5432/db", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", tc.env)
			got, err := RequireDatabaseURL()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RequireDatabaseURL() error = %v, want %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("RequireDatabaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidate_DatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr error
	}{
		{"unset", "", ErrDatabaseURLUnset},
		{"blank", "  ", ErrDatabaseURLUnset},
		{"set", "postgres://custom:5432/db", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_DATABASE_URL", tc.env)
			t.Setenv("CORS_ALLOWED_ORIGINS", "")
			if err := Load().Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
