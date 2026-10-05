package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
)

// The agents-only audit must read the agents source itself: the caller keeps
// the skills source untouched so the TUI's Skills tab scans skills, not agents.
func TestAuditInstalled_AgentsKindScansAgentsSource(t *testing.T) {
	skills := t.TempDir()
	agents := t.TempDir()
	if err := os.MkdirAll(filepath.Join(skills, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skills, "demo", "SKILL.md"), []byte("---\nname: demo\n---\n# demo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "helper.md"), []byte("---\nname: helper\n---\n# helper\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var results []*audit.Result
	captureStdout(t, func() {
		var err error
		results, _, err = auditInstalled(skills, agents, "global", "", "medium", kindAgents, auditOptions{Format: formatJSON}, audit.DefaultRegistry())
		if err != nil {
			t.Errorf("auditInstalled: %v", err)
		}
	})

	if len(results) != 1 || results[0].Kind != "agent" || !strings.HasPrefix(results[0].ScanTarget, agents) {
		t.Fatalf("expected the one agent from the agents source, got %+v", results)
	}
	if results[0].SkillName != "helper.md" {
		t.Fatalf("expected the name relative to the agents source, got %q", results[0].SkillName)
	}
}
