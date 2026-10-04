//go:build !windows

package sync

import (
	"fmt"
	"os"
)

// createLink creates a symlink on Unix systems.
// If relative is true, the symlink stores a relative path from linkPath's
// directory to sourcePath. Falls back to absolute if filepath.Rel fails.
func createLink(linkPath, sourcePath string, relative bool) error {
	return createLinkAs(linkPath, sourcePath, relative, nil)
}

// createSkillLink is createLink for a skill link; see skillLinkPath.
func createSkillLink(linkPath, sourcePath string, skill skillLinkPath, relative bool) error {
	return createLinkAs(linkPath, sourcePath, relative, &skill)
}

func createLinkAs(linkPath, sourcePath string, relative bool, skill *skillLinkPath) error {
	target := sourcePath
	if relative {
		if rel, err := relativeLinkText(linkPath, sourcePath, skill); err == nil {
			target = rel
		}
	}
	return os.Symlink(target, linkPath)
}

// canCreateRelativeLink returns true on Unix where os.Symlink always works.
func canCreateRelativeLink() bool { return true }

// platformCanCreateFileLink returns true on Unix where os.Symlink always works.
func platformCanCreateFileLink() bool { return true }

// fileLinkUsable reports whether a link to a file source can be read through.
// Unix symlinks always can; only Windows junctions cannot.
func fileLinkUsable(string) bool { return true }

// isJunctionOrSymlink checks if path is a symlink
func isJunctionOrSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// createJunction cannot recreate a Windows junction on another platform.
func createJunction(linkPath, sourcePath string) error {
	return fmt.Errorf("cannot restore Windows junction %s to %s on this platform", linkPath, sourcePath)
}
