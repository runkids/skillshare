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

func TestHandleOverview_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["skillCount"].(float64) != 0 {
		t.Errorf("expected 0 skills, got %v", resp["skillCount"])
	}
	if resp["targetCount"].(float64) != 0 {
		t.Errorf("expected 0 targets, got %v", resp["targetCount"])
	}
	if resp["isProjectMode"].(bool) {
		t.Error("expected isProjectMode false")
	}
}

func TestHandleOverview_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	addSkill(t, src, "beta")

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["skillCount"].(float64) != 2 {
		t.Errorf("expected 2 skills, got %v", resp["skillCount"])
	}
}

func TestHandleOverview_AgentCountIncludesNestedAgents(t *testing.T) {
	s, _ := newTestServer(t)
	agentsDir := s.agentsSource()
	if err := os.MkdirAll(filepath.Join(agentsDir, "demo"), 0755); err != nil {
		t.Fatalf("mkdir agents dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "top-level.md"), []byte("# Top"), 0644); err != nil {
		t.Fatalf("write top-level agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "demo", "nested-agent.md"), []byte("# Nested"), 0644); err != nil {
		t.Fatalf("write nested agent: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["agentCount"].(float64) != 2 {
		t.Errorf("expected 2 agents including nested entries, got %v", resp["agentCount"])
	}
}

func TestHandleOverview_ProjectMode(t *testing.T) {
	tmp := t.TempDir()
	s, _ := newTestServer(t)
	s.projectRoot = tmp                    // simulate project mode
	s.projectCfg = &config.ProjectConfig{} // provide empty project config

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	// Use mux directly to bypass config auto-reload middleware
	// (project config file doesn't exist in test)
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp["isProjectMode"].(bool) {
		t.Error("expected isProjectMode true")
	}
	if resp["projectRoot"] != tmp {
		t.Errorf("expected projectRoot %q, got %v", tmp, resp["projectRoot"])
	}
}
