package install

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// copyDir recursively copies src into dst, skipping any `.git` directory.
func copyDir(src, dst string) error {
	return copyDirWithin(src, dst, "", nil)
}

// copyDirExcluding recursively copies src into dst, skipping any `.git`
// directory and any excluded slash-normalized paths relative to src.
func copyDirExcluding(src, dst string, excludes map[string]bool) error {
	return copyDirWithin(src, dst, "", excludes)
}

// copyDirWithin recursively copies src into dst, dereferencing directory
// symlinks as real directories. When boundary is non-empty, symlink targets
// must resolve inside boundary and outside its `.git` directory.
func copyDirWithin(src, dst, boundary string, excludes map[string]bool) error {
	canonicalDst := sourcewalk.Canonical(dst)

	var resolvedBoundary string
	if boundary != "" {
		var err error
		resolvedBoundary, err = resolveCopyRoot(boundary)
		if err != nil {
			return fmt.Errorf("failed to resolve source boundary %s: %w", boundary, err)
		}
	}

	if resolvedBoundary != "" {
		// Walked once without writing, so a refused link leaves nothing half copied.
		if err := copyDirWithState(src, dst, canonicalDst, resolvedBoundary, "", excludes, map[string]bool{}, true); err != nil {
			return err
		}
	}
	return copyDirWithState(src, dst, canonicalDst, resolvedBoundary, "", excludes, map[string]bool{}, false)
}

// copyDirWithState copies src into dst; with checkOnly it walks the same
// links and checks them without writing.
func copyDirWithState(src, dst, canonicalDst, resolvedBoundary, relPrefix string, excludes, active map[string]bool, checkOnly bool) error {
	resolvedSrc, err := resolveCopyRoot(src)
	if err != nil {
		return err
	}
	if err := checkCopyBoundary(resolvedSrc, resolvedBoundary, relPrefix); err != nil {
		return err
	}
	if active[resolvedSrc] {
		label := relPrefix
		if label == "" {
			label = src
		}
		return fmt.Errorf("symlink cycle detected while copying %s", label)
	}
	active[resolvedSrc] = true
	defer delete(active, resolvedSrc)

	return filepath.Walk(resolvedSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(resolvedSrc, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)
		fullRel := joinRelPath(relPrefix, filepath.ToSlash(relPath))

		// Never traverse dst or the previous version a --force reinstall set
		// aside beside it when copying a source tree that contains dst.
		if info.IsDir() && (pathSameOrDescendant(path, canonicalDst) || isBackupOf(path, canonicalDst)) {
			return filepath.SkipDir
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if len(excludes) > 0 && info.IsDir() && fullRel != "" {
			if excludes[fullRel] {
				return filepath.SkipDir
			}
		}

		if info.IsDir() && relPath != "." && active[path] {
			return fmt.Errorf("symlink cycle detected while copying %s", fullRel)
		}

		if info.IsDir() {
			if checkOnly {
				return nil
			}
			return os.MkdirAll(dstPath, info.Mode().Perm()|0700)
		}

		if utils.IsLinkMode(path, info.Mode()) {
			resolvedTarget, resolveErr := resolveCopyRoot(path)
			if resolveErr != nil {
				if resolvedBoundary != "" && errors.Is(resolveErr, fs.ErrNotExist) {
					if err := danglingWithin(resolvedBoundary, path, fullRel); err != nil {
						return err
					}
				}
				return fmt.Errorf("failed to resolve symlink %s: %w", fullRel, resolveErr)
			}
			if err := checkCopyBoundary(resolvedTarget, resolvedBoundary, fullRel); err != nil {
				return err
			}
			targetInfo, statErr := os.Stat(resolvedTarget)
			if statErr != nil {
				return fmt.Errorf("failed to stat symlink target %s: %w", fullRel, statErr)
			}
			if targetInfo.IsDir() {
				if pathSameOrDescendant(resolvedTarget, canonicalDst) || isBackupOf(resolvedTarget, canonicalDst) {
					return nil
				}
				if info.Name() == ".git" || filepath.Base(resolvedTarget) == ".git" {
					return nil
				}
				if len(excludes) > 0 && fullRel != "" && excludes[fullRel] {
					return nil
				}
				return copyDirWithState(resolvedTarget, dstPath, canonicalDst, resolvedBoundary, fullRel, excludes, active, checkOnly)
			}
			if checkOnly {
				return nil
			}
			return copyFile(resolvedTarget, dstPath)
		}

		if checkOnly {
			return nil
		}
		return copyFile(path, dstPath)
	})
}

func resolveCopyRoot(path string) (string, error) {
	if info, err := os.Lstat(path); err == nil && utils.IsLinkMode(path, info.Mode()) {
		path = sourcewalk.Canonical(path)
	}
	return filepath.EvalSymlinks(path)
}

func checkCopyBoundary(resolvedTarget, resolvedBoundary, relPath string) error {
	if resolvedBoundary == "" {
		return nil
	}
	label := relPath
	if label == "" {
		label = resolvedTarget
	}
	if !pathSameOrDescendant(resolvedTarget, resolvedBoundary) || inGitDir(resolvedBoundary, resolvedTarget) {
		return fmt.Errorf("%s %w", label, ErrLinkOutside)
	}
	return nil
}

func joinRelPath(prefix, rel string) string {
	if rel == "." || rel == "" {
		return prefix
	}
	if prefix == "" {
		return rel
	}
	return prefix + "/" + rel
}

// isBackupOf reports whether path is a previous version of dst that
// setAsideInSource or swapStagedIntoSource keeps beside it.
func isBackupOf(path, dst string) bool {
	abs, err := filepath.Abs(path)
	return err == nil && filepath.Dir(abs) == filepath.Dir(dst) &&
		strings.HasPrefix(filepath.Base(abs), ".skillshare-"+filepath.Base(dst)+".old.")
}

func pathSameOrDescendant(path, parent string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absParent, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !startsWithParentTraversal(rel))
}

func startsWithParentTraversal(path string) bool {
	return len(path) > 2 && path[:2] == ".." && os.IsPathSeparator(path[2])
}

// copyFileWithin copies a single file, enforcing boundary when non-empty.
func copyFileWithin(src, dst, boundary string) error {
	if boundary != "" {
		resolvedBoundary, err := resolveCopyRoot(boundary)
		if err != nil {
			return fmt.Errorf("failed to resolve source boundary %s: %w", boundary, err)
		}
		resolvedTarget, err := resolveCopyRoot(src)
		if err != nil {
			return fmt.Errorf("failed to resolve symlink %s: %w", filepath.Base(src), err)
		}
		if err := checkCopyBoundary(resolvedTarget, resolvedBoundary, filepath.Base(src)); err != nil {
			return err
		}
		src = resolvedTarget
	}
	return copyFile(src, dst)
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
