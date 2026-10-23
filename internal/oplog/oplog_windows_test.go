package oplog

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestLogDir_WindowsLegacySelectedGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("SKILLSHARE_CONFIG", "")
	legacy := filepath.Join(home, ".config", "skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("source: skills\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := config.ReadOnlyConfigPath(); got != legacy {
		t.Fatalf("legacy selection=%q want %q", got, legacy)
	}
	canonical := config.ConfigPath()
	if err := Write(canonical, OpsFile, Entry{Command: "project", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(legacy, OpsFile, 0)
	if err != nil || len(entries) != 1 {
		t.Fatalf("legacy-selected global read missed canonical write: %v %v", entries, err)
	}
	if IsProjectConfig(legacy) {
		t.Fatal("legacy-selected global classified as project")
	}
	if err := Clear(legacy, OpsFile); err != nil {
		t.Fatal(err)
	}
	entries, err = Read(canonical, OpsFile, 0)
	if err != nil || len(entries) != 0 {
		t.Fatalf("legacy clear missed canonical write: %v %v", entries, err)
	}
}
