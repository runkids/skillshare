package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// skillLinkPath names a skill link destination by its source root and logical
// tail. Relative link text runs from the canonical source root through the
// logical tail, so a followed entry stays in the text and the OS resolves it
// when the link is opened.
type skillLinkPath struct {
	root string // skills source root
	tail string // logical path below root, slash-separated; "" for the root itself
}

// relativeLinkText computes the stored text of a relative link at linkPath.
// The OS resolves relative links from the link's real parent, so that parent
// is canonicalized. Agents and extras (skill == nil) canonicalize the whole
// destination; skills canonicalize only the source root and keep the tail.
// evalOrClean cannot be used for skills because it does not resolve Windows
// junctions in the link's parent.
func relativeLinkText(linkPath, sourcePath string, skill *skillLinkPath) (string, error) {
	if skill == nil {
		return filepath.Rel(evalOrClean(filepath.Dir(linkPath)), evalOrClean(sourcePath))
	}
	linkDir, err := sourcewalk.Canonicalize(filepath.Dir(linkPath))
	if err != nil {
		return "", err
	}
	root, err := sourcewalk.Canonicalize(skill.root)
	if err != nil {
		return "", err
	}
	return filepath.Rel(linkDir, filepath.Join(root, filepath.FromSlash(skill.tail)))
}

// shouldUseRelative returns true if both sourcePath and targetPath
// are under the given projectRoot, meaning a relative symlink
// between them would be portable across machines.
// Returns false if projectRoot is empty (global mode).
// Paths are resolved through EvalSymlinks so that symlinked ancestors
// do not fool the prefix check.
func shouldUseRelative(projectRoot, sourcePath, targetPath string) bool {
	if projectRoot == "" {
		return false
	}
	cleaned := evalOrClean(projectRoot)
	prefix := cleaned
	if prefix != string(filepath.Separator) {
		prefix += string(filepath.Separator)
	}
	src := evalOrClean(sourcePath)
	tgt := evalOrClean(targetPath)

	srcUnder := utils.PathHasPrefix(src, prefix) || utils.PathsEqual(src, cleaned)
	tgtUnder := utils.PathHasPrefix(tgt, prefix) || utils.PathsEqual(tgt, cleaned)
	return srcUnder && tgtUnder
}

// evalOrClean resolves symlinks in path. When the full path does not exist yet
// (e.g. the target directory on the first sync), it resolves the deepest
// existing ancestor and re-appends the missing tail, so a symlinked ancestor
// (macOS /var → /private/var) is canonicalized consistently whether or not the
// leaf exists. Without this, shouldUseRelative would flip between runs — raw on
// the first sync (leaf missing) and resolved on the second (leaf present) —
// causing a needless absolute→relative reformat on every second sync.
func evalOrClean(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	tail := ""
	cur := p
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			break // reached root without an existing ancestor
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(resolved, tail)
		}
		cur = parent
	}
	return p
}

// resolveReadlink converts a raw os.Readlink result to an absolute path,
// resolving relative targets against the link's parent directory (not CWD).
func resolveReadlink(dest, linkPath string) string {
	if filepath.IsAbs(dest) {
		return dest
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), dest))
}

// linkNeedsReformat returns true if dest (the raw os.Readlink result)
// uses the wrong format (relative vs absolute) for the desired mode.
func linkNeedsReformat(dest string, wantRelative bool) bool {
	if dest == "" {
		return false
	}
	if wantRelative && !canCreateRelativeLink() {
		// Platform cannot create relative symlinks (e.g. Windows without
		// Developer Mode falls back to junctions, which are always absolute).
		// Skip reformat to avoid remove→recreate loop on every sync.
		return false
	}
	return wantRelative == filepath.IsAbs(dest)
}

// reformatLink atomically replaces an existing symlink with a new one
// using the specified format. It creates a temp link and renames it
// over the original so the link is never missing. Falls back to
// remove→create when rename fails (e.g. Windows junctions).
func reformatLink(linkPath, sourcePath string, relative bool) error {
	return reformatLinkAs(linkPath, sourcePath, relative, nil)
}

// reformatSkillLink is reformatLink for a skill link; see skillLinkPath.
func reformatSkillLink(linkPath, sourcePath string, skill skillLinkPath, relative bool) error {
	return reformatLinkAs(linkPath, sourcePath, relative, &skill)
}

func reformatLinkAs(linkPath, sourcePath string, relative bool, skill *skillLinkPath) error {
	tmpPath := linkPath + ".ss-reformat"
	os.Remove(tmpPath) // clean up stale temp
	if err := createLinkAs(tmpPath, sourcePath, relative, skill); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, linkPath); err == nil {
		return nil
	}
	// Rename failed (Windows junction, cross-device, etc.); fall back
	os.Remove(tmpPath)
	if err := os.Remove(linkPath); err != nil {
		return fmt.Errorf("failed to remove old link: %w", err)
	}
	return createLinkAs(linkPath, sourcePath, relative, skill)
}
