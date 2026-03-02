package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzability_PureMarkdown(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "clean-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nDoes stuff"), 0644)
	os.WriteFile(filepath.Join(skillDir, "utils.sh"), []byte("echo hello"), 0644)

	result, err := ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Analyzability < 0.99 {
		t.Errorf("pure text skill should be ~1.0 analyzable, got %.2f", result.Analyzability)
	}
	if result.TotalBytes == 0 {
		t.Error("TotalBytes should be > 0")
	}
	if result.AuditableBytes == 0 {
		t.Error("AuditableBytes should be > 0")
	}
}

func TestAnalyzability_WithBinary(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "mixed-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0644)

	// Binary file: 1000 bytes with null bytes
	bin := make([]byte, 1000)
	bin[0] = 0 // null byte → binary
	os.WriteFile(filepath.Join(skillDir, "data.bin"), bin, 0644)

	result, err := ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	// SKILL.md = 7 bytes auditable, data.bin = 1000 bytes non-auditable
	// Analyzability = 7 / 1007 ≈ 0.007
	if result.Analyzability > 0.05 {
		t.Errorf("skill with large binary should have low analyzability, got %.2f", result.Analyzability)
	}
}

func TestAnalyzability_EmptySkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "empty-skill")
	os.MkdirAll(skillDir, 0755)

	result, err := ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Analyzability != 1.0 {
		t.Errorf("empty skill should be 1.0 analyzable, got %.2f", result.Analyzability)
	}
}

func TestAnalyzability_LowScoreEmitsWarning(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "opaque-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# X"), 0644)

	// Create large non-scannable file to push analyzability below 70%
	os.WriteFile(filepath.Join(skillDir, "logo.png"), make([]byte, 100), 0644)

	result, err := ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Pattern == "low-analyzability" {
			found = true
			if f.Severity != SeverityInfo {
				t.Errorf("expected INFO severity, got %s", f.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected low-analyzability finding when score=%.2f", result.Analyzability)
	}
}

func TestAnalyzability_HighScoreNoWarning(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "transparent-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill\nLong content here"), 0644)

	result, err := ScanSkill(skillDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Findings {
		if f.Pattern == "low-analyzability" {
			t.Error("should not emit low-analyzability for high score")
		}
	}
}
