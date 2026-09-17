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

// readPending returns an interrupted write's journal and whether its native
// file was written, in which case recovery records the journal's ownership.
func (s *Service) readPending() (*journal, bool, error) {
	data, exists, _, err := safeRead(filepath.Join(s.StateDir, "mcp", "pending.json"))
	if err != nil || !exists {
		return nil, false, err
	}
	var pending journal
	if json.Unmarshal(data, &pending) != nil || pending.Path == "" || pending.State.Version != 1 || pending.State.Entries == nil {
		return nil, false, fmt.Errorf("MCP recovery journal is invalid; manual recovery required")
	}
	current, exists, _, err := safeRead(pending.Path)
	if err != nil {
		return nil, false, err
	}
	return &pending, exists && digest(current) == pending.After, nil
}

func (s *Service) recoverPending() error {
	pending, written, err := s.readPending()
	if err != nil || pending == nil {
		return err
	}
	if written {
		if err := writeJSONFile(s.statePath(), pending.State); err != nil {
			return err
		}
	}
	// Otherwise the write did not occur, or the file changed again; ownership
	// hashes flag any entry that no longer matches, so the journal is obsolete.
	return os.Remove(filepath.Join(s.StateDir, "mcp", "pending.json"))
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
		if _, _, _, _, err := refreshFile(f); err != nil {
			return result, err
		}
	}
	for _, f := range p.files {
		if bytes.Equal(f.before, f.after) || len(f.changes) == 0 {
			continue
		}
		data, exists, mode, native, err := refreshFile(f)
		if err != nil {
			return result, err
		}
		// Apply the previewed entry edits to the file as it is now, keeping
		// settings the Agent wrote since the preview.
		// ponytail: the Agent can still write between this read and the rename;
		// closing that window needs a lock that Agents do not take.
		after, err := native.Edit(f.changes)
		if err != nil {
			return result, fmt.Errorf("%s: %w", f.path, err)
		}
		id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), backupSuffix(f.path))
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
		pending := journal{Path: f.path, Before: digest(data), After: digest(after), BeforeExists: exists, State: state}
		journalPath := filepath.Join(s.StateDir, "mcp", "pending.json")
		if err := writeJSONFile(journalPath, pending); err != nil {
			return result, err
		}
		if err := atomicWrite(f.path, after, mode); err != nil {
			// The rename is the last step, so the file is untouched; a journal left
			// behind by a failed removal is recovered by the next operation.
			_ = os.Remove(journalPath)
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
		s.pruneBackups(f.path)
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

// refreshFile rereads an Agent file and confirms its MCP entries still match
// the preview. Other settings may change: Agents such as Claude Code rewrite
// their configuration files constantly.
func refreshFile(f *filePlan) ([]byte, bool, os.FileMode, *Native, error) {
	data, exists, mode, err := safeRead(f.path)
	if err != nil {
		return nil, false, 0, nil, err
	}
	native, err := ParseNative(f.target, data)
	if err != nil {
		return nil, false, 0, nil, fmt.Errorf("%s: %w", f.path, err)
	}
	if sectionDigest(native) != f.section {
		return nil, false, 0, nil, fmt.Errorf("%s MCP entries changed since preview; preview again", f.path)
	}
	return data, exists, mode, native, nil
}
