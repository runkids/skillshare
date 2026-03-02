package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListRules_ReturnsAllBuiltin(t *testing.T) {
	resetForTest()
	os.Setenv("SKILLSHARE_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	defer os.Unsetenv("SKILLSHARE_CONFIG")

	rules, err := ListRules()
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected rules, got 0")
	}
	for _, r := range rules {
		if !r.Enabled {
			t.Fatalf("rule %s should be enabled", r.ID)
		}
		if r.Source != "builtin" {
			t.Fatalf("rule %s should be builtin, got %s", r.ID, r.Source)
		}
	}
}

func TestListRules_WithDisabledRule(t *testing.T) {
	resetForTest()
	dir := t.TempDir()
	os.Setenv("SKILLSHARE_CONFIG", filepath.Join(dir, "config.yaml"))
	defer os.Unsetenv("SKILLSHARE_CONFIG")

	rulesYAML := "rules:\n  - id: prompt-injection-0\n    enabled: false\n"
	os.WriteFile(filepath.Join(dir, "audit-rules.yaml"), []byte(rulesYAML), 0644)

	rules, err := ListRules()
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	found := false
	for _, r := range rules {
		if r.ID == "prompt-injection-0" {
			found = true
			if r.Enabled {
				t.Fatal("prompt-injection-0 should be disabled")
			}
			if r.Source != "global" {
				t.Fatalf("expected source=global, got %s", r.Source)
			}
		}
	}
	if !found {
		t.Fatal("prompt-injection-0 not found in list")
	}
}

func TestListRules_PatternGroups(t *testing.T) {
	resetForTest()
	os.Setenv("SKILLSHARE_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	defer os.Unsetenv("SKILLSHARE_CONFIG")

	rules, err := ListRules()
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	patterns := PatternSummary(rules)
	if len(patterns) == 0 {
		t.Fatal("expected pattern groups")
	}
	var caCount int
	for _, p := range patterns {
		if p.Pattern == "credential-access" {
			caCount = p.Total
		}
	}
	if caCount < 50 {
		t.Fatalf("credential-access should have 50+ rules, got %d", caCount)
	}
}
