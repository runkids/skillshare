package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLocalSkillVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "reads metadata.version",
			content: `---
name: skillshare
description: test
metadata:
  version: v0.17.11
---
# Skillshare Skill`,
			expected: "0.17.11",
		},
		{
			name: "returns empty when no version field",
			content: `---
name: skillshare
---
# Skillshare Skill`,
			expected: "",
		},
		{
			name:     "returns empty for empty file",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create source/skillshare/SKILL.md structure
			sourceDir := t.TempDir()
			skillDir := filepath.Join(sourceDir, "skillshare")
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadLocalSkillVersion(sourceDir)
			if got != tt.expected {
				t.Errorf("ReadLocalSkillVersion() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadLocalSkillVersion_NoSkillDir(t *testing.T) {
	sourceDir := t.TempDir()
	got := ReadLocalSkillVersion(sourceDir)
	if got != "" {
		t.Errorf("expected empty string for missing skillshare dir, got %q", got)
	}
}

func TestSkillOutdated(t *testing.T) {
	tests := []struct {
		local, remote string
		want          bool
	}{
		{"0.23.4", "0.23.5", true},
		{"0.23.5", "0.23.5", false},
		{"0.24.0", "0.23.5", false}, // a local skill ahead of main is not outdated
		{"0.23.5", "", false},       // offline
		{"", "0.23.5", false},
		{"custom", "0.23.5", false},
	}
	for _, tt := range tests {
		if got := SkillOutdated(tt.local, tt.remote); got != tt.want {
			t.Errorf("SkillOutdated(%q, %q) = %v, want %v", tt.local, tt.remote, got, tt.want)
		}
	}
}

func TestCachedRemoteSkillVersion_FreshCacheSkipsFetch(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	// A version GitHub will never serve, so a fetch would be visible.
	SaveRemoteSkillVersion("9.9.9")

	if got := CachedRemoteSkillVersion(); got != "9.9.9" {
		t.Fatalf("CachedRemoteSkillVersion() = %q, want the cached 9.9.9", got)
	}
}
