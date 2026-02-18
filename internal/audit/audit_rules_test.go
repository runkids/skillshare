package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBuiltinRules(t *testing.T) {
	resetForTest()
	rules, err := loadBuiltinRules()
	if err != nil {
		t.Fatalf("loadBuiltinRules() error: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected non-empty rules")
	}
	// Verify each rule has required fields
	for _, r := range rules {
		if r.ID == "" {
			t.Error("rule has empty ID")
		}
		if r.Severity == "" {
			t.Error("rule has empty Severity")
		}
		if r.Pattern == "" {
			t.Errorf("rule %s has empty Pattern", r.ID)
		}
		if r.Message == "" {
			t.Errorf("rule %s has empty Message", r.ID)
		}
		if r.Regex == nil {
			t.Errorf("rule %s has nil Regex", r.ID)
		}
	}
}

func TestRuleCount(t *testing.T) {
	resetForTest()
	rules, err := loadBuiltinRules()
	if err != nil {
		t.Fatalf("loadBuiltinRules() error: %v", err)
	}
	expected := len(builtinYAML())
	if got := len(rules); got != expected {
		t.Errorf("expected %d builtin rules, got %d", expected, got)
	}
}

func TestCompileRules_InvalidRegex(t *testing.T) {
	yr := []yamlRule{{
		ID:       "bad-regex",
		Severity: SeverityHigh,
		Pattern:  "test",
		Message:  "test",
		Regex:    "[invalid",
	}}
	_, err := compileRules(yr)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "bad-regex") {
		t.Errorf("error should mention rule ID, got: %s", err)
	}
}

func TestCompileRules_InvalidExcludeRegex(t *testing.T) {
	yr := []yamlRule{{
		ID:       "bad-exclude",
		Severity: SeverityHigh,
		Pattern:  "test",
		Message:  "test",
		Regex:    "ok",
		Exclude:  "[invalid",
	}}
	_, err := compileRules(yr)
	if err == nil {
		t.Fatal("expected error for invalid exclude regex")
	}
	if !strings.Contains(err.Error(), "bad-exclude") {
		t.Errorf("error should mention rule ID, got: %s", err)
	}
}

func TestCompileRules_ValidateSeverity(t *testing.T) {
	yr := []yamlRule{{
		ID:       "typo-sev",
		Severity: "CRTICAL",
		Pattern:  "test",
		Message:  "test",
		Regex:    "test",
	}}
	_, err := compileRules(yr)
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
	if !strings.Contains(err.Error(), "CRTICAL") {
		t.Errorf("error should mention bad severity, got: %s", err)
	}
}

func TestCompileRules_EmptyRegex(t *testing.T) {
	yr := []yamlRule{{
		ID:       "empty-regex",
		Severity: SeverityHigh,
		Pattern:  "test",
		Message:  "test",
		Regex:    "",
	}}
	_, err := compileRules(yr)
	if err == nil {
		t.Fatal("expected error for empty regex")
	}
}

func TestExcludeRegex(t *testing.T) {
	// suspicious-fetch should have an exclude for localhost
	resetForTest()
	rules, err := loadBuiltinRules()
	if err != nil {
		t.Fatalf("loadBuiltinRules() error: %v", err)
	}

	var fetchRule *rule
	for i := range rules {
		if rules[i].ID == "suspicious-fetch-0" {
			fetchRule = &rules[i]
			break
		}
	}
	if fetchRule == nil {
		t.Fatal("suspicious-fetch-0 rule not found")
	}
	if fetchRule.Exclude == nil {
		t.Fatal("suspicious-fetch-0 should have Exclude regex")
	}

	// External URL should match regex but NOT exclude
	if !fetchRule.Regex.MatchString("curl https://evil.com") {
		t.Error("should match external URL")
	}
	if fetchRule.Exclude.MatchString("curl https://evil.com") {
		t.Error("should not exclude external URL")
	}

	// Localhost should match both regex and exclude
	if !fetchRule.Regex.MatchString("curl http://localhost:3000/api") {
		t.Error("should match localhost URL")
	}
	if !fetchRule.Exclude.MatchString("curl http://localhost:3000/api") {
		t.Error("should exclude localhost URL")
	}
}

func TestMergeRules_Append(t *testing.T) {
	base := []yamlRule{
		{ID: "rule-1", Severity: SeverityHigh, Pattern: "p1", Message: "m1", Regex: "r1"},
	}
	overlay := []yamlRule{
		{ID: "rule-2", Severity: SeverityMedium, Pattern: "p2", Message: "m2", Regex: "r2"},
	}
	merged := mergeYAMLRules(base, overlay)
	if len(merged) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(merged))
	}
	if merged[1].ID != "rule-2" {
		t.Errorf("expected appended rule-2, got %s", merged[1].ID)
	}
}

func TestMergeRules_Disable(t *testing.T) {
	base := []yamlRule{
		{ID: "rule-1", Severity: SeverityHigh, Pattern: "p1", Message: "m1", Regex: "r1"},
		{ID: "rule-2", Severity: SeverityHigh, Pattern: "p2", Message: "m2", Regex: "r2"},
	}
	f := false
	overlay := []yamlRule{
		{ID: "rule-1", Enabled: &f},
	}
	merged := mergeYAMLRules(base, overlay)

	// After compiling, rule-1 should be excluded
	compiled, err := compileRules(merged)
	if err != nil {
		t.Fatalf("compileRules error: %v", err)
	}
	if len(compiled) != 1 {
		t.Fatalf("expected 1 rule after disabling, got %d", len(compiled))
	}
	if compiled[0].ID != "rule-2" {
		t.Errorf("expected rule-2 to survive, got %s", compiled[0].ID)
	}
}

func TestMergeRules_Override(t *testing.T) {
	base := []yamlRule{
		{ID: "rule-1", Severity: SeverityHigh, Pattern: "p1", Message: "original", Regex: "r1"},
	}
	overlay := []yamlRule{
		{ID: "rule-1", Severity: SeverityCritical, Pattern: "p1", Message: "overridden", Regex: "r1"},
	}
	merged := mergeYAMLRules(base, overlay)
	if len(merged) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(merged))
	}
	if merged[0].Message != "overridden" {
		t.Errorf("expected overridden message, got %s", merged[0].Message)
	}
	if merged[0].Severity != SeverityCritical {
		t.Errorf("expected CRITICAL severity, got %s", merged[0].Severity)
	}
}

func TestLoadUserRules_NotFound(t *testing.T) {
	rules, err := loadUserRules("/nonexistent/path/audit-rules.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil rules for missing file, got %d rules", len(rules))
	}
}

func TestRulesWithProject(t *testing.T) {
	resetForTest()

	// Set up a temp dir as config home (so global user rules don't interfere)
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(cfgDir, 0755)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(cfgDir, "config.yaml"))

	// Create a project with custom rules
	projectRoot := filepath.Join(tmpDir, "project")
	os.MkdirAll(filepath.Join(projectRoot, ".skillshare"), 0755)
	os.WriteFile(filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml"), []byte(`rules:
  - id: custom-project-rule
    severity: HIGH
    pattern: custom
    message: "Custom project rule"
    regex: 'CUSTOM_PATTERN'
`), 0644)

	rules, err := RulesWithProject(projectRoot)
	if err != nil {
		t.Fatalf("RulesWithProject() error: %v", err)
	}

	// Should have builtin rules + 1 custom rule
	builtinCount := len(builtinYAML())
	if len(rules) != builtinCount+1 {
		t.Errorf("expected %d rules (%d builtin + 1 custom), got %d", builtinCount+1, builtinCount, len(rules))
	}

	// Find the custom rule
	found := false
	for _, r := range rules {
		if r.ID == "custom-project-rule" {
			found = true
			if r.Pattern != "custom" {
				t.Errorf("expected pattern 'custom', got %s", r.Pattern)
			}
		}
	}
	if !found {
		t.Error("custom-project-rule not found in merged rules")
	}
}

func TestGlobalUserRules(t *testing.T) {
	resetForTest()

	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(cfgDir, 0755)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(cfgDir, "config.yaml"))

	// Create global user rules that add a custom rule
	os.WriteFile(filepath.Join(cfgDir, "audit-rules.yaml"), []byte(`rules:
  - id: global-custom
    severity: MEDIUM
    pattern: global-test
    message: "Global custom rule"
    regex: 'GLOBAL_TEST'
`), 0644)

	rules, err := Rules()
	if err != nil {
		t.Fatalf("Rules() error: %v", err)
	}

	expected := len(builtinYAML()) + 1
	if len(rules) != expected {
		t.Errorf("expected %d rules (%d builtin + 1 global custom), got %d", expected, len(builtinYAML()), len(rules))
	}

	found := false
	for _, r := range rules {
		if r.ID == "global-custom" {
			found = true
		}
	}
	if !found {
		t.Error("global-custom rule not found")
	}
}

func TestGlobalUserRules_DisableBuiltin(t *testing.T) {
	resetForTest()

	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(cfgDir, 0755)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(cfgDir, "config.yaml"))

	// Disable a builtin rule
	os.WriteFile(filepath.Join(cfgDir, "audit-rules.yaml"), []byte(`rules:
  - id: system-writes-0
    enabled: false
`), 0644)

	rules, err := Rules()
	if err != nil {
		t.Fatalf("Rules() error: %v", err)
	}

	expected := len(builtinYAML()) - 1
	if len(rules) != expected {
		t.Errorf("expected %d rules (%d builtin - 1 disabled), got %d", expected, len(builtinYAML()), len(rules))
	}

	for _, r := range rules {
		if r.ID == "system-writes-0" {
			t.Error("system-writes-0 should be disabled")
		}
	}
}
