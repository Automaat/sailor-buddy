package config

import (
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
		"DatabaseURL":       {cfg.DatabaseURL, "postgres://sailor:sailor@localhost:5432/sailor?sslmode=disable"},
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
