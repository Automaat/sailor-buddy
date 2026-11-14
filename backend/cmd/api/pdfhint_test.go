package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPDFRendererHint checks which settings the startup log blames when
// Chromium cannot start, against a fake procfs. An unconfined process on a
// restricted host gets both hints: Docker's default seccomp profile blocks the
// namespace before AppArmor sees it.
func TestPDFRendererHint(t *testing.T) {
	const (
		apparmorHint   = "apparmor=unconfined"
		seccompHint    = "seccomp=backend/seccomp-chromium.json"
		sandboxOffHint = "SAILOR_CHROME_NO_SANDBOX"
		restrictPath   = "sys/kernel/apparmor_restrict_unprivileged_userns"
	)
	restrictedUnconfined := map[string]string{
		"self/attr/apparmor/current": "unconfined\n",
		restrictPath:                 "1\n",
	}
	tests := []struct {
		name       string
		files      map[string]string
		sandboxOff bool
		want       []string
		notWant    []string
	}{
		{
			name:  "unconfined on restricted host",
			files: restrictedUnconfined,
			want:  []string{apparmorHint, seccompHint},
		},
		{
			name: "legacy label path, NUL-terminated",
			files: map[string]string{
				"self/attr/current": "unconfined\x00",
				restrictPath:        "1\n",
			},
			want: []string{apparmorHint, seccompHint},
		},
		{
			name: "docker-default on restricted host",
			files: map[string]string{
				"self/attr/apparmor/current": "docker-default (enforce)\n",
				restrictPath:                 "1\n",
			},
			want:    []string{seccompHint},
			notWant: []string{apparmorHint},
		},
		{
			name: "unconfined on unrestricted host",
			files: map[string]string{
				"self/attr/apparmor/current": "unconfined\n",
				restrictPath:                 "0\n",
			},
			want:    []string{seccompHint},
			notWant: []string{apparmorHint},
		},
		{
			name:    "no AppArmor",
			files:   map[string]string{"self/attr/current": ""},
			want:    []string{seccompHint},
			notWant: []string{apparmorHint},
		},
		{
			name:    "procfs unreadable",
			want:    []string{seccompHint},
			notWant: []string{apparmorHint},
		},
		{
			name:       "sandbox off, unconfined on restricted host",
			files:      restrictedUnconfined,
			sandboxOff: true,
			want:       []string{sandboxOffHint},
			notWant:    []string{apparmorHint, seccompHint},
		},
		{
			name:       "sandbox off, no AppArmor",
			sandboxOff: true,
			want:       []string{sandboxOffHint},
			notWant:    []string{apparmorHint, seccompHint},
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
			got := pdfRendererHint(dir, tt.sandboxOff)
			for _, s := range tt.want {
				if !strings.Contains(got, s) {
					t.Errorf("pdfRendererHint() = %q, want it to mention %q", got, s)
				}
			}
			for _, s := range tt.notWant {
				if strings.Contains(got, s) {
					t.Errorf("pdfRendererHint() = %q, must not mention %q", got, s)
				}
			}
		})
	}
}
