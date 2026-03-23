package mcp_test

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/mcp"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestCopyToTarget_NewFile(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "generated", "mcp.json")
	target := filepath.Join(dir, "target", "mcp.json")
	writeFile(t, generated, `{"servers":{}}`)

	result := mcp.CopyToTarget("claude", generated, target, false)

	if result.Status != "copied" {
		t.Errorf("status: got %q, want %q", result.Status, "copied")
	}
	if result.Target != "claude" {
		t.Errorf("target: got %q, want %q", result.Target, "claude")
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %q", result.Error)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("target file not created: %v", err)
	}
	if string(data) != `{"servers":{}}` {
		t.Errorf("content mismatch: got %q", string(data))
	}
}

func TestCopyToTarget_UnchangedContent(t *testing.T) {
	dir := t.TempDir()
	content := `{"servers":{}}`
	generated := filepath.Join(dir, "generated", "mcp.json")
	target := filepath.Join(dir, "target", "mcp.json")
	writeFile(t, generated, content)
	writeFile(t, target, content)

	result := mcp.CopyToTarget("cursor", generated, target, false)

	if result.Status != "ok" {
		t.Errorf("status: got %q, want %q", result.Status, "ok")
	}
}

func TestCopyToTarget_ChangedContent(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "generated", "mcp.json")
	target := filepath.Join(dir, "target", "mcp.json")
	writeFile(t, generated, `{"servers":{"fetch":{}}}`)
	writeFile(t, target, `{"servers":{}}`)

	result := mcp.CopyToTarget("vscode", generated, target, false)

	if result.Status != "updated" {
		t.Errorf("status: got %q, want %q", result.Status, "updated")
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %q", result.Error)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(data) != `{"servers":{"fetch":{}}}` {
		t.Errorf("content not updated: got %q", string(data))
	}
}

func TestCopyToTarget_DryRun_NewFile(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "generated", "mcp.json")
	target := filepath.Join(dir, "target", "mcp.json")
	writeFile(t, generated, `{"servers":{}}`)

	result := mcp.CopyToTarget("claude", generated, target, true)

	if result.Status != "copied" {
		t.Errorf("status: got %q, want %q", result.Status, "copied")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("dry-run should not create the file")
	}
}

func TestCopyToTarget_DryRun_ChangedContent(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "generated", "mcp.json")
	target := filepath.Join(dir, "target", "mcp.json")
	writeFile(t, generated, `{"new":"content"}`)
	writeFile(t, target, `{"old":"content"}`)

	result := mcp.CopyToTarget("claude", generated, target, true)

	if result.Status != "updated" {
		t.Errorf("status: got %q, want %q", result.Status, "updated")
	}

	data, _ := os.ReadFile(target)
	if string(data) != `{"old":"content"}` {
		t.Error("dry-run should not modify existing file")
	}
}
