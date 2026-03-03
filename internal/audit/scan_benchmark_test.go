package audit

import (
	"fmt"
	"strings"
	"testing"
)

var benchmarkFindingSink int

func BenchmarkScanContentWithRules_Scaling(b *testing.B) {
	ruleCounts := []int{32, 128, 512}
	lineCounts := []int{200, 2000}

	for _, ruleCount := range ruleCounts {
		for _, lineCount := range lineCounts {
			prefilterRules := compileBenchmarkRules(b, ruleCount, true)
			noPrefilterRules := compileBenchmarkRules(b, ruleCount, false)

			prefilterContent := buildBenchmarkContent(lineCount, ruleCount, true)
			noPrefilterContent := buildBenchmarkContent(lineCount, ruleCount, false)

			b.Run(fmt.Sprintf("prefilter/rules=%d/lines=%d", ruleCount, lineCount), func(b *testing.B) {
				runScanBenchmark(b, prefilterContent, prefilterRules)
			})
			b.Run(fmt.Sprintf("no_prefilter/rules=%d/lines=%d", ruleCount, lineCount), func(b *testing.B) {
				runScanBenchmark(b, noPrefilterContent, noPrefilterRules)
			})
		}
	}
}

func runScanBenchmark(b *testing.B, content []byte, rules []rule) {
	b.Helper()
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findings := ScanContentWithRules(content, "bench.txt", rules)
		benchmarkFindingSink += len(findings)
	}
}

func compileBenchmarkRules(b *testing.B, count int, withPrefilter bool) []rule {
	b.Helper()

	yr := make([]yamlRule, 0, count)
	for i := 0; i < count; i++ {
		re := fmt.Sprintf(`(?i)\beviltoken%04d\b`, i)
		if !withPrefilter {
			// Alternation intentionally avoids a shared literal prefilter.
			re = fmt.Sprintf(`(?i)(?:\bfoo%04d\b|\bbar%04d\b)`, i, i)
		}
		yr = append(yr, yamlRule{
			ID:       fmt.Sprintf("bench-%d", i),
			Severity: SeverityLow,
			Pattern:  "bench-pattern",
			Message:  "benchmark rule",
			Regex:    re,
		})
	}

	rules, err := compileRules(yr)
	if err != nil {
		b.Fatalf("compile benchmark rules: %v", err)
	}

	for _, r := range rules {
		if withPrefilter && r.prefilter == "" {
			b.Fatalf("expected prefilter for rule %s", r.ID)
		}
		if !withPrefilter && r.prefilter != "" {
			b.Fatalf("unexpected prefilter for no-prefilter rule %s: %q", r.ID, r.prefilter)
		}
	}
	return rules
}

func buildBenchmarkContent(lineCount, ruleCount int, forPrefilterRules bool) []byte {
	var sb strings.Builder
	for i := 0; i < lineCount; i++ {
		if i%250 == 0 {
			id := i % ruleCount
			if forPrefilterRules {
				fmt.Fprintf(&sb, "run tool with eviltoken%04d for audit test\n", id)
			} else {
				fmt.Fprintf(&sb, "run tool with foo%04d for audit test\n", id)
			}
			continue
		}
		fmt.Fprintf(&sb, "normal content line %d without risky token\n", i)
	}
	return []byte(sb.String())
}
