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
	if err := scanner.Err(); err != nil {
		return nil, err
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
