package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Result reports applied files individually; a multi-file operation is not a
// filesystem transaction and may leave completed files when another fails.
type Result struct {
	Plan      *Plan    `json:"plan"`
	Applied   []string `json:"applied"`
	BackupIDs []string `json:"backupIds"`
}

type journal struct {
	Path         string `json:"path"`
	Before       string `json:"before"`
	After        string `json:"after"`
	BeforeExists bool   `json:"beforeExists"`
	State        ledger `json:"state"`
}

type backupRecord struct {
	ID        string                    `json:"id"`
	Owner     string                    `json:"owner"`
	Target    string                    `json:"target"`
	Path      string                    `json:"path"`
	Before    map[string]map[string]any `json:"before"`
	After     map[string]map[string]any `json:"after"`
	Ownership map[string]ownership      `json:"ownership"`
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if _, _, _, err := safeRead(path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mcp-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), 0600)
}

func (s *Service) recoverPending() error {
	path := filepath.Join(s.StateDir, "mcp", "pending.json")
	data, exists, _, err := safeRead(path)
	if err != nil || !exists {
		return err
	}
	var pending journal
	if json.Unmarshal(data, &pending) != nil || pending.Path == "" || pending.State.Version != 1 {
		return fmt.Errorf("MCP recovery journal is invalid; manual recovery required")
	}
	current, exists, _, err := safeRead(pending.Path)
	if err != nil {
		return err
	}
	switch {
	case exists && digest(current) == pending.After:
		if err := writeJSONFile(s.statePath(), pending.State); err != nil {
			return err
		}
	case exists == pending.BeforeExists && digest(current) == pending.Before:
		// Native write did not occur. Keep the previous ownership ledger.
	default:
		return fmt.Errorf("MCP interrupted write was followed by external changes to %s; recover from backup before syncing", pending.Path)
	}
	return os.Remove(path)
}

// Apply requires the revision shown in a preview. An empty revision is reserved
// for direct CLI sync, which computes a fresh plan while holding the lock.
func (s *Service) Apply(revision string) (*Result, error) {
	lock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	if err := s.recoverPending(); err != nil {
		return nil, err
	}
	p, err := s.Preview()
	if err != nil {
		return nil, err
	}
	if revision != "" && revision != p.Revision {
		return nil, fmt.Errorf("MCP configuration changed since preview; preview again")
	}
	return s.applyPlan(p)
}

func (s *Service) applyPlan(p *Plan) (*Result, error) {
	result := &Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}
	if p.Blocked {
		return result, fmt.Errorf("MCP conflicts found; no files changed")
	}
	if err := p.source.CheckUnchanged(); err != nil {
		return result, err
	}
	state, stateBytes, err := s.loadLedger()
	if err != nil {
		return result, err
	}
	if !bytes.Equal(stateBytes, p.stateBytes) {
		return result, fmt.Errorf("MCP ownership changed; preview again")
	}
	// Check every file before the first write, and each file again at its write.
	for _, f := range p.files {
		if err := checkFile(f); err != nil {
			return result, err
		}
	}
	for _, f := range p.files {
		if bytes.Equal(f.before, f.after) || len(f.changes) == 0 {
			continue
		}
		if err := checkFile(f); err != nil {
			return result, err
		}
		native, err := ParseNative(f.target, f.before)
		if err != nil {
			return result, err
		}
		id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), digest([]byte(f.path))[:8])
		backup := backupRecord{ID: id, Owner: p.source.ConfigPath, Target: f.target, Path: f.path, Before: map[string]map[string]any{}, After: f.changes, Ownership: map[string]ownership{}}
		for name := range f.changes {
			key := ownershipKey(f.path, name)
			backup.Before[name] = native.Entries[name]
			if owned, ok := state.Entries[key]; ok {
				backup.Ownership[key] = owned
			}
			if owned, ok := p.state.Entries[key]; ok {
				state.Entries[key] = owned
			} else {
				delete(state.Entries, key)
			}
		}
		backupPath := filepath.Join(s.StateDir, "mcp", "backups", id+".json")
		if err := writeJSONFile(backupPath, backup); err != nil {
			return result, fmt.Errorf("MCP backup failed: %w", err)
		}
		pending := journal{Path: f.path, Before: digest(f.before), After: digest(f.after), BeforeExists: f.exists, State: state}
		journalPath := filepath.Join(s.StateDir, "mcp", "pending.json")
		if err := writeJSONFile(journalPath, pending); err != nil {
			return result, err
		}
		if err := atomicWrite(f.path, f.after, f.mode); err != nil {
			return result, fmt.Errorf("MCP write failed for %s: %w", f.path, err)
		}
		result.Applied = append(result.Applied, f.path)
		result.BackupIDs = append(result.BackupIDs, id)
		if err := writeJSONFile(s.statePath(), state); err != nil {
			return result, fmt.Errorf("MCP file written but ownership update failed; retry sync to recover: %w", err)
		}
		if err := os.Remove(journalPath); err != nil {
			return result, err
		}
	}
	finalState, err := json.Marshal(p.state)
	if err != nil {
		return result, err
	}
	currentState, _ := json.Marshal(state)
	if !bytes.Equal(finalState, currentState) {
		if err := writeJSONFile(s.statePath(), p.state); err != nil {
			return result, err
		}
	}
	return result, nil
}

func checkFile(f *filePlan) error {
	data, exists, _, err := safeRead(f.path)
	if err != nil {
		return err
	}
	if exists != f.exists || !bytes.Equal(data, f.before) {
		return fmt.Errorf("%s changed since preview; preview again", f.path)
	}
	return nil
}
