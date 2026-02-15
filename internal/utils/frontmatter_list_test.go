package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatterList_InlineFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [claude, cursor]\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 2 || result[0] != "claude" || result[1] != "cursor" {
		t.Errorf("got %v, want [claude cursor]", result)
	}
}

func TestParseFrontmatterList_BlockFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets:\n  - claude\n  - cursor\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 2 || result[0] != "claude" || result[1] != "cursor" {
		t.Errorf("got %v, want [claude cursor]", result)
	}
}

func TestParseFrontmatterList_FieldAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}

func TestParseFrontmatterList_EmptyList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: []\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}

func TestParseFrontmatterList_FileNotFound(t *testing.T) {
	result := ParseFrontmatterList("/nonexistent/SKILL.md", "targets")
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}

func TestParseFrontmatterList_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("# Just content, no frontmatter"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("got %v, want nil", result)
	}
}
