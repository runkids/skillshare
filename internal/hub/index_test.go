package hub

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}

func TestBuildIndex_BasicSkills(t *testing.T) {
	source := t.TempDir()
	createSkill(t, source, "alpha", "---\nname: alpha\ndescription: First skill\n---\n# Alpha")
	createSkill(t, source, "beta", "---\nname: beta\ndescription: Second skill\n---\n# Beta")

	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if idx.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", idx.SchemaVersion)
	}
	if idx.SourcePath != source {
		t.Errorf("sourcePath = %q, want %q", idx.SourcePath, source)
	}
	if len(idx.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(idx.Skills))
	}
	if idx.Skills[0].Name != "alpha" {
		t.Errorf("skills[0].name = %q, want alpha", idx.Skills[0].Name)
	}
	if idx.Skills[0].Description != "First skill" {
		t.Errorf("skills[0].description = %q, want 'First skill'", idx.Skills[0].Description)
	}
	if idx.Skills[1].Name != "beta" {
		t.Errorf("skills[1].name = %q, want beta", idx.Skills[1].Name)
	}
}

func TestBuildIndex_EmptySource(t *testing.T) {
	source := t.TempDir()
	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex on empty dir: %v", err)
	}
	if len(idx.Skills) != 0 {
		t.Errorf("got %d skills, want 0", len(idx.Skills))
	}
	if idx.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", idx.SchemaVersion)
	}
}

func TestBuildIndex_MissingDirectory(t *testing.T) {
	_, err := BuildIndex("/nonexistent/path/that/should/not/exist", false)
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
}

func TestBuildIndex_DeterministicSort(t *testing.T) {
	source := t.TempDir()
	createSkill(t, source, "zulu", "---\nname: zulu\n---\n# Z")
	createSkill(t, source, "alpha", "---\nname: alpha\n---\n# A")
	createSkill(t, source, "mike", "---\nname: mike\n---\n# M")

	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Skills) != 3 {
		t.Fatalf("got %d skills, want 3", len(idx.Skills))
	}
	names := []string{idx.Skills[0].Name, idx.Skills[1].Name, idx.Skills[2].Name}
	expected := []string{"alpha", "mike", "zulu"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("skills[%d].name = %q, want %q", i, n, expected[i])
		}
	}
}

func TestBuildIndex_DescriptionPipeScalar(t *testing.T) {
	source := t.TempDir()
	createSkill(t, source, "pipe-skill", "---\nname: pipe-skill\ndescription: |\n---\n# Pipe")

	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}
	if idx.Skills[0].Description != "" {
		t.Errorf("description = %q, want empty for pipe scalar", idx.Skills[0].Description)
	}
}

func TestBuildIndex_DescriptionVariants(t *testing.T) {
	tests := []struct {
		frontmatter string
		want        string
	}{
		{"description: Simple text", "Simple text"},
		{"description: \"Quoted text\"", "Quoted text"},
		{"description: 'Single quoted'", "Single quoted"},
		{"description: |", ""},
		{"description: >", ""},
		{"description: |+", ""},
		{"description: |-", ""},
		{"description: >+", ""},
		{"description: >-", ""},
	}
	for _, tt := range tests {
		t.Run(tt.frontmatter, func(t *testing.T) {
			source := t.TempDir()
			content := "---\nname: test\n" + tt.frontmatter + "\n---\n# Test"
			createSkill(t, source, "test", content)

			idx, err := BuildIndex(source, false)
			if err != nil {
				t.Fatalf("BuildIndex: %v", err)
			}
			if len(idx.Skills) != 1 {
				t.Fatalf("got %d skills, want 1", len(idx.Skills))
			}
			if idx.Skills[0].Description != tt.want {
				t.Errorf("description = %q, want %q", idx.Skills[0].Description, tt.want)
			}
		})
	}
}

func TestBuildIndex_MinimalOmitsMetadata(t *testing.T) {
	source := t.TempDir()
	createSkill(t, source, "my-skill", "---\nname: my-skill\ndescription: A skill\n---\n# Content")

	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}

	// Marshal to JSON and check that metadata fields are absent.
	data, err := json.Marshal(idx.Skills[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Minimal mode should only have name, description, source.
	for _, key := range []string{"flatName", "relPath", "type", "repoUrl", "version", "installedAt", "isInRepo"} {
		if _, ok := raw[key]; ok {
			t.Errorf("minimal mode should not contain %q, but it does", key)
		}
	}
}

func TestBuildIndex_FullIncludesMetadata(t *testing.T) {
	source := t.TempDir()

	// Create a nested skill inside a _-prefixed directory (tracked repo).
	nested := filepath.Join(source, "_team", "frontend")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("---\nname: frontend\n---\n# FE"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx, err := BuildIndex(source, true)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}

	s := idx.Skills[0]
	// flatName should differ from name for nested skills.
	if s.FlatName == "" {
		t.Error("full mode: flatName should be set for nested skill")
	}
	if s.FlatName == s.Name {
		t.Errorf("flatName %q should differ from name %q for nested skill", s.FlatName, s.Name)
	}
	// isInRepo should be true for _-prefixed.
	if s.IsInRepo == nil || !*s.IsInRepo {
		t.Error("full mode: isInRepo should be true for _-prefixed skill")
	}
}

func TestBuildIndex_FullOmitsRedundant(t *testing.T) {
	source := t.TempDir()
	// Standalone skill — flatName == name, relPath == source.
	createSkill(t, source, "standalone", "---\nname: standalone\n---\n# S")

	idx, err := BuildIndex(source, true)
	if err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}

	data, err := json.Marshal(idx.Skills[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// flatName == name → should not be emitted.
	if _, ok := raw["flatName"]; ok {
		t.Error("flatName should be omitted when equal to name")
	}
	// relPath == source → should not be emitted.
	if _, ok := raw["relPath"]; ok {
		t.Error("relPath should be omitted when equal to source")
	}
	// isInRepo false → should not be emitted.
	if _, ok := raw["isInRepo"]; ok {
		t.Error("isInRepo false should be omitted")
	}
}

func TestWriteIndex(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "sub", "index.json")

	idx := &Index{
		SchemaVersion: 1,
		GeneratedAt:   "2026-01-01T00:00:00Z",
		Skills: []SkillEntry{
			{Name: "test", Source: "test"},
		},
	}
	if err := WriteIndex(outPath, idx); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var parsed Index
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if parsed.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", parsed.SchemaVersion)
	}
	if len(parsed.Skills) != 1 {
		t.Errorf("got %d skills, want 1", len(parsed.Skills))
	}
}

func TestBuildIndex_WithAudit(t *testing.T) {
	source := t.TempDir()
	// A clean skill — no dangerous patterns.
	createSkill(t, source, "safe-skill", "---\nname: safe-skill\ndescription: A safe skill\n---\n# Safe content\nJust helpful tips.")

	idx, err := BuildIndex(source, false, true)
	if err != nil {
		t.Fatalf("BuildIndex with audit: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}

	s := idx.Skills[0]
	if s.RiskScore == nil {
		t.Fatal("riskScore should not be nil when audit=true")
	}
	if *s.RiskScore != 0 {
		t.Errorf("riskScore = %d, want 0 for clean skill", *s.RiskScore)
	}
	if s.RiskLabel != "clean" {
		t.Errorf("riskLabel = %q, want 'clean'", s.RiskLabel)
	}
	if s.AuditedAt == "" {
		t.Error("auditedAt should be set when audit=true")
	}

	// Verify JSON serialization includes risk fields.
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := raw["riskScore"]; !ok {
		t.Error("riskScore should be present in JSON (even when 0)")
	}
	if _, ok := raw["riskLabel"]; !ok {
		t.Error("riskLabel should be present in JSON")
	}
	if _, ok := raw["auditedAt"]; !ok {
		t.Error("auditedAt should be present in JSON")
	}
}

func TestBuildIndex_WithoutAudit(t *testing.T) {
	source := t.TempDir()
	createSkill(t, source, "my-skill", "---\nname: my-skill\n---\n# Content")

	idx, err := BuildIndex(source, false)
	if err != nil {
		t.Fatalf("BuildIndex without audit: %v", err)
	}
	if len(idx.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(idx.Skills))
	}

	s := idx.Skills[0]
	if s.RiskScore != nil {
		t.Errorf("riskScore should be nil when audit not requested, got %d", *s.RiskScore)
	}
	if s.RiskLabel != "" {
		t.Errorf("riskLabel should be empty when audit not requested, got %q", s.RiskLabel)
	}
	if s.AuditedAt != "" {
		t.Errorf("auditedAt should be empty when audit not requested, got %q", s.AuditedAt)
	}

	// Verify JSON omits risk fields.
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"riskScore", "riskLabel", "auditedAt"} {
		if _, ok := raw[key]; ok {
			t.Errorf("audit field %q should not be in JSON when audit=false", key)
		}
	}
}

func TestWriteIndex_NilIndex(t *testing.T) {
	if err := WriteIndex("/tmp/should-not-exist.json", nil); err == nil {
		t.Fatal("expected error for nil index")
	}
}
