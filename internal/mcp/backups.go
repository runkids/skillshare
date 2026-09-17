package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

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
	dir := filepath.Join(s.StateDir, "mcp", "backups")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, _, _, err := safeRead(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var record backupRecord
		if json.Unmarshal(data, &record) != nil || !backupID.MatchString(record.ID) {
			return nil, fmt.Errorf("invalid MCP backup metadata")
		}
		if record.Owner == owner {
			result = append(result, BackupInfo{ID: record.ID, Target: record.Target, Path: record.Path})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result, nil
}
