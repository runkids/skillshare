package main

import (
	"strings"
	"testing"
)

func TestPatternDefinitions(t *testing.T) {
	if len(skillPatterns) != 6 {
		t.Errorf("expected 6 patterns, got %d", len(skillPatterns))
	}
	for _, p := range skillPatterns {
		if p.Name == "" {
			t.Error("pattern name must not be empty")
		}
		if p.Name != "none" && p.Description == "" {
			t.Errorf("pattern %q must have a description", p.Name)
		}
	}
}

func TestCategoryDefinitions(t *testing.T) {
	if len(skillCategories) != 9 {
		t.Errorf("expected 9 categories, got %d", len(skillCategories))
	}
	for _, c := range skillCategories {
		if c.Key == "" || c.Label == "" {
			t.Error("category key and label must not be empty")
		}
	}
}

func TestGeneratePatternTemplate_ToolWrapper(t *testing.T) {
	tmpl := generatePatternTemplate("my-api", "tool-wrapper", "library")
	if !strings.Contains(tmpl, "name: my-api") {
		t.Error("should contain skill name in frontmatter")
	}
	if !strings.Contains(tmpl, "pattern: tool-wrapper") {
		t.Error("should contain pattern field")
	}
	if !strings.Contains(tmpl, "category: library") {
		t.Error("should contain category field")
	}
	if !strings.Contains(tmpl, "references/conventions.md") {
		t.Error("tool-wrapper body should reference conventions.md")
	}
}

func TestGeneratePatternTemplate_NoCategoryOmitsField(t *testing.T) {
	tmpl := generatePatternTemplate("my-skill", "reviewer", "")
	if strings.Contains(tmpl, "category:") {
		t.Error("should omit category field when empty")
	}
	if !strings.Contains(tmpl, "pattern: reviewer") {
		t.Error("should still have pattern field")
	}
}

func TestGeneratePatternTemplate_NonePattern(t *testing.T) {
	tmpl := generatePatternTemplate("my-skill", "none", "")
	if strings.Contains(tmpl, "pattern:") {
		t.Error("none pattern should not have pattern field")
	}
}

func TestGeneratePatternTemplate_AllPatterns(t *testing.T) {
	for _, p := range skillPatterns {
		tmpl := generatePatternTemplate("test-skill", p.Name, "quality")
		if !strings.Contains(tmpl, "name: test-skill") {
			t.Errorf("pattern %q: missing name", p.Name)
		}
		if tmpl == "" {
			t.Errorf("pattern %q: empty template", p.Name)
		}
	}
}

func TestFindPattern(t *testing.T) {
	p := findPattern("reviewer")
	if p == nil {
		t.Fatal("should find reviewer pattern")
	}
	if p.Name != "reviewer" {
		t.Errorf("expected reviewer, got %s", p.Name)
	}

	if findPattern("nonexistent") != nil {
		t.Error("should return nil for unknown pattern")
	}
}
