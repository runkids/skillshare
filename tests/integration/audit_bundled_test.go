//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestAudit_BundledSettingsPolicy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
	sb.CreateSkill("bundled", map[string]string{
		"SKILL.md": "---\nname: bundled\n---\n# Bundled fixture\n",
	})
	settings := filepath.Join(sb.SourcePath, "bundled", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(settings, `{"apiKeyHelper":"printf fixture-private-value"}`)
	run := func(severity string, code int) {
		t.Helper()
		r := sb.RunCLI("audit", "bundled", "--analyzer", "structure", "--threshold", "high", "--format", "json")
		r.AssertExitCode(t, code)
		payload := parseAuditJSONPayload(t, r)
		if len(payload.Results) != 1 || len(payload.Results[0].Findings) != 1 || payload.Results[0].Findings[0].RuleID != "bundled-helper-command" || payload.Results[0].Findings[0].Severity != severity {
			t.Fatalf("unexpected report: %s", r.Stdout)
		}
		if strings.Contains(r.Stdout, "fixture-private-value") {
			t.Fatal("credential leaked")
		}
	}
	run("HIGH", 1)
	rulesPath := filepath.Join(filepath.Dir(sb.ConfigPath), "audit-rules.yaml")
	sb.WriteFile(rulesPath, "rules:\n  - id: bundled-helper-command\n    severity: LOW\n")
	run("LOW", 0)
	sb.WriteFile(rulesPath, "rules:\n  - id: bundled-helper-command\n    enabled: false\n")
	r := sb.RunCLI("audit", "bundled", "--analyzer", "structure", "--format", "json")
	r.AssertSuccess(t)
	for _, result := range parseAuditJSONPayload(t, r).Results {
		if len(result.Findings) != 0 {
			t.Fatalf("disabled rule ran: %s", r.Stdout)
		}
	}
}
