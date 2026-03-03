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

func TestSetSeverity_NewEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := SetSeverity(path, "prompt-injection-0", "medium"); err != nil {
		t.Fatalf("SetSeverity: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "prompt-injection-0") {
		t.Fatal("expected rule ID in file")
	}
	if !strings.Contains(content, "severity: MEDIUM") {
		t.Fatalf("expected severity: MEDIUM, got:\n%s", content)
	}
}

func TestSetSeverity_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	os.WriteFile(path, []byte("rules:\n  - id: rule-1\n    severity: HIGH\n"), 0644)

	if err := SetSeverity(path, "rule-1", "LOW"); err != nil {
		t.Fatalf("SetSeverity: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "severity: LOW") {
		t.Fatalf("expected updated severity, got:\n%s", content)
	}
	if strings.Count(content, "rule-1") != 1 {
		t.Fatalf("expected exactly one entry, got:\n%s", content)
	}
}

func TestSetSeverity_InvalidLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	err := SetSeverity(path, "rule-1", "BANANA")
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestSetPatternSeverity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := SetPatternSeverity(path, "destructive-commands", "medium"); err != nil {
		t.Fatalf("SetPatternSeverity: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "pattern: destructive-commands") {
		t.Fatal("expected pattern entry")
	}
	if !strings.Contains(content, "severity: MEDIUM") {
		t.Fatalf("expected severity: MEDIUM, got:\n%s", content)
	}
}

func TestResetRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	os.WriteFile(path, []byte("rules:\n  - id: test\n    enabled: false\n"), 0644)

	if err := ResetRules(path); err != nil {
		t.Fatalf("ResetRules: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be deleted after reset")
	}
}

func TestResetRules_NoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")
	// Should not error if file doesn't exist
	if err := ResetRules(path); err != nil {
		t.Fatalf("ResetRules should be no-op for missing file: %v", err)
	}
}

func TestToggleRules_BatchDisable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	ids := []string{"rule-a", "rule-b", "rule-c"}
	if err := ToggleRules(path, ids, false); err != nil {
		t.Fatalf("ToggleRules: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	for _, id := range ids {
		if !strings.Contains(content, id) {
			t.Fatalf("expected %s in file, got:\n%s", id, content)
		}
	}
	if strings.Count(content, "enabled: false") != 3 {
		t.Fatalf("expected 3 disabled entries, got:\n%s", content)
	}
}

func TestToggleRules_BatchEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	// Pre-populate with 3 disabled rules
	initial := "rules:\n  - id: rule-a\n    enabled: false\n  - id: rule-b\n    enabled: false\n  - id: rule-c\n    enabled: false\n"
	os.WriteFile(path, []byte(initial), 0644)

	if err := ToggleRules(path, []string{"rule-a", "rule-c"}, true); err != nil {
		t.Fatalf("ToggleRules: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "rule-a") {
		t.Fatal("rule-a should be removed after re-enable")
	}
	if !strings.Contains(content, "rule-b") {
		t.Fatal("rule-b should remain disabled")
	}
	if strings.Contains(content, "rule-c") {
		t.Fatal("rule-c should be removed after re-enable")
	}
}

func TestToggleRules_MixedExistingAndNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	// Pre-populate with one existing rule
	initial := "rules:\n  - id: existing-rule\n    severity: HIGH\n"
	os.WriteFile(path, []byte(initial), 0644)

	if err := ToggleRules(path, []string{"existing-rule", "new-rule"}, false); err != nil {
		t.Fatalf("ToggleRules: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Count(content, "existing-rule") != 1 {
		t.Fatalf("expected exactly one existing-rule entry, got:\n%s", content)
	}
	if !strings.Contains(content, "new-rule") {
		t.Fatal("expected new-rule in file")
	}
	if strings.Count(content, "enabled: false") != 2 {
		t.Fatalf("expected 2 disabled entries, got:\n%s", content)
	}
}

func TestSetSeverityBatch_MultipleIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := SetSeverityBatch(path, []string{"rule-x", "rule-y", "rule-z"}, "high"); err != nil {
		t.Fatalf("SetSeverityBatch: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	for _, id := range []string{"rule-x", "rule-y", "rule-z"} {
		if !strings.Contains(content, id) {
			t.Fatalf("expected %s in file, got:\n%s", id, content)
		}
	}
	if strings.Count(content, "severity: HIGH") != 3 {
		t.Fatalf("expected 3 HIGH severity entries, got:\n%s", content)
	}
}

func TestSetSeverityBatch_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	initial := "rules:\n  - id: rule-1\n    severity: LOW\n  - id: rule-2\n    severity: INFO\n"
	os.WriteFile(path, []byte(initial), 0644)

	if err := SetSeverityBatch(path, []string{"rule-1", "rule-2"}, "critical"); err != nil {
		t.Fatalf("SetSeverityBatch: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Count(content, "severity: CRITICAL") != 2 {
		t.Fatalf("expected 2 CRITICAL entries, got:\n%s", content)
	}
	if strings.Count(content, "rule-1") != 1 || strings.Count(content, "rule-2") != 1 {
		t.Fatalf("expected exactly one entry per rule, got:\n%s", content)
	}
}

func TestSetSeverityBatch_InvalidLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	err := SetSeverityBatch(path, []string{"rule-1"}, "BANANA")
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
	// File should NOT be created for invalid input
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("file should not exist after invalid severity")
	}
}

func TestSetSeverityBatch_Shorthand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")
	if err := SetSeverityBatch(path, []string{"rule-1"}, "c"); err != nil {
		t.Fatalf("SetSeverityBatch with shorthand: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "severity: CRITICAL") {
		t.Fatalf("shorthand 'c' should resolve to CRITICAL, got:\n%s", content)
	}
}

func TestSetSeverity_PreservesRegex(t *testing.T) {
	resetForTest()

	dir := t.TempDir()
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(dir, "config.yaml"))
	path := filepath.Join(dir, "audit-rules.yaml")

	// Pick a known builtin rule ID.
	builtins := builtinYAML()
	if len(builtins) == 0 {
		t.Fatal("no builtin rules")
	}
	target := builtins[0]

	// Set severity override (writes only id+severity to YAML).
	if err := SetSeverity(path, target.ID, "LOW"); err != nil {
		t.Fatalf("SetSeverity: %v", err)
	}

	// Merge and compile — must not fail with "empty regex".
	user, err := loadUserRules(path)
	if err != nil {
		t.Fatalf("loadUserRules: %v", err)
	}
	merged := mergeYAMLRules(builtinYAML(), user)
	compiled, err := compileRules(merged)
	if err != nil {
		t.Fatalf("compileRules should succeed after severity override: %v", err)
	}

	// Find the target rule and verify severity was changed.
	found := false
	for _, r := range compiled {
		if r.ID == target.ID {
			found = true
			if r.Severity != SeverityLow {
				t.Errorf("severity: want LOW, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Errorf("rule %s not found in compiled rules", target.ID)
	}
}

func TestTogglePattern_EnableClearsIDDisables(t *testing.T) {
	resetForTest()

	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")

	// Find two rule IDs from the same pattern.
	builtins := builtinYAML()
	var pattern string
	var ids []string
	patternIDs := make(map[string][]string)
	for _, r := range builtins {
		patternIDs[r.Pattern] = append(patternIDs[r.Pattern], r.ID)
	}
	for p, pids := range patternIDs {
		if len(pids) >= 2 {
			pattern = p
			ids = pids[:2]
			break
		}
	}
	if pattern == "" {
		t.Skip("no pattern with 2+ rules found")
	}

	// Disable both IDs individually.
	for _, id := range ids {
		if err := ToggleRule(path, id, false); err != nil {
			t.Fatalf("ToggleRule disable %s: %v", id, err)
		}
	}

	// Verify they are disabled.
	rules, _ := loadUserRules(path)
	for _, r := range rules {
		for _, id := range ids {
			if r.ID == id && (r.Enabled == nil || *r.Enabled) {
				t.Fatalf("expected %s to be disabled", id)
			}
		}
	}

	// Enable the whole pattern — should clear ID-level disables.
	if err := TogglePattern(path, pattern, true); err != nil {
		t.Fatalf("TogglePattern enable: %v", err)
	}

	// Verify no disabled entries remain.
	rules, _ = loadUserRules(path)
	for _, r := range rules {
		if r.Enabled != nil && !*r.Enabled {
			for _, id := range ids {
				if r.ID == id {
					t.Errorf("rule %s should no longer be disabled after pattern enable", id)
				}
			}
		}
	}
}

func TestTogglePattern_EnableKeepsSeverityOverride(t *testing.T) {
	resetForTest()

	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")

	// Find a rule from a known pattern.
	builtins := builtinYAML()
	var target yamlRule
	for _, r := range builtins {
		if r.Pattern != "" {
			target = r
			break
		}
	}
	if target.ID == "" {
		t.Fatal("no builtin rule with pattern found")
	}

	// Disable the ID and set severity override.
	if err := ToggleRule(path, target.ID, false); err != nil {
		t.Fatalf("ToggleRule: %v", err)
	}
	if err := SetSeverity(path, target.ID, "INFO"); err != nil {
		t.Fatalf("SetSeverity: %v", err)
	}

	// Enable the whole pattern.
	if err := TogglePattern(path, target.Pattern, true); err != nil {
		t.Fatalf("TogglePattern: %v", err)
	}

	// The severity override should be kept, enabled:false should be cleared.
	rules, _ := loadUserRules(path)
	found := false
	for _, r := range rules {
		if r.ID == target.ID {
			found = true
			if r.Enabled != nil && !*r.Enabled {
				t.Error("enabled:false should have been cleared")
			}
			if r.Severity != "INFO" {
				t.Errorf("severity override should be preserved, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("entry with severity override should still exist")
	}
}

func TestTogglePattern_EnableClearsCustomRuleDisables(t *testing.T) {
	resetForTest()

	dir := t.TempDir()
	path := filepath.Join(dir, "audit-rules.yaml")

	// Write a custom rule in the same pattern as a builtin, then disable it.
	disabled := false
	writeRulesFile(path, []yamlRule{
		{
			ID:       "my-custom-cred-rule",
			Severity: "HIGH",
			Pattern:  "credential-access",
			Message:  "Custom credential check",
			Regex:    "SUPER_SECRET",
			Enabled:  &disabled,
		},
	})

	// Enable the whole pattern — custom rule should also be re-enabled.
	if err := TogglePattern(path, "credential-access", true); err != nil {
		t.Fatalf("TogglePattern enable: %v", err)
	}

	rules, _ := loadUserRules(path)
	found := false
	for _, r := range rules {
		if r.ID == "my-custom-cred-rule" {
			found = true
			if r.Enabled != nil && !*r.Enabled {
				t.Error("custom rule should no longer be disabled after pattern enable")
			}
			// Regex and other fields should still be intact.
			if r.Regex != "SUPER_SECRET" {
				t.Errorf("regex should be preserved, got %q", r.Regex)
			}
		}
	}
	if !found {
		t.Error("custom rule entry should still exist (has regex)")
	}
}

func boolPtr(b bool) *bool { return &b }
