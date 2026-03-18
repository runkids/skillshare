package skill

import (
	"strings"
	"testing"
)

func TestFindPattern_Found(t *testing.T) {
	p := FindPattern("reviewer")
	if p == nil {
		t.Fatal("expected to find reviewer pattern")
	}
	if p.Name != "reviewer" {
		t.Errorf("expected Name=reviewer, got %s", p.Name)
	}
	if len(p.ScaffoldDirs) == 0 {
		t.Error("reviewer should have scaffold dirs")
	}
}

func TestFindPattern_NotFound(t *testing.T) {
	if FindPattern("nonexistent") != nil {
		t.Error("expected nil for unknown pattern")
	}
}

func TestGenerateContent_NonePattern(t *testing.T) {
	content := GenerateContent("my-skill", "none", "")
	if strings.Contains(content, "pattern:") {
		t.Error("none pattern should not have pattern field in frontmatter")
	}
	if !strings.Contains(content, "name: my-skill") {
		t.Error("should contain skill name in frontmatter")
	}
	if !strings.Contains(content, "# My Skill") {
		t.Error("should contain title heading")
	}
}

func TestGenerateContent_ReviewerWithCategory(t *testing.T) {
	content := GenerateContent("code-review", "reviewer", "quality")
	if !strings.Contains(content, "pattern: reviewer") {
		t.Error("should contain pattern field")
	}
	if !strings.Contains(content, "category: quality") {
		t.Error("should contain category field")
	}
	if !strings.Contains(content, "references/review-checklist.md") {
		t.Error("reviewer body should reference checklist")
	}
	if !strings.Contains(content, "name: code-review") {
		t.Error("should contain skill name in frontmatter")
	}
}

func TestGenerateContent_WithoutCategory(t *testing.T) {
	content := GenerateContent("my-tool", "tool-wrapper", "")
	if strings.Contains(content, "category:") {
		t.Error("should omit category field when empty")
	}
	if !strings.Contains(content, "pattern: tool-wrapper") {
		t.Error("should still have pattern field")
	}
}

func TestGenerateContent_AllPatterns(t *testing.T) {
	for _, p := range Patterns {
		content := GenerateContent("test-skill", p.Name, "quality")
		if !strings.Contains(content, "name: test-skill") {
			t.Errorf("pattern %q: missing name in frontmatter", p.Name)
		}
		if content == "" {
			t.Errorf("pattern %q: generated empty content", p.Name)
		}
	}
}

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-skill", "My Skill"},
		{"api", "Api"},
		{"code-review-tool", "Code Review Tool"},
		{"a", "A"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ToTitleCase(tt.input)
		if got != tt.want {
			t.Errorf("ToTitleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidNameRe(t *testing.T) {
	valid := []string{"my-skill", "tool_wrapper", "_private", "a", "abc123"}
	for _, name := range valid {
		if !ValidNameRe.MatchString(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	invalid := []string{"My-Skill", "123abc", "-start", "", "has space"}
	for _, name := range invalid {
		if ValidNameRe.MatchString(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
