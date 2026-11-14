package profileapply

import (
	"fmt"
	"maps"
	"path/filepath"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

type Snapshot struct {
	Target    string `json:"target"`
	Path      string `json:"path"`
	Restore   string `json:"restore"`
	Retention string `json:"retention"`
}

func snapshots(v *View, targets map[string]config.TargetConfig, entries []ssync.EntryChange, moved []ssync.MovedTarget, create func(string, string, string) (string, error)) ([]Snapshot, []ssync.DiscardEntry, error) {
	if create == nil {
		create = backup.CreateInDir
	}
	configured := v.Config.Targets
	if v.Project != nil {
		configured = maps.Clone(configured)
		_, unresolved := config.ResolveValidProjectTargets(v.Scope.ProjectRoot, v.Project)
		// Reserve unresolved names too, matching the restore destination resolver.
		for name := range unresolved {
			configured[name] = config.TargetConfig{}
		}
	}

	var saved []Snapshot
	var discard []ssync.DiscardEntry
	// Resolve every location and key before creating even the first backup.
	// Full configured names participate, including targets outside this run.
	keys := make(map[string]string)
	for _, entry := range entries {
		if entry.Class != ssync.EntryModifiedCopy || entry.Fingerprint == "" {
			return nil, nil, fmt.Errorf("cannot discard %s/%s: no verifiable modified-copy fingerprint", entry.Target, entry.Entry)
		}
		target, ok := targets[entry.Target]
		if !ok {
			return nil, nil, fmt.Errorf("snapshot target %q is not eligible", entry.Target)
		}
		dir := entry.Dir
		if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
			return nil, nil, fmt.Errorf("snapshot %s has no clean absolute directory", entry.Target)
		}
		key := entry.Target
		current, err := filepath.Abs(target.SkillsConfig().Path)
		if err != nil {
			return nil, nil, err
		}
		if dir != current {
			allowed := false
			for _, candidate := range moved {
				if filepath.Clean(candidate.Path) == dir {
					return nil, nil, fmt.Errorf("snapshot %s: former directory is currently owned", entry.Target)
				}
				if candidate.Name == entry.Target && !candidate.Pinned && !candidate.Disabled {
					allowed = true
				}
			}
			if v.Project == nil || !allowed {
				return nil, nil, fmt.Errorf("snapshot %s: directory is outside frozen cleanup inputs", entry.Target)
			}
			rel, err := filepath.Rel(v.Scope.ProjectRoot, dir)
			if err != nil {
				return nil, nil, err
			}
			key = backup.MovedEntryName(entry.Target, rel)
		}
		resolved, err := backup.SkillsEntryDir(v.Scope.ProjectRoot, key, configured)
		if err == nil {
			resolved, err = filepath.Abs(resolved)
		}
		if err != nil || resolved != dir {
			return nil, nil, fmt.Errorf("snapshot %s destination is unavailable or ambiguous: %v", key, err)
		}
		if keys[dir] == "" {
			keys[dir] = key
		}
	}
	paths := make(map[string]string)
	for _, entry := range entries {
		path := paths[entry.Dir]
		if path == "" {
			var err error
			path, err = create(v.BackupDir, keys[entry.Dir], entry.Dir)
			if err != nil || path == "" || filepath.Base(path) != keys[entry.Dir] {
				return saved, nil, fmt.Errorf("snapshot %s failed (no edits discarded): %v", entry.Target, err)
			}
			paths[entry.Dir] = path
			saved = append(saved, Snapshot{Target: entry.Target, Path: path, Restore: v.RestoreCommand(entry.Target, path), Retention: "Existing backup retention applies; restore before cleanup removes this snapshot."})
		}
		actual, err := ssync.CopyOutputFingerprint(filepath.Join(path, entry.Entry))
		if err != nil || actual != entry.Fingerprint {
			return saved, nil, fmt.Errorf("snapshot verification failed for %s/%s; no edits discarded", entry.Target, entry.Entry)
		}
		discard = append(discard, ssync.DiscardEntry{Target: entry.Target, Dir: entry.Dir, Entry: entry.Entry, Fingerprint: entry.Fingerprint})
	}
	return saved, discard, nil
}
