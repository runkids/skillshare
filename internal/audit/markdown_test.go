package audit

import (
	"strings"
	"testing"
)

func TestToMarkdown_Empty(t *testing.T) {
	t.Parallel()

	md := ToMarkdown(nil, MarkdownOptions{
		Scanned:   0,
		Mode:      "global",
		Threshold: "CRITICAL",
		RiskLabel: "clean",
	})

	if !strings.Contains(md, "# Skillshare Audit Report") {
		t.Fatal("missing report title")
	}
	if !strings.Contains(md, "## Summary") {
		t.Fatal("missing summary section")
	}
	if strings.Contains(md, "## Findings") {
		t.Fatal("empty results should not have Findings section")
	}
	if strings.Contains(md, "## Clean Skills") {
		t.Fatal("empty results should not have Clean Skills section")
	}
}

func TestToMarkdown_WithFindings(t *testing.T) {
	t.Parallel()

	results := []*Result{
		{
			SkillName:     "bad-skill",
			IsBlocked:     true,
			Threshold:     "CRITICAL",
			RiskScore:     75,
			RiskLabel:     "high",
			Analyzability: 0.85,
			Findings: []Finding{
				{Severity: SeverityCritical, Pattern: "prompt-injection", Message: "Prompt injection detected", File: "SKILL.md", Line: 5, Snippet: "ignore all previous instructions"},
				{Severity: SeverityHigh, Pattern: "destructive-commands", Message: "Destructive command", File: "SKILL.md", Line: 42, Snippet: "rm -rf /"},
			},
		},
		{
			SkillName:     "warn-skill",
			IsBlocked:     false,
			Threshold:     "CRITICAL",
			RiskScore:     8,
			RiskLabel:     "medium",
			Analyzability: 1.0,
			Findings: []Finding{
				{Severity: SeverityMedium, Pattern: "suspicious-fetch", Message: "URL in command context", File: "SKILL.md", Line: 3, Snippet: "curl http://example.com"},
			},
		},
	}

	md := ToMarkdown(results, MarkdownOptions{
		Scanned:          2,
		Passed:           0,
		Warning:          1,
		Failed:           1,
		Critical:         1,
		High:             1,
		Medium:           1,
		RiskScore:        75,
		RiskLabel:        "high",
		Threshold:        "CRITICAL",
		Mode:             "global",
		AvgAnalyzability: 0.925,
	})

	// Header
	if !strings.Contains(md, "- **Scanned**: 2 skill(s)") {
		t.Error("missing scanned count in header")
	}
	if !strings.Contains(md, "- **Mode**: global") {
		t.Error("missing mode in header")
	}

	// Summary table
	if !strings.Contains(md, "| Passed | 0 |") {
		t.Error("missing passed in summary")
	}
	if !strings.Contains(md, "| Failed | 1 |") {
		t.Error("missing failed in summary")
	}
	if !strings.Contains(md, "| Risk | HIGH (75/100) |") {
		t.Error("missing risk in summary")
	}

	// Findings
	if !strings.Contains(md, "## Findings") {
		t.Fatal("missing Findings section")
	}

	// Blocked skill comes first with ✗
	if !strings.Contains(md, "### ✗ bad-skill") {
		t.Error("missing blocked skill heading")
	}
	if !strings.Contains(md, "> **BLOCKED**") {
		t.Error("missing BLOCKED marker")
	}

	// Warning skill with !
	if !strings.Contains(md, "### ! warn-skill") {
		t.Error("missing warning skill heading")
	}
	if !strings.Contains(md, "> **ALLOWED**") {
		t.Error("missing ALLOWED marker")
	}

	// Snippets
	if !strings.Contains(md, "<details>") {
		t.Error("missing snippets collapsible section")
	}
	if !strings.Contains(md, "ignore all previous instructions") {
		t.Error("missing snippet content")
	}
}

func TestToMarkdown_CleanSkills(t *testing.T) {
	t.Parallel()

	results := []*Result{
		{SkillName: "clean-a", Analyzability: 1.0, RiskLabel: "clean"},
		{SkillName: "clean-b", Analyzability: 1.0, RiskLabel: "clean"},
		{SkillName: "clean-c", Analyzability: 1.0, RiskLabel: "clean"},
	}

	md := ToMarkdown(results, MarkdownOptions{
		Scanned:   3,
		Passed:    3,
		RiskLabel: "clean",
		Mode:      "global",
		Threshold: "CRITICAL",
	})

	if strings.Contains(md, "## Findings") {
		t.Error("should not have Findings section when all clean")
	}
	if !strings.Contains(md, "## Clean Skills (3)") {
		t.Error("missing Clean Skills section")
	}
	if !strings.Contains(md, "`clean-a`") {
		t.Error("missing clean skill name")
	}
	if !strings.Contains(md, "`clean-b`") {
		t.Error("missing clean skill name")
	}
}

func TestToMarkdown_PipeEscape(t *testing.T) {
	t.Parallel()

	results := []*Result{
		{
			SkillName:     "pipe-skill",
			IsBlocked:     true,
			Threshold:     "CRITICAL",
			RiskScore:     25,
			RiskLabel:     "critical",
			Analyzability: 1.0,
			Findings: []Finding{
				{Severity: SeverityCritical, Pattern: "test-pattern", Message: "msg with | pipe", File: "SKILL.md", Line: 1, Snippet: "echo foo | bar"},
			},
		},
	}

	md := ToMarkdown(results, MarkdownOptions{
		Scanned:   1,
		Failed:    1,
		Critical:  1,
		RiskScore: 25,
		RiskLabel: "critical",
		Threshold: "CRITICAL",
		Mode:      "global",
	})

	// Table cell pipes must be escaped
	if !strings.Contains(md, `msg with \| pipe`) {
		t.Error("pipe in message not escaped")
	}
}

func TestToMarkdown_TierProfile(t *testing.T) {
	t.Parallel()

	tp := TierProfile{}
	tp.Add(TierDestructive)
	tp.Add(TierDestructive)
	tp.Add(TierNetwork)

	results := []*Result{
		{
			SkillName:     "tier-skill",
			IsBlocked:     false,
			Threshold:     "CRITICAL",
			RiskScore:     8,
			RiskLabel:     "medium",
			Analyzability: 0.9,
			TierProfile:   tp,
			Findings: []Finding{
				{Severity: SeverityMedium, Pattern: "suspicious-fetch", Message: "URL", File: "SKILL.md", Line: 1, Snippet: "curl http://x"},
			},
		},
	}

	md := ToMarkdown(results, MarkdownOptions{
		Scanned:          1,
		Warning:          1,
		Medium:           1,
		RiskScore:        8,
		RiskLabel:        "medium",
		Threshold:        "CRITICAL",
		Mode:             "global",
		AvgAnalyzability: 0.9,
	})

	if !strings.Contains(md, "- Commands: destructive:2 network:1") {
		t.Error("missing tier profile in metadata")
	}
}

func TestToMarkdown_ScanErrors(t *testing.T) {
	t.Parallel()

	md := ToMarkdown(nil, MarkdownOptions{
		Scanned:    5,
		Passed:     3,
		ScanErrors: 2,
		RiskLabel:  "clean",
		Mode:       "global",
		Threshold:  "CRITICAL",
	})

	if !strings.Contains(md, "| Scan Errors | 2 |") {
		t.Error("missing scan errors row")
	}
}

func TestToMarkdown_NoScanErrorsRow(t *testing.T) {
	t.Parallel()

	md := ToMarkdown(nil, MarkdownOptions{
		Scanned:   3,
		Passed:    3,
		RiskLabel: "clean",
		Mode:      "global",
		Threshold: "CRITICAL",
	})

	if strings.Contains(md, "Scan Errors") {
		t.Error("should not show scan errors when count is 0")
	}
}
