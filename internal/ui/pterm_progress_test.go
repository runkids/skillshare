package ui

import (
	"testing"
	"time"
)

func TestNormalizeGitProgressMessage(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strip transfer rate suffix",
			in:   "Receiving objects:  69% (268052/388481), 234.42 MiB | 15.94 MiB/s",
			want: "Receiving objects: 69%",
		},
		{
			name: "strip remote prefix",
			in:   "remote: Compressing objects: 100% (32/32), done.",
			want: "Compressing objects: 100%",
		},
		{
			name: "keep normal message",
			in:   "Cloning repository...",
			want: "Cloning repository...",
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
		msg, ok := normalizeSpinnerUpdate("Receiving objects: 69% (1/2)", "Receiving objects: 69%", time.Time{})
		if ok || msg != "" {
			t.Fatalf("expected dedupe skip, got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("throttle git progress", func(t *testing.T) {
		msg, ok := normalizeSpinnerUpdate("Receiving objects: 70% (2/2)", "Receiving objects: 69%", time.Now())
		if ok || msg != "" {
			t.Fatalf("expected throttled skip, got ok=%v msg=%q", ok, msg)
		}
	})

	t.Run("allow non-git status even when recent", func(t *testing.T) {
		msg, ok := normalizeSpinnerUpdate("Cloning repository...", "Downloading via GitHub API...", time.Now())
		if !ok || msg != "Cloning repository..." {
			t.Fatalf("expected pass-through status, got ok=%v msg=%q", ok, msg)
		}
	})
}
