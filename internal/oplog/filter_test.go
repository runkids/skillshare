package oplog

import (
	"testing"
	"time"
)

func TestFilterEntries_Empty(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
		{Timestamp: "2026-01-11T10:00:00Z", Command: "install", Status: "error"},
	}
	got := FilterEntries(entries, Filter{})
	if len(got) != 2 {
		t.Fatalf("empty filter should return all entries, got %d", len(got))
	}
}

func TestFilterEntries_CmdOnly(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
		{Timestamp: "2026-01-11T10:00:00Z", Command: "install", Status: "ok"},
		{Timestamp: "2026-01-12T10:00:00Z", Command: "sync", Status: "error"},
	}
	got := FilterEntries(entries, Filter{Cmd: "sync"})
	if len(got) != 2 {
		t.Fatalf("expected 2 sync entries, got %d", len(got))
	}
	for _, e := range got {
		if e.Command != "sync" {
			t.Errorf("expected command sync, got %s", e.Command)
		}
	}
}

func TestFilterEntries_CmdCaseInsensitive(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "SYNC", Status: "ok"},
	}
	got := FilterEntries(entries, Filter{Cmd: "sync"})
	if len(got) != 1 {
		t.Fatalf("case-insensitive cmd filter should match, got %d", len(got))
	}
}

func TestFilterEntries_StatusOnly(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
		{Timestamp: "2026-01-11T10:00:00Z", Command: "install", Status: "error"},
		{Timestamp: "2026-01-12T10:00:00Z", Command: "audit", Status: "ok"},
	}
	got := FilterEntries(entries, Filter{Status: "error"})
	if len(got) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(got))
	}
	if got[0].Command != "install" {
		t.Errorf("expected install, got %s", got[0].Command)
	}
}

func TestFilterEntries_SinceOnly(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
		{Timestamp: "2026-01-15T10:00:00Z", Command: "install", Status: "ok"},
		{Timestamp: "2026-01-20T10:00:00Z", Command: "audit", Status: "ok"},
	}
	since, _ := time.Parse(time.RFC3339, "2026-01-14T00:00:00Z")
	got := FilterEntries(entries, Filter{Since: since})
	if len(got) != 2 {
		t.Fatalf("expected 2 entries after since, got %d", len(got))
	}
}

func TestFilterEntries_Combined(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
		{Timestamp: "2026-01-15T10:00:00Z", Command: "sync", Status: "error"},
		{Timestamp: "2026-01-20T10:00:00Z", Command: "install", Status: "ok"},
		{Timestamp: "2026-01-25T10:00:00Z", Command: "sync", Status: "ok"},
	}
	since, _ := time.Parse(time.RFC3339, "2026-01-14T00:00:00Z")
	got := FilterEntries(entries, Filter{Cmd: "sync", Status: "ok", Since: since})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry matching all filters, got %d", len(got))
	}
	if got[0].Timestamp != "2026-01-25T10:00:00Z" {
		t.Errorf("expected jan 25 entry, got %s", got[0].Timestamp)
	}
}

func TestFilterEntries_NoMatch(t *testing.T) {
	entries := []Entry{
		{Timestamp: "2026-01-10T10:00:00Z", Command: "sync", Status: "ok"},
	}
	got := FilterEntries(entries, Filter{Cmd: "install"})
	if len(got) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(got))
	}
}

func TestParseSince_Relative(t *testing.T) {
	tests := []struct {
		input  string
		suffix string
	}{
		{"30m", "m"},
		{"2h", "h"},
		{"3d", "d"},
		{"1w", "w"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSince(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.IsZero() {
				t.Fatal("expected non-zero time")
			}
			if got.After(time.Now()) {
				t.Error("relative time should be in the past")
			}
		})
	}
}

func TestParseSince_AbsoluteDate(t *testing.T) {
	got, err := ParseSince("2026-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := time.Parse("2006-01-02", "2026-01-15")
	if !got.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestParseSince_RFC3339(t *testing.T) {
	got, err := ParseSince("2026-01-15T10:30:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected, _ := time.Parse(time.RFC3339, "2026-01-15T10:30:00Z")
	if !got.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestParseSince_Invalid(t *testing.T) {
	_, err := ParseSince("xyz")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestParseSince_Empty(t *testing.T) {
	got, err := ParseSince("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Error("empty input should return zero time")
	}
}
