package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectPluginAndHookDiffOnlyIncludesSupportedTargets(t *testing.T) {
	root := t.TempDir()
	pluginSource := filepath.Join(root, "plugins")
	hookSource := filepath.Join(root, "hooks")

	if err := os.MkdirAll(filepath.Join(pluginSource, "claude-only", ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginSource, "claude-only", ".claude-plugin", "plugin.json"), []byte(`{"name":"claude-only"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginSource, "generated"), 0o755); err != nil {
		t.Fatalf("mkdir generated plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginSource, "generated", "skillshare.plugin.yaml"), []byte("shared:\n  name: generated\n"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(hookSource, "claude-only"), 0o755); err != nil {
		t.Fatalf("mkdir hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hookSource, "claude-only", "hook.yaml"), []byte("claude:\n  events:\n    SessionStart:\n      - command: \"{HOOK_ROOT}/scripts/start.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write hook manifest: %v", err)
	}

	pluginDiff := collectPluginDiff(pluginSource, "")
	if len(pluginDiff) != 4 {
		t.Fatalf("expected 3 plugin diff entries, got %+v", pluginDiff)
	}
	targets := map[string]bool{}
	for _, entry := range pluginDiff {
		if entry.Name == "claude-only" {
			targets[entry.Target] = true
		}
	}
	if !targets["claude"] || !targets["codex"] {
		t.Fatalf("expected generated plugin targets for claude-only bundle, got %+v", pluginDiff)
	}

	hookDiff := collectHookDiff(hookSource, "")
	if len(hookDiff) != 1 || hookDiff[0].Target != "claude" {
		t.Fatalf("expected only claude hook diff entry, got %+v", hookDiff)
	}
}
