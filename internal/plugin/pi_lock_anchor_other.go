//go:build !windows

package plugin

import (
	"os"
	"syscall"
)

func openPiLockAnchor(path string) (*os.File, error) { return os.Open(path) }

func removePiLockDirectory(path string, _ *os.File) error { return syscall.Rmdir(path) }
