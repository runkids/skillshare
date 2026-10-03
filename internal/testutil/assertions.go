package testutil

import (
	"regexp"
	"strings"
	"testing"
)

// AssertExitCode checks the command exit code
func (r *Result) AssertExitCode(t *testing.T, expected int) {
	t.Helper()
	if r.ExitCode != expected {
		t.Errorf("expected exit code %d, got %d\nstdout: %s\nstderr: %s",
			expected, r.ExitCode, r.Stdout, r.Stderr)
	}
}

// AssertSuccess checks for successful execution (exit code 0)
func (r *Result) AssertSuccess(t *testing.T) {
	t.Helper()
	r.AssertExitCode(t, 0)
}

// AssertFailure checks for failed execution (non-zero exit code)
func (r *Result) AssertFailure(t *testing.T) {
	t.Helper()
	if r.ExitCode == 0 {
		t.Errorf("expected failure, but command succeeded\nstdout: %s", r.Stdout)
	}
}

// AssertOutputContains checks if stdout contains the given string
func (r *Result) AssertOutputContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.Stdout, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, r.Stdout)
	}
}

// AssertOutputNotContains checks if stdout does not contain the given string
func (r *Result) AssertOutputNotContains(t *testing.T, substr string) {
	t.Helper()
	if strings.Contains(r.Stdout, substr) {
		t.Errorf("expected output not to contain %q, got:\n%s", substr, r.Stdout)
	}
}

// AssertErrorContains checks if stderr contains the given string
func (r *Result) AssertErrorContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.Stderr, substr) {
		t.Errorf("expected error to contain %q, got:\n%s", substr, r.Stderr)
	}
}

// AssertAnyOutputContains checks if stdout or stderr contains the given string
func (r *Result) AssertAnyOutputContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.Stdout, substr) && !strings.Contains(r.Stderr, substr) {
		t.Errorf("expected output or error to contain %q, got:\nstdout: %s\nstderr: %s",
			substr, r.Stdout, r.Stderr)
	}
}

// AssertRowContains checks that stdout has a result row for label whose value
// starts with value: "✓ claude   4 linked" matches ("claude", "4 linked").
func (r *Result) AssertRowContains(t *testing.T, label, value string) {
	t.Helper()
	row := regexp.MustCompile(`(?m)^(?:. )?` + regexp.QuoteMeta(label) + ` +` + regexp.QuoteMeta(value))
	if !row.MatchString(r.Stdout) {
		t.Errorf("expected a row %q with %q, got:\n%s", label, value, r.Stdout)
	}
}
