package main

import (
	"bytes"
	"strings"
	"testing"
)

func makeRunbook(steps string) string {
	return "# Test Runbook\n\n## Steps\n\n" + steps
}

func TestRunRunbook_SimplePass(t *testing.T) {
	md := makeRunbook(`### Step 1: Echo hello

` + "```bash" + `
echo hello
` + "```" + `

**Expected:**
- hello
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 1 {
		t.Fatalf("expected 1 step, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
	if report.Steps[0].Status != "passed" {
		t.Errorf("expected passed, got %s", report.Steps[0].Status)
	}
}

func TestRunRunbook_Failure(t *testing.T) {
	md := makeRunbook(`### Step 1: Fail

` + "```bash" + `
echo "nope" && exit 1
` + "```" + `

**Expected:**
- success
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if report.Steps[0].Status != "failed" {
		t.Errorf("expected failed, got %s", report.Steps[0].Status)
	}
}

func TestRunRunbook_SkipsManual(t *testing.T) {
	md := makeRunbook(`### Step 1: Manual step

` + "```go" + `
fmt.Println("manual")
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Summary.Skipped)
	}
	if report.Steps[0].Status != "skipped" {
		t.Errorf("expected skipped, got %s", report.Steps[0].Status)
	}
}

func TestRunRunbook_DryRun(t *testing.T) {
	md := makeRunbook(`### Step 1: Echo

` + "```bash" + `
echo should not run
` + "```" + `

### Step 2: Another

` + "```bash" + `
echo also not run
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 2 {
		t.Fatalf("expected 2 steps, got %d", report.Summary.Total)
	}
	if report.Summary.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", report.Summary.Skipped)
	}
	for i, sr := range report.Steps {
		if sr.Status != "skipped" {
			t.Errorf("step %d: expected skipped, got %s", i, sr.Status)
		}
		if sr.Stdout != "" {
			t.Errorf("step %d: expected no stdout in dry run, got %q", i, sr.Stdout)
		}
	}
}

func TestRunRunbook_AssertionFailureExitZero(t *testing.T) {
	md := makeRunbook(`### Step 1: Wrong output

` + "```bash" + `
echo "apple orange"
` + "```" + `

**Expected:**
- banana
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Steps[0].Status != "failed" {
		t.Errorf("expected failed due to assertion mismatch, got %s", report.Steps[0].Status)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if len(report.Steps[0].Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(report.Steps[0].Assertions))
	}
	if report.Steps[0].Assertions[0].Matched {
		t.Error("expected assertion to not match")
	}
}

func TestShouldRun_NoFilter(t *testing.T) {
	opts := RunOptions{}
	for _, n := range []int{1, 2, 3, 10} {
		if !opts.shouldRun(n) {
			t.Errorf("shouldRun(%d) = false, want true (no filter)", n)
		}
	}
}

func TestShouldRun_StepsFilter(t *testing.T) {
	opts := RunOptions{Steps: []int{1, 3}}
	cases := []struct {
		step int
		want bool
	}{
		{1, true}, {2, false}, {3, true}, {4, false},
	}
	for _, tc := range cases {
		if got := opts.shouldRun(tc.step); got != tc.want {
			t.Errorf("shouldRun(%d) = %v, want %v", tc.step, got, tc.want)
		}
	}
}

func TestShouldRun_FromFilter(t *testing.T) {
	opts := RunOptions{From: 3}
	cases := []struct {
		step int
		want bool
	}{
		{1, false}, {2, false}, {3, true}, {4, true}, {10, true},
	}
	for _, tc := range cases {
		if got := opts.shouldRun(tc.step); got != tc.want {
			t.Errorf("shouldRun(%d) = %v, want %v", tc.step, got, tc.want)
		}
	}
}

func TestRunRunbook_StepsFilter(t *testing.T) {
	md := makeRunbook(`### Step 1: First

` + "```bash" + `
echo one
` + "```" + `

### Step 2: Second

` + "```bash" + `
echo two
` + "```" + `

### Step 3: Third

` + "```bash" + `
echo three
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{Steps: []int{1, 3}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 3 {
		t.Fatalf("expected 3 steps, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Summary.Skipped)
	}
	// Step 2 should be skipped.
	if report.Steps[1].Status != StatusSkipped {
		t.Errorf("step 2: expected skipped, got %s", report.Steps[1].Status)
	}
}

func TestRunRunbook_FromFilter(t *testing.T) {
	md := makeRunbook(`### Step 1: First

` + "```bash" + `
echo one
` + "```" + `

### Step 2: Second

` + "```bash" + `
echo two
` + "```" + `

### Step 3: Third

` + "```bash" + `
echo three
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{From: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 3 {
		t.Fatalf("expected 3 steps, got %d", report.Summary.Total)
	}
	if report.Summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Summary.Skipped)
	}
	// Step 1 should be skipped.
	if report.Steps[0].Status != StatusSkipped {
		t.Errorf("step 1: expected skipped, got %s", report.Steps[0].Status)
	}
}

func TestRunRunbook_FailFast(t *testing.T) {
	md := makeRunbook(`### Step 1: Fail early

` + "```bash" + `
echo "step1" && exit 1
` + "```" + `

### Step 2: Should skip

` + "```bash" + `
echo "step2"
` + "```" + `

### Step 3: Should also skip

` + "```bash" + `
echo "step3"
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{FailFast: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 3 {
		t.Fatalf("expected 3 steps, got %d", report.Summary.Total)
	}
	if report.Steps[0].Status != StatusFailed {
		t.Errorf("step 1: expected failed, got %s", report.Steps[0].Status)
	}
	if report.Steps[1].Status != StatusSkipped {
		t.Errorf("step 2: expected skipped, got %s", report.Steps[1].Status)
	}
	if report.Steps[2].Status != StatusSkipped {
		t.Errorf("step 3: expected skipped, got %s", report.Steps[2].Status)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if report.Summary.Skipped != 2 {
		t.Errorf("expected 2 skipped, got %d", report.Summary.Skipped)
	}
}

func TestRunRunbook_FailFastAssertionOnly(t *testing.T) {
	md := makeRunbook(`### Step 1: Exit 0 but assertion fails

` + "```bash" + `
echo "apple"
` + "```" + `

**Expected:**
- banana

### Step 2: Should skip

` + "```bash" + `
echo "step2"
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{FailFast: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Steps[0].Status != StatusFailed {
		t.Errorf("step 1: expected failed (assertion), got %s", report.Steps[0].Status)
	}
	if report.Steps[1].Status != StatusSkipped {
		t.Errorf("step 2: expected skipped, got %s", report.Steps[1].Status)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Summary.Failed)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Summary.Skipped)
	}
}

func TestRunRunbook_EnvSeeding(t *testing.T) {
	md := makeRunbook(`### Step 1: Check env

` + "```bash" + `
echo "MY_VAR=$MY_VAR"
` + "```" + `

**Expected:**
- MY_VAR=hello
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{
		Env: map[string]string{"MY_VAR": "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Steps[0].Status != StatusPassed {
		t.Errorf("expected passed, got %s (stdout=%q)", report.Steps[0].Status, report.Steps[0].Stdout)
	}
}

func TestRunRunbook_DependsOn(t *testing.T) {
	md := makeRunbook(`### Step 1: Fail

` + "```bash" + `
echo "step1" && exit 1
` + "```" + `

### Step 2: Depends on step 1

<!-- runbook: depends=1 -->

` + "```bash" + `
echo "step2"
` + "```" + `

### Step 3: Independent

` + "```bash" + `
echo "step3"
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Summary.Total != 3 {
		t.Fatalf("expected 3 steps, got %d", report.Summary.Total)
	}
	// Step 1: fails.
	if report.Steps[0].Status != StatusFailed {
		t.Errorf("step 1: expected failed, got %s", report.Steps[0].Status)
	}
	// Step 2: skipped because depends on step 1.
	if report.Steps[1].Status != StatusSkipped {
		t.Errorf("step 2: expected skipped (depends), got %s", report.Steps[1].Status)
	}
	if !strings.Contains(report.Steps[1].Error, "depends") {
		t.Errorf("step 2 error should mention depends, got: %q", report.Steps[1].Error)
	}
	// Step 3: independent, should pass.
	if report.Steps[2].Status != StatusPassed {
		t.Errorf("step 3: expected passed (independent), got %s", report.Steps[2].Status)
	}
}

func TestRunRunbook_Retry(t *testing.T) {
	// Step uses a counter file to fail on first attempt and pass on second.
	md := makeRunbook(`### Step 1: Retry step

<!-- runbook: retry=2 -->

` + "```bash" + `
F="/tmp/runbook_retry_test_$$"
if [ -f "$F" ]; then
  echo "passed"
  rm -f "$F"
else
  touch "$F"
  echo "failed" && exit 1
fi
` + "```" + `

**Expected:**
- passed
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Steps[0].Status != StatusPassed {
		t.Errorf("expected passed after retry, got %s (stdout=%q, stderr=%q)",
			report.Steps[0].Status, report.Steps[0].Stdout, report.Steps[0].Stderr)
	}
}

func TestRunRunbook_DependsOnWithFailFast(t *testing.T) {
	// Ensure depends-skip doesn't cascade via fail-fast.
	md := makeRunbook(`### Step 1: Fail

` + "```bash" + `
echo "step1" && exit 1
` + "```" + `

### Step 2: Depends on step 1

<!-- runbook: depends=1 -->

` + "```bash" + `
echo "step2"
` + "```" + `

### Step 3: Independent

` + "```bash" + `
echo "step3"
` + "```" + `
`)

	report, err := RunRunbook(strings.NewReader(md), "test", RunOptions{FailFast: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Step 1 fails → fail-fast triggers.
	if report.Steps[0].Status != StatusFailed {
		t.Errorf("step 1: expected failed, got %s", report.Steps[0].Status)
	}
	// Step 2: skipped (fail-fast, since step 1 already failed).
	if report.Steps[1].Status != StatusSkipped {
		t.Errorf("step 2: expected skipped, got %s", report.Steps[1].Status)
	}
	// Step 3: also skipped via fail-fast.
	if report.Steps[2].Status != StatusSkipped {
		t.Errorf("step 3: expected skipped, got %s", report.Steps[2].Status)
	}
}

func TestRunRunbook_JSONOutput(t *testing.T) {
	md := makeRunbook(`### Step 1: Echo

` + "```bash" + `
echo ok
` + "```" + `
`)

	var buf bytes.Buffer
	_, err := RunRunbook(strings.NewReader(md), "json-test", RunOptions{JSONOutput: &buf})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"runbook": "json-test"`) {
		t.Errorf("JSON output missing runbook name, got: %s", buf.String())
	}
}
