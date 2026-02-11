package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"skillshare/internal/oplog"
)

var testANSIRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return testANSIRegex.ReplaceAllString(s, "")
}

func TestPrintLogEntriesTTYTwoLine_Basic(t *testing.T) {
	entries := []oplog.Entry{
		{
			Timestamp: "2026-02-10T03:28:00Z",
			Command:   "sync",
			Args: map[string]any{
				"targets": 1,
				"scope":   "project",
			},
			Status:   "ok",
			Duration: 32,
		},
	}

	var buf bytes.Buffer
	printLogEntriesTTYTwoLine(&buf, entries, 120)

	out := stripANSI(buf.String())
	if !strings.Contains(out, "TIME") || !strings.Contains(out, "CMD") {
		t.Fatalf("expected table header, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-02-10 03:28 | SYNC") {
		t.Fatalf("expected core row with timestamp and command, got:\n%s", out)
	}
	if !strings.Contains(out, "ok") || !strings.Contains(out, "32ms") {
		t.Fatalf("expected status and duration, got:\n%s", out)
	}
	if !strings.Contains(out, "targets: 1") {
		t.Fatalf("expected targets detail line, got:\n%s", out)
	}
	if !strings.Contains(out, "scope: project") {
		t.Fatalf("expected scope detail line, got:\n%s", out)
	}
}

func TestPrintLogEntriesTTYTwoLine_LongDetailWrapsWithoutTruncation(t *testing.T) {
	entry := oplog.Entry{
		Timestamp: "2026-02-10T03:04:00Z",
		Command:   "install",
		Args: map[string]any{
			"source": "https://github.com/openai/skills/tree/main/very/long/path/with/many/segments",
		},
		Message:  "tracked repo '_openai-skills' already exists and needs overwrite flag to proceed safely",
		Status:   "error",
		Duration: 2150,
	}

	// TTY detail should not be truncated.
	detail := formatLogDetail(entry, false)
	if strings.Contains(detail, "...") {
		t.Fatalf("expected untruncated detail in TTY mode, got: %s", detail)
	}
	if len(detail) <= logDetailTruncateLen {
		t.Fatalf("expected long detail (> %d), got len=%d", logDetailTruncateLen, len(detail))
	}

	var buf bytes.Buffer
	printLogEntriesTTYTwoLine(&buf, []oplog.Entry{entry}, 62)
	out := stripANSI(buf.String())

	if lines := strings.Count(out, "\n"); lines < 4 {
		t.Fatalf("expected wrapped multi-line output, got:\n%s", out)
	}
	if !strings.Contains(out, "overwrite") || !strings.Contains(out, "proceed") || !strings.Contains(out, "safely") {
		t.Fatalf("expected full detail content to be preserved, got:\n%s", out)
	}
}

func TestPrintLogEntriesTTYTwoLine_BlankLineBetweenEntries(t *testing.T) {
	entries := []oplog.Entry{
		{
			Timestamp: "2026-02-10T03:28:00Z",
			Command:   "sync",
			Args:      map[string]any{"targets": 1},
			Status:    "ok",
			Duration:  32,
		},
		{
			Timestamp: "2026-02-10T03:29:00Z",
			Command:   "audit",
			Args:      map[string]any{"scope": "all", "scanned": 5, "passed": 5},
			Status:    "ok",
			Duration:  100,
		},
	}

	var buf bytes.Buffer
	printLogEntriesTTYTwoLine(&buf, entries, 120)
	out := stripANSI(buf.String())

	// Find lines for both entries
	lines := strings.Split(out, "\n")
	syncIdx, auditIdx := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "SYNC") {
			syncIdx = i
		}
		if strings.Contains(line, "AUDIT") {
			auditIdx = i
		}
	}
	if syncIdx < 0 || auditIdx < 0 {
		t.Fatalf("expected both SYNC and AUDIT entries, got:\n%s", out)
	}

	// There should be a blank line between the last detail of SYNC and the AUDIT header
	hasBlank := false
	for i := syncIdx + 1; i < auditIdx; i++ {
		if strings.TrimSpace(lines[i]) == "" {
			hasBlank = true
			break
		}
	}
	if !hasBlank {
		t.Fatalf("expected blank line between entries, got:\n%s", out)
	}
}

func TestPrintLogEntriesTTYTwoLine_NoDetailOnlyCoreLine(t *testing.T) {
	entry := oplog.Entry{
		Timestamp: "2026-02-10T03:05:00Z",
		Command:   "sync",
		Status:    "ok",
		Duration:  2,
	}

	var buf bytes.Buffer
	printLogEntriesTTYTwoLine(&buf, []oplog.Entry{entry}, 100)
	out := stripANSI(buf.String())

	if strings.Contains(out, "detail:") {
		t.Fatalf("did not expect detail line, got:\n%s", out)
	}
}

func TestPrintLogEntriesTTYTwoLine_AuditSkillsWrappedAndIndented(t *testing.T) {
	entry := oplog.Entry{
		Timestamp: "2026-02-10T03:45:00Z",
		Command:   "audit",
		Args: map[string]any{
			"scope":   "all",
			"scanned": 33,
			"passed":  29,
			"failed":  4,
			"failed_skills": []any{
				"cloudflare-deploy",
				"doc",
				"pdf",
				"spreadsheet",
				"ultra-long-skill-name-for-wrap-checking",
			},
		},
		Status:   "blocked",
		Duration: 906,
	}

	var buf bytes.Buffer
	printLogEntriesTTYTwoLine(&buf, []oplog.Entry{entry}, 64)
	out := stripANSI(buf.String())

	if !strings.Contains(out, "failed skills:") {
		t.Fatalf("expected failed skills line, got:\n%s", out)
	}
	if !strings.Contains(out, "- cloudflare-deploy") {
		t.Fatalf("expected failed skill list item, got:\n%s", out)
	}
	if !strings.Contains(out, "ultra-long-skill-name-for-wrap-checking") {
		t.Fatalf("expected full failed skill list content, got:\n%s", out)
	}
}

func TestPrintLogEntriesNonTTY_RemainsSingleLineAndTruncated(t *testing.T) {
	entry := oplog.Entry{
		Timestamp: "2026-02-10T03:04:00Z",
		Command:   "install",
		Args: map[string]any{
			"source": "https://github.com/openai/skills/tree/main/very/long/path/with/many/segments",
		},
		Message:  "tracked repo '_openai-skills' already exists and needs overwrite flag to proceed safely",
		Status:   "error",
		Duration: 155,
	}

	var buf bytes.Buffer
	printLogEntriesNonTTY(&buf, []oplog.Entry{entry})
	out := buf.String()

	if strings.Contains(out, "TIME | CMD | STATUS | DUR") {
		t.Fatalf("did not expect table header in non-TTY output, got:\n%s", out)
	}
	if !strings.Contains(out, "install") || !strings.Contains(out, "error") {
		t.Fatalf("expected classic non-TTY row format, got:\n%s", out)
	}
	if !strings.Contains(out, "...") {
		t.Fatalf("expected truncated detail in non-TTY output, got:\n%s", out)
	}
}

func TestFormatInstallLogDetail_IncludesInstalledSkills(t *testing.T) {
	args := map[string]any{
		"source":           "https://github.com/example/skills",
		"mode":             "project",
		"threshold":        "high",
		"skip_audit":       true,
		"skill_count":      2,
		"installed_skills": []string{"skill-a", "skill-b"},
		"failed_skills":    []string{"skill-c"},
	}

	detail := formatInstallLogDetail(args)
	if !strings.Contains(detail, "mode=project") {
		t.Fatalf("expected mode in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "threshold=HIGH") {
		t.Fatalf("expected threshold in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "skills=2") {
		t.Fatalf("expected skill count in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "skip-audit") {
		t.Fatalf("expected skip-audit in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "installed=skill-a, skill-b") {
		t.Fatalf("expected installed skills in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "failed=skill-c") {
		t.Fatalf("expected failed skills in detail, got: %s", detail)
	}
}

func TestFormatAuditLogDetail_IncludesExtendedFields(t *testing.T) {
	args := map[string]any{
		"scope":      "all",
		"mode":       "global",
		"threshold":  "medium",
		"scanned":    10,
		"passed":     5,
		"warning":    3,
		"failed":     2,
		"critical":   2,
		"high":       1,
		"medium":     2,
		"low":        4,
		"info":       6,
		"risk_score": 67,
		"risk_label": "high",
	}

	detail := formatAuditLogDetail(args)
	if !strings.Contains(detail, "threshold=MEDIUM") {
		t.Fatalf("expected threshold in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "sev(c/h/m/l/i)=2/1/2/4/6") {
		t.Fatalf("expected extended severity summary in detail, got: %s", detail)
	}
	if !strings.Contains(detail, "risk=HIGH(67/100)") {
		t.Fatalf("expected risk score/label in detail, got: %s", detail)
	}
}

func TestPrintLogAuditSkillLinesNonTTY_IncludesLowAndInfoSkills(t *testing.T) {
	entry := oplog.Entry{
		Command: "audit",
		Args: map[string]any{
			"failed_skills":  []string{"critical-a"},
			"warning_skills": []string{"high-a"},
			"low_skills":     []string{"low-a"},
			"info_skills":    []string{"info-a"},
		},
	}

	var buf bytes.Buffer
	printLogAuditSkillLinesNonTTY(&buf, entry)
	out := buf.String()

	if !strings.Contains(out, "failed skills: critical-a") {
		t.Fatalf("expected failed skills output, got:\n%s", out)
	}
	if !strings.Contains(out, "warning skills: high-a") {
		t.Fatalf("expected warning skills output, got:\n%s", out)
	}
	if !strings.Contains(out, "low skills: low-a") {
		t.Fatalf("expected low skills output, got:\n%s", out)
	}
	if !strings.Contains(out, "info skills: info-a") {
		t.Fatalf("expected info skills output, got:\n%s", out)
	}
}
