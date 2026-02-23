package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

func TestDigestInstallWarnings_ClassifiesAuditAndNonAudit(t *testing.T) {
	warnings := []string{
		"audit HIGH: Python shell execution detected (SKILL.md:10)\n       \"subprocess.run(\"",
		"audit LOW: External URL in markdown link (README.md:4)",
		"audit findings detected, but none at/above block threshold (CRITICAL)",
		"audit scan error: timeout",
		"some non-audit warning",
	}

	digest := digestInstallWarnings(warnings)

	if got := digest.findingCounts["HIGH"]; got != 1 {
		t.Fatalf("expected HIGH=1, got %d", got)
	}
	if got := digest.findingCounts["LOW"]; got != 1 {
		t.Fatalf("expected LOW=1, got %d", got)
	}
	if len(digest.statusLines) != 1 {
		t.Fatalf("expected 1 status line, got %d", len(digest.statusLines))
	}
	if len(digest.otherAuditLines) != 1 {
		t.Fatalf("expected 1 other audit line, got %d", len(digest.otherAuditLines))
	}
	if len(digest.nonAuditLines) != 1 {
		t.Fatalf("expected 1 non-audit line, got %d", len(digest.nonAuditLines))
	}
}

func TestRenderInstallWarnings_CompactSuppressesExtraDetails(t *testing.T) {
	warnings := []string{
		"audit HIGH: Finding one (a.md:1)",
		"audit HIGH: Finding two (b.md:2)",
		"audit LOW: Finding three (c.md:3)",
		"audit LOW: Finding four (d.md:4)",
	}

	output := captureStdout(t, func() {
		renderInstallWarnings("demo-skill", warnings, false)
	})
	output = stripANSIWarnings(output)

	if !strings.Contains(output, "demo-skill: audit summary: HIGH=2, LOW=2") {
		t.Fatalf("expected compact summary in output, got:\n%s", output)
	}
	if !strings.Contains(output, "suppressed 1 audit finding line(s); re-run with --audit-verbose for full details") {
		t.Fatalf("expected suppression hint in output, got:\n%s", output)
	}
}

func TestSummarizeBatchInstallWarnings_AggregatesAcrossSkills(t *testing.T) {
	results := []skillInstallResult{
		{
			skill: install.SkillInfo{Name: "alpha"},
			warnings: []string{
				"audit HIGH: finding-1 (a.md:1)",
				"audit LOW: finding-2 (a.md:2)",
				"audit findings detected, but none at/above block threshold (CRITICAL)",
			},
		},
		{
			skill: install.SkillInfo{Name: "beta"},
			warnings: []string{
				"audit HIGH: finding-3 (b.md:1)",
				"audit HIGH: finding-4 (b.md:2)",
				"audit scan error: timeout",
			},
		},
		{
			skill: install.SkillInfo{Name: "gamma"},
			warnings: []string{
				"audit skipped (--skip-audit)",
				"custom warning",
			},
		},
	}

	summary := summarizeBatchInstallWarnings(results)

	if summary.totalFindings != 4 {
		t.Fatalf("expected total findings=4, got %d", summary.totalFindings)
	}
	if summary.findingCounts["HIGH"] != 3 {
		t.Fatalf("expected HIGH=3, got %d", summary.findingCounts["HIGH"])
	}
	if summary.findingCounts["LOW"] != 1 {
		t.Fatalf("expected LOW=1, got %d", summary.findingCounts["LOW"])
	}
	if summary.skillsWithFindings != 2 {
		t.Fatalf("expected skillsWithFindings=2, got %d", summary.skillsWithFindings)
	}
	if summary.belowThresholdSkillCount != 1 {
		t.Fatalf("expected belowThresholdSkillCount=1, got %d", summary.belowThresholdSkillCount)
	}
	if summary.scanErrorSkillCount != 1 {
		t.Fatalf("expected scanErrorSkillCount=1, got %d", summary.scanErrorSkillCount)
	}
	if summary.skippedAuditSkillCount != 1 {
		t.Fatalf("expected skippedAuditSkillCount=1, got %d", summary.skippedAuditSkillCount)
	}
	if summary.highCriticalBySkill["alpha"] != 1 || summary.highCriticalBySkill["beta"] != 2 {
		t.Fatalf("unexpected high/critical skill map: %#v", summary.highCriticalBySkill)
	}
	if len(summary.nonAuditLines) != 1 || summary.nonAuditLines[0] != "gamma: custom warning" {
		t.Fatalf("unexpected non-audit lines: %#v", summary.nonAuditLines)
	}
}

func TestRenderBatchInstallWarningsCompact_PrintsAggregateNotPerFinding(t *testing.T) {
	results := []skillInstallResult{
		{
			skill: install.SkillInfo{Name: "alpha"},
			warnings: []string{
				"audit HIGH: finding-1 (a.md:1)",
				"audit LOW: finding-2 (a.md:2)",
				"audit findings detected, but none at/above block threshold (CRITICAL)",
			},
		},
		{
			skill: install.SkillInfo{Name: "beta"},
			warnings: []string{
				"audit HIGH: finding-3 (b.md:1)",
				"audit HIGH: finding-4 (b.md:2)",
			},
		},
	}

	output := captureStdout(t, func() {
		renderBatchInstallWarningsCompact(results, 5)
	})
	output = stripANSIWarnings(output)

	if !strings.Contains(output, "audit findings across 2 skill(s): HIGH=3, LOW=1") {
		t.Fatalf("expected aggregate finding summary, got:\n%s", output)
	}
	if !strings.Contains(output, "skills with HIGH/CRITICAL findings: 2") {
		t.Fatalf("expected high/critical skill count, got:\n%s", output)
	}
	if !strings.Contains(output, "top HIGH/CRITICAL: beta(2), alpha(1)") {
		t.Fatalf("expected top high/critical list, got:\n%s", output)
	}
	if strings.Contains(output, "alpha: audit HIGH:") || strings.Contains(output, "beta: audit HIGH:") {
		t.Fatalf("expected no per-finding lines in compact batch output, got:\n%s", output)
	}
}

func TestParseAuditBlockedFailure_ExtractsThresholdAndFindingCount(t *testing.T) {
	message := "security audit failed — findings at/above CRITICAL detected:\n" +
		"  CRITICAL: Command may exfiltrate sensitive data (SKILL.md:123)\n" +
		"  CRITICAL: Prompt injection attempt detected (SKILL.md:456)\n\n" +
		"Use --force to override or --skip-audit to bypass scanning: blocked by security audit"

	digest := parseAuditBlockedFailure(message)
	if digest.threshold != "CRITICAL" {
		t.Fatalf("expected threshold CRITICAL, got %q", digest.threshold)
	}
	if digest.findingCount != 2 {
		t.Fatalf("expected findingCount 2, got %d", digest.findingCount)
	}
	if digest.firstFinding != "CRITICAL: Command may exfiltrate sensitive data (SKILL.md:123)" {
		t.Fatalf("unexpected first finding: %q", digest.firstFinding)
	}
}

func TestCompactInstallFailureMessage_RemovesRepeatedForceGuidance(t *testing.T) {
	raw := "security audit failed — findings at/above CRITICAL detected:\n" +
		"  CRITICAL: Accessing .env secrets file (SKILL.md:72)\n\n" +
		"Use --force to override or --skip-audit to bypass scanning: blocked by security audit"
	err := fmt.Errorf("%s: %w", raw, audit.ErrBlocked)

	msg := compactInstallFailureMessage(skillInstallResult{
		skill:   install.SkillInfo{Name: "linear-claude-skill"},
		success: false,
		message: raw,
		err:     err,
	})

	if strings.Contains(msg, "Use --force") {
		t.Fatalf("compact message should not include repeated force guidance: %q", msg)
	}
	if !strings.Contains(msg, "blocked by security audit (CRITICAL, 1 finding)") {
		t.Fatalf("unexpected compact blocked message: %q", msg)
	}
	if !strings.Contains(msg, "Accessing .env secrets file (SKILL.md:72)") {
		t.Fatalf("missing first finding detail in compact message: %q", msg)
	}
}

func TestPrintSkillListCompact_SmallList(t *testing.T) {
	skills := make([]install.SkillInfo, 5)
	for i := range skills {
		skills[i] = install.SkillInfo{Name: fmt.Sprintf("skill-%d", i), Path: fmt.Sprintf("path/%d", i)}
	}

	output := captureStdout(t, func() {
		printSkillListCompact(skills)
	})
	output = stripANSIWarnings(output)

	for _, s := range skills {
		if !strings.Contains(output, s.Name) {
			t.Fatalf("expected skill %q in output, got:\n%s", s.Name, output)
		}
	}
	if strings.Contains(output, "more skill(s)") {
		t.Fatalf("should not contain truncation message for small list, got:\n%s", output)
	}
}

func TestPrintSkillListCompact_LargeList(t *testing.T) {
	skills := make([]install.SkillInfo, 30)
	for i := range skills {
		skills[i] = install.SkillInfo{Name: fmt.Sprintf("skill-%02d", i), Path: fmt.Sprintf("path/%d", i)}
	}

	output := captureStdout(t, func() {
		printSkillListCompact(skills)
	})
	output = stripANSIWarnings(output)

	// First 10 should appear
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("skill-%02d", i)
		if !strings.Contains(output, name) {
			t.Fatalf("expected skill %q in output, got:\n%s", name, output)
		}
	}
	// skill-10 onward should NOT appear as individual lines
	if strings.Contains(output, "skill-10") && !strings.Contains(output, "20 more") {
		t.Fatalf("expected truncation for large list, got:\n%s", output)
	}
	if !strings.Contains(output, "... and 20 more skill(s)") {
		t.Fatalf("expected '... and 20 more skill(s)' in output, got:\n%s", output)
	}
}

func TestPrintSkillListCompact_ExactThreshold(t *testing.T) {
	skills := make([]install.SkillInfo, 20)
	for i := range skills {
		skills[i] = install.SkillInfo{Name: fmt.Sprintf("skill-%02d", i), Path: "."}
	}

	output := captureStdout(t, func() {
		printSkillListCompact(skills)
	})
	output = stripANSIWarnings(output)

	// All 20 should appear (≤20 threshold)
	if strings.Contains(output, "more skill(s)") {
		t.Fatalf("should not truncate at exactly 20 skills, got:\n%s", output)
	}
}

func TestCountSkillsWithWarnings(t *testing.T) {
	results := []skillInstallResult{
		{skill: install.SkillInfo{Name: "a"}, warnings: []string{"w1"}},
		{skill: install.SkillInfo{Name: "b"}, warnings: nil},
		{skill: install.SkillInfo{Name: "c"}, warnings: []string{"w2", "w3"}},
	}
	if got := countSkillsWithWarnings(results); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
}

func TestSortResultsByHighCritical(t *testing.T) {
	results := []skillInstallResult{
		{skill: install.SkillInfo{Name: "low-only"}, warnings: []string{"audit LOW: f1"}},
		{skill: install.SkillInfo{Name: "two-high"}, warnings: []string{"audit HIGH: f1", "audit HIGH: f2"}},
		{skill: install.SkillInfo{Name: "one-crit"}, warnings: []string{"audit CRITICAL: f1"}},
		{skill: install.SkillInfo{Name: "no-warn"}, warnings: nil},
	}

	sorted := sortResultsByHighCritical(results)
	if sorted[0].skill.Name != "two-high" {
		t.Fatalf("expected two-high first, got %s", sorted[0].skill.Name)
	}
	if sorted[1].skill.Name != "one-crit" {
		t.Fatalf("expected one-crit second, got %s", sorted[1].skill.Name)
	}
}

func TestRenderInstallWarningsHighCriticalOnly_FiltersLowFindings(t *testing.T) {
	warnings := []string{
		"audit CRITICAL: Critical finding (a.md:1)",
		"audit HIGH: High finding (b.md:2)",
		"audit LOW: Low finding one (c.md:3)",
		"audit LOW: Low finding two (d.md:4)",
		"audit LOW: Low finding three (e.md:5)",
		"audit INFO: Info finding (f.md:6)",
		"audit findings detected, but none at/above block threshold (CRITICAL)",
	}

	output := captureStdout(t, func() {
		renderInstallWarningsHighCriticalOnly("test-skill", warnings)
	})
	output = stripANSIWarnings(output)

	// HIGH and CRITICAL should appear as full lines
	if !strings.Contains(output, "test-skill: audit CRITICAL: Critical finding") {
		t.Fatalf("expected CRITICAL finding in output, got:\n%s", output)
	}
	if !strings.Contains(output, "test-skill: audit HIGH: High finding") {
		t.Fatalf("expected HIGH finding in output, got:\n%s", output)
	}
	// LOW/INFO should NOT appear as individual lines
	if strings.Contains(output, "Low finding one") {
		t.Fatalf("LOW findings should be summarized, not shown individually, got:\n%s", output)
	}
	// Should have a summary line for LOW and INFO
	if !strings.Contains(output, "also: LOW=3, INFO=1") {
		t.Fatalf("expected summary for LOW/INFO, got:\n%s", output)
	}
}

func TestDisplayInstallResults_VerboseLargeBatch_ShowsCompactThenHighCritical(t *testing.T) {
	// Build 25 results with warnings (>20 threshold)
	results := make([]skillInstallResult, 25)
	for i := range results {
		name := fmt.Sprintf("skill-%02d", i)
		w := []string{fmt.Sprintf("audit LOW: finding in %s", name)}
		if i < 3 {
			// First 3 have HIGH findings
			w = append(w, fmt.Sprintf("audit HIGH: critical finding in %s", name))
		}
		results[i] = skillInstallResult{
			skill:    install.SkillInfo{Name: name},
			success:  true,
			message:  "installed",
			warnings: w,
		}
	}

	output := captureStdout(t, func() {
		spinner := &ui.Spinner{}
		displayInstallResults(results, spinner, true)
	})
	output = stripANSIWarnings(output)

	// Should contain compact batch summary
	if !strings.Contains(output, "compact batch view") {
		t.Fatalf("expected compact batch view for large verbose batch, got:\n%s", output)
	}
	// Should contain verbose detail header
	if !strings.Contains(output, "HIGH/CRITICAL detail (top skills)") {
		t.Fatalf("expected verbose detail header, got:\n%s", output)
	}
	// Should mention remaining skills
	if !strings.Contains(output, "more skill(s) with findings") {
		t.Fatalf("expected remaining count, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close write pipe: %v", err)
	}
	os.Stdout = old

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}

	return string(data)
}

func stripANSIWarnings(s string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(s, "")
}
