package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
)

func TestInstallRemoteExtra_ReportsHiddenAuditCoverage(t *testing.T) {
	req := newRemoteExtraRequest(t)
	req.Opts.SkipAudit = false
	req.Opts.AuditProjectRoot = t.TempDir()
	rules := filepath.Join(req.Opts.AuditProjectRoot, ".skillshare", "audit-rules.yaml")
	if err := os.MkdirAll(filepath.Dir(rules), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rules, []byte("rules:\n  - id: synthetic-hidden\n    severity: CRITICAL\n    pattern: synthetic\n    message: synthetic finding\n    regex: 'SYNTHETIC_HIDDEN_TEXT'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(t.TempDir(), "seed")
	runGit(t, "", "clone", req.RawSource, seed)
	runGit(t, seed, "config", "user.name", "Fixture")
	runGit(t, seed, "config", "user.email", "fixture@example.invalid")
	file := filepath.Join(seed, ".private", "note.md")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("SYNTHETIC_HIDDEN_TEXT\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "hidden text policy fixture")
	runGit(t, seed, "push", "origin", "HEAD:main")
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	// Characterize the existing scanner policy, rather than broadening it:
	// hidden-parent Markdown is counted as skipped, not ordinary text-scanned.
	candidate, cleanup, err := prepareExtraCandidate(req.Source, req.ExtrasRoot, filepath.Join(req.ExtrasRoot, req.Name), req.Opts)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if candidate.audit.Audit == nil || candidate.audit.Audit.SkippedFiles == 0 {
		t.Fatalf("hidden-parent scan limit not characterized: %+v", candidate.audit)
	}
	for _, finding := range candidate.audit.Audit.Findings {
		if finding.RuleID == "synthetic-hidden" {
			t.Fatal("ordinary text scan unexpectedly widened into hidden parent")
		}
	}
	result, err := InstallRemoteExtra(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.AuditSkipped {
		t.Fatal("partial scanner coverage must not claim --skip-audit")
	}
	if warnings := strings.ToLower(strings.Join(result.Warnings, "\n")); !strings.Contains(warnings, "audit") || !strings.Contains(warnings, "skip") {
		t.Errorf("successful install omits audit coverage warning: %+v", result)
	}
	if data, err := os.ReadFile(filepath.Join(req.ExtrasRoot, req.Name, ".private", "note.md")); err != nil || string(data) != "SYNTHETIC_HIDDEN_TEXT\n" {
		t.Errorf("accepted hidden content missing: %q %v", data, err)
	}
}
