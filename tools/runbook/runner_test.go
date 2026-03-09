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
