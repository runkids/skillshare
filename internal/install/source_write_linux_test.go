//go:build linux

package install

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestSwapStagedIntoSourceCrossDevice stages on /dev/shm, a different
// filesystem from the test temp dir, so MoveIn's rename fails with EXDEV and
// the copy fallback runs through the source handle.
func TestSwapStagedIntoSourceCrossDevice(t *testing.T) {
	source := t.TempDir()
	staged, err := os.MkdirTemp("/dev/shm", "skillshare-stage-")
	if err != nil {
		t.Skip("/dev/shm not available:", err)
	}
	t.Cleanup(func() { os.RemoveAll(staged) })
	if dev(t, source) == dev(t, staged) {
		t.Skip("/dev/shm shares a filesystem with the temp dir")
	}
	if err := os.WriteFile(filepath.Join(staged, "SKILL.md"), []byte("staged"), 0o644); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(source, "linked")); err != nil {
		t.Fatal(err)
	}

	if err := swapStagedIntoSource(source, staged, filepath.Join(source, "plain")); err != nil {
		t.Fatalf("cross-device swap: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(source, "plain", "SKILL.md")); string(data) != "staged" {
		t.Fatalf("plain skill = %q, want staged content", data)
	}
	if err := swapStagedIntoSource(source, staged, filepath.Join(source, "linked", "child")); err == nil {
		t.Fatal("cross-device swap below a link must be refused")
	}
	if entries, _ := os.ReadDir(external); len(entries) != 0 {
		t.Fatalf("external tree changed: %v", entries)
	}
}

func dev(t *testing.T, path string) uint64 {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatal(err)
	}
	return uint64(st.Dev)
}
