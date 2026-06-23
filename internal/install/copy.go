package install

import (
	"io"
	"os"
	"path/filepath"
)

// copyDir recursively copies src into dst, skipping any `.git` directory.
func copyDir(src, dst string) error {
	return copyDirExcluding(src, dst, nil)
}

// copyDirExcluding recursively copies src into dst, skipping any `.git`
// directory and any subdirectory whose slash-normalized path relative to src
// appears in excludes. The excludes keys must use forward slashes (e.g.
// "skills/officecli-pptx"). Passing a nil or empty map is equivalent to
// copyDir.
func copyDirExcluding(src, dst string, excludes map[string]bool) error {
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		// If dst is inside src (for example `skillshare install ./ -p`),
		// never traverse the destination subtree as source input.
		if info.IsDir() && pathSameOrDescendant(path, absDst) {
			return filepath.SkipDir
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip excluded subdirectories (e.g. child skill dirs when copying
		// the root of an orchestrator repo so they do not duplicate into the
		// root install).
		if len(excludes) > 0 && info.IsDir() && relPath != "." {
			if excludes[filepath.ToSlash(relPath)] {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func pathSameOrDescendant(path, parent string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parent, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !startsWithParentTraversal(rel))
}

func startsWithParentTraversal(path string) bool {
	return len(path) > 2 && path[:2] == ".." && os.IsPathSeparator(path[2])
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
