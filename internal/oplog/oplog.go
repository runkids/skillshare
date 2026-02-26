// Package oplog provides a persistent operation log for CLI commands.
// Entries are stored in JSONL format (one JSON object per line).
package oplog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

const (
	OpsFile   = "operations.log"
	AuditFile = "audit.log"
)

// Entry represents one log line in JSONL format.
type Entry struct {
	Timestamp string         `json:"ts"`
	Command   string         `json:"cmd"`
	Args      map[string]any `json:"args,omitempty"`
	Status    string         `json:"status"`
	Message   string         `json:"msg,omitempty"`
	Duration  int64          `json:"ms,omitempty"`
}

// LogDir returns the logs directory derived from a config file path.
// For project mode: .skillshare/logs/ (alongside project config)
// For global mode: $XDG_STATE_HOME/skillshare/logs/
func LogDir(configPath string) string {
	configDir := filepath.Dir(configPath)
	if filepath.Base(configDir) == ".skillshare" {
		return filepath.Join(configDir, "logs")
	}
	return filepath.Join(config.StateDir(), "logs")
}

// Write appends a single JSONL entry to the named log file.
func Write(configPath, filename string, e Entry) error {
	dir := LogDir(configPath)
	dirInfo, statErr := os.Stat(dir)
	logsDirMissing := os.IsNotExist(statErr)
	if statErr != nil && !logsDirMissing {
		return statErr
	}
	if statErr == nil && !dirInfo.IsDir() {
		return &os.PathError{Op: "mkdir", Path: dir, Err: os.ErrInvalid}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if logsDirMissing {
		// Backward compatibility: when an existing project first creates logs/,
		// ensure logs/ is ignored so operation logs aren't accidentally committed.
		ensureProjectLogGitignore(configPath)
	}

	f, err := os.OpenFile(filepath.Join(dir, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(e)
}

func ensureProjectLogGitignore(configPath string) {
	configDir := filepath.Dir(configPath)
	if filepath.Base(configDir) != ".skillshare" {
		return
	}
	_ = install.UpdateGitIgnore(configDir, "logs")
}

// WriteWithLimit appends an entry and truncates the file if it exceeds maxEntries.
// maxEntries <= 0 means unlimited (same as Write).
func WriteWithLimit(configPath, filename string, e Entry, maxEntries int) error {
	if err := Write(configPath, filename, e); err != nil {
		return err
	}

	if maxEntries <= 0 {
		return nil
	}

	// Hysteresis: only truncate when 20% over limit to avoid frequent rewrites
	threshold := maxEntries + maxEntries/5
	path := filepath.Join(LogDir(configPath), filename)
	entries, err := readAllEntries(path)
	if err != nil || len(entries) <= threshold {
		return nil
	}

	// Keep newest maxEntries (entries are in file order = oldest first)
	keep := entries[len(entries)-maxEntries:]
	return rewriteEntries(path, keep)
}

// readAllEntries reads all entries from the log file in file order (oldest first).
func readAllEntries(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		all = append(all, e)
	}
	return all, scanner.Err()
}

// rewriteEntries atomically replaces the log file with the given entries.
func rewriteEntries(path string, entries []Entry) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, path)
}

// Read returns the last `limit` entries from the named log file (newest first).
// If limit <= 0, all entries are returned.
func Read(configPath, filename string, limit int) ([]Entry, error) {
	path := filepath.Join(LogDir(configPath), filename)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Entry
	dec := json.NewDecoder(f)
	for dec.More() {
		var e Entry
		if err := dec.Decode(&e); err != nil {
			// Skip malformed entries — advance past the bad line
			if dec.More() {
				continue
			}
			break
		}
		all = append(all, e)
	}

	// Reverse to newest-first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	return all, nil
}

// DeleteEntries removes entries that match the given list from the named log file.
// Matching is by content (timestamp + command + status + duration) rather than index,
// because Read() returns newest-first while the file stores oldest-first.
// Returns the number of entries actually deleted.
func DeleteEntries(configPath, filename string, matches []Entry) (int, error) {
	if len(matches) == 0 {
		return 0, nil
	}

	path := filepath.Join(LogDir(configPath), filename)
	all, err := readAllEntries(path)
	if err != nil {
		return 0, err
	}

	// Build match set keyed by (ts, cmd, status, duration)
	type entryKey struct {
		ts     string
		cmd    string
		status string
		dur    int64
	}
	matchCounts := make(map[entryKey]int, len(matches))
	for _, m := range matches {
		k := entryKey{m.Timestamp, m.Command, m.Status, m.Duration}
		matchCounts[k]++
	}

	kept := make([]Entry, 0, len(all))
	deleted := 0
	for _, e := range all {
		k := entryKey{e.Timestamp, e.Command, e.Status, e.Duration}
		if matchCounts[k] > 0 {
			matchCounts[k]--
			deleted++
			continue
		}
		kept = append(kept, e)
	}

	if deleted == 0 {
		return 0, nil
	}

	if err := rewriteEntries(path, kept); err != nil {
		return 0, err
	}
	return deleted, nil
}

// Clear truncates the named log file.
func Clear(configPath, filename string) error {
	path := filepath.Join(LogDir(configPath), filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Truncate(path, 0)
}

// NewEntry creates an Entry with the current timestamp.
func NewEntry(cmd, status string, duration time.Duration) Entry {
	return Entry{
		Timestamp: time.Now().Format(time.RFC3339),
		Command:   cmd,
		Status:    status,
		Duration:  duration.Milliseconds(),
	}
}
