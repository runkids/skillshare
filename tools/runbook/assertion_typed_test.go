package main

import (
	"testing"
)

func TestRunAssertions_Substring(t *testing.T) {
	r := &StepResult{Stdout: "hello world", ExitCode: 0}
	results := RunAssertions(r, []string{"hello", "Not missing"})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Matched || results[0].Type != AssertSubstring {
		t.Errorf("'hello' should match as substring")
	}
	if !results[1].Matched || !results[1].Negated {
		t.Errorf("'Not missing' should match (negated)")
	}
}

func TestRunAssertions_ExitCode_Match(t *testing.T) {
	r := &StepResult{Stdout: "", ExitCode: 0}
	results := RunAssertions(r, []string{"exit_code: 0"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Matched {
		t.Errorf("exit_code: 0 should match when exit code is 0")
	}
	if results[0].Type != AssertExitCode {
		t.Errorf("type should be exit_code, got %s", results[0].Type)
	}
}

func TestRunAssertions_ExitCode_Mismatch(t *testing.T) {
	r := &StepResult{Stdout: "", ExitCode: 1}
	results := RunAssertions(r, []string{"exit_code: 0"})

	if results[0].Matched {
		t.Errorf("exit_code: 0 should NOT match when exit code is 1")
	}
	if results[0].Detail == "" {
		t.Errorf("should have detail on mismatch")
	}
}

func TestRunAssertions_ExitCode_Negated(t *testing.T) {
	r := &StepResult{Stdout: "", ExitCode: 1}
	results := RunAssertions(r, []string{"exit_code: !0"})

	if !results[0].Matched {
		t.Errorf("exit_code: !0 should match when exit code is 1")
	}
	if !results[0].Negated {
		t.Errorf("should be flagged as negated")
	}
}

func TestRunAssertions_ExitCode_NegatedFail(t *testing.T) {
	r := &StepResult{Stdout: "", ExitCode: 0}
	results := RunAssertions(r, []string{"exit_code: !0"})

	if results[0].Matched {
		t.Errorf("exit_code: !0 should NOT match when exit code is 0")
	}
}

func TestRunAssertions_Regex_Match(t *testing.T) {
	r := &StepResult{Stdout: "synced 42 skills in 1.2s", ExitCode: 0}
	results := RunAssertions(r, []string{`regex: \d+ skills`})

	if !results[0].Matched {
		t.Errorf("regex should match")
	}
	if results[0].Type != AssertRegex {
		t.Errorf("type should be regex, got %s", results[0].Type)
	}
}

func TestRunAssertions_Regex_NoMatch(t *testing.T) {
	r := &StepResult{Stdout: "no numbers here", ExitCode: 0}
	results := RunAssertions(r, []string{`regex: \d+ skills`})

	if results[0].Matched {
		t.Errorf("regex should not match")
	}
}

func TestRunAssertions_Regex_Invalid(t *testing.T) {
	r := &StepResult{Stdout: "test", ExitCode: 0}
	results := RunAssertions(r, []string{`regex: [invalid`})

	if results[0].Matched {
		t.Errorf("invalid regex should not match")
	}
	if results[0].Detail == "" {
		t.Errorf("should have error detail")
	}
}

func TestRunAssertions_JQ_Match(t *testing.T) {
	r := &StepResult{Stdout: `{"count": 5, "status": "ok"}`, ExitCode: 0}
	results := RunAssertions(r, []string{"jq: .count > 0"})

	if !results[0].Matched {
		t.Errorf("jq should match: %s", results[0].Detail)
	}
	if results[0].Type != AssertJQ {
		t.Errorf("type should be jq, got %s", results[0].Type)
	}
}

func TestRunAssertions_JQ_NoMatch(t *testing.T) {
	r := &StepResult{Stdout: `{"count": 0}`, ExitCode: 0}
	results := RunAssertions(r, []string{"jq: .count > 0"})

	if results[0].Matched {
		t.Errorf("jq should not match when count is 0")
	}
}

func TestRunAssertions_JQ_InvalidJSON(t *testing.T) {
	r := &StepResult{Stdout: "not json", ExitCode: 0}
	results := RunAssertions(r, []string{"jq: .count"})

	if results[0].Matched {
		t.Errorf("jq should fail on non-JSON input")
	}
	if results[0].Detail == "" {
		t.Errorf("should have error detail")
	}
}

func TestRunAssertions_Mixed(t *testing.T) {
	r := &StepResult{
		Stdout:   `{"installed": true, "name": "my-skill"}`,
		Stderr:   "Installed: my-skill",
		ExitCode: 0,
	}
	results := RunAssertions(r, []string{
		"Installed",            // substring on combined
		"exit_code: 0",        // exit code
		"jq: .installed",      // jq on stdout only
		"Not error",           // negated substring
	})

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Matched {
			t.Errorf("result[%d] (%s) should have matched: %s", i, res.Pattern, res.Detail)
		}
	}
}

func TestCheckAssertions_NonZeroExitWithExitCodeAssertion(t *testing.T) {
	result := StepResult{
		Step:     Step{Expected: []string{"exit_code: 1", "error"}},
		Status:   StatusFailed,
		ExitCode: 1,
		Stdout:   "error: something went wrong",
	}

	checkAssertions(&result, result.Step)

	// With exit_code: 1 assertion, the step should PASS even though exit code is non-zero.
	if result.Status != StatusPassed {
		t.Errorf("expected passed (exit_code: 1 matches), got %s", result.Status)
	}
	if len(result.Assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(result.Assertions))
	}
}

func TestCheckAssertions_NoAssertions_PreservesStatus(t *testing.T) {
	result := StepResult{
		Step:     Step{Expected: nil},
		Status:   StatusFailed,
		ExitCode: 1,
	}

	checkAssertions(&result, result.Step)

	// No assertions → status unchanged.
	if result.Status != StatusFailed {
		t.Errorf("expected status unchanged (failed), got %s", result.Status)
	}
}
