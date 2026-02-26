package main

import (
	"testing"
	"time"

	"skillshare/internal/oplog"
)

func TestComputeLogStats_Basic(t *testing.T) {
	entries := []oplog.Entry{
		{Timestamp: "2026-02-26T10:00:00Z", Command: "sync", Status: "ok", Duration: 800},
		{Timestamp: "2026-02-26T09:00:00Z", Command: "sync", Status: "error", Duration: 500},
		{Timestamp: "2026-02-26T08:00:00Z", Command: "install", Status: "ok", Duration: 1200},
		{Timestamp: "2026-02-26T07:00:00Z", Command: "install", Status: "partial", Duration: 900},
	}

	stats := computeLogStats(entries)

	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.SuccessRate != 0.5 {
		t.Errorf("SuccessRate = %f, want 0.5", stats.SuccessRate)
	}
	syncStats := stats.ByCommand["sync"]
	if syncStats.Total != 2 || syncStats.OK != 1 || syncStats.Error != 1 {
		t.Errorf("sync stats = %+v, want total=2 ok=1 error=1", syncStats)
	}
	installStats := stats.ByCommand["install"]
	if installStats.Total != 2 || installStats.OK != 1 || installStats.Partial != 1 {
		t.Errorf("install stats = %+v, want total=2 ok=1 partial=1", installStats)
	}
	if stats.LastOperation == nil || stats.LastOperation.Command != "sync" {
		t.Errorf("LastOperation.Command = %v, want sync", stats.LastOperation)
	}
}

func TestComputeLogStats_Empty(t *testing.T) {
	stats := computeLogStats(nil)
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", stats.SuccessRate)
	}
}

func TestRenderStatsCLI_Empty(t *testing.T) {
	stats := computeLogStats(nil)
	output := renderStatsCLI(stats)
	if output != "No log entries\n" {
		t.Errorf("got %q, want %q", output, "No log entries\n")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m"},
		{3 * time.Hour, "3h"},
		{48 * time.Hour, "2d"},
	}
	for _, tt := range tests {
		got := formatRelativeTime(tt.d)
		if got != tt.want {
			t.Errorf("formatRelativeTime(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
