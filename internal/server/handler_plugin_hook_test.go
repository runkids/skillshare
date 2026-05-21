package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlePluginsAndHooks(t *testing.T) {
	s, _ := newTestServer(t)

	pluginRoot := filepath.Join(s.pluginSourceDir(), "demo")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	hookRoot := filepath.Join(s.hookSourceDir(), "audit")
	if err := os.MkdirAll(hookRoot, 0o755); err != nil {
		t.Fatalf("mkdir hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hookRoot, "hook.yaml"), []byte("claude:\n  events:\n    PreToolUse:\n      - command: \"{HOOK_ROOT}/scripts/pre.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write hook.yaml: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("plugins status = %d body=%s", rr.Code, rr.Body.String())
	}
	var pluginsResp struct {
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &pluginsResp); err != nil {
		t.Fatalf("unmarshal plugins: %v", err)
	}
	if len(pluginsResp.Plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(pluginsResp.Plugins))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/hooks", nil)
	rr = httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("hooks status = %d body=%s", rr.Code, rr.Body.String())
	}
	var hooksResp struct {
		Hooks []map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &hooksResp); err != nil {
		t.Fatalf("unmarshal hooks: %v", err)
	}
	if len(hooksResp.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooksResp.Hooks))
	}
}

func TestHandlePluginAndHookDiffOnlyIncludesSupportedTargets(t *testing.T) {
	s, _ := newTestServer(t)

	pluginRoot := filepath.Join(s.pluginSourceDir(), "claude-only")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".claude-plugin"), 0o755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"), []byte(`{"name":"claude-only"}`), 0o644); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	hookRoot := filepath.Join(s.hookSourceDir(), "claude-only")
	if err := os.MkdirAll(hookRoot, 0o755); err != nil {
		t.Fatalf("mkdir hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hookRoot, "hook.yaml"), []byte("claude:\n  events:\n    SessionStart:\n      - command: \"{HOOK_ROOT}/scripts/start.sh\"\n"), 0o644); err != nil {
		t.Fatalf("write hook.yaml: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("plugins diff status = %d body=%s", rr.Code, rr.Body.String())
	}
	var pluginsResp struct {
		Plugins []struct {
			Name   string `json:"name"`
			Target string `json:"target"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &pluginsResp); err != nil {
		t.Fatalf("unmarshal plugins diff: %v", err)
	}
	if len(pluginsResp.Plugins) != 2 {
		t.Fatalf("expected generated plugin targets, got %+v", pluginsResp.Plugins)
	}
	targets := map[string]bool{}
	for _, entry := range pluginsResp.Plugins {
		targets[entry.Target] = true
	}
	if !targets["claude"] || !targets["codex"] {
		t.Fatalf("expected claude and codex plugin diff entries, got %+v", pluginsResp.Plugins)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/hooks/diff", nil)
	rr = httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("hooks diff status = %d body=%s", rr.Code, rr.Body.String())
	}
	var hooksResp struct {
		Hooks []struct {
			Name   string `json:"name"`
			Target string `json:"target"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &hooksResp); err != nil {
		t.Fatalf("unmarshal hooks diff: %v", err)
	}
	if len(hooksResp.Hooks) != 1 || hooksResp.Hooks[0].Target != "claude" {
		t.Fatalf("expected only claude hook diff entry, got %+v", hooksResp.Hooks)
	}
}
