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
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

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
