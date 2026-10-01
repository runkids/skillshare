package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// timestampPattern matches a snapshot folder name, e.g. 2024-01-15_14-30-45.
var timestampPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}$`)

var (
	// ErrInvalidTimestamp is returned for a name that is not a snapshot folder name.
	ErrInvalidTimestamp = errors.New("invalid backup timestamp")
	// ErrNotFound is returned when no snapshot has the timestamp.
	ErrNotFound = errors.New("backup not found")
)

// ValidTimestamp reports whether ts has the snapshot folder name format, so it
// can never name a path outside the backup directory.
func ValidTimestamp(ts string) bool {
	return timestampPattern.MatchString(ts)
}

// DeleteInDir removes the snapshot folder named timestamp from backupDir.
func DeleteInDir(backupDir, timestamp string) error {
	if !ValidTimestamp(timestamp) {
		return fmt.Errorf("%w: %q", ErrInvalidTimestamp, timestamp)
	}
	if backupDir == "" {
		return fmt.Errorf("cannot determine backup directory: home directory not found")
	}
	path := filepath.Join(backupDir, timestamp)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) || err == nil && !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotFound, timestamp)
	}
	if err != nil {
		return fmt.Errorf("inspect backup %s: %w", timestamp, err)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete backup %s: %w", timestamp, err)
	}
	return nil
}

// DeleteAllInDir removes every snapshot in backupDir and returns how many it
// removed. It keeps going past a snapshot it cannot remove and reports them all.
func DeleteAllInDir(backupDir string) (int, error) {
	backups, err := ListInDir(backupDir)
	if err != nil {
		return 0, err
	}
	removed := 0
	var errs []error
	for _, b := range backups {
		if err := DeleteInDir(backupDir, b.Timestamp); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// TotalSizeInDir returns the total size of all backups in backupDir in bytes.
func TotalSizeInDir(backupDir string) (int64, error) {
	backups, err := ListInDir(backupDir)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, b := range backups {
		total += dirSize(b.Path)
	}
	return total, nil
}

// AgentsEntrySuffix ends the name of an agents snapshot inside a backup
// ("<target>-agents"); skills snapshots use the bare target name.
const AgentsEntrySuffix = "-agents"

// AgentsEntryTarget returns the target an agents snapshot entry belongs to.
func AgentsEntryTarget(entry string) (string, bool) {
	name, ok := strings.CutSuffix(entry, AgentsEntrySuffix)
	return name, ok && name != ""
}
