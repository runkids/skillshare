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

func TestHandleListRules_Empty(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Rules []any `json:"rules"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(resp.Rules))
	}
}

func TestHandleListRules_UsesProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	projectRoot := filepath.Join(tmp, "project")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	homeRuleDir := filepath.Join(homeDir, ".codex")
	os.MkdirAll(homeRuleDir, 0755)
	os.WriteFile(filepath.Join(homeRuleDir, "AGENTS.md"), []byte("home rule"), 0644)

	projectRuleDir := filepath.Join(projectRoot, ".codex")
	os.MkdirAll(projectRuleDir, 0755)
	os.WriteFile(filepath.Join(projectRuleDir, "AGENTS.md"), []byte("project rule"), 0644)

	projectCfgDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(projectCfgDir, 0755)
	os.WriteFile(filepath.Join(projectCfgDir, "config.yaml"), []byte("targets: []\n"), 0644)

	cfg := &config.Config{Source: filepath.Join(tmp, "skills"), Targets: map[string]config.TargetConfig{}}
	s := NewProject(cfg, &config.ProjectConfig{}, projectRoot, "127.0.0.1:0", "", "")

	req := httptest.NewRequest(http.MethodGet, "/api/rules", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(resp.Rules))
	}
	for _, item := range resp.Rules {
		if item["content"] == "project rule" {
			stats, ok := item["stats"].(map[string]any)
			if !ok {
				t.Fatalf("expected stats object on project rule, got %T", item["stats"])
			}
			if int(stats["wordCount"].(float64)) != 2 {
				t.Fatalf("stats.wordCount = %v, want 2", stats["wordCount"])
			}
			if int(stats["lineCount"].(float64)) != 1 {
				t.Fatalf("stats.lineCount = %v, want 1", stats["lineCount"])
			}
			if int(stats["tokenCount"].(float64)) <= 0 {
				t.Fatalf("stats.tokenCount = %v, want > 0", stats["tokenCount"])
			}
			return
		}
	}
	t.Fatal("expected project rule content in response")
}
