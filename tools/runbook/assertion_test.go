package main

import "testing"

func TestMatchAssertions_SubstringMatch(t *testing.T) {
	results := MatchAssertions("config_created=yes\nstatus=ok", []string{"config_created=yes"})
	if len(results) != 1 || !results[0].Matched {
		t.Fatal("expected substring match")
	}
}

func TestMatchAssertions_NegatedPasses(t *testing.T) {
	results := MatchAssertions("all good", []string{"No error found"})
	if len(results) != 1 || !results[0].Matched || !results[0].Negated {
		t.Fatal("negated pattern should pass when text is absent")
	}
}

func TestMatchAssertions_NegatedFails(t *testing.T) {
	results := MatchAssertions("error found in log", []string{"No error found"})
	if len(results) != 1 || results[0].Matched {
		t.Fatal("negated pattern should fail when text is present")
	}
	if !results[0].Negated {
		t.Fatal("expected Negated flag")
	}
}

func TestMatchAssertions_EqualsStyle(t *testing.T) {
	results := MatchAssertions("claude_ok=yes\nother=no", []string{"claude_ok=yes"})
	if len(results) != 1 || !results[0].Matched {
		t.Fatal("equals-style pattern should match")
	}
}

func TestMatchAssertions_CaseInsensitive(t *testing.T) {
	results := MatchAssertions("Config_Created=YES", []string{"config_created=yes"})
	if len(results) != 1 || !results[0].Matched {
		t.Fatal("matching should be case-insensitive")
	}
}

func TestMatchAssertions_PatternNotFound(t *testing.T) {
	results := MatchAssertions("nothing here", []string{"missing_key=true"})
	if len(results) != 1 || results[0].Matched {
		t.Fatal("pattern not found should yield matched=false")
	}
}

func TestAllPassed_AllTrue(t *testing.T) {
	results := []AssertionResult{
		{Pattern: "a", Matched: true},
		{Pattern: "b", Matched: true},
	}
	if !AllPassed(results) {
		t.Fatal("AllPassed should return true when all matched")
	}
}

func TestAllPassed_OneFalse(t *testing.T) {
	results := []AssertionResult{
		{Pattern: "a", Matched: true},
		{Pattern: "b", Matched: false},
	}
	if AllPassed(results) {
		t.Fatal("AllPassed should return false when one is unmatched")
	}
}

func TestMatchAssertions_Multiple(t *testing.T) {
	output := "status=ok\ncount=3\nmode=merge"
	expected := []string{"status=ok", "count=3", "Not missing_key", "mode=merge"}

	results := MatchAssertions(output, expected)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Matched {
			t.Errorf("result[%d] (%s) should have matched", i, r.Pattern)
		}
	}
	if !results[2].Negated {
		t.Error("result[2] should be negated")
	}
}

func TestMatchAssertions_EmptyExpected(t *testing.T) {
	results := MatchAssertions("some output", nil)
	if len(results) != 0 {
		t.Fatalf("empty expected should return empty results, got %d", len(results))
	}
}

func TestRunAssertions_RegexMultilineDefault(t *testing.T) {
	// ^pattern$ should match individual lines, not just the whole string.
	result := &StepResult{
		Stdout:   "line1\n42\nline3",
		Stderr:   "",
		ExitCode: 0,
	}
	results := RunAssertions(result, []string{`regex: ^42$`})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Matched {
		t.Fatal("regex ^42$ should match line '42' in multi-line output with auto (?m)")
	}
}

func TestRunAssertions_RegexExplicitFlags(t *testing.T) {
	// When pattern already has (? flags, don't prepend (?m).
	result := &StepResult{
		Stdout:   "ABC\ndef",
		Stderr:   "",
		ExitCode: 0,
	}
	results := RunAssertions(result, []string{`regex: (?i)abc`})
	if len(results) != 1 || !results[0].Matched {
		t.Fatal("regex with explicit (?i) flag should still work")
	}
}

func TestRunAssertions_RegexWithoutMultiline(t *testing.T) {
	// Verify ^ matches line boundary (not just string start).
	result := &StepResult{
		Stdout:   "header\nfoo=bar\ntrailer",
		Stderr:   "",
		ExitCode: 0,
	}
	results := RunAssertions(result, []string{`regex: ^foo=bar$`})
	if len(results) != 1 || !results[0].Matched {
		t.Fatal("regex ^foo=bar$ should match middle line with auto (?m)")
	}
}
