package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestSyncBundleCodexMergesHooksAndEnablesFeature(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(t.TempDir(), "hooks", "audit")
	writeHookBundle(t, source, `
codex:
  events:
    PreToolUse:
      - command: "{HOOK_ROOT}/scripts/pre.sh"
`)
	writeFile(t, filepath.Join(source, "scripts", "pre.sh"), "#!/bin/sh\nexit 0\n")
	writeFile(t, config.CodexHooksConfigPath(""), `{"hooks":{"PreToolUse":[{"command":"echo unmanaged"}]}}`)

	bundle := mustBundle(t, source, "audit")
	res, err := SyncBundle(bundle, "", "codex")
	if err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", res.Warnings)
	}

	hooksData, err := os.ReadFile(config.CodexHooksConfigPath(""))
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}
	text := string(hooksData)
	if !strings.Contains(text, "echo unmanaged") {
		t.Fatalf("expected unmanaged hook to remain:\n%s", text)
	}
	if !strings.Contains(text, filepath.ToSlash(filepath.Join(config.CodexHooksRoot(""), "audit", "scripts", "pre.sh"))) {
		t.Fatalf("expected managed hook path in hooks.json:\n%s", text)
	}
	cfgData, err := os.ReadFile(config.CodexConfigPath())
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	if !strings.Contains(string(cfgData), "[features]") || !strings.Contains(string(cfgData), "codex_hooks = true") {
		t.Fatalf("expected codex_hooks feature enabled:\n%s", string(cfgData))
	}
}

func TestImportClaudeHooksCreatesBundle(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourceRoot := filepath.Join(t.TempDir(), "hooks")

	hookRoot := filepath.Join(home, ".claude", "hooks", "skillshare", "audit")
	writeFile(t, filepath.Join(hookRoot, "scripts", "pre.sh"), "#!/bin/sh\n")
	writeFile(t, config.ClaudeSettingsPath(""), `{"hooks":{"PreToolUse":[{"command":"`+filepath.ToSlash(filepath.Join(hookRoot, "scripts", "pre.sh"))+`"}]}}`)

	bundles, err := Import(sourceRoot, ImportOptions{From: "claude", OwnedOnly: true})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("expected 1 imported bundle, got %d", len(bundles))
	}
	data, err := os.ReadFile(filepath.Join(sourceRoot, "audit", "hook.yaml"))
	if err != nil {
		t.Fatalf("read imported hook.yaml: %v", err)
	}
	if !strings.Contains(string(data), "{HOOK_ROOT}/scripts/pre.sh") {
		t.Fatalf("expected HOOK_ROOT rewrite in hook.yaml:\n%s", string(data))
	}
}

func TestSyncBundleCodexWarnsOnUnsupportedEvents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(t.TempDir(), "hooks", "audit")
	writeHookBundle(t, source, `
codex:
  events:
    UnsupportedEvent:
      - command: "{HOOK_ROOT}/scripts/pre.sh"
`)
	writeFile(t, filepath.Join(source, "scripts", "pre.sh"), "#!/bin/sh\nexit 0\n")

	bundle := mustBundle(t, source, "audit")
	res, err := SyncBundle(bundle, "", "codex")
	if err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected unsupported event warning")
	}
}

func TestSyncBundleClaudePreservesMatcherGroupsAndMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := filepath.Join(t.TempDir(), "hooks", "audit")
	writeHookBundle(t, source, `
claude:
  events:
    PreToolUse:
      - matcher: Bash
        command: "{HOOK_ROOT}/scripts/pre.sh"
        timeout: 8000
        status_message: "Working..."
        if: "Bash(git *)"
    SessionStart:
      - command: "{HOOK_ROOT}/scripts/start.sh"
`)
	writeFile(t, filepath.Join(source, "scripts", "pre.sh"), "#!/bin/sh\nexit 0\n")
	writeFile(t, filepath.Join(source, "scripts", "start.sh"), "#!/bin/sh\nexit 0\n")

	managedRoot := filepath.Join(config.ClaudeHooksRoot(""), "audit")
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo unmanaged", "timeout": 1},
						map[string]any{"type": "command", "command": filepath.ToSlash(filepath.Join(managedRoot, "scripts", "old.sh")), "statusMessage": "old"},
					},
				},
				map[string]any{
					"matcher": map[string]any{"tool_name": "Edit"},
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo edit"},
					},
				},
			},
			"SessionStart": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo existing"},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(settings)
	writeFile(t, config.ClaudeSettingsPath(""), string(data))

	bundle := mustBundle(t, source, "audit")
	if _, err := SyncBundle(bundle, "", "claude"); err != nil {
		t.Fatalf("SyncBundle() error = %v", err)
	}

	var synced struct {
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	readJSONFile(t, config.ClaudeSettingsPath(""), &synced)

	preGroups := synced.Hooks["PreToolUse"]
	if len(preGroups) != 2 {
		t.Fatalf("expected 2 PreToolUse groups, got %#v", preGroups)
	}
	bashHooks := findClaudeGroupHooks(t, preGroups, "Bash")
	if len(bashHooks) != 2 {
		t.Fatalf("expected unmanaged + managed Bash hooks, got %#v", bashHooks)
	}
	if command, _ := bashHooks[1]["command"].(string); !strings.Contains(command, "/audit/scripts/pre.sh") {
		t.Fatalf("expected rewritten managed hook, got %#v", bashHooks[1])
	}
	if bashHooks[1]["statusMessage"] != "Working..." || int(bashHooks[1]["timeout"].(float64)) != 8000 {
		t.Fatalf("expected metadata preserved, got %#v", bashHooks[1])
	}
	if bashHooks[1]["if"] != "Bash(git *)" {
		t.Fatalf("expected if condition preserved, got %#v", bashHooks[1])
	}
	if strings.Contains(string(readRawFile(t, config.ClaudeSettingsPath(""))), "old.sh") {
		t.Fatalf("expected managed old hook to be removed")
	}

	startGroups := synced.Hooks["SessionStart"]
	if len(startGroups) != 1 {
		t.Fatalf("expected 1 SessionStart group, got %#v", startGroups)
	}
	if startGroups[0]["matcher"] != "" {
		t.Fatalf("expected non-tool default matcher to normalize to empty string, got %#v", startGroups[0]["matcher"])
	}
}

func TestImportClaudeHooksMergesLegacyAndLocalizesCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourceRoot := filepath.Join(t.TempDir(), "hooks")

	gitnexusScript := filepath.Join(home, ".claude", "hooks", "gitnexus", "gitnexus-hook.cjs")
	rewriteScript := filepath.Join(home, ".claude", "hooks", "rtk-rewrite.sh")
	legacyScript := filepath.Join(home, ".claude", "legacy", "legacy-hook.cjs")
	writeFile(t, gitnexusScript, "console.log('gitnexus')\n")
	writeFile(t, rewriteScript, "#!/bin/sh\nexit 0\n")
	writeFile(t, legacyScript, "console.log('legacy')\n")

	writeFile(t, config.ClaudeSettingsPath(""), `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "node \"`+filepath.ToSlash(gitnexusScript)+`\"",
            "timeout": 8000,
            "statusMessage": "Enriching..."
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "`+filepath.ToSlash(rewriteScript)+`"
          }
        ]
      }
    ]
  }
}`)
	writeFile(t, filepath.Join(home, ".claude", "hooks.json"), `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": {"tool_name": "Grep|Glob|Bash"},
        "hooks": [
          {
            "type": "command",
            "command": "node \"`+filepath.ToSlash(legacyScript)+`\""
          }
        ]
      }
    ]
  }
}`)

	bundles, err := Import(sourceRoot, ImportOptions{From: "claude", All: true})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(bundles) != 3 {
		t.Fatalf("expected 3 imported bundles, got %d", len(bundles))
	}

	gitnexusHook := readRawFile(t, filepath.Join(sourceRoot, "gitnexus", "hook.yaml"))
	if !strings.Contains(string(gitnexusHook), `matcher: Bash`) || !strings.Contains(string(gitnexusHook), `status_message: Enriching...`) {
		t.Fatalf("expected gitnexus metadata and matcher preserved:\n%s", gitnexusHook)
	}
	if !strings.Contains(string(gitnexusHook), `node "{HOOK_ROOT}/scripts/gitnexus-hook.cjs"`) {
		t.Fatalf("expected localized node command, got:\n%s", gitnexusHook)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "gitnexus", "scripts", "gitnexus-hook.cjs")); err != nil {
		t.Fatalf("expected localized gitnexus script copy: %v", err)
	}

	rewriteHook := readRawFile(t, filepath.Join(sourceRoot, "rtk-rewrite", "hook.yaml"))
	if !strings.Contains(string(rewriteHook), `command: '{HOOK_ROOT}/scripts/rtk-rewrite.sh'`) &&
		!strings.Contains(string(rewriteHook), `command: "{HOOK_ROOT}/scripts/rtk-rewrite.sh"`) {
		t.Fatalf("expected localized direct script command, got:\n%s", rewriteHook)
	}

	legacyHook := readRawFile(t, filepath.Join(sourceRoot, "legacy", "hook.yaml"))
	if !strings.Contains(string(legacyHook), `tool_name: Grep|Glob|Bash`) {
		t.Fatalf("expected legacy matcher object preserved, got:\n%s", legacyHook)
	}
}

func TestImportClaudeHooksOwnedOnlyAndConflict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourceRoot := filepath.Join(t.TempDir(), "hooks")

	managedScript := filepath.Join(config.ClaudeHooksRoot(""), "audit", "scripts", "pre.sh")
	unmanagedScript := filepath.Join(home, ".claude", "hooks", "gitnexus", "gitnexus-hook.cjs")
	writeFile(t, managedScript, "#!/bin/sh\nexit 0\n")
	writeFile(t, unmanagedScript, "console.log('gitnexus')\n")
	writeFile(t, config.ClaudeSettingsPath(""), `{
  "hooks": {
    "PreToolUse": [
      {"hooks": [{"type":"command","command":"`+filepath.ToSlash(managedScript)+`"}]},
      {"hooks": [{"type":"command","command":"node \"`+filepath.ToSlash(unmanagedScript)+`\""}]}
    ]
  }
}`)

	bundles, err := Import(sourceRoot, ImportOptions{From: "claude", OwnedOnly: true})
	if err != nil {
		t.Fatalf("Import() owned-only error = %v", err)
	}
	if len(bundles) != 1 || bundles[0].Name != "audit" {
		t.Fatalf("expected only managed audit bundle, got %+v", bundles)
	}
	if _, err := Import(sourceRoot, ImportOptions{From: "claude", All: true, OwnedOnly: true}); err == nil {
		t.Fatalf("expected --all and --owned-only conflict")
	}
}

func writeHookBundle(t *testing.T, root, yamlText string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	writeFile(t, filepath.Join(root, "hook.yaml"), yamlText)
}

func mustBundle(t *testing.T, source, name string) Bundle {
	t.Helper()
	cfg, warnings, err := readHookConfig(filepath.Join(source, "hook.yaml"))
	if err != nil {
		t.Fatalf("readHookConfig: %v", err)
	}
	targets := map[string]int{}
	if cfg.Claude != nil {
		targets["claude"] = countEntries(cfg.Claude.Events)
	}
	if cfg.Codex != nil {
		targets["codex"] = countEntries(cfg.Codex.Events)
	}
	return Bundle{Name: name, SourceDir: source, Config: cfg, Targets: targets, Warnings: warnings}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string, dst any) {
	t.Helper()
	data := readRawFile(t, path)
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("unmarshal %s: %v\n%s", path, err, data)
	}
}

func readRawFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func findClaudeGroupHooks(t *testing.T, groups []map[string]any, matcher string) []map[string]any {
	t.Helper()
	for _, group := range groups {
		if group["matcher"] != matcher {
			continue
		}
		rawHooks, ok := group["hooks"].([]any)
		if !ok {
			t.Fatalf("hooks field has unexpected type: %#v", group["hooks"])
		}
		var out []map[string]any
		for _, item := range rawHooks {
			hook, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("hook item has unexpected type: %#v", item)
			}
			out = append(out, hook)
		}
		return out
	}
	t.Fatalf("matcher %q not found in %#v", matcher, groups)
	return nil
}
