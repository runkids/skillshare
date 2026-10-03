package main

import (
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
