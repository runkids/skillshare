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

func TestParseFrontmatterList_MetadataTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\nmetadata:\n  targets: [claude, cursor]\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 2 || result[0] != "claude" || result[1] != "cursor" {
		t.Errorf("got %v, want [claude cursor]", result)
	}
}

func TestParseFrontmatterList_MetadataOverridesTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [claude, cursor]\nmetadata:\n  targets: [claude]\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("got %v, want [claude]", result)
	}
}

func TestParseFrontmatterList_MetadataExistsButNoField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [claude]\nmetadata:\n  pattern: tool-wrapper\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("got %v, want [claude] (fallback to top-level)", result)
	}
}

func TestParseFrontmatterList_MetadataNotAMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [claude]\nmetadata: just-a-string\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("got %v, want [claude] (fallback when metadata is not a map)", result)
	}
}

func TestParseFrontmatterList_MetadataEmptyTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [claude]\nmetadata:\n  targets: []\n---\n# Content"), 0644)

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("got %v, want nil (metadata.targets is empty, no fallback)", result)
	}
}
