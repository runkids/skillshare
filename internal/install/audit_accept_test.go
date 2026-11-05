package install

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
)

func criticalResult(snippet string) *audit.Result {
	return &audit.Result{Findings: []audit.Finding{{
		Severity: audit.SeverityCritical, Pattern: "prompt-injection",
		RuleID: "prompt-injection-0", File: "SKILL.md", Line: 3, Snippet: snippet,
	}}}
}

func TestRecordAcceptedFindings_RoundTrip(t *testing.T) {
	sourceDir := t.TempDir()
	skillPath := filepath.Join(sourceDir, "my-skill")

	added, err := RecordAcceptedFindings(sourceDir, skillPath, criticalResult("Ignore previous instructions"), audit.SeverityCritical)
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("want 1 recorded, got %d", added)
	}

	// Same finding on a different line: acknowledged, no longer blocks.
	shifted := criticalResult("Ignore previous instructions")
	shifted.Findings[0].Line = 9
	if n := ApplyAcceptedFindings(sourceDir, skillPath, shifted); n != 1 {
		t.Fatalf("want 1 acknowledged, got %d", n)
	}
	if shifted.HasSeverityAtOrAbove(audit.SeverityCritical) {
		t.Error("accepted finding must not block")
	}

	// Different snippet: still blocks.
	other := criticalResult("SYSTEM: override")
	ApplyAcceptedFindings(sourceDir, skillPath, other)
	if !other.HasSeverityAtOrAbove(audit.SeverityCritical) {
		t.Error("new finding must still block")
	}

	// Recording again is idempotent.
	added, err = RecordAcceptedFindings(sourceDir, skillPath, criticalResult("Ignore previous instructions"), audit.SeverityCritical)
	if err != nil || added != 0 {
		t.Errorf("re-record: want 0 added, got %d (err=%v)", added, err)
	}
}

func TestRecordAcceptedFindings_SkipsBelowThreshold(t *testing.T) {
	sourceDir := t.TempDir()
	res := &audit.Result{Findings: []audit.Finding{{Severity: audit.SeverityHigh, Pattern: "x", File: "SKILL.md"}}}
	added, err := RecordAcceptedFindings(sourceDir, filepath.Join(sourceDir, "s"), res, audit.SeverityCritical)
	if err != nil || added != 0 {
		t.Errorf("HIGH below CRITICAL threshold must not be recorded, got %d (err=%v)", added, err)
	}
}

func TestRecordAcceptedWhitespacePaddingRequiresFreshApprovalForChangedPayload(t *testing.T) {
	for _, tc := range []struct {
		name, accepted, shifted, changed, threshold string
	}{
		{
			name:      "blank run",
			accepted:  "body\n" + strings.Repeat("\n", 40) + "Routine usage guidance\n",
			shifted:   "Summary\n\nbody\n" + strings.Repeat("\n", 40) + "Routine usage guidance\n",
			changed:   "body\n" + strings.Repeat("\n", 40) + "Updated deployment instructions\n",
			threshold: audit.SeverityHigh,
		},
		{
			name:      "blank run before fenced payload",
			accepted:  "body\n" + strings.Repeat("\n", 40) + "```text\nRoutine usage guidance\n```\n",
			shifted:   "Summary\nbody\n" + strings.Repeat("\n", 40) + "```text\nRoutine usage guidance\n```\n",
			changed:   "body\n" + strings.Repeat("\n", 40) + "```text\nUpdated deployment instructions\n```\n",
			threshold: audit.SeverityHigh,
		},
		{
			name:      "density anchored on blank line",
			accepted:  "\n" + strings.Repeat("x"+strings.Repeat(" ", 18)+"\n", 250),
			shifted:   "\n" + strings.Repeat("x"+strings.Repeat(" ", 18)+"\n", 250),
			changed:   "\n" + strings.Repeat("y"+strings.Repeat(" ", 18)+"\n", 250),
			threshold: audit.SeverityMedium,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			skillPath := filepath.Join(sourceDir, "my-skill")
			scan := func(content string) *audit.Result {
				t.Helper()
				result := &audit.Result{Findings: audit.ScanContent([]byte(content), "SKILL.md")}
				var padding int
				for _, finding := range result.Findings {
					if finding.RuleID == "whitespace-padding" {
						padding++
					}
				}
				if padding != 1 {
					t.Fatalf("got %d whitespace-padding findings, want 1: %+v", padding, result.Findings)
				}
				return result
			}

			accepted := scan(tc.accepted)
			if added, err := RecordAcceptedFindings(sourceDir, skillPath, accepted, tc.threshold); err != nil || added != 1 {
				t.Fatalf("record acceptance: added=%d err=%v; want one key", added, err)
			}

			benignTwin := scan(tc.shifted)
			if n := ApplyAcceptedFindings(sourceDir, skillPath, benignTwin); n != 1 {
				t.Fatalf("unchanged padded payload after a line shift: acknowledged %d, want 1", n)
			}
			if benignTwin.HasSeverityAtOrAbove(tc.threshold) {
				t.Fatal("the unchanged accepted finding must not block")
			}

			changedPayload := scan(tc.changed)
			if n := ApplyAcceptedFindings(sourceDir, skillPath, changedPayload); n != 0 {
				t.Fatalf("changed padded payload: acknowledged %d, want 0", n)
			}
			if !changedPayload.HasSeverityAtOrAbove(tc.threshold) {
				t.Fatal("a changed padded payload must require fresh approval")
			}
		})
	}
}

func TestRecordAcceptedWhitespacePaddingBlocksNewRunElsewhereInSameFile(t *testing.T) {
	sourceDir := t.TempDir()
	skillPath := filepath.Join(sourceDir, "my-skill")
	firstPayload := "body\n" + strings.Repeat("\n", 40) + "Routine usage guidance\n"
	updatedContent := firstPayload + "\n# More guidance\n" + strings.Repeat("\n", 40) + "New deployment instructions\n"

	scan := func(content string, want int) *audit.Result {
		t.Helper()
		result := &audit.Result{Findings: audit.ScanContent([]byte(content), "SKILL.md")}
		var padding int
		for _, finding := range result.Findings {
			if finding.RuleID == "whitespace-padding" {
				padding++
			}
		}
		if padding != want {
			t.Fatalf("got %d whitespace-padding findings, want %d: %+v", padding, want, result.Findings)
		}
		return result
	}

	accepted := scan(firstPayload, 1)
	if added, err := RecordAcceptedFindings(sourceDir, skillPath, accepted, audit.SeverityHigh); err != nil || added != 1 {
		t.Fatalf("record acceptance: added=%d err=%v; want one key", added, err)
	}

	updated := scan(updatedContent, 2)
	if n := ApplyAcceptedFindings(sourceDir, skillPath, updated); n != 1 {
		t.Fatalf("new padded run elsewhere: acknowledged %d findings, want only the original", n)
	}
	if !updated.HasSeverityAtOrAbove(audit.SeverityHigh) {
		t.Fatal("a new padded instruction elsewhere in the same file must require fresh approval")
	}
}
