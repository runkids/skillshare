package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	return copyDirWithState(src, dst, canonicalDst, resolvedBoundary, "", excludes, map[string]bool{})
}

func copyDirWithState(src, dst, canonicalDst, resolvedBoundary, relPrefix string, excludes, active map[string]bool) error {
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

		// Never traverse dst when copying a source tree that contains it.
		if info.IsDir() && pathSameOrDescendant(path, canonicalDst) {
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
			return os.MkdirAll(dstPath, info.Mode().Perm()|0700)
		}

		if utils.IsLinkMode(path, info.Mode()) {
			resolvedTarget, resolveErr := resolveCopyRoot(path)
			if resolveErr != nil {
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
				if pathSameOrDescendant(resolvedTarget, canonicalDst) {
					return nil
				}
				if info.Name() == ".git" || filepath.Base(resolvedTarget) == ".git" {
					return nil
				}
				if len(excludes) > 0 && fullRel != "" && excludes[fullRel] {
					return nil
				}
				return copyDirWithState(resolvedTarget, dstPath, canonicalDst, resolvedBoundary, fullRel, excludes, active)
			}
			return copyFile(resolvedTarget, dstPath)
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
	if !pathSameOrDescendant(resolvedTarget, resolvedBoundary) {
		return fmt.Errorf("symlink %s escapes repository boundary", label)
	}
	if pathSameOrDescendant(resolvedTarget, filepath.Join(resolvedBoundary, ".git")) {
		return fmt.Errorf("symlink %s targets .git directory", label)
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
