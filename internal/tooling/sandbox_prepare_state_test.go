package tooling

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepareSandboxHostStateCopiesOnlyReferencedHookScripts(t *testing.T) {
	hostHome := t.TempDir()
	sandboxHome := t.TempDir()

	quotedScript := filepath.Join(hostHome, ".claude", "hooks", "quoted", "gitnexus-hook.cjs")
	directScript := filepath.Join(hostHome, ".claude", "hooks", "direct", "rewrite.sh")
	unrelatedFile := filepath.Join(hostHome, ".claude", "notes", "ignore-me.txt")

	writeSandboxTestFile(t, quotedScript, "console.log('quoted')\n")
	writeSandboxTestFile(t, directScript, "#!/bin/sh\nexit 0\n")
	writeSandboxTestFile(t, unrelatedFile, "ignore\n")
	writeSandboxTestFile(t, filepath.Join(hostHome, ".claude", "settings.json"), `{
  "hooks": {
    "PreToolUse": [
      {
        "hooks": [
          {"type": "command", "command": "node \"`+filepath.ToSlash(quotedScript)+`\""}
        ]
      }
    ]
  },
  "notes_path": "`+filepath.ToSlash(unrelatedFile)+`"
}`)
	writeSandboxTestFile(t, filepath.Join(hostHome, ".claude", "hooks.json"), `{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {"type": "command", "command": "`+filepath.ToSlash(directScript)+`"}
        ]
      }
    ]
  }
}`)
	writeSandboxTestFile(t, filepath.Join(hostHome, ".codex", "config.toml"), "")
	writeSandboxTestFile(t, filepath.Join(hostHome, ".codex", "hooks.json"), `{"hooks":{}}`)

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	scriptPath := filepath.Join(repoRoot, "scripts", "prepare_sandbox_host_state.sh")

	cmd := exec.Command("bash", scriptPath, hostHome, sandboxHome)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare_sandbox_host_state.sh failed: %v\n%s", err, output)
	}

	for _, path := range []string{
		filepath.Join(sandboxHome, ".claude", "hooks", "quoted", "gitnexus-hook.cjs"),
		filepath.Join(sandboxHome, ".claude", "hooks", "direct", "rewrite.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected copied hook script %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(sandboxHome, ".claude", "notes", "ignore-me.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected unrelated file to stay uncopied, err=%v", err)
	}

	settingsData, err := os.ReadFile(filepath.Join(sandboxHome, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read sandbox settings: %v", err)
	}
	settingsText := string(settingsData)
	if strings.Contains(settingsText, filepath.ToSlash(hostHome)) {
		t.Fatalf("expected sandbox settings to rewrite host home:\n%s", settingsText)
	}
	if !strings.Contains(settingsText, filepath.ToSlash(sandboxHome)) {
		t.Fatalf("expected sandbox settings to contain sandbox home:\n%s", settingsText)
	}
}

func writeSandboxTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
