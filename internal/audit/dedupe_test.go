package audit

import "testing"

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
