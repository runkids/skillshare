package audit

import (
	"regexp"
	"testing"
)

func TestDeriveRulePrefilter_ConcatLiteral(t *testing.T) {
	raw := `\beval\s*\(`
	re := regexp.MustCompile(raw)

	lit, fold := deriveRulePrefilter(raw, re)
	if lit != "eval" {
		t.Fatalf("prefilter = %q, want %q", lit, "eval")
	}
	if fold {
		t.Fatal("prefilter fold should be false")
	}
}

func TestDeriveRulePrefilter_CaseInsensitive(t *testing.T) {
	raw := `(?i)\bhttps?://`
	re := regexp.MustCompile(raw)

	lit, fold := deriveRulePrefilter(raw, re)
	if lit != "http" {
		t.Fatalf("prefilter = %q, want %q", lit, "http")
	}
	if !fold {
		t.Fatal("prefilter fold should be true")
	}
}

func TestDeriveRulePrefilter_AlternationWithoutCommonLiteral(t *testing.T) {
	raw := `(?i)\b(?:curl|wget)\b`
	re := regexp.MustCompile(raw)

	lit, fold := deriveRulePrefilter(raw, re)
	if lit != "" || fold {
		t.Fatalf("prefilter = (%q, %v), want empty", lit, fold)
	}
}

func TestRulePrefilterAllows_CaseFold(t *testing.T) {
	r := rule{prefilter: "http", prefilterFold: true}
	line := "Use HTTP://example.com"

	lower := ""
	ready := false
	if !rulePrefilterAllows(r, line, &lower, &ready) {
		t.Fatal("expected case-fold prefilter to match")
	}
}
