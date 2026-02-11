package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestNormalizeThreshold(t *testing.T) {
	t.Parallel()

	got, err := NormalizeThreshold("")
	if err != nil {
		t.Fatalf("NormalizeThreshold empty returned error: %v", err)
	}
	if got != SeverityCritical {
		t.Fatalf("expected default %s, got %s", SeverityCritical, got)
	}

	got, err = NormalizeThreshold("high")
	if err != nil {
		t.Fatalf("NormalizeThreshold(high) returned error: %v", err)
	}
	if got != SeverityHigh {
		t.Fatalf("expected HIGH, got %s", got)
	}

	if _, err := NormalizeThreshold("urgent"); err == nil {
		t.Fatal("expected invalid threshold error")
	}
}

func TestResultHasSeverityAtOrAbove(t *testing.T) {
	t.Parallel()

	r := &Result{
		Findings: []Finding{
			{Severity: SeverityLow},
			{Severity: SeverityInfo},
		},
	}

	if r.HasSeverityAtOrAbove(SeverityMedium) {
		t.Fatal("did not expect LOW/INFO to block MEDIUM threshold")
	}
	if !r.HasSeverityAtOrAbove(SeverityLow) {
		t.Fatal("expected LOW to block LOW threshold")
	}
	if !r.HasSeverityAtOrAbove(SeverityInfo) {
		t.Fatal("expected INFO to block INFO threshold")
	}
}

func TestRiskScoreAndLabel(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
		{Severity: SeverityLow},
		{Severity: SeverityInfo},
	}
	score := CalculateRiskScore(findings)
	if score != 52 {
		t.Fatalf("expected risk score 52, got %d", score)
	}
	if label := RiskLabelFromScore(score); label != "high" {
		t.Fatalf("expected high label, got %s", label)
	}

	heavy := make([]Finding, 0, 10)
	for i := 0; i < 10; i++ {
		heavy = append(heavy, Finding{Severity: SeverityCritical})
	}
	if capped := CalculateRiskScore(heavy); capped != 100 {
		t.Fatalf("expected score cap 100, got %d", capped)
	}
}

func TestScanFileWithRules_Boundaries(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	rules := []rule{{
		ID:       "test-rule",
		Severity: SeverityHigh,
		Pattern:  "test",
		Message:  "test message",
		Regex:    regexp.MustCompile("DANGER"),
	}}

	regularFile := filepath.Join(tmp, "SKILL.md")
	if err := os.WriteFile(regularFile, []byte("DANGER here"), 0644); err != nil {
		t.Fatalf("write regular file: %v", err)
	}
	regular, err := ScanFileWithRules(regularFile, rules)
	if err != nil {
		t.Fatalf("ScanFileWithRules regular returned error: %v", err)
	}
	if len(regular.Findings) != 1 {
		t.Fatalf("expected one finding for regular file, got %d", len(regular.Findings))
	}

	binaryFile := filepath.Join(tmp, "binary.txt")
	if err := os.WriteFile(binaryFile, []byte{0, 'D', 'A', 'N', 'G', 'E', 'R'}, 0644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	binary, err := ScanFileWithRules(binaryFile, rules)
	if err != nil {
		t.Fatalf("ScanFileWithRules binary returned error: %v", err)
	}
	if len(binary.Findings) != 0 {
		t.Fatalf("expected binary file to be skipped, got %d findings", len(binary.Findings))
	}

	largeFile := filepath.Join(tmp, "large.md")
	oversize := make([]byte, maxScanFileSize+1)
	for i := range oversize {
		oversize[i] = 'A'
	}
	copy(oversize, []byte("DANGER"))
	if err := os.WriteFile(largeFile, oversize, 0644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	large, err := ScanFileWithRules(largeFile, rules)
	if err != nil {
		t.Fatalf("ScanFileWithRules large returned error: %v", err)
	}
	if len(large.Findings) != 0 {
		t.Fatalf("expected large file to be skipped, got %d findings", len(large.Findings))
	}
}
