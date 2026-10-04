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

	"skillshare/internal/utils"
)

// createLink creates a directory junction on Windows (no admin required).
// If relative is true, first tries os.Symlink with a relative path
// (requires Developer Mode). Falls back to junction with absolute paths.
// A file source only gets a symlink: a junction to a file cannot be read.
func createLink(linkPath, sourcePath string, relative bool) error {
	return createLinkAs(linkPath, sourcePath, relative, nil)
}

// createSkillLink is createLink for a skill link; see skillLinkPath.
func createSkillLink(linkPath, sourcePath string, skill skillLinkPath, relative bool) error {
	return createLinkAs(linkPath, sourcePath, relative, &skill)
}

func createLinkAs(linkPath, sourcePath string, relative bool, skill *skillLinkPath) error {
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

	// If relative requested, try os.Symlink with relative path first
	if relative {
		rel, relErr := relativeLinkText(absTarget, absSource, skill)
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
	junctionErr := createJunction(absTarget, absSource)
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

// createJunction is also used to restore an original junction exactly, without
// requiring symlink privileges or falling back to a different link kind.
func createJunction(linkPath, sourcePath string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, sourcePath)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mklink /J: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
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
