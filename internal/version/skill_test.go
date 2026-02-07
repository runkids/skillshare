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
			name: "reads version from SKILL.md",
			content: `---
name: skillshare
version: 0.4.5
---
# Skillshare Skill`,
			expected: "0.4.5",
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
