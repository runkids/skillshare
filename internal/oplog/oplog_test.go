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
