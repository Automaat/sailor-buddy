package config

import "testing"

func TestProductionConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		valid bool
	}{
		{"valid production", "", "", true},
		{"unknown mode", "SAILOR_ENV", "prod", false},
		{"emulator", "FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099", false},
		{"default project", "SAILOR_FIREBASE_PROJECT_ID", "", false},
		{"demo project", "SAILOR_FIREBASE_PROJECT_ID", "demo-club", false},
		{"sandbox disabled", "SAILOR_CHROME_NO_SANDBOX", "true", false},
		{"implicit origins", "CORS_ALLOWED_ORIGINS", "", false},
		{"HTTP origin", "CORS_ALLOWED_ORIGINS", "http://club.example.com", false},
		{"userinfo", "CORS_ALLOWED_ORIGINS", "https://user@club.example.com", false},
		{"path", "CORS_ALLOWED_ORIGINS", "https://club.example.com/x", false},
		{"query", "CORS_ALLOWED_ORIGINS", "https://club.example.com?x", false},
		{"fragment", "CORS_ALLOWED_ORIGINS", "https://club.example.com#x", false},
		{"owned wildcard", "CORS_ALLOWED_ORIGINS", "https://*.example.com", true},
		{"development emulator", "SAILOR_ENV", "development", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SAILOR_ENV", "production")
			t.Setenv("SAILOR_DATABASE_URL", "postgres://private.example/club")
			t.Setenv("SAILOR_FIREBASE_PROJECT_ID", "sailor-club")
			t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
			t.Setenv("SAILOR_CHROME_NO_SANDBOX", "false")
			t.Setenv("CORS_ALLOWED_ORIGINS", "https://club.example.com")
			if tc.key != "" {
				t.Setenv(tc.key, tc.value)
			}
			if tc.name == "development emulator" {
				t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "localhost:9099")
			}
			if err := Load().Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, valid=%v", err, tc.valid)
			}
		})
	}
}
