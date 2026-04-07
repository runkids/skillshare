package sync

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExtraResult holds the result of an extras sync operation.
type ExtraResult struct {
	Synced   int      // Files synced (new + already correct)
	Skipped  int      // Files skipped (local conflict, no --force)
	Pruned   int      // Orphan files removed
	Errors   []string // Non-fatal error messages
	Warnings []string // Non-fatal warnings (e.g. flatten collisions)
}

// DiscoverExtraFiles recursively walks sourcePath and returns relative paths
// of all regular files. Directories named ".git" are skipped. Results are
// sorted for deterministic output.
func DiscoverExtraFiles(sourcePath string) ([]string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("extras source directory does not exist: %s", sourcePath)
		}
		return nil, fmt.Errorf("failed to stat extras source: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("extras source is not a directory: %s", sourcePath)
	}

	var files []string
	err = filepath.Walk(sourcePath, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible paths
		}
		if fi.IsDir() {
			if fi.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(sourcePath, path)
		if relErr != nil {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk extras source: %w", err)
	}

	sort.Strings(files)
	return files, nil
}

// SyncExtra synchronises extra files from sourcePath into targetPath.
//
// Supported modes:
//   - "merge" (default): per-file symlink from target to source
//   - "copy":            per-file copy
//   - "symlink":         entire directory symlink
//
// When dryRun is true the function counts what would happen but makes no
// filesystem changes.
func SyncExtra(sourcePath, targetPath, mode string, dryRun, force, flatten bool, projectRoot string) (*ExtraResult, error) {
	if mode == "" {
		mode = "merge"
	}
	if flatten && mode == "symlink" {
		return nil, fmt.Errorf("flatten cannot be used with symlink mode")
	}

	switch mode {
	case "symlink":
		return syncExtraSymlinkMode(sourcePath, targetPath, dryRun, force, projectRoot)
	case "merge", "copy":
		return syncExtraPerFile(sourcePath, targetPath, mode, dryRun, force, flatten, projectRoot)
	default:
		return nil, fmt.Errorf("unsupported extras sync mode: %q", mode)
	}
}

// syncExtraSymlinkMode symlinks the entire source directory to the target path.
func syncExtraSymlinkMode(sourcePath, targetPath string, dryRun, force bool, projectRoot string) (*ExtraResult, error) {
	result := &ExtraResult{}

	absSrc, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Check existing target
	info, lstatErr := os.Lstat(targetPath)
	if lstatErr == nil {
		// Something exists at targetPath
		if info.Mode()&os.ModeSymlink != 0 {
			// Already a symlink — check if correct
			dest, readErr := os.Readlink(targetPath)
			if readErr == nil {
				absDest, _ := filepath.Abs(dest)
				if absDest == absSrc {
					result.Synced = 1
					return result, nil
				}
			}
			// Wrong symlink
			if !force {
				result.Skipped = 1
				return result, nil
			}
			if !dryRun {
				os.Remove(targetPath)
			}
		} else {
			// Real file/dir
			if !force {
				result.Skipped = 1
				return result, nil
			}
			if !dryRun {
				os.RemoveAll(targetPath)
			}
		}
	}

	if dryRun {
		result.Synced = 1
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}
	relative := shouldUseRelative(projectRoot, absSrc, targetPath)
	if err := createLink(targetPath, absSrc, relative); err != nil {
		return nil, fmt.Errorf("failed to create directory symlink: %w", err)
	}
	result.Synced = 1
	return result, nil
}

// syncExtraPerFile handles merge (symlink) and copy modes on a per-file basis.
func syncExtraPerFile(sourcePath, targetPath, mode string, dryRun, force, flatten bool, projectRoot string) (*ExtraResult, error) {
	result := &ExtraResult{}

	files, err := DiscoverExtraFiles(sourcePath)
	if err != nil {
		return nil, err
	}

	absSrc, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	seen := make(map[string]string) // basename → original rel path (flatten only)

	for _, rel := range files {
		srcFile := filepath.Join(absSrc, rel)
		tgtRel := rel
		if flatten {
			base := filepath.Base(rel)
			if prev, exists := seen[base]; exists {
				result.Skipped++
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("flatten conflict: %s skipped (%s already synced from %s)", rel, base, prev))
				continue
			}
			seen[base] = rel
			tgtRel = base
		}
		tgtFile := filepath.Join(targetPath, tgtRel)

		synced, skipped, syncErr := syncOneExtraFile(srcFile, tgtFile, mode, dryRun, force, projectRoot)
		if syncErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", rel, syncErr))
			continue
		}
		result.Synced += synced
		result.Skipped += skipped
	}

	// Prune orphans (only when not dry-run)
	if !dryRun {
		sourceSet := make(map[string]bool, len(files))
		if flatten {
			for base := range seen {
				sourceSet[base] = true
			}
		} else {
			for _, f := range files {
				sourceSet[f] = true
			}
		}
		pruned, pruneErrors := pruneExtraOrphans(targetPath, sourceSet, mode)
		result.Pruned = pruned
		result.Errors = append(result.Errors, pruneErrors...)
	}

	return result, nil
}

// syncOneExtraFile syncs a single file. Returns (synced, skipped, error).
func syncOneExtraFile(srcFile, tgtFile, mode string, dryRun, force bool, projectRoot string) (int, int, error) {
	// Ensure parent directory exists
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(tgtFile), 0755); err != nil {
			return 0, 0, fmt.Errorf("failed to create parent dir: %w", err)
		}
	}

	// Check if target already exists
	info, lstatErr := os.Lstat(tgtFile)
	if lstatErr == nil {
		isSymlink := info.Mode()&os.ModeSymlink != 0

		// Already correct? → skip (idempotent)
		if mode == "merge" && isSymlink {
			dest, readErr := os.Readlink(tgtFile)
			if readErr == nil {
				absDest, _ := filepath.Abs(dest)
				if absDest == srcFile {
					return 1, 0, nil
				}
			}
		}
		if mode == "copy" && !isSymlink && !info.IsDir() {
			srcInfo, srcErr := os.Stat(srcFile)
			if srcErr == nil && srcInfo.Size() == info.Size() && contentEqual(srcFile, tgtFile) {
				return 1, 0, nil
			}
		}

		// Symlinks left over from a different mode are safe to replace
		autoReplace := isSymlink
		if !autoReplace && !force {
			return 0, 1, nil
		}

		if !dryRun {
			if err := os.Remove(tgtFile); err != nil {
				return 0, 0, fmt.Errorf("failed to remove conflicting file: %w", err)
			}
		}
	}

	if dryRun {
		return 1, 0, nil
	}

	switch mode {
	case "merge":
		relative := shouldUseRelative(projectRoot, srcFile, tgtFile)
		if err := createLink(tgtFile, srcFile, relative); err != nil {
			return 0, 0, fmt.Errorf("failed to create symlink: %w", err)
		}
	case "copy":
		if err := copyFile(srcFile, tgtFile); err != nil {
			return 0, 0, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	return 1, 0, nil
}

// pruneExtraOrphans walks the target directory and removes files that have no
// corresponding source. In merge mode only symlinks are pruned; user-created
// local files are preserved. Empty parent directories are cleaned up.
// Hidden files (names starting with ".") are skipped.
func pruneExtraOrphans(targetPath string, sourceFiles map[string]bool, mode string) (pruned int, errors []string) {
	// Collect paths to prune (walk first, delete after to avoid mutation during walk)
	var toRemove []string

	_ = filepath.Walk(targetPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		rel, relErr := filepath.Rel(targetPath, path)
		if relErr != nil {
			return nil
		}

		// Check if corresponding source file exists
		if sourceFiles[rel] {
			return nil // source exists, keep it
		}

		// Source doesn't exist — candidate for pruning
		if mode == "merge" {
			// In merge mode, only prune symlinks (don't delete user's local files)
			if info.Mode()&os.ModeSymlink == 0 {
				return nil
			}
		}

		toRemove = append(toRemove, path)
		return nil
	})

	for _, path := range toRemove {
		if err := os.Remove(path); err != nil {
			errors = append(errors, fmt.Sprintf("prune %s: %v", path, err))
			continue
		}
		pruned++

		// Clean empty parent directories up to targetPath
		cleanEmptyParents(filepath.Dir(path), targetPath)
	}

	return pruned, errors
}

// contentEqual returns true if two files have identical content.
func contentEqual(a, b string) bool {
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer fa.Close()

	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer fb.Close()

	bufA := make([]byte, 4096)
	bufB := make([]byte, 4096)
	for {
		nA, errA := fa.Read(bufA)
		nB, errB := fb.Read(bufB)
		if !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false
		}
		if errA == io.EOF && errB == io.EOF {
			return true
		}
		if errA != nil || errB != nil {
			return false
		}
	}
}

// EffectiveMode returns the mode to use for sync, defaulting to "merge".
func EffectiveMode(mode string) string {
	if mode == "" {
		return "merge"
	}
	return mode
}

// FlattenRel returns the target-relative path for a source file under flatten mode.
// When flatten is true, it uses only the basename and tracks seen basenames to skip
// collisions (first-wins, matching sync behavior). Returns ("", false) for collisions.
func FlattenRel(rel string, flatten bool, seen map[string]bool) (tgtRel string, ok bool) {
	if !flatten {
		return rel, true
	}
	base := filepath.Base(rel)
	if seen[base] {
		return "", false
	}
	seen[base] = true
	return base, true
}

// CheckSyncStatus compares source files against the target directory and
// returns a status string: "synced" or "drift".
func CheckSyncStatus(sourceFiles []string, sourceDir, targetDir, mode string, flatten bool) string {
	seen := make(map[string]bool)
	for _, rel := range sourceFiles {
		tgtRel, ok := FlattenRel(rel, flatten, seen)
		if !ok {
			continue
		}
		targetFile := filepath.Join(targetDir, tgtRel)
		sourceFile := filepath.Join(sourceDir, rel)

		tInfo, err := os.Lstat(targetFile)
		if err != nil {
			return "drift"
		}

		switch mode {
		case "symlink", "merge":
			if tInfo.Mode()&os.ModeSymlink != 0 {
				link, readErr := os.Readlink(targetFile)
				if readErr != nil || link != sourceFile {
					return "drift"
				}
			} else {
				return "drift"
			}
		case "copy":
			if !tInfo.Mode().IsRegular() {
				return "drift"
			}
		}
	}
	return "synced"
}

// cleanEmptyParents removes empty directories from dir up to (but not
// including) stopAt.
func cleanEmptyParents(dir, stopAt string) {
	for dir != stopAt && dir != filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

// ExtraCollectResult holds results from collecting extra files.
type ExtraCollectResult struct {
	Collected int
	Skipped   int
	Errors    []string
}

// CollectExtraFiles scans targetDir for non-symlink local files,
// copies them to sourceDir, and replaces originals with symlinks.
// When flatten is true, collected files are placed in the source root
// (basename only) rather than preserving the target subdirectory structure.
func CollectExtraFiles(sourceDir, targetDir string, dryRun, flatten bool, projectRoot string) (*ExtraCollectResult, error) {
	result := &ExtraCollectResult{}

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory does not exist: %s", targetDir)
	}

	// Ensure source directory exists
	if !dryRun {
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create source directory: %w", err)
		}
	}

	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		// Skip directories and .git
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// filepath.Walk follows symlinks, so check via Lstat to detect them
		linfo, err := os.Lstat(path)
		if err != nil {
			return nil
		}
		if linfo.Mode()&os.ModeSymlink != 0 {
			result.Skipped++
			return nil
		}

		rel, err := filepath.Rel(targetDir, path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cannot compute relative path: %s", path))
			return nil
		}

		destRel := rel
		if flatten {
			destRel = filepath.Base(rel)
		}
		destPath := filepath.Join(sourceDir, destRel)

		// Skip if already exists in source
		if _, err := os.Stat(destPath); err == nil {
			result.Skipped++
			return nil
		}

		if dryRun {
			result.Collected++
			return nil
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("mkdir failed: %v", err))
			return nil
		}

		// Read source content
		content, err := os.ReadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("read failed: %v", err))
			return nil
		}

		// Write to source dir
		if err := os.WriteFile(destPath, content, info.Mode().Perm()); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write failed: %v", err))
			return nil
		}

		// Remove original and create symlink
		if err := os.Remove(path); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("remove original failed: %v", err))
			return nil
		}

		relative := shouldUseRelative(projectRoot, destPath, path)
		if err := createLink(path, destPath, relative); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("symlink failed: %v", err))
			return nil
		}

		result.Collected++
		return nil
	})

	if err != nil {
		return result, fmt.Errorf("walk error: %w", err)
	}

	return result, nil
}
