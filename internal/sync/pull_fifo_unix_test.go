//go:build !windows

package sync

import (
	"path/filepath"
	"syscall"
	"testing"
)

// A FIFO named SKILL.md would block collect when it is read.
func TestHasSkillFile_RejectsFifoSkillMd(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "SKILL.md"), 0644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if HasSkillFile(dir) {
		t.Error("a FIFO named SKILL.md is not a skill file")
	}
}
