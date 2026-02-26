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
		{
			name: "handles folded block scalar >-",
			content: `---
name: my-skill
description: >-
  Verify and fix ASCII box-drawing
  diagram alignment in markdown files.
---
# Content`,
			field:    "description",
			expected: "Verify and fix ASCII box-drawing diagram alignment in markdown files.",
		},
		{
			name: "handles folded block scalar >",
			content: `---
description: >
  First line
  second line
---`,
			field:    "description",
			expected: "First line second line",
		},
		{
			name: "handles literal block scalar |",
			content: `---
description: |
  Line one
  Line two
---`,
			field:    "description",
			expected: "Line one Line two",
		},
		{
			name: "handles block scalar followed by another field",
			content: `---
description: >-
  Multi-line description
  goes here.
version: 1.0.0
---`,
			field:    "description",
			expected: "Multi-line description goes here.",
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

func TestReadSkillBody(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "returns body after frontmatter",
			content: `---
name: my-skill
description: A skill
---
# My Skill

This is the body content.`,
			expected: "# My Skill\n\nThis is the body content.",
		},
		{
			name:     "returns full content when no frontmatter",
			content:  "# Just Content\n\nNo frontmatter here.",
			expected: "# Just Content\n\nNo frontmatter here.",
		},
		{
			name:     "returns empty for empty file",
			content:  "",
			expected: "",
		},
		{
			name: "returns empty when frontmatter has no closing delimiter",
			content: `---
name: broken
no closing`,
			expected: "",
		},
		{
			name: "returns empty body when nothing after frontmatter",
			content: `---
name: my-skill
---`,
			expected: "",
		},
		{
			name: "preserves multiline body content",
			content: `---
name: my-skill
---
Line 1
Line 2

Line 4`,
			expected: "Line 1\nLine 2\n\nLine 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			filePath := filepath.Join(dir, "SKILL.md")
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadSkillBody(filePath)
			if got != tt.expected {
				t.Errorf("ReadSkillBody() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadSkillBody_FileNotExist(t *testing.T) {
	got := ReadSkillBody("/nonexistent/path/SKILL.md")
	if got != "" {
		t.Errorf("expected empty string for non-existent file, got %q", got)
	}
}

func TestParseFrontmatterFields(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		fields   []string
		expected map[string]string
	}{
		{
			name: "extracts multiple fields at once",
			content: `---
name: my-skill
description: A great skill
license: MIT
---
# Content`,
			fields:   []string{"description", "license"},
			expected: map[string]string{"description": "A great skill", "license": "MIT"},
		},
		{
			name: "missing fields are omitted from result",
			content: `---
name: my-skill
---
# Content`,
			fields:   []string{"description", "license"},
			expected: map[string]string{},
		},
		{
			name: "partial match returns found fields only",
			content: `---
name: my-skill
license: Apache-2.0
---`,
			fields:   []string{"description", "license"},
			expected: map[string]string{"license": "Apache-2.0"},
		},
		{
			name:     "empty fields list returns empty map",
			content:  "---\nname: x\n---",
			fields:   []string{},
			expected: map[string]string{},
		},
		{
			name:     "non-existent file returns empty map",
			content:  "", // will use non-existent path
			fields:   []string{"name"},
			expected: map[string]string{},
		},
		{
			name: "numeric and boolean values are converted",
			content: `---
version: 2
enabled: true
---`,
			fields:   []string{"version", "enabled"},
			expected: map[string]string{"version": "2", "enabled": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			if tt.name == "non-existent file returns empty map" {
				filePath = "/nonexistent/path/SKILL.md"
			} else {
				dir := t.TempDir()
				filePath = filepath.Join(dir, "SKILL.md")
				if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := ParseFrontmatterFields(filePath, tt.fields)
			if len(got) != len(tt.expected) {
				t.Errorf("ParseFrontmatterFields() returned %d fields, want %d: got=%v", len(got), len(tt.expected), got)
				return
			}
			for k, want := range tt.expected {
				if got[k] != want {
					t.Errorf("ParseFrontmatterFields()[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}
