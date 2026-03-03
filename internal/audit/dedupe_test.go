package audit

import (
	"strings"
	"testing"
)

func TestDeduplicateGlobal_Empty(t *testing.T) {
	result := DeduplicateGlobal(nil)
	if len(result) != 0 {
		t.Errorf("expected nil, got %d findings", len(result))
	}
}

func TestDeduplicateGlobal_NoDuplicates(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
		{Severity: SeverityHigh, Pattern: "b", File: "f2", Line: 2, Snippet: "y"},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestDeduplicateGlobal_ExactDuplicate(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
		{Severity: SeverityHigh, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 1 {
		t.Errorf("expected 1, got %d", len(result))
	}
}

func TestDeduplicateGlobal_KeepsHigherSeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityMedium, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
		{Severity: SeverityCritical, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Severity != SeverityCritical {
		t.Errorf("severity = %q, want CRITICAL", result[0].Severity)
	}
}

func TestDeduplicateGlobal_WhitespaceNormalization(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Pattern: "a", File: "f1", Line: 1, Snippet: "  foo   bar  "},
		{Severity: SeverityHigh, Pattern: "a", File: "f1", Line: 1, Snippet: "foo bar"},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 1 {
		t.Errorf("expected 1 after whitespace normalization, got %d", len(result))
	}
}

func TestDeduplicateGlobal_PreservesOrder(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh, Pattern: "c", File: "f3", Line: 3, Snippet: "z"},
		{Severity: SeverityCritical, Pattern: "a", File: "f1", Line: 1, Snippet: "x"},
		{Severity: SeverityMedium, Pattern: "b", File: "f2", Line: 2, Snippet: "y"},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	if result[0].Pattern != "c" || result[1].Pattern != "a" || result[2].Pattern != "b" {
		t.Errorf("order changed: %s, %s, %s", result[0].Pattern, result[1].Pattern, result[2].Pattern)
	}
}

func TestComputeFingerprint_Stable(t *testing.T) {
	f := Finding{
		RuleID:   "shell-exec",
		Pattern:  "shell-exec",
		Analyzer: "static",
		File:     "install.sh",
		Line:     10,
		Snippet:  "eval $USER_INPUT",
	}
	fp1 := ComputeFingerprint(f)
	fp2 := ComputeFingerprint(f)
	if fp1 != fp2 {
		t.Errorf("fingerprint not stable: %s != %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("expected 64-char hex, got %d chars", len(fp1))
	}
}

func TestComputeFingerprint_CaseInsensitive(t *testing.T) {
	f1 := Finding{Pattern: "Shell-Exec", File: "INSTALL.sh", Line: 1, Snippet: "Eval"}
	f2 := Finding{Pattern: "shell-exec", File: "install.sh", Line: 1, Snippet: "eval"}
	if ComputeFingerprint(f1) != ComputeFingerprint(f2) {
		t.Error("fingerprint should be case-insensitive")
	}
}

func TestComputeFingerprint_DifferentFields(t *testing.T) {
	base := Finding{Pattern: "a", File: "f", Line: 1, Snippet: "s"}
	diff := Finding{Pattern: "a", File: "f", Line: 2, Snippet: "s"}
	if ComputeFingerprint(base) == ComputeFingerprint(diff) {
		t.Error("different line should produce different fingerprint")
	}
}

func TestComputeFingerprint_WhitespaceNormalized(t *testing.T) {
	f1 := Finding{Pattern: "a", File: "f", Line: 1, Snippet: "  foo   bar  "}
	f2 := Finding{Pattern: "a", File: "f", Line: 1, Snippet: "foo bar"}
	if ComputeFingerprint(f1) != ComputeFingerprint(f2) {
		t.Error("whitespace-normalized snippets should have same fingerprint")
	}
}

func TestDeduplicateGlobal_PrefersFingerprint(t *testing.T) {
	fp := strings.Repeat("a", 64) // fake fingerprint
	findings := []Finding{
		{Severity: SeverityHigh, Pattern: "x", File: "f1", Line: 1, Snippet: "s", Fingerprint: fp},
		{Severity: SeverityCritical, Pattern: "y", File: "f2", Line: 2, Snippet: "t", Fingerprint: fp},
	}
	result := DeduplicateGlobal(findings)
	if len(result) != 1 {
		t.Fatalf("expected 1 (same fingerprint), got %d", len(result))
	}
	if result[0].Severity != SeverityCritical {
		t.Errorf("severity = %q, want CRITICAL", result[0].Severity)
	}
}

func TestNormalizeSnippet(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"  foo   bar  ", "foo bar"},
		{"hello\tworld", "hello world"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"", ""},
	}
	for _, tc := range tests {
		got := normalizeSnippet(tc.in)
		if got != tc.want {
			t.Errorf("normalizeSnippet(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
