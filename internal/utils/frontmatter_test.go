package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatterField(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		field    string
		expected string
	}{
		{
			name: "extracts name field",
			content: `---
name: my-skill
version: 1.0.0
---
# Content`,
			field:    "name",
			expected: "my-skill",
		},
		{
			name: "extracts version field",
			content: `---
name: my-skill
version: 2.3.1
---
# Content`,
			field:    "version",
			expected: "2.3.1",
		},
		{
			name: "handles quoted values",
			content: `---
name: "quoted-skill"
---
# Content`,
			field:    "name",
			expected: "quoted-skill",
		},
		{
			name: "handles single-quoted values",
			content: `---
name: 'single-quoted'
---
# Content`,
			field:    "name",
			expected: "single-quoted",
		},
		{
			name:     "returns empty for missing field",
			content:  "---\nname: my-skill\n---\n# Content",
			field:    "version",
			expected: "",
		},
		{
			name:     "returns empty for no frontmatter",
			content:  "# Just content\nNo frontmatter here",
			field:    "name",
			expected: "",
		},
		{
			name:     "returns empty for empty file",
			content:  "",
			field:    "name",
			expected: "",
		},
		{
			name: "handles extra spaces",
			content: `---
name:   spaced-value
---`,
			field:    "name",
			expected: "spaced-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ParseFrontmatterField(filePath, tt.field)
			if got != tt.expected {
				t.Errorf("ParseFrontmatterField(%q) = %q, want %q", tt.field, got, tt.expected)
			}
		})
	}
}

func TestParseFrontmatterField_FileNotExist(t *testing.T) {
	got := ParseFrontmatterField("/nonexistent/path/SKILL.md", "name")
	if got != "" {
		t.Errorf("expected empty string for non-existent file, got %q", got)
	}
}
