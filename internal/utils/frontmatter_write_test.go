package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetFrontmatterList_SetMetadataTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n# Content\nBody text here"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "targets:") {
		t.Error("expected targets field in output")
	}
	if !strings.Contains(content, "claude") {
		t.Error("expected 'claude' in output")
	}
	if !strings.Contains(content, "# Content") {
		t.Error("body content should be preserved")
	}
	if !strings.Contains(content, "Body text here") {
		t.Error("body text should be preserved")
	}
	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("re-parsed targets = %v, want [claude]", result)
	}
}

func TestSetFrontmatterList_RemoveTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\nmetadata:\n  targets:\n    - claude\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("expected nil targets after removal, got %v", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "name: my-skill") {
		t.Error("name field should be preserved")
	}
}

func TestSetFrontmatterList_RemovesLegacyTopLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [cursor]\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("re-parsed targets = %v, want [claude]", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# Content") {
		t.Error("body should be preserved")
	}
}

func TestSetFrontmatterList_RemoveAll_CleansLegacyToo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ntargets: [cursor]\nmetadata:\n  targets: [claude]\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if result != nil {
		t.Errorf("expected nil after remove, got %v", result)
	}
}

func TestSetFrontmatterList_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\ndescription: A great skill\nmetadata:\n  pattern: tool-wrapper\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude", "cursor"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "name: my-skill") {
		t.Error("name should be preserved")
	}
	if !strings.Contains(content, "description: A great skill") {
		t.Error("description should be preserved")
	}
	if !strings.Contains(content, "pattern: tool-wrapper") {
		t.Error("metadata.pattern should be preserved")
	}
	result := ParseFrontmatterList(path, "targets")
	if len(result) != 2 {
		t.Errorf("expected 2 targets, got %v", result)
	}
}

func TestSetFrontmatterList_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("# Just content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 1 || result[0] != "claude" {
		t.Errorf("expected [claude], got %v", result)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# Just content") {
		t.Error("body should be preserved")
	}
}

func TestSetFrontmatterList_FileNotFound(t *testing.T) {
	err := SetFrontmatterList("/nonexistent/SKILL.md", "metadata.targets", []string{"claude"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSetFrontmatterList_MultipleTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: my-skill\n---\n# Content"), 0644)

	err := SetFrontmatterList(path, "metadata.targets", []string{"claude", "cursor", "copilot"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := ParseFrontmatterList(path, "targets")
	if len(result) != 3 {
		t.Errorf("expected 3 targets, got %v", result)
	}
}
