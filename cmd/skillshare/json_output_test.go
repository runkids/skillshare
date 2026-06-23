package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestWriteJSONError(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "skillshare-json-output-")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	defer tmpFile.Close()

	oldStdout := os.Stdout
	os.Stdout = tmpFile

	outErr := writeJSONError(errors.New("sync failed"))

	tmpFile.Close()
	os.Stdout = oldStdout

	data, readErr := os.ReadFile(tmpFile.Name())
	if readErr != nil {
		t.Fatalf("reading captured stdout: %v", readErr)
	}

	var output map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, string(data))
	}
	if got, ok := output["error"]; !ok {
		t.Fatalf("missing error field in JSON output: %v", output)
	} else if got != "sync failed" {
		t.Fatalf("unexpected error field: %v", got)
	}

	var silent *jsonSilentError
	if !errors.As(outErr, &silent) {
		t.Fatalf("expected writeJSONError to return *jsonSilentError, got %T", outErr)
	}
	if silent.Error() != "sync failed" {
		t.Fatalf("unexpected silent error message: %v", silent.Error())
	}
}

func TestIsStructuredOutput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"--json", []string{"search", "foo", "--json"}, true},
		{"-j short alias", []string{"list", "-j"}, true},
		{"--format json", []string{"audit", "--format", "json"}, true},
		{"--format sarif", []string{"audit", "--format", "sarif"}, true},
		{"--format markdown", []string{"audit", "--format", "markdown"}, true},
		{"--format text is not structured", []string{"audit", "--format", "text"}, false},
		{"no flags", []string{"search", "foo"}, false},
		{"empty args", []string{}, false},
		{"--format without value", []string{"audit", "--format"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStructuredOutput(tt.args); got != tt.want {
				t.Errorf("isStructuredOutput(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestWriteJSONErrorIsWrappedWithErrorsAs(t *testing.T) {
	inner := writeJSONError(errors.New("wrapped sync failed"))
	if inner == nil {
		t.Fatalf("unexpected nil error")
	}

	outer := fmt.Errorf("command wrapper: %w", inner)

	var silent *jsonSilentError
	if !errors.As(outer, &silent) {
		t.Fatalf("expected wrapped error to be recognized via errors.As as *jsonSilentError")
	}
	if silent == nil || silent.Error() == "" {
		t.Fatal("expected wrapped error unwrap chain to include jsonSilentError")
	}
}

func TestJSONUISuppressorSuppressesSlogUntilFlush(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "skillshare-json-stderr-")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	oldStderr := os.Stderr
	os.Stderr = tmpFile
	t.Cleanup(func() { os.Stderr = oldStderr })

	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	suppressor := newJSONUISuppressor(true)
	slog.Warn("structured cleanup warning")
	suppressor.Flush()
	slog.Warn("restored warning")

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "structured cleanup warning") {
		t.Fatalf("structured-mode slog warning leaked to stderr: %q", got)
	}
	if !strings.Contains(got, "restored warning") {
		t.Fatalf("expected logger to be restored after Flush, got: %q", got)
	}
}
