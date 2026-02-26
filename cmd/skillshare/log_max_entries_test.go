package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestLogMaxEntries(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		want       int
	}{
		{
			name:       "not set uses default 1000",
			configYAML: "source: /tmp/skills\n",
			want:       config.DefaultLogMaxEntries,
		},
		{
			name:       "explicit positive value",
			configYAML: "source: /tmp/skills\nlog:\n  max_entries: 500\n",
			want:       500,
		},
		{
			name:       "explicit zero means unlimited",
			configYAML: "source: /tmp/skills\nlog:\n  max_entries: 0\n",
			want:       0,
		},
		{
			name:       "negative value falls back to default",
			configYAML: "source: /tmp/skills\nlog:\n  max_entries: -5\n",
			want:       config.DefaultLogMaxEntries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			os.WriteFile(cfgPath, []byte(tt.configYAML), 0o644)

			t.Setenv("SKILLSHARE_CONFIG", cfgPath)

			got := logMaxEntries()
			if got != tt.want {
				t.Errorf("logMaxEntries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLogMaxEntries_MissingConfig(t *testing.T) {
	t.Setenv("SKILLSHARE_CONFIG", "/nonexistent/config.yaml")
	got := logMaxEntries()
	if got != config.DefaultLogMaxEntries {
		t.Errorf("logMaxEntries() = %d, want default %d", got, config.DefaultLogMaxEntries)
	}
}
