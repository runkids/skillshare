package main

import (
	"strings"
	"testing"
)

func TestRunRunbook_SetupPersistsEnv(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Check var
` + "```bash\necho \"VAR=$MY_VAR\"\n```" + `
**Expected:**
- exit_code: 0
- VAR=hello
`
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		Setup: "export MY_VAR=hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range report.Steps {
		t.Logf("step %d (%s): status=%s stdout=%q stderr=%q error=%q exit=%d",
			s.Step.Number, s.Step.Title, s.Status, s.Stdout, s.Stderr, s.Error, s.ExitCode)
		for _, a := range s.Assertions {
			t.Logf("  assertion: %s matched=%v detail=%q", a.Pattern, a.Matched, a.Detail)
		}
	}
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
}

func TestRunRunbook_SetupFailSkipsAll(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Should be skipped
` + "```bash\necho ok\n```" + `
**Expected:**
- exit_code: 0
`
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		Setup: "exit 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d (passed=%d failed=%d)",
			report.Summary.Skipped, report.Summary.Passed, report.Summary.Failed)
	}
}

func TestRunRunbook_TeardownRuns(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Create marker
` + "```bash\necho ok\n```" + `
**Expected:**
- exit_code: 0
`
	// Teardown runs but its result doesn't affect pass/fail.
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		Teardown: "echo teardown-ran",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
	// Teardown is not in steps count.
	if report.Summary.Total != 1 {
		t.Errorf("expected 1 total, got %d", report.Summary.Total)
	}
}

func TestRunRunbook_TeardownFailureIgnored(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Passes
` + "```bash\necho ok\n```" + `
**Expected:**
- exit_code: 0
`
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		Teardown: "exit 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Step still passes despite teardown failure.
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("teardown failure should not affect results, got %d failed", report.Summary.Failed)
	}
}

func TestRunRunbook_SetupAndTeardown(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Use setup var
` + "```bash\necho \"X=$X\"\n```" + `
**Expected:**
- exit_code: 0
- X=42
`
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		Setup:    "export X=42",
		Teardown: "echo done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", report.Summary.Passed)
	}
}

func TestRunBuildHook_Success(t *testing.T) {
	result := runBuildHook("echo building && echo done")
	if !result.OK {
		t.Errorf("expected OK, got exit %d", result.ExitCode)
	}
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestRunBuildHook_Failure(t *testing.T) {
	result := runBuildHook("exit 42")
	if result.OK {
		t.Error("expected failure")
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestRunRunbook_DryRunIgnoresHooks(t *testing.T) {
	md := `# Test
## Steps
### Step 1: Noop
` + "```bash\necho ok\n```" + `
`
	report, err := RunRunbook(strings.NewReader(md), "test.md", RunOptions{
		DryRun:   true,
		Setup:    "exit 1", // would fail if executed
		Teardown: "exit 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Dry-run skips everything including hooks.
	if report.Summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", report.Summary.Skipped)
	}
}
