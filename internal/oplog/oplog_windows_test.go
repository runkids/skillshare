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
	if got, want := LogDir(legacy), filepath.Join(config.StateDir(), "logs"); got != want {
		t.Fatalf("legacy-selected global logs=%q want %q", got, want)
	}
	if IsProjectConfig(legacy) {
		t.Fatal("legacy-selected global classified as project")
	}
}
