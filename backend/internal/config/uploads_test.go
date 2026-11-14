package config

import "testing"

func TestUploadLimits_RejectInvalidEnvironment(t *testing.T) {
	for _, key := range []string{"SAILOR_UPLOAD_MAX_BYTES", "SAILOR_UPLOAD_MAX_FILES", "SAILOR_UPLOAD_REQUESTS_PER_MINUTE"} {
		for _, value := range []string{"", "0", "-1", "unlimited", "9223372036854775808"} {
			t.Run(key+"="+value, func(t *testing.T) {
				t.Setenv(key, value)
				t.Setenv("SAILOR_DATABASE_URL", "postgres://example/test")
				if err := Load().Validate(); err == nil {
					t.Fatal("accepted invalid upload limit")
				}
			})
		}
	}
}

func TestUploadLimits_LoadFiniteOverrides(t *testing.T) {
	t.Setenv("SAILOR_UPLOAD_MAX_BYTES", "1048576")
	t.Setenv("SAILOR_UPLOAD_MAX_FILES", "12")
	t.Setenv("SAILOR_UPLOAD_REQUESTS_PER_MINUTE", "7")
	got := Load().UploadLimits
	if *got != (UploadLimits{Bytes: 1048576, Files: 12, Requests: 7}) {
		t.Fatalf("limits: %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
}
