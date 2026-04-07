//go:build windows

package sync

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	gosync "sync"
)

// createLink creates a directory junction on Windows (no admin required).
// If relative is true, first tries os.Symlink with a relative path
// (requires Developer Mode). Falls back to junction with absolute paths.
func createLink(linkPath, sourcePath string, relative bool) error {
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	absTarget, err := filepath.Abs(linkPath)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	if _, err := os.Stat(absSource); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", absSource)
	}

	if info, err := os.Lstat(absTarget); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("target already exists as a junction/symlink: %s", absTarget)
		}
		return fmt.Errorf("target already exists: %s", absTarget)
	}

	// If relative requested, try os.Symlink with relative path first
	if relative {
		// Resolve real paths: OS resolves relative symlinks from the
		// real parent directory, not the lexical one.
		linkDir := evalOrClean(filepath.Dir(absTarget))
		src := evalOrClean(absSource)
		rel, relErr := filepath.Rel(linkDir, src)
		if relErr == nil {
			if symlinkErr := os.Symlink(rel, linkPath); symlinkErr == nil {
				return nil
			}
		}
	}

	// Try junction (no admin required, but requires absolute paths)
	var stderr bytes.Buffer
	cmd := exec.Command("cmd", "/c", "mklink", "/J", absTarget, absSource)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return nil
	}

	junctionErr := strings.TrimSpace(stderr.String())

	// Fallback to symlink with absolute path
	if symlinkErr := os.Symlink(absSource, absTarget); symlinkErr == nil {
		return nil
	}

	errMsg := "failed to create link"
	if junctionErr != "" {
		errMsg = fmt.Sprintf("%s\n  junction error: %s", errMsg, junctionErr)
	} else {
		errMsg = fmt.Sprintf("%s\n  junction: mklink /J command failed", errMsg)
	}
	errMsg = fmt.Sprintf("%s\n  symlink: requires Administrator or Developer Mode", errMsg)
	errMsg = fmt.Sprintf("%s\n  target: %s\n  source: %s", errMsg, absTarget, absSource)

	return errors.New(errMsg)
}

// canCreateRelativeLink probes whether the OS can create relative symlinks.
// On Windows this requires Developer Mode; without it createLink falls back
// to junctions which are always absolute.
var relativeProbe struct {
	once gosync.Once
	ok   bool
}

func canCreateRelativeLink() bool {
	relativeProbe.once.Do(func() {
		dir, err := os.MkdirTemp("", "ss-relprobe-*")
		if err != nil {
			return
		}
		defer os.RemoveAll(dir)
		target := filepath.Join(dir, "t")
		if err := os.Mkdir(target, 0755); err != nil {
			return
		}
		link := filepath.Join(dir, "l")
		relativeProbe.ok = os.Symlink("t", link) == nil
	})
	return relativeProbe.ok
}

// isJunctionOrSymlink checks if path is a junction or symlink
func isJunctionOrSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	// Both junctions and symlinks have ModeSymlink on Windows
	return info.Mode()&os.ModeSymlink != 0
}
