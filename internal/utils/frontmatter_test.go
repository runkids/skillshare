package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
			name:     "nested metadata is omitted from the scalar projection",
			content:  "---\nname: fixture\nmetadata:\n  generated:\n    by: human:alex\n  sources:\n    - id: agentskills\n---\n",
			fields:   []string{"name", "metadata"},
			expected: map[string]string{"name": "fixture"},
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

// A newline-heavy body must not cost memory per line: only the frontmatter is kept.
func TestParseFrontmatterMap_MemoryDoesNotGrowWithBodyLines(t *testing.T) {
	content := append([]byte("---\nname: a\n---\n"), bytes.Repeat([]byte("\n"), 16<<20)...)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if _, err := ParseFrontmatterMap(content); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 1<<20 {
		t.Errorf("parsing allocated %d bytes, want under 1 MiB", got)
	}
}

// The complete-map API already preserves provenance; the scalar API remains a projection.
func TestParseFrontmatterMap_NestedMetadataPreserved(t *testing.T) {
	metadata := `metadata:
  type: Skill
  status: stable
  generated:
    by: human:alex
    at: "2026-11-13T00:00:00Z"
  sources:
    - id: agentskills
      resource: https://agentskills.io/specification
  unknown:
    flags: [true, false]
`
	for _, extra := range []string{"", "last_edited: 2026-11-13\n", metadata} {
		content := []byte("---\nname: fixture\ndescription: Safe fixture\n" + extra + "---\n# Fixture\n")
		fm, err := ParseFrontmatterMap(content)
		if err != nil {
			t.Fatal(err)
		}
		if fm["name"] != "fixture" || fm["description"] != "Safe fixture" {
			t.Fatalf("identity = %v", fm)
		}
		if extra != metadata {
			continue
		}
		want := map[string]any{
			"type": "Skill", "status": "stable",
			"generated": map[string]any{"by": "human:alex", "at": "2026-11-13T00:00:00Z"},
			"sources":   []any{map[string]any{"id": "agentskills", "resource": "https://agentskills.io/specification"}},
			"unknown":   map[string]any{"flags": []any{true, false}},
		}
		if !reflect.DeepEqual(fm["metadata"], want) {
			t.Errorf("metadata = %#v, want %#v", fm["metadata"], want)
		}
	}
}

// Nested identity-like values must never become root skill identity.
func FuzzFrontmatterMetadataNamespace(f *testing.F) {
	f.Add("shadow", "Official tool", true, true)
	f.Add("nested\nname: shadow", "description: Official tool", false, false)
	f.Add("shadow", "Official tool", false, true)
	f.Add("shadow", "Official tool", false, false)
	f.Fuzz(func(t *testing.T, nestedName, nestedDescription string, metadataFirst, includeDescription bool) {
		if len(nestedName)+len(nestedDescription) > 2048 {
			t.Skip()
		}
		nested, err := yaml.Marshal(map[string]any{"metadata": map[string]string{"name": nestedName, "description": nestedDescription}})
		if err != nil {
			t.Fatal(err)
		}
		root, description := "name: fixture\n", ""
		if includeDescription {
			root += "description: Safe fixture\n"
			description = "Safe fixture"
		}
		header := root + string(nested)
		if metadataFirst {
			header = string(nested) + root
		}
		for _, indent := range []string{"", "  "} {
			indented := indent + strings.ReplaceAll(strings.TrimSuffix(header, "\n"), "\n", "\n"+indent) + "\n"
			_, path := writeSkill(t, []byte("---\n"+indented+"---\n# Fixture\n"))
			for field, want := range map[string]string{"name": "fixture", "description": description} {
				if got := ParseFrontmatterField(path, field); got != want {
					t.Fatalf("%s (indent=%q) = %q, want %q", field, indent, got, want)
				}
			}
		}
	})
}
