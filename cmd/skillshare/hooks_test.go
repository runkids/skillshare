package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdHooksSyncFiltersByName(t *testing.T) {
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "hooks", "audit", "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir audit: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "hooks", "notify", "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir notify: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "audit", "hook.yaml"), []byte("name: audit\nclaude:\n  events:\n    PreToolUse:\n      - command: \"{HOOK_ROOT}/scripts/pre.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write audit hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "notify", "hook.yaml"), []byte("name: notify\nclaude:\n  events:\n    PreToolUse:\n      - command: \"{HOOK_ROOT}/scripts/pre.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write notify hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "audit", "scripts", "pre.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write audit script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "notify", "scripts", "pre.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write notify script: %v", err)
	}

	output := captureStdout(t, func() {
		if err := cmdHooksSync([]string{"-p", "audit", "--target", "claude", "--json"}); err != nil {
			t.Fatalf("cmdHooksSync: %v", err)
		}
	})

	var payload struct {
		Hooks []struct {
			Name   string `json:"name"`
			Target string `json:"target"`
			Root   string `json:"root"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, output)
	}
	if len(payload.Hooks) != 1 || payload.Hooks[0].Name != "audit" || payload.Hooks[0].Target != "claude" {
		t.Fatalf("unexpected hooks payload: %+v", payload.Hooks)
	}
	if strings.Contains(output, "notify") {
		t.Fatalf("expected filtered sync output, got %s", output)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "hooks", "skillshare", "audit", "scripts", "pre.sh")); err != nil {
		t.Fatalf("audit hook not rendered: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "hooks", "skillshare", "notify")); !os.IsNotExist(err) {
		t.Fatalf("notify hook should not be rendered, err=%v", err)
	}
}

func TestCmdHooksSyncTextShowsWarningsForRenderedBundles(t *testing.T) {
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	if err := os.MkdirAll(filepath.Join(root, ".skillshare", "hooks", "audit", "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir audit: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "hooks.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write codex hooks: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "audit", "hook.yaml"), []byte("name: audit\ncodex:\n  events:\n    UnsupportedEvent:\n      - command: \"{HOOK_ROOT}/scripts/pre.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write audit hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skillshare", "hooks", "audit", "scripts", "pre.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write audit script: %v", err)
	}

	output := stripANSIWarnings(captureStdout(t, func() {
		if err := cmdHooksSync([]string{"-p", "audit", "--target", "codex"}); err != nil {
			t.Fatalf("cmdHooksSync: %v", err)
		}
	}))
	if !strings.Contains(output, "audit ->") {
		t.Fatalf("expected rendered row, got %s", output)
	}
	if !strings.Contains(output, "unsupported codex event UnsupportedEvent") {
		t.Fatalf("expected warning output, got %s", output)
	}
}
