package main

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/audit"
)

func TestParseAuditMinSeverity(t *testing.T) {
	for _, level := range []string{"critical", "high", "medium", "low", "info", "h"} {
		opts, _, err := parseAuditArgs([]string{"--min-severity", level})
		want, _ := audit.NormalizeSeverity(level)
		if err != nil || opts.MinSeverity != want {
			t.Fatalf("%s: %+v %v", level, opts, err)
		}
	}
	for _, args := range [][]string{{"--min-severity"}, {"--min-severity", "bogus"}, {"--min-severity", ""}, {"--min-severity", "-g"}} {
		if _, _, err := parseAuditArgs(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestAuditTUIDisplayFloor(t *testing.T) {
	m := testAuditModel()
	m.minSeverity = audit.SeverityHigh
	r := &audit.Result{SkillName: "low", RiskScore: 15, IsBlocked: true, Threshold: audit.SeverityLow, Findings: []audit.Finding{{Severity: audit.SeverityLow, Message: "hidden message"}}}
	got := m.renderDetailContent(auditItem{result: r})
	if strings.Contains(got, "hidden message") || !strings.Contains(got, "0 visible, 1 hidden") || !strings.Contains(got, "BLOCKED") {
		t.Fatal(got)
	}
}

func TestAuditTUIAllHiddenKeepsFullSummary(t *testing.T) {
	r := &audit.Result{SkillName: "low", Findings: []audit.Finding{{Severity: audit.SeverityLow}}, IsBlocked: true}
	m := newAuditTUIModel([]*audit.Result{r}, nil, auditRunSummary{Scanned: 1, Failed: 1, Low: 1}, nil, nil, auditRunSummary{}, auditTabSkills, audit.SeverityHigh)
	m.termWidth = 180
	if len(m.allItems) != 1 || !strings.Contains(m.renderSummaryNote(), "0 visible, 1 hidden") {
		t.Fatal(m.renderTitleLine(), m.renderSummaryNote())
	}
}

func TestAuditTUIAgentFloorKeepsBlocked(t *testing.T) {
	blocked := &audit.Result{SkillName: "agent-blocked", IsBlocked: true, Findings: []audit.Finding{{Severity: audit.SeverityLow}}}
	hidden := &audit.Result{SkillName: "agent-allowed", Findings: []audit.Finding{{Severity: audit.SeverityLow}}}
	m := newAuditTUIModel(nil, nil, auditRunSummary{}, []*audit.Result{blocked, hidden}, nil, auditRunSummary{Scanned: 2, Failed: 1, Warning: 1, Low: 2}, auditTabAgents, audit.SeverityHigh)
	if len(m.allItems) != 1 || m.allItems[0].result.SkillName != "agent-blocked" || !strings.Contains(m.renderSummaryNote(), "0 visible, 2 hidden") {
		t.Fatal(m.allItems, m.renderSummaryNote())
	}
}

func TestAuditTUIFloorKeepsCleanResources(t *testing.T) {
	clean := &audit.Result{SkillName: "clean"}
	hidden := &audit.Result{SkillName: "low", Findings: []audit.Finding{{Severity: audit.SeverityLow}}}
	m := newAuditTUIModel([]*audit.Result{clean, hidden}, nil, auditRunSummary{Scanned: 2, Passed: 1, Warning: 1, Low: 1}, nil, nil, auditRunSummary{}, auditTabSkills, audit.SeverityHigh)
	if len(m.allItems) != 1 || m.allItems[0].result.SkillName != "clean" {
		t.Fatal(m.allItems)
	}
}

func TestAuditTUIShowsIncompleteScan(t *testing.T) {
	summary := auditRunSummary{Scanned: 2, SourceWarnings: []string{"source link _dev not followed: target is missing"}, Incomplete: true}
	m := newAuditTUIModel([]*audit.Result{{SkillName: "a"}, {SkillName: "b"}}, nil, summary, nil, nil, auditRunSummary{}, auditTabSkills, audit.SeverityHigh)
	m.termWidth = 180
	if note := m.renderSummaryNote(); !strings.Contains(note, "scan incomplete: 1 source link not followed") {
		t.Fatal(note)
	}
}

func TestLoweredFindingExplainsItself(t *testing.T) {
	f := audit.Finding{Severity: audit.SeverityHigh, Pattern: "credential-access", Message: "Reading accessing AWS credentials", File: "hook.py", Line: 3, Snippet: "# cat ~/.aws/credentials", SeverityContext: audit.SeverityContextPythonInert}
	r := &audit.Result{SkillName: "sentinel", Threshold: audit.SeverityCritical, Findings: []audit.Finding{f}}
	outputs := map[string]string{
		"text":   captureStdout(t, func() { printSkillResult(r, kindSkills, 0, "") }),
		"tui":    xansi.Strip(testAuditModel().renderDetailContent(auditItem{result: r})),
		"update": strings.Join(batchAuditEntryFromAuditResult("sentinel", r, false).warnings, "\n"),
	}
	files := testAuditModel()
	files.findings, files.finding = []audit.Finding{f}, 0
	outputs["tui files"] = xansi.Strip(files.findingNote())
	for name, got := range outputs {
		if !strings.Contains(got, "lowered from CRITICAL") {
			t.Errorf("%s output does not explain the lowered severity:\n%s", name, got)
		}
	}
	groups := groupAuditFindings(digestInstallWarnings(batchAuditEntryFromAuditResult("sentinel", r, false).warnings))
	if len(groups) != 1 || len(groups[0].locations) != 1 || groups[0].locations[0] != "hook.py:3" {
		t.Errorf("update warning no longer parses its location: %+v", groups)
	}
}
