package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_DefaultFallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home, _ := os.UserHomeDir()

	got := DataDir()
	want := filepath.Join(home, ".local", "share", "skillshare")
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestDataDir_RespectsXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")

	got := DataDir()
	want := filepath.Join("/custom/data", "skillshare")
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestStateDir_DefaultFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, _ := os.UserHomeDir()

	got := StateDir()
	want := filepath.Join(home, ".local", "state", "skillshare")
	if got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}
}

func TestStateDir_RespectsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")

	got := StateDir()
	want := filepath.Join("/custom/state", "skillshare")
	if got != want {
		t.Errorf("StateDir() = %q, want %q", got, want)
	}
}
