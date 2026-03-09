package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecute_SimpleEcho(t *testing.T) {
	r := Execute(context.Background(), Step{
		Number:  1,
		Title:   "echo test",
		Command: "echo hello",
	})

	if r.Status != "passed" {
		t.Fatalf("expected passed, got %s", r.Status)
	}
	if r.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", r.ExitCode)
	}
	if got := strings.TrimSpace(r.Stdout); got != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", got)
	}
}

func TestExecute_ExitOne(t *testing.T) {
	r := Execute(context.Background(), Step{
		Number:  2,
		Title:   "exit 1",
		Command: "exit 1",
	})

	if r.Status != "failed" {
		t.Fatalf("expected failed, got %s", r.Status)
	}
	if r.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", r.ExitCode)
	}
}

func TestExecute_Multiline(t *testing.T) {
	r := Execute(context.Background(), Step{
		Number:  3,
		Title:   "multiline",
		Command: "echo line1\necho line2",
	})

	if r.Status != "passed" {
		t.Fatalf("expected passed, got %s", r.Status)
	}
	lines := strings.TrimSpace(r.Stdout)
	if lines != "line1\nline2" {
		t.Fatalf("expected 'line1\\nline2', got %q", lines)
	}
}

func TestExecute_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Small sleep to ensure the timeout has a chance to fire.
	time.Sleep(5 * time.Millisecond)

	r := Execute(ctx, Step{
		Number:  4,
		Title:   "timeout",
		Command: "sleep 10",
	})

	if r.Status != "failed" {
		t.Fatalf("expected failed, got %s", r.Status)
	}
	if r.Error == "" {
		t.Fatal("expected error message for timeout")
	}
}

func TestExecute_CapturesStderr(t *testing.T) {
	r := Execute(context.Background(), Step{
		Number:  5,
		Title:   "stderr",
		Command: "echo err >&2",
	})

	if r.Status != "passed" {
		t.Fatalf("expected passed, got %s", r.Status)
	}
	if got := strings.TrimSpace(r.Stderr); got != "err" {
		t.Fatalf("expected stderr 'err', got %q", got)
	}
}

func TestExecute_MergedCodeBlocks(t *testing.T) {
	r := Execute(context.Background(), Step{
		Number:  6,
		Title:   "merged blocks",
		Command: "echo first\n---\necho second",
	})

	if r.Status != "passed" {
		t.Fatalf("expected passed, got %s", r.Status)
	}
	lines := strings.TrimSpace(r.Stdout)
	if lines != "first\nsecond" {
		t.Fatalf("expected 'first\\nsecond', got %q", lines)
	}
}
