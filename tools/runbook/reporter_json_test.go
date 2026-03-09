package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func newTestReport() Report {
	return Report{
		Version:    "1.0",
		Runbook:    "test-runbook.md",
		DurationMs: 150,
		Summary: Summary{
			Total:   3,
			Passed:  2,
			Failed:  0,
			Skipped: 1,
		},
		Steps: []StepResult{
			{
				Step:       Step{Number: 1, Title: "build", Command: "make build"},
				Status:     "passed",
				DurationMs: 80,
			},
			{
				Step:       Step{Number: 2, Title: "test", Command: "make test"},
				Status:     "passed",
				DurationMs: 60,
			},
			{
				Step:       Step{Number: 3, Title: "deploy", Executor: "manual"},
				Status:     "skipped",
				DurationMs: 0,
			},
		},
	}
}

func TestJSON_RoundTrip(t *testing.T) {
	report := newTestReport()
	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}

	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.Runbook != report.Runbook {
		t.Errorf("Runbook = %q, want %q", got.Runbook, report.Runbook)
	}
	if got.DurationMs != report.DurationMs {
		t.Errorf("DurationMs = %d, want %d", got.DurationMs, report.DurationMs)
	}
}

func TestJSON_SummaryFields(t *testing.T) {
	report := newTestReport()
	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}

	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.Summary.Total != 3 {
		t.Errorf("Summary.Total = %d, want 3", got.Summary.Total)
	}
	if got.Summary.Passed != 2 {
		t.Errorf("Summary.Passed = %d, want 2", got.Summary.Passed)
	}
	if got.Summary.Failed != 0 {
		t.Errorf("Summary.Failed = %d, want 0", got.Summary.Failed)
	}
	if got.Summary.Skipped != 1 {
		t.Errorf("Summary.Skipped = %d, want 1", got.Summary.Skipped)
	}
}

func TestJSON_VersionPresent(t *testing.T) {
	report := newTestReport()
	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	v, ok := raw["version"]
	if !ok {
		t.Fatal("version field missing from JSON output")
	}

	var version string
	if err := json.Unmarshal(v, &version); err != nil {
		t.Fatalf("version unmarshal: %v", err)
	}
	if version != "1.0" {
		t.Errorf("version = %q, want %q", version, "1.0")
	}
}

func TestJSON_StepsLength(t *testing.T) {
	report := newTestReport()
	var buf bytes.Buffer
	if err := WriteJSONReport(&buf, report); err != nil {
		t.Fatalf("WriteJSONReport: %v", err)
	}

	var got Report
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if len(got.Steps) != 3 {
		t.Errorf("len(Steps) = %d, want 3", len(got.Steps))
	}
}
