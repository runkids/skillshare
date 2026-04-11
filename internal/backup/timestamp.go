package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	backupTimestampLayout       = "2006-01-02_15-04-05.000"
	legacyBackupTimestampLayout = "2006-01-02_15-04-05"
)

var backupTimestampLayouts = []string{
	backupTimestampLayout,
	legacyBackupTimestampLayout,
}

// NewTimestamp returns the current backup timestamp with millisecond precision.
func NewTimestamp() string {
	return time.Now().Format(backupTimestampLayout)
}

func allocateBackupPath(backupDir, targetName string) (string, string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		timestamp := NewTimestamp()
		backupPath := filepath.Join(backupDir, timestamp, targetName)
		_, err := os.Stat(backupPath)
		if err == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		if os.IsNotExist(err) {
			return timestamp, backupPath, nil
		}
		return "", "", err
	}
	return "", "", fmt.Errorf("failed to allocate unique backup path for %s", targetName)
}

func parseBackupTimestamp(value string) (time.Time, error) {
	var lastErr error
	for _, layout := range backupTimestampLayouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unsupported timestamp format")
	}
	return time.Time{}, lastErr
}

func formatBackupTimestampLabel(ts time.Time) string {
	if ts.Nanosecond() == 0 {
		return ts.Format("2006-01-02 15:04:05")
	}
	return ts.Format("2006-01-02 15:04:05.000")
}
