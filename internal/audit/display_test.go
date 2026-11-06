package audit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDisplayFindings(t *testing.T) {
	levels := []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
	findings := make([]Finding, len(levels))
	for i, s := range levels {
		findings[i] = Finding{Severity: s, Message: s + "|message", Snippet: s + " snippet"}
	}
	original := append([]Finding(nil), findings...)
	for i, floor := range levels {
		visible := DisplayFindings(findings, floor)
		if !reflect.DeepEqual(visible, findings[:i+1]) {
			t.Fatalf("%s: %v", floor, visible)
		}
		r := &Result{SkillName: "mixed", Findings: findings, IsBlocked: true, Threshold: SeverityLow, RiskScore: 85}
		md := ToMarkdown([]*Result{r}, MarkdownOptions{MinSeverity: floor, ScanErrors: 1, RiskScore: 85})
		if !strings.Contains(md, fmt.Sprintf("%d visible, %d hidden", i+1, 4-i)) || !strings.Contains(md, "**BLOCKED**") || !strings.Contains(md, "| Scan Errors | 1 |") {
			t.Fatal(md)
		}
		for j, s := range levels {
			if strings.Contains(md, s+" snippet") != (j <= i) || strings.Contains(md, s+"\\|message") != (j <= i) {
				t.Fatalf("%s: %s", floor, md)
			}
		}
	}
	if !reflect.DeepEqual(findings, original) {
		t.Fatal("display mutated findings")
	}
	if got := DisplayFindings(findings, ""); !reflect.DeepEqual(got, findings) {
		t.Fatal(got)
	}
	for _, findings := range [][]Finding{nil, {{Severity: SeverityLow, Message: "hidden"}}} {
		md := ToMarkdown([]*Result{{SkillName: "low", Findings: findings, RiskScore: 15}}, MarkdownOptions{MinSeverity: SeverityHigh})
		if !strings.Contains(md, fmt.Sprintf("0 visible, %d hidden", len(findings))) || strings.Contains(md, "| hidden |") {
			t.Fatal(md)
		}
	}
}

// Any valid floor selects an inclusive prefix without changing its input.
func FuzzDisplayFindings(f *testing.F) {
	f.Add(uint8(0), uint8(4))
	f.Add(uint8(4), uint8(0))
	f.Fuzz(func(t *testing.T, a, b uint8) {
		levels := []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
		input := []Finding{{Severity: levels[int(a)%5]}}
		got := DisplayFindings(input, levels[int(b)%5])
		if (len(got) == 1) != (a%5 <= b%5) || input[0].Severity != levels[int(a)%5] {
			t.Fatal(got, input)
		}
	})
}

func TestMarkdownFloorHidesAllowedRowsButKeepsBlocked(t *testing.T) {
	results := []*Result{
		{SkillName: "allowed-low", Findings: []Finding{{Severity: SeverityLow}}},
		{SkillName: "blocked-low", IsBlocked: true, Findings: []Finding{{Severity: SeverityLow}}},
	}
	md := ToMarkdown(results, MarkdownOptions{MinSeverity: SeverityHigh})
	if strings.Contains(md, "allowed-low") || !strings.Contains(md, "blocked-low") || !strings.Contains(md, "**BLOCKED**") {
		t.Fatal(md)
	}
}

func TestDisplayFloorPreservesAcceptedFindingAndBlocking(t *testing.T) {
	r := &Result{Findings: []Finding{{Severity: SeverityHigh, Pattern: "accepted"}, {Severity: SeverityLow, Pattern: "low"}}}
	r.MarkAcknowledged([]string{AcceptKey(r.Findings[0])})
	before := append([]Finding(nil), r.Findings...)
	blocked := r.HasSeverityAtOrAbove(SeverityHigh)
	visible := DisplayFindings(r.Findings, SeverityHigh)
	if len(visible) != 1 || !visible[0].Acknowledged || blocked || r.HasSeverityAtOrAbove(SeverityHigh) || !reflect.DeepEqual(r.Findings, before) {
		t.Fatalf("visible=%+v result=%+v", visible, r)
	}
}
