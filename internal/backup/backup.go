package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projectdir"
)

const stagingDirPrefix = ".snapshot-"

// BackupDir returns the global backup directory path.
func BackupDir() string {
	return filepath.Join(config.DataDir(), "backups")
}

// ProjectBackupDir returns the project-level backup directory path.
func ProjectBackupDir(projectRoot string) string {
	return filepath.Join(projectdir.Resolve(projectRoot), "backups")
}

// Create creates a backup of the target directory using the global backup dir.
func Create(targetName, targetPath string) (string, error) {
	return CreateInDir(BackupDir(), targetName, targetPath)
}

// CreateInDir creates a backup of the target directory in the specified backup dir.
// Returns the backup path, or ("", nil) when there is nothing to back up.
func CreateInDir(backupDir, targetName, targetPath string) (string, error) {
	return createInDir(backupDir, targetName, targetPath, copyDir)
}

func createInDir(backupDir, targetName, targetPath string, copyDirectory func(string, string) error) (backupPath string, retErr error) {
	if backupDir == "" {
		return "", fmt.Errorf("cannot determine backup directory: home directory not found")
	}

	// Check if target exists and has content
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Nothing to backup
		}
		return "", err
	}

	// Skip if it's already a symlink (no local data to backup)
	if info.Mode()&os.ModeSymlink != 0 {
		return "", nil
	}

	// Check if directory has any content
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to inspect backup target: %w", err)
	}
	if len(entries) == 0 {
		return "", nil // Empty, nothing to backup
	}

	// Copy into a hidden staging directory so a failed copy can never be
	// discovered or restored as a valid snapshot.
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	timestampDir := filepath.Join(backupDir, timestamp)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	stagingPath := ""
	cleanupStaging := false
	defer func() {
		var cleanupErrs []error
		if cleanupStaging {
			if err := os.RemoveAll(stagingPath); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("remove incomplete backup: %w", err))
			}
		}
		if backupPath == "" {
			if err := removeDirIfEmpty(timestampDir); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("remove empty backup timestamp: %w", err))
			}
		}
		if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
			retErr = errors.Join(retErr, cleanupErr)
		}
	}()

	stagingPath, err = os.MkdirTemp(backupDir, stagingDirPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create backup staging directory: %w", err)
	}
	cleanupStaging = true
	if err := os.Chmod(stagingPath, 0755); err != nil {
		return "", fmt.Errorf("failed to set backup staging permissions: %w", err)
	}

	// Copy target contents to backup. copyDir skips symlinks: a merge-mode
	// skill symlink points into the source, which is the single source of
	// truth and already safe — resolving it would copy the source's real
	// content (weights, .venv, browser profiles) into every snapshot.
	// Only local, non-symlinked content is at risk from sync, so only that
	// is worth backing up. Symlinks are recreated by `skillshare sync`.
	if err := copyDirectory(targetPath, stagingPath); err != nil {
		return "", fmt.Errorf("failed to backup: %w", err)
	}

	// A target holding nothing but symlinks yields an empty backup. Discard it:
	// an empty restore point is useless and would consume a MaxCount slot,
	// evicting older snapshots that do have content.
	copied, err := os.ReadDir(stagingPath)
	if err != nil {
		return "", fmt.Errorf("failed to inspect staged backup: %w", err)
	}
	if len(copied) == 0 {
		return "", nil
	}

	if err := os.MkdirAll(timestampDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup timestamp directory: %w", err)
	}
	backupPath = filepath.Join(timestampDir, targetName)
	if _, err := os.Lstat(backupPath); err == nil {
		// Timestamp precision is one second. If the same target is backed up
		// twice within that window, preserve the already completed snapshot.
		return backupPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to inspect backup path: %w", err)
	}
	if err := os.Rename(stagingPath, backupPath); err != nil {
		backupPath = ""
		return "", fmt.Errorf("failed to finalize backup: %w", err)
	}
	cleanupStaging = false
	return backupPath, nil
}

func removeDirIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns all backups from the global backup dir, sorted by date (newest first).
func List() ([]BackupInfo, error) {
	return ListInDir(BackupDir())
}

// ListInDir returns all backups from the specified directory, sorted by date (newest first).
func ListInDir(backupDir string) ([]BackupInfo, error) {
	if backupDir == "" {
		return nil, fmt.Errorf("cannot determine backup directory: home directory not found")
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), stagingDirPrefix) {
			continue
		}

		backupPath := filepath.Join(backupDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// List targets in this backup
		targetEntries, _ := os.ReadDir(backupPath)
		var targets []string
		for _, t := range targetEntries {
			if t.IsDir() {
				targets = append(targets, t.Name())
			}
		}

		backups = append(backups, BackupInfo{
			Timestamp: entry.Name(),
			Path:      backupPath,
			Targets:   targets,
			Date:      info.ModTime(),
		})
	}

	// Sort by date (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date.After(backups[j].Date)
	})

	return backups, nil
}

// BackupInfo holds information about a backup
type BackupInfo struct {
	Timestamp string
	Path      string
	Targets   []string
	Date      time.Time
}

// TargetBackupSummary holds aggregated backup info for a single target.
type TargetBackupSummary struct {
	TargetName  string
	BackupCount int
	Latest      time.Time
	Oldest      time.Time
}

// ListTargetsWithBackups scans the backup directory and returns per-target
// summaries (count, oldest, latest) sorted by target name.
// Returns nil, nil for a non-existent directory.
func ListTargetsWithBackups(backupDir string) ([]TargetBackupSummary, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Aggregate per target: count and time range.
	type accumulator struct {
		count  int
		latest time.Time
		oldest time.Time
	}
	targets := make(map[string]*accumulator)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ts, parseErr := time.ParseInLocation("2006-01-02_15-04-05", entry.Name(), time.Local)
		if parseErr != nil {
			continue // skip directories that don't match the timestamp format
		}

		targetEntries, readErr := os.ReadDir(filepath.Join(backupDir, entry.Name()))
		if readErr != nil {
			continue
		}

		for _, te := range targetEntries {
			if !te.IsDir() {
				continue
			}
			name := te.Name()
			acc, ok := targets[name]
			if !ok {
				acc = &accumulator{oldest: ts, latest: ts}
				targets[name] = acc
			}
			acc.count++
			if ts.Before(acc.oldest) {
				acc.oldest = ts
			}
			if ts.After(acc.latest) {
				acc.latest = ts
			}
		}
	}

	summaries := make([]TargetBackupSummary, 0, len(targets))
	for name, acc := range targets {
		summaries = append(summaries, TargetBackupSummary{
			TargetName:  name,
			BackupCount: acc.count,
			Latest:      acc.latest,
			Oldest:      acc.oldest,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].TargetName < summaries[j].TargetName
	})

	return summaries, nil
}

// copyDir copies a directory recursively, skipping symlinks and junctions.
// Uses os.ReadDir + os.Lstat instead of filepath.Walk to avoid failures
// when os.Lstat on Windows junctions returns nil info with an error.
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

		// Use Lstat to detect symlinks/junctions without following them
		info, err := os.Lstat(srcPath)
		if err != nil {
			// Cannot stat (e.g. broken junction on Windows) — skip
			continue
		}

		// Skip symlinks and junctions — they point to source, not local data
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
