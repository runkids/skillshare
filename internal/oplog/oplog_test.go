package oplog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempConfigPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Isolate XDG_STATE_HOME so global-mode LogDir() doesn't collide across tests
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	return filepath.Join(dir, "config.yaml")
}

func TestLogDir_Global(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")

	got := LogDir("/home/user/.config/skillshare/config.yaml")
	want := "/custom/state/skillshare/logs"
	if got != want {
		t.Errorf("LogDir(global) = %q, want %q", got, want)
	}
}

func TestLogDir_Project(t *testing.T) {
	got := LogDir("/project/.skillshare/config.yaml")
	want := "/project/.skillshare/logs"
	if got != want {
		t.Errorf("LogDir(project) = %q, want %q", got, want)
	}
}

func TestWriteAndRead(t *testing.T) {
	cfgPath := tempConfigPath(t)

	e1 := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "install", Status: "ok", Duration: 100}
	e2 := Entry{Timestamp: "2026-01-01T10:01:00Z", Command: "sync", Status: "ok", Duration: 200}
	e3 := Entry{Timestamp: "2026-01-01T10:02:00Z", Command: "update", Status: "error", Message: "timeout"}

	for _, e := range []Entry{e1, e2, e3} {
		if err := Write(cfgPath, OpsFile, e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	// Read all — should be newest first
	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Read() got %d entries, want 3", len(entries))
	}
	if entries[0].Command != "update" {
		t.Errorf("entries[0].Command = %q, want %q", entries[0].Command, "update")
	}
	if entries[2].Command != "install" {
		t.Errorf("entries[2].Command = %q, want %q", entries[2].Command, "install")
	}
}

func TestReadWithLimit(t *testing.T) {
	cfgPath := tempConfigPath(t)

	for i := 0; i < 10; i++ {
		e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok"}
		if err := Write(cfgPath, OpsFile, e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	entries, err := Read(cfgPath, OpsFile, 3)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Read(limit=3) got %d entries, want 3", len(entries))
	}
}

func TestReadEmptyFile(t *testing.T) {
	cfgPath := tempConfigPath(t)

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if entries != nil {
		t.Errorf("Read() on non-existent file should return nil, got %v", entries)
	}
}

func TestClear(t *testing.T) {
	cfgPath := tempConfigPath(t)

	e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok"}
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if err := Clear(cfgPath, OpsFile); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() after clear error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Read() after clear got %d entries, want 0", len(entries))
	}
}

func TestClearNonExistent(t *testing.T) {
	cfgPath := tempConfigPath(t)
	if err := Clear(cfgPath, OpsFile); err != nil {
		t.Errorf("Clear() on non-existent file should not error, got: %v", err)
	}
}

func TestWriteWithArgs(t *testing.T) {
	cfgPath := tempConfigPath(t)

	e := Entry{
		Timestamp: "2026-01-01T10:00:00Z",
		Command:   "install",
		Args:      map[string]any{"source": "anthropics/skills", "track": true},
		Status:    "ok",
		Duration:  500,
	}
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Args["source"] != "anthropics/skills" {
		t.Errorf("Args[source] = %v, want %q", entries[0].Args["source"], "anthropics/skills")
	}
	if entries[0].Args["track"] != true {
		t.Errorf("Args[track] = %v, want true", entries[0].Args["track"])
	}
}

func TestAuditFileIsSeparate(t *testing.T) {
	cfgPath := tempConfigPath(t)

	opsEntry := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "install", Status: "ok"}
	auditEntry := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "audit", Status: "ok"}

	if err := Write(cfgPath, OpsFile, opsEntry); err != nil {
		t.Fatalf("Write ops error: %v", err)
	}
	if err := Write(cfgPath, AuditFile, auditEntry); err != nil {
		t.Fatalf("Write audit error: %v", err)
	}

	ops, _ := Read(cfgPath, OpsFile, 0)
	audit, _ := Read(cfgPath, AuditFile, 0)

	if len(ops) != 1 || ops[0].Command != "install" {
		t.Errorf("ops file should have 1 install entry, got %d", len(ops))
	}
	if len(audit) != 1 || audit[0].Command != "audit" {
		t.Errorf("audit file should have 1 audit entry, got %d", len(audit))
	}
}

func TestNewEntry(t *testing.T) {
	e := NewEntry("sync", "ok", 350*time.Millisecond)
	if e.Command != "sync" {
		t.Errorf("Command = %q, want %q", e.Command, "sync")
	}
	if e.Status != "ok" {
		t.Errorf("Status = %q, want %q", e.Status, "ok")
	}
	if e.Duration != 350 {
		t.Errorf("Duration = %d, want 350", e.Duration)
	}
	if e.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}

	// Ensure it can be written
	cfgPath := tempConfigPath(t)
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write(NewEntry) error: %v", err)
	}
}

func TestLogDirCreatesDirectory(t *testing.T) {
	cfgPath := tempConfigPath(t)
	dir := LogDir(cfgPath)

	// Dir shouldn't exist yet
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("logs dir should not exist before Write()")
	}

	e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "test", Status: "ok"}
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("logs dir should exist after Write(), got: %v", err)
	}
}

func TestWrite_ProjectFirstLogCreationAddsLogsToGitignore(t *testing.T) {
	root := t.TempDir()
	projectSkillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(projectSkillshareDir, 0755); err != nil {
		t.Fatalf("failed to create .skillshare dir: %v", err)
	}

	cfgPath := filepath.Join(projectSkillshareDir, "config.yaml")
	entry := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok"}

	if err := Write(cfgPath, OpsFile, entry); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	gitignorePath := filepath.Join(projectSkillshareDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("expected .gitignore to exist, read error: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "logs/") {
		t.Fatalf("expected .gitignore to include logs/, got:\n%s", text)
	}

	if err := Write(cfgPath, AuditFile, entry); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}

	content2, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore after second write: %v", err)
	}
	if strings.Count(string(content2), "logs/") != 1 {
		t.Fatalf("expected logs/ to appear exactly once, got:\n%s", string(content2))
	}
}

func TestWriteWithLimit_Truncates(t *testing.T) {
	cfgPath := tempConfigPath(t)
	maxEntries := 10
	// threshold = 10 + 10/5 = 12
	// Write 13 entries: entry 13 triggers truncation (13 > 12), keeping newest 10
	total := 13

	for i := 0; i < total; i++ {
		e := Entry{
			Timestamp: fmt.Sprintf("2026-01-01T10:%02d:00Z", i),
			Command:   fmt.Sprintf("cmd-%d", i),
			Status:    "ok",
		}
		if err := WriteWithLimit(cfgPath, OpsFile, e, maxEntries); err != nil {
			t.Fatalf("WriteWithLimit() error on entry %d: %v", i, err)
		}
	}

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != maxEntries {
		t.Fatalf("got %d entries, want %d", len(entries), maxEntries)
	}

	// Newest entry should be cmd-12 (Read returns newest first)
	if entries[0].Command != "cmd-12" {
		t.Errorf("newest entry = %q, want %q", entries[0].Command, "cmd-12")
	}
	// Oldest kept entry should be cmd-3
	if entries[len(entries)-1].Command != "cmd-3" {
		t.Errorf("oldest kept entry = %q, want %q", entries[len(entries)-1].Command, "cmd-3")
	}
}

func TestWriteWithLimit_ZeroMeansUnlimited(t *testing.T) {
	cfgPath := tempConfigPath(t)

	for i := 0; i < 20; i++ {
		e := Entry{
			Timestamp: fmt.Sprintf("2026-01-01T10:%02d:00Z", i),
			Command:   "sync",
			Status:    "ok",
		}
		if err := WriteWithLimit(cfgPath, OpsFile, e, 0); err != nil {
			t.Fatalf("WriteWithLimit() error: %v", err)
		}
	}

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 20 {
		t.Errorf("got %d entries, want 20 (unlimited)", len(entries))
	}
}

func TestDeleteEntries_Basic(t *testing.T) {
	cfgPath := tempConfigPath(t)

	entries := []Entry{
		{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100},
		{Timestamp: "2026-01-01T10:01:00Z", Command: "install", Status: "ok", Duration: 200},
		{Timestamp: "2026-01-01T10:02:00Z", Command: "audit", Status: "error", Duration: 300},
	}
	for _, e := range entries {
		if err := Write(cfgPath, OpsFile, e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	// Delete the middle entry
	deleted, err := DeleteEntries(cfgPath, OpsFile, []Entry{entries[1]})
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	remaining, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("got %d entries, want 2", len(remaining))
	}
	// Read returns newest first
	if remaining[0].Command != "audit" {
		t.Errorf("remaining[0].Command = %q, want audit", remaining[0].Command)
	}
	if remaining[1].Command != "sync" {
		t.Errorf("remaining[1].Command = %q, want sync", remaining[1].Command)
	}
}

func TestDeleteEntries_NoMatch(t *testing.T) {
	cfgPath := tempConfigPath(t)

	e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100}
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	noMatch := Entry{Timestamp: "2099-01-01T00:00:00Z", Command: "nope", Status: "ok", Duration: 0}
	deleted, err := DeleteEntries(cfgPath, OpsFile, []Entry{noMatch})
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}

	remaining, _ := Read(cfgPath, OpsFile, 0)
	if len(remaining) != 1 {
		t.Fatalf("got %d entries, want 1", len(remaining))
	}
}

func TestDeleteEntries_DeleteAll(t *testing.T) {
	cfgPath := tempConfigPath(t)

	entries := []Entry{
		{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100},
		{Timestamp: "2026-01-01T10:01:00Z", Command: "install", Status: "ok", Duration: 200},
	}
	for _, e := range entries {
		if err := Write(cfgPath, OpsFile, e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	deleted, err := DeleteEntries(cfgPath, OpsFile, entries)
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	remaining, _ := Read(cfgPath, OpsFile, 0)
	if len(remaining) != 0 {
		t.Fatalf("got %d entries, want 0", len(remaining))
	}
}

func TestDeleteEntries_DuplicateEntries(t *testing.T) {
	cfgPath := tempConfigPath(t)

	// Write 3 identical entries
	e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100}
	for range 3 {
		if err := Write(cfgPath, OpsFile, e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	// Delete only 1 of the 3 duplicates
	deleted, err := DeleteEntries(cfgPath, OpsFile, []Entry{e})
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	remaining, _ := Read(cfgPath, OpsFile, 0)
	if len(remaining) != 2 {
		t.Fatalf("got %d entries, want 2", len(remaining))
	}
}

func TestDeleteEntries_EmptyMatches(t *testing.T) {
	cfgPath := tempConfigPath(t)

	e := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100}
	if err := Write(cfgPath, OpsFile, e); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	deleted, err := DeleteEntries(cfgPath, OpsFile, nil)
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}

func TestDeleteEntries_SeparateFiles(t *testing.T) {
	cfgPath := tempConfigPath(t)

	opsEntry := Entry{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100}
	auditEntry := Entry{Timestamp: "2026-01-01T10:01:00Z", Command: "audit", Status: "ok", Duration: 200}

	if err := Write(cfgPath, OpsFile, opsEntry); err != nil {
		t.Fatalf("Write ops error: %v", err)
	}
	if err := Write(cfgPath, AuditFile, auditEntry); err != nil {
		t.Fatalf("Write audit error: %v", err)
	}

	// Delete from audit file only — ops should be untouched
	deleted, err := DeleteEntries(cfgPath, AuditFile, []Entry{auditEntry})
	if err != nil {
		t.Fatalf("DeleteEntries() error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	ops, _ := Read(cfgPath, OpsFile, 0)
	audit, _ := Read(cfgPath, AuditFile, 0)
	if len(ops) != 1 {
		t.Fatalf("ops should have 1 entry, got %d", len(ops))
	}
	if len(audit) != 0 {
		t.Fatalf("audit should have 0 entries, got %d", len(audit))
	}
}

func TestRewriteEntries_PreservesOriginalOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Seed the log with known entries via rewriteEntries
	original := []Entry{
		{Timestamp: "2026-01-01T10:00:00Z", Command: "sync", Status: "ok", Duration: 100},
		{Timestamp: "2026-01-01T10:01:00Z", Command: "install", Status: "ok", Duration: 200},
	}
	if err := rewriteEntries(path, original); err != nil {
		t.Fatalf("initial rewriteEntries() error: %v", err)
	}

	// Make directory read-only so .tmp file creation fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("chmod error: %v", err)
	}
	defer os.Chmod(dir, 0755) // restore for cleanup

	// Attempt rewrite — should fail
	replacement := []Entry{
		{Timestamp: "2026-01-01T11:00:00Z", Command: "update", Status: "ok", Duration: 999},
	}
	err := rewriteEntries(path, replacement)
	if err == nil {
		// Restore permissions before failing so cleanup works
		os.Chmod(dir, 0755)
		t.Fatal("rewriteEntries() should have failed on read-only directory")
	}

	// Restore permissions to read the file
	os.Chmod(dir, 0755)

	// Verify original file is preserved intact
	entries, err := readAllEntries(path)
	if err != nil {
		t.Fatalf("readAllEntries() error: %v", err)
	}
	if len(entries) != len(original) {
		t.Fatalf("got %d entries, want %d — original was corrupted", len(entries), len(original))
	}
	if entries[0].Command != "sync" || entries[1].Command != "install" {
		t.Errorf("original entries changed: got %v", entries)
	}
}

func TestRewriteEntries_NoTmpFileLeftOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Write initial data
	if err := rewriteEntries(path, []Entry{
		{Timestamp: "2026-01-01T10:00:00Z", Command: "cmd1", Status: "ok"},
	}); err != nil {
		t.Fatalf("initial rewriteEntries() error: %v", err)
	}

	// Make directory read-only
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatalf("chmod error: %v", err)
	}
	defer os.Chmod(dir, 0755)

	_ = rewriteEntries(path, []Entry{
		{Timestamp: "2026-01-01T11:00:00Z", Command: "cmd2", Status: "ok"},
	})

	os.Chmod(dir, 0755)

	// Verify no .tmp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("expected .tmp file to not exist after failed rewrite, but it does")
	}
}

func TestWriteWithLimit_HysteresisAvoidsTruncation(t *testing.T) {
	cfgPath := tempConfigPath(t)
	maxEntries := 10

	// Write 11 entries — threshold is 10 + 10/5 = 12, so 11 <= 12 should NOT truncate
	for i := 0; i < 11; i++ {
		e := Entry{
			Timestamp: fmt.Sprintf("2026-01-01T10:%02d:00Z", i),
			Command:   fmt.Sprintf("cmd-%d", i),
			Status:    "ok",
		}
		if err := WriteWithLimit(cfgPath, OpsFile, e, maxEntries); err != nil {
			t.Fatalf("WriteWithLimit() error: %v", err)
		}
	}

	entries, err := Read(cfgPath, OpsFile, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(entries) != 11 {
		t.Errorf("got %d entries, want 11 (hysteresis should prevent truncation)", len(entries))
	}
}
