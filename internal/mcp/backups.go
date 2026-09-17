package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// keepBackups bounds the backups kept per Agent file; every write adds one.
const keepBackups = 20

// BackupInfo exposes metadata only, never the backed-up connection values.
type BackupInfo struct {
	ID     string `json:"id"`
	Target string `json:"target"`
	Path   string `json:"path"`
}

func (s *Service) Backups() ([]BackupInfo, error) {
	result := []BackupInfo{}
	owner, err := filepath.Abs(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	records, err := s.backupRecords()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.Owner == owner {
			result = append(result, BackupInfo{ID: record.ID, Target: record.Target, Path: record.Path})
		}
	}
	return result, nil
}

// backupRecords returns backups newest first. Unreadable files are skipped so
// one damaged backup cannot hide the others.
func (s *Service) backupRecords() ([]backupRecord, error) {
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []backupRecord
	// IDs start with a 19-digit nanosecond time, so name order is time order.
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		data, _, _, err := safeRead(filepath.Join(dir, name))
		var record backupRecord
		if err != nil || json.Unmarshal(data, &record) != nil || !backupID.MatchString(record.ID) || record.ID+".json" != name {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

// backupSuffix ends every backup ID of one Agent file, so pruning can select
// them by name without decoding each backup.
func backupSuffix(path string) string { return digest([]byte(path))[:8] }

// pruneBackups keeps the newest backups of one Agent file. It is best effort:
// a backup that fails to delete only costs disk space.
func (s *Service) pruneBackups(path string) {
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	entries, _ := os.ReadDir(dir)
	suffix := "-" + backupSuffix(path) + ".json"
	kept := 0
	// IDs start with a 19-digit nanosecond time, so name order is time order.
	for i := len(entries) - 1; i >= 0; i-- {
		if name := entries[i].Name(); strings.HasSuffix(name, suffix) {
			if kept++; kept > keepBackups {
				_ = os.Remove(filepath.Join(dir, name))
			}
		}
	}
}
