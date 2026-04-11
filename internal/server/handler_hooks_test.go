package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestHandleListHooks_Empty(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("source: "+filepath.Join(tmp, "skills")+"\nmode: merge\ntargets: {}\n"), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	s := New(cfg, "127.0.0.1:0", "", "")

	req := httptest.NewRequest(http.MethodGet, "/api/hooks", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Hooks []any `json:"hooks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Hooks) != 0 {
		t.Fatalf("expected 0 hooks, got %d", len(resp.Hooks))
	}
}

func TestHandleListHooks_UsesProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	homeHooksDir := filepath.Join(homeDir, ".claude")
	os.MkdirAll(homeHooksDir, 0755)
	os.WriteFile(filepath.Join(homeHooksDir, "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"home"}]}]}}`), 0644)

	projectHooksDir := filepath.Join(projectRoot, ".claude")
	os.MkdirAll(projectHooksDir, 0755)
	os.WriteFile(filepath.Join(projectHooksDir, "settings.json"), []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"project"}]}]}}`), 0644)

	projectCfgDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(projectCfgDir, 0755)
	os.WriteFile(filepath.Join(projectCfgDir, "config.yaml"), []byte("targets: []\n"), 0644)

	cfg := &config.Config{Source: filepath.Join(tmp, "skills"), Targets: map[string]config.TargetConfig{}}
	s := NewProject(cfg, &config.ProjectConfig{}, projectRoot, "127.0.0.1:0", "", "")

	req := httptest.NewRequest(http.MethodGet, "/api/hooks", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Hooks []map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(resp.Hooks))
	}
	for _, item := range resp.Hooks {
		if item["command"] == "project" {
			return
		}
	}
	t.Fatal("expected project hook command in response")
}
