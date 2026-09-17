package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
)

var backupID = regexp.MustCompile(`^[0-9]+-[a-f0-9]{8}$`)

// PreviewRestore restores only entries from one operation, never the entire
// native file. Later changes to those entries require manual reconciliation.
func (s *Service) PreviewRestore(id string) (*Plan, error) {
	if !backupID.MatchString(id) {
		return nil, fmt.Errorf("invalid MCP backup ID")
	}
	data, exists, _, err := safeRead(filepath.Join(s.StateDir, "mcp", "backups", id+".json"))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("MCP backup not found")
	}
	var backup backupRecord
	if json.Unmarshal(data, &backup) != nil || backup.ID != id {
		return nil, fmt.Errorf("invalid MCP backup")
	}
	source, err := LoadSource(s.ConfigPath)
	if err != nil {
		return nil, err
	}
	if backup.Owner != source.ConfigPath {
		return nil, fmt.Errorf("backup belongs to another config or scope")
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return nil, err
	}
	before, exists, mode, err := safeRead(backup.Path)
	if err != nil {
		return nil, err
	}
	native, err := ParseNative(backup.Target, before)
	if err != nil {
		return nil, err
	}
	p := &Plan{SourcePath: source.Path, source: source, state: state, stateBytes: stateBytes, Changes: []Change{}}
	f := &filePlan{path: backup.Path, target: backup.Target, before: before, exists: exists, mode: mode, changes: backup.Before}
	for _, name := range sortedKeys(backup.Before) {
		key := ownershipKey(backup.Path, name)
		change := Change{Target: backup.Target, Path: backup.Path, Name: name, Action: "restore"}
		owned, managed := state.Entries[key]
		if !sameEntry(native.Entries[name], backup.After[name]) || managed && owned.Owner != backup.Owner {
			change.Action, change.Message = "conflict", "entry changed after the backup; restore would overwrite newer changes"
			p.Blocked = true
		}
		if previous, ok := backup.Ownership[key]; ok {
			p.state.Entries[key] = previous
		} else {
			delete(p.state.Entries, key)
		}
		p.Changes = append(p.Changes, change)
	}
	if !p.Blocked {
		f.after, err = native.Edit(f.changes)
		if err != nil {
			return nil, err
		}
	}
	p.files = []*filePlan{f}
	p.Revision = digest([]byte(digest(data) + digest(before) + digest(stateBytes) + digest(source.configBytes) + digest(source.bytes)))
	return p, nil
}

// Restore creates a new backup before reverting native entries. Source settings
// remain unchanged, so a later sync may reapply them.
func (s *Service) Restore(id, revision string) (*Result, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	p, err := s.PreviewRestore(id)
	if err != nil {
		return nil, err
	}
	if revision != "" && p.Revision != revision {
		return nil, fmt.Errorf("configuration changed; preview restore again")
	}
	return s.applyPlan(p)
}
