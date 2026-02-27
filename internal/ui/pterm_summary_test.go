package ui

import (
	"testing"
	"time"
)

func TestFormatSummaryLine_Plain(t *testing.T) {
	line := formatSummaryLine("Collect", 1200*time.Millisecond,
		Metric{Label: "collected", Count: 5},
		Metric{Label: "skipped", Count: 2},
		Metric{Label: "failed", Count: 0},
	)
	want := "Collect complete: 5 collected, 2 skipped, 0 failed (1.2s)"
	if line != want {
		t.Fatalf("got %q, want %q", line, want)
	}
}

func TestFormatSummaryLine_NoDuration(t *testing.T) {
	line := formatSummaryLine("Backup", 0,
		Metric{Label: "created", Count: 3},
		Metric{Label: "skipped", Count: 0},
	)
	want := "Backup complete: 3 created, 0 skipped"
	if line != want {
		t.Fatalf("got %q, want %q", line, want)
	}
}

func TestFormatSummaryLine_ZeroCounts(t *testing.T) {
	line := formatSummaryLine("Uninstall", 500*time.Millisecond,
		Metric{Label: "removed", Count: 0},
		Metric{Label: "skipped", Count: 0},
		Metric{Label: "failed", Count: 0},
	)
	want := "Uninstall complete: 0 removed, 0 skipped, 0 failed (0.5s)"
	if line != want {
		t.Fatalf("got %q, want %q", line, want)
	}
}
