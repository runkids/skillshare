package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToggleRule_DisableNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := ToggleRule(path, "prompt-injection-0", false); err != nil {
		t.Fatalf("ToggleRule: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "prompt-injection-0") {
		t.Fatal("expected rule ID in file")
	}
	if !strings.Contains(content, "enabled: false") {
		t.Fatal("expected enabled: false")
	}
}

func TestToggleRule_EnableExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	initial := "rules:\n  - id: prompt-injection-0\n    enabled: false\n"
	os.WriteFile(path, []byte(initial), 0644)
	if err := ToggleRule(path, "prompt-injection-0", true); err != nil {
		t.Fatalf("ToggleRule: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "prompt-injection-0") {
		t.Fatal("re-enabled rule should be removed from override file")
	}
}

func TestTogglePattern_Disable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := TogglePattern(path, "credential-access", false); err != nil {
		t.Fatalf("TogglePattern: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "pattern: credential-access") {
		t.Fatal("expected pattern entry")
	}
	if !strings.Contains(content, "enabled: false") {
		t.Fatal("expected enabled: false")
	}
}

func TestTogglePattern_Enable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	initial := "rules:\n  - pattern: credential-access\n    enabled: false\n"
	os.WriteFile(path, []byte(initial), 0644)
	if err := TogglePattern(path, "credential-access", true); err != nil {
		t.Fatalf("TogglePattern: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "credential-access") {
		t.Fatal("re-enabled pattern should be removed")
	}
}

func TestToggleRule_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "audit-rules.yaml")
	if err := ToggleRule(path, "test-rule", false); err != nil {
		t.Fatalf("ToggleRule: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestToggleRule_DisableExistingUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	// Pre-populate with an enabled entry for the same ID
	initial := "rules:\n  - id: rule-1\n    severity: HIGH\n    pattern: test\n    message: test\n    regex: 'foo'\n"
	os.WriteFile(path, []byte(initial), 0644)

	if err := ToggleRule(path, "rule-1", false); err != nil {
		t.Fatalf("ToggleRule: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "enabled: false") {
		t.Fatal("expected enabled: false after disabling existing entry")
	}
	// Should still have exactly one entry for rule-1
	if strings.Count(content, "rule-1") != 1 {
		t.Fatalf("expected exactly one rule-1 entry, got:\n%s", content)
	}
}

func TestTogglePattern_DisableExistingUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	// Pre-populate with a pattern-level entry (already disabled)
	boolFalse := false
	writeRulesFile(path, []yamlRule{
		{Pattern: "credential-access", Enabled: &boolFalse},
	})

	// Disabling again should be idempotent
	if err := TogglePattern(path, "credential-access", false); err != nil {
		t.Fatalf("TogglePattern: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Count(content, "credential-access") != 1 {
		t.Fatalf("expected exactly one credential-access entry, got:\n%s", content)
	}
}

func TestWriteRulesFile_EmptyRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := writeRulesFile(path, nil); err != nil {
		t.Fatalf("writeRulesFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "rules: []") {
		t.Fatalf("empty rules should produce 'rules: []', got:\n%s", content)
	}
	if !strings.Contains(content, "# Custom audit rules") {
		t.Fatal("expected header comment")
	}
}

func TestWriteRulesFile_Header(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := writeRulesFile(path, []yamlRule{{ID: "test", Enabled: boolPtr(false)}}); err != nil {
		t.Fatalf("writeRulesFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.HasPrefix(content, "# Custom audit rules for skillshare.") {
		t.Fatal("expected header at start of file")
	}
	if !strings.Contains(content, "https://skillshare.runkids.cc/docs/reference/commands/audit#custom-rules") {
		t.Fatal("expected docs URL in header")
	}
}

func boolPtr(b bool) *bool { return &b }
