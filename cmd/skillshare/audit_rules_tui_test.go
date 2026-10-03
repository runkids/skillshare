package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/audit"
)

func testARModel() arModel {
	rules := []audit.CompiledRule{
		{ID: "curl-pipe-shell-1", Pattern: "curl-pipe-shell", Severity: "CRITICAL", Enabled: true},
		{ID: "prompt-injection-1", Pattern: "prompt-injection", Severity: "HIGH", Enabled: false},
	}
	m := newARModel(rules, modeGlobal)
	m.width = 140
	return m
}

func TestAuditRulesTitleLine_ShowsCountsAndSeverityTabs(t *testing.T) {
	got := xansi.Strip(testARModel().renderTitleLine())
	for _, want := range []string{"skillshare audit rules", "global", "2 patterns", "2 rules", "All 2", "Crit 1", "Off 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("title line %q is missing %q", got, want)
		}
	}
}

func TestAuditRulesReset_NoLeavesTheRulesAlone(t *testing.T) {
	next, _ := testARModel().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	m := next.(arModel)
	if !m.pendingReset || !strings.Contains(xansi.Strip(m.renderBottom()), "Reset all rules?") {
		t.Fatalf("R should ask on the key line; got %q", xansi.Strip(m.renderBottom()))
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(arModel)
	if m.pendingReset || m.flashMsg != "" {
		t.Fatalf("n should cancel without resetting; pending=%v flash=%q", m.pendingReset, m.flashMsg)
	}
}
