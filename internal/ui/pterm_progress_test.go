package ui

import (
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
