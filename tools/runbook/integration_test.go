package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegration_SelfContainedRunbook(t *testing.T) {
	md := `# Self-Contained Integration Test

## Scope

Verify RunRunbook end-to-end with simple bash commands.

## Steps

### Step 1: Create temp directory

` + "```bash\nDIR=$(mktemp -d) && echo \"created $DIR\" && [ -d \"$DIR\" ] && echo ok\n```" + `

Expected:

- created
- ok

### Step 2: Echo multiple lines

` + "```bash\necho hello && echo world\n```" + `

Expected:

- hello
- world

### Step 3: Arithmetic check

` + "```bash\necho $((2 + 3))\n```" + `

Expected:

- 5
`

	var jsonBuf bytes.Buffer
	report, err := RunRunbook(strings.NewReader(md), "self-contained-test", RunOptions{
		Timeout:    30 * time.Second,
		JSONOutput: &jsonBuf,
	})
	if err != nil {
		t.Fatalf("RunRunbook error: %v", err)
	}

	// Verify summary counts.
	if report.Summary.Total != 3 {
		t.Errorf("Total = %d, want 3", report.Summary.Total)
	}
	if report.Summary.Passed != 3 {
		t.Errorf("Passed = %d, want 3", report.Summary.Passed)
	}
	if report.Summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", report.Summary.Failed)
	}

	// Verify each step passed.
	for i, sr := range report.Steps {
		if sr.Status != "passed" {
			t.Errorf("Step %d (%s): status = %q, want passed; stdout=%q stderr=%q",
				i+1, sr.Step.Title, sr.Status, sr.Stdout, sr.Stderr)
		}
	}

	// Verify JSON output is valid.
	if jsonBuf.Len() == 0 {
		t.Fatal("JSON output is empty")
	}
	var parsed Report
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON output is not valid: %v\nraw: %s", err, jsonBuf.String())
	}
	if parsed.Runbook != "self-contained-test" {
		t.Errorf("JSON runbook = %q, want %q", parsed.Runbook, "self-contained-test")
	}
	if parsed.Summary.Total != 3 {
		t.Errorf("JSON summary total = %d, want 3", parsed.Summary.Total)
	}
	if parsed.Summary.Passed != 3 {
		t.Errorf("JSON summary passed = %d, want 3", parsed.Summary.Passed)
	}

	t.Logf("Report: %d total, %d passed, %d failed, %d skipped, duration=%dms",
		report.Summary.Total, report.Summary.Passed, report.Summary.Failed,
		report.Summary.Skipped, report.DurationMs)
}

func TestIntegration_ParseAllRealRunbooks(t *testing.T) {
	dir := filepath.Join("..", "..", "ai_docs", "tests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("Skipping: directory %s does not exist: %v", dir, err)
	}

	var runbooks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_runbook.md") {
			runbooks = append(runbooks, e.Name())
		}
	}

	if len(runbooks) == 0 {
		t.Skip("No runbook files found")
	}

	t.Logf("Found %d runbook files", len(runbooks))

	for _, name := range runbooks {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()

			rb, err := ParseRunbook(f)
			if err != nil {
				t.Fatalf("ParseRunbook error: %v", err)
			}

			// Non-empty title.
			if rb.Meta.Title == "" {
				t.Error("Title should not be empty")
			}

			// At least 1 step.
			if len(rb.Steps) == 0 {
				t.Fatal("Should have at least 1 step")
			}

			// Each step has a title.
			for i, s := range rb.Steps {
				if s.Title == "" {
					t.Errorf("Step %d has empty title", i)
				}
			}

			// ClassifyAll produces at least one auto step.
			classified := ClassifyAll(rb.Steps)
			autoCount := 0
			for _, s := range classified {
				if s.Executor == "auto" {
					autoCount++
				}
			}
			if autoCount == 0 {
				t.Error("ClassifyAll should produce at least 1 auto step")
			}

			t.Logf("  Title: %s | Steps: %d | Auto: %d | Manual: %d",
				rb.Meta.Title, len(rb.Steps), autoCount, len(rb.Steps)-autoCount)
		})
	}
}

func TestIntegration_DryRunAllRunbooks(t *testing.T) {
	dir := filepath.Join("..", "..", "ai_docs", "tests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("Skipping: directory %s does not exist: %v", dir, err)
	}

	var runbooks []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_runbook.md") {
			runbooks = append(runbooks, e.Name())
		}
	}

	if len(runbooks) == 0 {
		t.Skip("No runbook files found")
	}

	t.Logf("Found %d runbook files for dry-run", len(runbooks))

	for _, name := range runbooks {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()

			report, err := RunRunbook(f, name, RunOptions{
				DryRun:  true,
				Timeout: 30 * time.Second,
			})
			if err != nil {
				t.Fatalf("RunRunbook dry-run error: %v", err)
			}

			// All steps should be skipped in dry-run mode.
			for i, sr := range report.Steps {
				if sr.Status != "skipped" {
					t.Errorf("Step %d (%s): status = %q, want skipped",
						i, sr.Step.Title, sr.Status)
				}
			}

			if report.Summary.Skipped != report.Summary.Total {
				t.Errorf("Skipped = %d, Total = %d — all should be skipped",
					report.Summary.Skipped, report.Summary.Total)
			}

			if report.Summary.Failed != 0 {
				t.Errorf("Failed = %d, want 0 in dry-run", report.Summary.Failed)
			}

			t.Logf("  %s: %d steps, all skipped", name, report.Summary.Total)
		})
	}
}
