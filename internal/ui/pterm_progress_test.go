package ui

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// padTo pads s to minProgressWidth (same constant used in normalizeGitProgressMessage).
func padTo(s string) string {
	if len(s) < minProgressWidth {
		return s + strings.Repeat(" ", minProgressWidth-len(s))
	}
	return s
}

func TestNormalizeGitProgressMessage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strip transfer rate suffix",
			in:   "Receiving objects:  69% (268052/388481), 234.42 MiB | 15.94 MiB/s",
			want: padTo("Receiving objects: 69%"),
		},
		{
			name: "strip remote prefix",
			in:   "remote: Compressing objects: 100% (32/32), done.",
			want: padTo("Compressing objects: 100%"),
		},
		{
			name: "keep normal message",
			in:   "Cloning repository...",
			want: padTo("Cloning repository..."),
		},
		{
			name: "long message not padded",
			in:   "This is a long message that exceeds the minimum progress width limit easily",
			want: "This is a long message that exceeds the minimum progress width limit easily",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeGitProgressMessage(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeGitProgressMessage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestProgressBarRender(t *testing.T) {
	t.Run("zero progress", func(t *testing.T) {
		p := &ProgressBar{total: 10, title: "Scanning"}
		out := captureRender(p)
		if !strings.Contains(out, "0%") {
			t.Fatalf("expected 0%%, got %q", out)
		}
		if !strings.Contains(out, "0/10") {
			t.Fatalf("expected 0/10, got %q", out)
		}
		// Should have 36 empty dots and 0 fill blocks
		stripped := StripANSI(out)
		fillCount := strings.Count(stripped, barFill)
		emptyCount := strings.Count(stripped, barEmpty)
		if fillCount != 0 {
			t.Fatalf("expected 0 fill blocks at 0%%, got %d", fillCount)
		}
		if emptyCount != barWidth {
			t.Fatalf("expected %d empty dots at 0%%, got %d", barWidth, emptyCount)
		}
	})

	t.Run("50 percent", func(t *testing.T) {
		p := &ProgressBar{total: 100, current: 50, title: "Installing"}
		out := captureRender(p)
		if !strings.Contains(out, "50%") {
			t.Fatalf("expected 50%%, got %q", out)
		}
		stripped := StripANSI(out)
		fillCount := strings.Count(stripped, barFill)
		emptyCount := strings.Count(stripped, barEmpty)
		if fillCount+emptyCount != barWidth {
			t.Fatalf("expected %d total chars, got fill=%d empty=%d", barWidth, fillCount, emptyCount)
		}
		if fillCount != 18 { // 50% of 36
			t.Fatalf("expected 18 fill at 50%%, got %d", fillCount)
		}
	})

	t.Run("100 percent", func(t *testing.T) {
		p := &ProgressBar{total: 5, current: 5, title: "Done"}
		out := captureRender(p)
		if !strings.Contains(out, "100%") {
			t.Fatalf("expected 100%%, got %q", out)
		}
		stripped := StripANSI(out)
		fillCount := strings.Count(stripped, barFill)
		emptyCount := strings.Count(stripped, barEmpty)
		if fillCount != barWidth {
			t.Fatalf("expected all %d fill at 100%%, got %d", barWidth, fillCount)
		}
		if emptyCount != 0 {
			t.Fatalf("expected 0 empty at 100%%, got %d", emptyCount)
		}
	})

	t.Run("increment changes title to Done", func(t *testing.T) {
		p := &ProgressBar{total: 1, title: "Working"}
		p.Increment()
		if p.title != "Done" {
			t.Fatalf("expected title 'Done' after reaching total, got %q", p.title)
		}
	})

	t.Run("stopped bar ignores increment", func(t *testing.T) {
		p := &ProgressBar{total: 10, current: 5, stopped: true}
		p.Increment()
		if p.current != 5 {
			t.Fatalf("expected current unchanged at 5, got %d", p.current)
		}
	})
}

// captureRender calls render on a non-TTY bar and returns the raw output.
func captureRender(p *ProgressBar) string {
	// render writes to ProgressWriter; capture by temporarily replacing it.
	r, w, _ := os.Pipe()
	prev := ProgressWriter
	ProgressWriter = w
	p.tty = true // enable render path
	p.renderNow()
	w.Close()
	ProgressWriter = prev

	buf, _ := io.ReadAll(r)
	return string(buf)
}

func TestNormalizeSpinnerUpdate(t *testing.T) {
	t.Run("dedupe", func(t *testing.T) {
		// lastMessage is already normalized (padded), so dedupe still triggers.
		msg, ok := normalizeSpinnerUpdate("Receiving objects: 69% (1/2)", padTo("Receiving objects: 69%"), time.Time{})
		if ok || msg != "" {
			t.Fatalf("expected dedupe skip, got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("throttle git progress", func(t *testing.T) {
		msg, ok := normalizeSpinnerUpdate("Receiving objects: 70% (2/2)", padTo("Receiving objects: 69%"), time.Now())
		if ok || msg != "" {
			t.Fatalf("expected throttled skip, got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("allow non-git status even when recent", func(t *testing.T) {
		msg, ok := normalizeSpinnerUpdate("Cloning repository...", padTo("Downloading via GitHub API..."), time.Now())
		if !ok || msg != padTo("Cloning repository...") {
			t.Fatalf("expected pass-through status, got ok=%v msg=%q", ok, msg)
		}
	})
}
