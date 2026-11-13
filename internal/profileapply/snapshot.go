package profileapply

import (
	"fmt"
	"path/filepath"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

type Snapshot struct {
	Target string `json:"target"`
	Path string `json:"path"`
	Restore string `json:"restore"`
	Retention string `json:"retention"`
}

func snapshots(v *View, targets map[string]config.TargetConfig, entries []ssync.EntryChange, create func(string, string, string) (string, error)) ([]Snapshot, []ssync.DiscardEntry, error) {
	if create == nil { create = backup.CreateInDir }
	var saved []Snapshot
	var discard []ssync.DiscardEntry
	paths := make(map[string]string)
	for _, entry := range entries {
		if entry.Class != ssync.EntryModifiedCopy || entry.Fingerprint == "" { return saved, nil, fmt.Errorf("cannot discard %s/%s: no verifiable modified-copy fingerprint", entry.Target, entry.Entry) }
		target, ok := targets[entry.Target]
		if !ok { return saved, nil, fmt.Errorf("snapshot target %q is not eligible", entry.Target) }
		path := paths[entry.Target]
		if path == "" {
			var err error
			path, err = create(v.BackupDir, entry.Target, target.SkillsConfig().Path)
			if err != nil || path == "" { return saved, nil, fmt.Errorf("snapshot %s failed (no edits discarded): %v", entry.Target, err) }
			paths[entry.Target] = path
			saved = append(saved, Snapshot{Target: entry.Target, Path: path, Restore: v.RestoreCommand(entry.Target, path), Retention: "Existing backup retention applies; restore before cleanup removes this snapshot."})
		}
		actual, err := ssync.CopyOutputFingerprint(filepath.Join(path, entry.Entry))
		if err != nil || actual != entry.Fingerprint { return saved, nil, fmt.Errorf("snapshot verification failed for %s/%s; no edits discarded", entry.Target, entry.Entry) }
		discard = append(discard, ssync.DiscardEntry{Target: entry.Target, Entry: entry.Entry, Fingerprint: entry.Fingerprint})
	}
	return saved, discard, nil
}
