//go:build windows

package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	gosync "sync"

	"skillshare/internal/utils"
)

// createLink creates a directory junction on Windows (no admin required).
// If relative is true, first tries os.Symlink with a relative path
// (requires Developer Mode). Falls back to junction with absolute paths.
// A file source only gets a symlink: a junction to a file cannot be read.
func createLink(linkPath, sourcePath string, relative bool, sourceRoot ...string) error {
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	absTarget, err := filepath.Abs(linkPath)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}

	srcInfo, statErr := os.Stat(absSource)
	if os.IsNotExist(statErr) {
		return fmt.Errorf("source directory does not exist: %s", absSource)
	}
	isFile := statErr == nil && !srcInfo.IsDir()

	if info, err := os.Lstat(absTarget); err == nil {
		if utils.IsLinkMode(absTarget, info.Mode()) {
			return fmt.Errorf("target already exists as a junction/symlink: %s", absTarget)
		}
		return fmt.Errorf("target already exists: %s", absTarget)
	}

	// Windows resolves relative links reached through a junction using the
	// alias route, not the physical parent used by filepath.Rel. Use an
	// absolute link there; shouldUseRelative applies the same policy on reruns.
	if relative && relativeLinkPathSupported(filepath.Dir(absTarget)) {
		// Resolve real paths: OS resolves relative symlinks from the
		// real parent directory, not the lexical one.
		linkDir := evalOrClean(filepath.Dir(absTarget))
		src := linkSourcePath(absSource, sourceRoot)
		rel, relErr := filepath.Rel(linkDir, src)
		if relErr == nil {
			if symlinkErr := os.Symlink(rel, linkPath); symlinkErr == nil {
				return nil
			}
		}
	}

	if isFile {
		if symlinkErr := os.Symlink(absSource, absTarget); symlinkErr == nil {
			return nil
		}
		return fmt.Errorf("failed to create file link\n  symlink: requires Administrator or Developer Mode\n  target: %s\n  source: %s", absTarget, absSource)
	}

	// Try junction (no admin required, but requires absolute paths)
	junctionErr := utils.CreateJunction(absTarget, absSource)
	if junctionErr == nil {
		return nil
	}

	// Fallback to symlink with absolute path
	if symlinkErr := os.Symlink(absSource, absTarget); symlinkErr == nil {
		return nil
	}

	errMsg := fmt.Sprintf("failed to create link\n  junction error: %s", junctionErr)
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

// platformCanCreateFileLink probes whether the OS can create file symlinks,
// which requires Developer Mode (or Administrator) on Windows.
var fileLinkProbe struct {
	once gosync.Once
	ok   bool
}

func platformCanCreateFileLink() bool {
	fileLinkProbe.once.Do(func() {
		dir, err := os.MkdirTemp("", "ss-fileprobe-*")
		if err != nil {
			return
		}
		defer os.RemoveAll(dir)
		target := filepath.Join(dir, "t")
		if err := os.WriteFile(target, nil, 0644); err != nil {
			return
		}
		fileLinkProbe.ok = os.Symlink(target, filepath.Join(dir, "l")) == nil
	})
	return fileLinkProbe.ok
}

// fileLinkUsable reports whether a link to a file source can be read through:
// it must be a real symlink resolving to a file. A junction (never a symlink
// on Go 1.23+) to a file looks right to Readlink but cannot be opened.
func fileLinkUsable(path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Stat(path)
	return err == nil && !target.IsDir()
}

// isJunctionOrSymlink checks if path is a junction or symlink. Since Go 1.23
// only symlinks have ModeSymlink; junctions are reported as ModeIrregular.
func isJunctionOrSymlink(path string) bool {
	return utils.IsSymlinkOrJunction(path)
}

// relativeLinkPathSupported rejects junction ancestors, including a configured
// target directory itself. Absolute links remain valid through either route.
func relativeLinkPathSupported(path string) bool {
	for path = filepath.Clean(path); ; path = filepath.Dir(path) {
		if utils.IsJunction(path) {
			return false
		}
		if filepath.Dir(path) == path {
			return true
		}
	}
}
