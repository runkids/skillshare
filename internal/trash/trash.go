package trash

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"skillshare/internal/config"
)

const defaultMaxAge = 7 * 24 * time.Hour // 7 days

// TrashDir returns the global trash directory path.
func TrashDir() string {
	return filepath.Join(config.DataDir(), "trash")
}

// ProjectTrashDir returns the project-level trash directory path.
func ProjectTrashDir(root string) string {
	return filepath.Join(root, ".skillshare", "trash")
}

// TrashEntry holds information about a trashed item.
type TrashEntry struct {
	Name      string    // Original skill name
	Timestamp string    // Timestamp portion of dir name
	Path      string    // Full path to trashed directory
	Date      time.Time // Parsed or stat-based date
	Size      int64     // Total size in bytes
}

// MoveToTrash moves a skill directory to the trash.
// Uses os.Rename for atomic same-device moves, falls back to copy+delete.
func MoveToTrash(srcPath, name, trashBase string) (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	trashName := name + "_" + timestamp
	trashPath := filepath.Join(trashBase, trashName)

	if err := os.MkdirAll(filepath.Dir(trashPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Try atomic rename first (same device)
	if err := os.Rename(srcPath, trashPath); err == nil {
		return trashPath, nil
	}

	// Fallback: copy then delete (cross-device)
	if err := copyDir(srcPath, trashPath); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	if err := os.RemoveAll(srcPath); err != nil {
		return trashPath, fmt.Errorf("copied to trash but failed to remove original: %w", err)
	}

	return trashPath, nil
}

// List returns all trashed items sorted by date (newest first).
// Walks recursively to find nested entries (e.g., "org/_team-skills_<ts>").
func List(trashBase string) []TrashEntry {
	var items []TrashEntry

	filepath.WalkDir(trashBase, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == trashBase {
			return nil
		}

		name, ts := parseTrashName(d.Name())
		if name == "" {
			return nil // intermediate directory — keep walking
		}

		// Compute full nested name from relative path
		parentRel, _ := filepath.Rel(trashBase, filepath.Dir(path))
		fullName := name
		if parentRel != "." {
			fullName = filepath.Join(parentRel, name)
		}

		date, parseErr := time.Parse("2006-01-02_15-04-05", ts)
		if parseErr != nil {
			if info, serr := d.Info(); serr == nil {
				date = info.ModTime()
			}
		}

		items = append(items, TrashEntry{
			Name:      fullName,
			Timestamp: ts,
			Path:      path,
			Date:      date,
			Size:      dirSize(path),
		})

		return fs.SkipDir // don't descend into trashed content
	})

	sort.Slice(items, func(i, j int) bool {
		return items[i].Date.After(items[j].Date)
	})

	return items
}

// Cleanup removes trashed items older than maxAge.
// Returns the number of items removed.
func Cleanup(trashBase string, maxAge time.Duration) (int, error) {
	if maxAge == 0 {
		maxAge = defaultMaxAge
	}

	items := List(trashBase)
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, item := range items {
		if item.Date.Before(cutoff) {
			if err := os.RemoveAll(item.Path); err != nil {
				return removed, fmt.Errorf("failed to clean up %s: %w", item.Name, err)
			}
			cleanEmptyParents(item.Path, trashBase)
			removed++
		}
	}

	return removed, nil
}

// cleanEmptyParents removes empty parent directories between path and stopAt.
func cleanEmptyParents(path, stopAt string) {
	dir := filepath.Dir(path)
	for dir != stopAt && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

// TotalSize returns the total size of all trashed items in bytes.
func TotalSize(trashBase string) int64 {
	items := List(trashBase)
	var total int64
	for _, item := range items {
		total += item.Size
	}
	return total
}

// FindByName returns the most recent trashed item matching the given skill name.
// Returns nil if not found.
func FindByName(trashBase, name string) *TrashEntry {
	items := List(trashBase)
	for i := range items {
		if items[i].Name == name {
			return &items[i] // List is sorted newest-first
		}
	}
	return nil
}

// Restore moves a trashed skill back to the destination directory.
// Returns an error if the destination already exists.
func Restore(entry *TrashEntry, destDir string) error {
	destPath := filepath.Join(destDir, entry.Name)

	// Ensure parent directory exists for nested names (e.g., "org/_team-skills")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("'%s' already exists in %s (use --force on uninstall to replace)", entry.Name, destDir)
	}

	// Try atomic rename first
	if err := os.Rename(entry.Path, destPath); err == nil {
		return nil
	}

	// Fallback: copy then delete
	if err := copyDir(entry.Path, destPath); err != nil {
		return fmt.Errorf("failed to restore: %w", err)
	}

	if err := os.RemoveAll(entry.Path); err != nil {
		return fmt.Errorf("restored but failed to remove trash entry: %w", err)
	}

	return nil
}

// parseTrashName splits "skillname_YYYY-MM-DD_HH-MM-SS" into name and timestamp.
func parseTrashName(dirName string) (string, string) {
	// Timestamp format: YYYY-MM-DD_HH-MM-SS (19 chars)
	const tsLen = 19 // "2006-01-02_15-04-05"
	// Need at least "x_" + timestamp
	if len(dirName) < tsLen+2 {
		return "", ""
	}

	// The timestamp is always the last 19 characters, preceded by "_"
	sep := len(dirName) - tsLen - 1
	if dirName[sep] != '_' {
		return "", ""
	}

	name := dirName[:sep]
	ts := dirName[sep+1:]

	// Validate timestamp format
	if _, err := time.Parse("2006-01-02_15-04-05", ts); err != nil {
		return "", ""
	}

	return name, ts
}

// dirSize calculates total size of a directory.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// copyDir copies a directory recursively.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := os.Lstat(srcPath)
		if err != nil {
			continue
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
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
