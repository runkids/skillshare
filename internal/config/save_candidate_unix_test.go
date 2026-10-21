//go:build unix

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPrepareSave_FirstSaveHonorsUmask(t *testing.T) {
	const childEnv = "SKILLSHARE_TEST_CONFIG_UMASK"
	if os.Getenv(childEnv) != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestPrepareSave_FirstSaveHonorsUmask$")
		cmd.Env = append(os.Environ(), childEnv+"=1", "SKILLSHARE_CONFIG="+filepath.Join(t.TempDir(), "config.yaml"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("umask subprocess: %v\n%s", err, output)
		}
		return
	}
	// Umask is process-wide. Only this isolated subprocess changes it.
	previous := syscall.Umask(0077)
	defer syscall.Umask(previous)
	cfg := &Config{Mode: "copy"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("first save mode = %04o, want 0600 under umask 0077", got)
	}
	// Project candidate replacement retains that mode even after the umask is relaxed.
	syscall.Umask(0022)
	cfg.Mode = "merge"
	candidate, err := PrepareSave(ConfigPath(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Write(); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("replacement mode = %04o, want retained 0600", got)
	}
}
