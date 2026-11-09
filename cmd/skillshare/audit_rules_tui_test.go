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

func TestAuditRulesShowPythonContext(t *testing.T) {
	r := audit.CompiledRule{ID: "credential-access-aws-credentials", Pattern: "credential-access", Severity: "CRITICAL", Enabled: true, Source: "builtin", SeverityContext: audit.SeverityContextPythonInert}
	outputs := map[string]string{
		"table": captureStdout(t, func() { _ = printAuditRulesTable([]audit.CompiledRule{r}) }),
		"tui":   xansi.Strip(testARModel().renderRuleDetail(arRuleItem{rule: r})),
	}
	for name, got := range outputs {
		if !strings.Contains(got, "HIGH in Python comments and docstrings") {
			t.Errorf("%s output does not show the Python context:\n%s", name, got)
		}
	}
	if !strings.Contains(outputs["tui"], "setting a severity applies it everywhere") {
		t.Errorf("rule detail does not say a chosen severity applies everywhere:\n%s", outputs["tui"])
	}
}
