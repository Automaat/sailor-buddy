package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPDFRendererHint checks which setting the startup log blames when the
// Chromium sandbox cannot start, against a fake procfs.
func TestPDFRendererHint(t *testing.T) {
	const (
		apparmorHint = "apparmor=unconfined"
		seccompHint  = "seccomp=backend/seccomp-chromium.json"
	)
	tests := []struct {
		name     string
		files    map[string]string
		wantHint string
	}{
		{
			name: "unconfined on restricted host",
			files: map[string]string{
				"self/attr/apparmor/current":                       "unconfined\n",
				"sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
			},
			wantHint: apparmorHint,
		},
		{
			name: "legacy label path, NUL-terminated",
			files: map[string]string{
				"self/attr/current": "unconfined\x00",
				"sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
			},
			wantHint: apparmorHint,
		},
		{
			name: "docker-default on restricted host",
			files: map[string]string{
				"self/attr/apparmor/current":                       "docker-default (enforce)\n",
				"sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
			},
			wantHint: seccompHint,
		},
		{
			name: "unconfined on unrestricted host",
			files: map[string]string{
				"self/attr/apparmor/current":                       "unconfined\n",
				"sys/kernel/apparmor_restrict_unprivileged_userns": "0\n",
			},
			wantHint: seccompHint,
		},
		{
			name: "no AppArmor",
			files: map[string]string{
				"self/attr/current": "",
			},
			wantHint: seccompHint,
		},
		{
			name:     "procfs unreadable",
			files:    nil,
			wantHint: seccompHint,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := pdfRendererHint(dir); !strings.Contains(got, tt.wantHint) {
				t.Fatalf("pdfRendererHint() = %q, want it to mention %q", got, tt.wantHint)
			}
		})
	}
}
