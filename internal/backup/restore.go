package backup

import (
	"fmt"
	"os"
	"path/filepath"
)

// RestoreOptions holds options for restore operation
type RestoreOptions struct {
	Force bool // Overwrite existing files
}

// ValidateRestore checks if a restore would succeed without modifying the destination.
func ValidateRestore(backupPath, targetName, destPath string, opts RestoreOptions) error {
	targetBackupPath := filepath.Join(backupPath, targetName)

	// Verify backup source exists
	if _, err := os.Stat(targetBackupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target '%s' not found in backup", targetName)
		}
		return fmt.Errorf("cannot access backup: %w", err)
	}

	// Check if destination exists
	info, err := os.Lstat(destPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			if !opts.Force {
				entries, _ := os.ReadDir(destPath)
				if len(entries) > 0 {
					return fmt.Errorf("destination is not empty: %s (use --force to overwrite)", destPath)
				}
			}
			return nil
		}
		return fmt.Errorf("destination exists and is not a directory: %s", destPath)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access destination: %w", err)
	}

	return nil
}

// RestoreToPath restores a backup to a specific path.
// backupPath is the full path to the backup (e.g., ~/.config/skillshare/backups/2024-01-15_14-30-45)
// targetName is the name of the target to restore (e.g., "claude")
// destPath is where to restore to (e.g., ~/.claude/skills)
func RestoreToPath(backupPath, targetName, destPath string, opts RestoreOptions) error {
	if err := ValidateRestore(backupPath, targetName, destPath, opts); err != nil {
		return err
	}

	targetBackupPath := filepath.Join(backupPath, targetName)

	// Check if destination exists
	info, err := os.Stat(destPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - remove it
			if err := os.Remove(destPath); err != nil {
				return fmt.Errorf("failed to remove existing symlink: %w", err)
			}
		} else if info.IsDir() {
			// Remove existing directory for clean restore
			if err := os.RemoveAll(destPath); err != nil {
				return fmt.Errorf("failed to remove existing directory: %w", err)
			}
		} else {
			return fmt.Errorf("destination exists and is not a directory: %s", destPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access destination: %w", err)
	}

	// Copy backup to destination
	return copyDir(targetBackupPath, destPath)
}

// RestoreLatest restores the most recent backup for a target.
// Returns the timestamp of the restored backup.
func RestoreLatest(targetName, destPath string, opts RestoreOptions) (string, error) {
	backups, err := List()
	if err != nil {
		return "", err
	}

	// Find most recent backup containing the target
	for _, b := range backups {
		for _, t := range b.Targets {
			if t == targetName {
				if err := RestoreToPath(b.Path, targetName, destPath, opts); err != nil {
					return "", err
				}
				return b.Timestamp, nil
			}
		}
	}

	return "", fmt.Errorf("no backup found for target '%s'", targetName)
}

// FindBackupsForTarget returns all backups that contain the specified target
func FindBackupsForTarget(targetName string) ([]BackupInfo, error) {
	allBackups, err := List()
	if err != nil {
		return nil, err
	}

	var result []BackupInfo
	for _, b := range allBackups {
		for _, t := range b.Targets {
			if t == targetName {
				result = append(result, b)
				break
			}
		}
	}

	return result, nil
}

// GetBackupByTimestamp finds a backup by its timestamp
func GetBackupByTimestamp(timestamp string) (*BackupInfo, error) {
	backups, err := List()
	if err != nil {
		return nil, err
	}

	for _, b := range backups {
		if b.Timestamp == timestamp {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("backup not found: %s", timestamp)
}
