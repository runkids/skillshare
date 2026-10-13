package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"skillshare/internal/config"
)

// CleanupConfig holds backup cleanup configuration
type CleanupConfig struct {
	MaxAge    time.Duration // Maximum age of backups to keep (0 = no limit)
	MaxCount  int           // Maximum number of backups to keep (0 = no limit)
	MaxSizeMB int64         // Maximum total size in MB (0 = no limit)
}

// DefaultCleanupConfig returns sensible defaults for cleanup
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		MaxAge:    30 * 24 * time.Hour, // 30 days
		MaxCount:  10,                  // Keep last 10 backups
		MaxSizeMB: 500,                 // 500 MB max
	}
}

// RetentionConfig returns the defaults with the limits set in the global
// config's backup section. A nil config, or a negative limit, keeps the default.
func RetentionConfig(cfg *config.Config) CleanupConfig {
	c := DefaultCleanupConfig()
	if cfg == nil {
		return c
	}
	if n := cfg.Backup.MaxCount; n != nil && *n >= 0 {
		c.MaxCount = *n
	}
	if n := cfg.Backup.MaxSizeMB; n != nil && *n >= 0 {
		c.MaxSizeMB = *n
	}
	return c
}

// Cleanup removes old backups from the global backup dir.
// Returns the number of backups removed and any error encountered.
func Cleanup(cfg CleanupConfig) (int, error) {
	return CleanupInDir(BackupDir(), cfg)
}

// ErrCleanupBusy indicates a backup or another cleanup owns the directory lock.
var ErrCleanupBusy = errors.New("backup directory is busy; cleanup skipped, retry after the current backup or cleanup finishes")

// CleanupInDir removes old backups and abandoned staging directories.
// Returns the number of directories removed. Cleanup is skipped while a backup
// or another cleanup owns the directory lock.
func CleanupInDir(backupDir string, cfg CleanupConfig) (int, error) {
	removed, _, err := CleanupWithStatsInDir(backupDir, cfg)
	if errors.Is(err, ErrCleanupBusy) {
		return 0, nil
	}
	return removed, err
}

// CleanupWithStatsInDir returns the removed directory count and reclaimed bytes.
// Unlike automatic cleanup, it reports ErrCleanupBusy when cleanup is skipped.
func CleanupWithStatsInDir(backupDir string, cfg CleanupConfig) (int, int64, error) {
	return cleanupInDir(backupDir, cfg, false)
}

// PreviewCleanupInDir returns the directory count and bytes cleanup would reclaim,
// without removing snapshots or staging directories.
func PreviewCleanupInDir(backupDir string, cfg CleanupConfig) (int, int64, error) {
	return cleanupInDir(backupDir, cfg, true)
}

func cleanupInDir(backupDir string, cfg CleanupConfig, dryRun bool) (int, int64, error) {
	if backupDir == "" {
		return 0, 0, fmt.Errorf("cannot determine backup directory: home directory not found")
	}
	_, err := os.Stat(backupDir)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	var entries []os.DirEntry
	if dryRun {
		// Read before probing the lock: if no lock exists afterward, these
		// entries cannot include staging from a participating writer.
		entries, err = os.ReadDir(backupDir)
		if err != nil {
			return 0, 0, err
		}
	}
	lockPath := filepath.Join(backupDir, backupLockName)
	lock := flock.New(lockPath)
	if dryRun {
		lock = flock.New(lockPath, flock.SetFlag(os.O_RDONLY))
	}
	locked, err := lock.TryLock()
	// Legacy backups may have no lock file; a preview must not create one.
	if err != nil && !(dryRun && os.IsNotExist(err)) {
		return 0, 0, fmt.Errorf("lock backup directory: %w", err)
	}
	if err == nil {
		if !locked {
			return 0, 0, ErrCleanupBusy
		}
		defer lock.Unlock() //nolint:errcheck
	}

	if locked {
		// Refresh under the lock, excluding writers during discovery.
		entries, err = os.ReadDir(backupDir)
		if err != nil {
			return 0, 0, err
		}
	}
	backups, err := ListInDir(backupDir)
	if err != nil {
		return 0, 0, err
	}

	removed := 0
	var freed int64
	now := time.Now()
	var totalSize int64
	var cleanupErrs []error

	// No writer holds the shared lock, so every staging directory is abandoned.
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), stagingDirPrefix) {
			continue
		}
		path := filepath.Join(backupDir, entry.Name())
		size := dirSize(path)
		if !dryRun {
			if err := os.RemoveAll(path); err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("remove incomplete backup %s: %w", entry.Name(), err))
				continue
			}
		}
		removed++
		freed += size
	}

	// Backups are sorted by date (newest first)
	for i, backup := range backups {
		shouldRemove := false

		// Check age (skip if MaxAge is 0)
		if cfg.MaxAge > 0 && now.Sub(backup.Date) > cfg.MaxAge {
			shouldRemove = true
		}

		// Check count - keep most recent N backups (skip if MaxCount is 0)
		if cfg.MaxCount > 0 && i >= cfg.MaxCount {
			shouldRemove = true
		}

		// Check size - remove if total exceeds limit (skip if MaxSizeMB is 0).
		// i > 0 always keeps the newest backup: a single snapshot larger than
		// the cap must not leave the user with no restore point at all.
		size := dirSize(backup.Path)
		if !shouldRemove && cfg.MaxSizeMB > 0 && i > 0 &&
			totalSize+size > cfg.MaxSizeMB*1024*1024 {
			shouldRemove = true
		}

		if shouldRemove {
			if !dryRun {
				if err := os.RemoveAll(backup.Path); err != nil {
					cleanupErrs = append(cleanupErrs, fmt.Errorf("remove backup %s: %w", backup.Timestamp, err))
					totalSize += size
					continue
				}
			}
			removed++
			freed += size
			continue
		}
		totalSize += size
	}

	// Clean up empty timestamp directories
	if !dryRun {
		if err := cleanEmptyDirs(backupDir); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}

	return removed, freed, errors.Join(cleanupErrs...)
}

// CleanupByAge removes backups older than the specified duration.
// Returns the number of backups removed.
func CleanupByAge(maxAge time.Duration) (int, error) {
	return Cleanup(CleanupConfig{MaxAge: maxAge})
}

// CleanupByCount keeps only the N most recent backups.
// Returns the number of backups removed.
func CleanupByCount(maxCount int) (int, error) {
	return Cleanup(CleanupConfig{MaxCount: maxCount})
}

// dirSize calculates the total size of a directory
func dirSize(path string) int64 {
	size, _ := Stats(path)
	return size
}

// Stats returns the total size in bytes and the number of files under path,
// in a single walk.
func Stats(path string) (size int64, files int) {
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
			files++
		}
		return nil
	})
	return size, files
}

// cleanEmptyDirs removes empty directories in the backup directory.
func cleanEmptyDirs(path string) error {
	if path == "" {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read backup directory: %w", err)
	}

	var cleanupErrs []error
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), stagingDirPrefix) {
			continue
		}
		if entry.IsDir() {
			subPath := filepath.Join(path, entry.Name())
			subEntries, err := os.ReadDir(subPath)
			if err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("read backup timestamp %s: %w", entry.Name(), err))
				continue
			}
			if len(subEntries) == 0 {
				if err := os.Remove(subPath); err != nil && !os.IsNotExist(err) {
					cleanupErrs = append(cleanupErrs, fmt.Errorf("remove empty backup timestamp %s: %w", entry.Name(), err))
				}
			}
		}
	}
	return errors.Join(cleanupErrs...)
}

// Size returns the total size of a backup directory in bytes
func Size(path string) int64 {
	return dirSize(path)
}

// TotalSize returns the total size of all backups in bytes
func TotalSize() (int64, error) {
	backups, err := List()
	if err != nil {
		return 0, err
	}

	var total int64
	for _, b := range backups {
		total += dirSize(b.Path)
	}
	return total, nil
}
