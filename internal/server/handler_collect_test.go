package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
)

func TestHandleCollectScan_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets    []any `json:"targets"`
		TotalCount int   `json:"totalCount"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.TotalCount != 0 {
		t.Errorf("expected 0 total, got %d", resp.TotalCount)
	}
}

func TestHandleCollectScan_WithLocalSkills(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Create a local skill in target
	localSkill := filepath.Join(tgtPath, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TotalCount int `json:"totalCount"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.TotalCount != 1 {
		t.Errorf("expected 1 local skill, got %d", resp.TotalCount)
	}
}

func TestHandleCollect_NoSkills(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty skills, got %d", rr.Code)
	}
}

func TestHandleCollectScan_GlobalCopyModeInheritedTarget_SkipsManaged(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nmode: copy\ntargets:\n  claude:\n    path: " + tgtPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	managedSkill := filepath.Join(tgtPath, "managed-skill")
	os.MkdirAll(managedSkill, 0755)
	os.WriteFile(filepath.Join(managedSkill, "SKILL.md"), []byte("managed"), 0644)

	localSkill := filepath.Join(tgtPath, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local"), 0644)

	if err := ssync.WriteManifest(tgtPath, &ssync.Manifest{
		Managed: map[string]string{"managed-skill": "abc123"},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []struct {
			TargetName string `json:"targetName"`
			Skills     []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"targets"`
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Fatalf("expected 1 local skill, got %d", resp.TotalCount)
	}
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if len(resp.Targets[0].Skills) != 1 {
		t.Fatalf("expected 1 skill for target, got %d", len(resp.Targets[0].Skills))
	}
	if resp.Targets[0].Skills[0].Name != "local-skill" {
		t.Fatalf("expected only local-skill, got %q", resp.Targets[0].Skills[0].Name)
	}
}

func TestHandleCollectScan_AgentKind(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	agentPath := filepath.Join(t.TempDir(), "claude-agents")
	agentsSource := filepath.Join(t.TempDir(), "agents-source")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Write config YAML with agents_source and agent target path.
	// The auto-reload middleware re-reads from disk on every API request.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nagents_source: " + agentsSource +
		"\nmode: merge\ntargets:\n  claude:\n    skills:\n      path: " + tgtPath +
		"\n    agents:\n      path: " + agentPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}
	os.MkdirAll(agentsSource, 0755)

	// Create a local .md agent file in the agent target directory.
	os.MkdirAll(agentPath, 0755)
	os.WriteFile(filepath.Join(agentPath, "helper.md"), []byte("# helper agent"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan?kind=agent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []struct {
			TargetName string `json:"targetName"`
			Skills     []struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"skills"`
		} `json:"targets"`
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Fatalf("expected totalCount=1, got %d", resp.TotalCount)
	}
	if len(resp.Targets) == 0 {
		t.Fatal("expected at least 1 target in response")
	}
	found := false
	for _, tgt := range resp.Targets {
		for _, sk := range tgt.Skills {
			if sk.Name == "helper.md" && sk.Kind == "agent" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected agent helper.md with kind=agent in response, got %+v", resp.Targets)
	}
}

func TestHandleCollectScan_BothKinds(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	agentPath := filepath.Join(t.TempDir(), "claude-agents")
	agentsSource := filepath.Join(t.TempDir(), "agents-source")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Write config YAML with both skills and agents paths.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nagents_source: " + agentsSource +
		"\nmode: merge\ntargets:\n  claude:\n    skills:\n      path: " + tgtPath +
		"\n    agents:\n      path: " + agentPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}
	os.MkdirAll(agentsSource, 0755)

	// Create a local skill in skill target.
	localSkill := filepath.Join(tgtPath, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local"), 0644)

	// Create a local agent in agent target.
	os.MkdirAll(agentPath, 0755)
	os.WriteFile(filepath.Join(agentPath, "reviewer.md"), []byte("# reviewer agent"), 0644)

	// No kind parameter — should return both.
	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.TotalCount != 2 {
		t.Fatalf("expected totalCount=2 (1 skill + 1 agent), got %d", resp.TotalCount)
	}
}

func TestHandleCollect_AgentItems(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	agentPath := filepath.Join(t.TempDir(), "claude-agents")
	agentsSource := filepath.Join(t.TempDir(), "agents-source")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Write config YAML with agents_source and agent target path.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nagents_source: " + agentsSource +
		"\nmode: merge\ntargets:\n  claude:\n    skills:\n      path: " + tgtPath +
		"\n    agents:\n      path: " + agentPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}
	os.MkdirAll(agentsSource, 0755)

	// Create a local .md agent file in the agent target directory.
	os.MkdirAll(agentPath, 0755)
	agentContent := "# helper agent\nThis is a test agent."
	os.WriteFile(filepath.Join(agentPath, "helper.md"), []byte(agentContent), 0644)

	body := `{"skills":[{"name":"helper.md","targetName":"claude","kind":"agent"}],"force":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Pulled  []string          `json:"pulled"`
		Skipped []string          `json:"skipped"`
		Failed  map[string]string `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Pulled) != 1 || resp.Pulled[0] != "helper.md" {
		t.Fatalf("expected pulled=[helper.md], got %v", resp.Pulled)
	}
	if len(resp.Failed) != 0 {
		t.Fatalf("expected no failures, got %v", resp.Failed)
	}

	// Verify the agent file was copied to agents source.
	destPath := filepath.Join(agentsSource, "helper.md")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("agent not found in source dir: %v", err)
	}
	if string(data) != agentContent {
		t.Fatalf("agent content mismatch: got %q, want %q", string(data), agentContent)
	}
}

func TestHandleCollect_MixedSkillsAndAgents(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	agentPath := filepath.Join(t.TempDir(), "claude-agents")
	agentsSource := filepath.Join(t.TempDir(), "agents-source")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Write config YAML with both skills and agents paths.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nagents_source: " + agentsSource +
		"\nmode: merge\ntargets:\n  claude:\n    skills:\n      path: " + tgtPath +
		"\n    agents:\n      path: " + agentPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}
	os.MkdirAll(agentsSource, 0755)

	// Create a local skill directory in skill target.
	localSkill := filepath.Join(tgtPath, "my-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local skill"), 0644)

	// Create a local agent file in agent target.
	os.MkdirAll(agentPath, 0755)
	os.WriteFile(filepath.Join(agentPath, "reviewer.md"), []byte("# reviewer"), 0644)

	body := `{"skills":[` +
		`{"name":"my-skill","targetName":"claude"},` +
		`{"name":"reviewer.md","targetName":"claude","kind":"agent"}` +
		`],"force":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Pulled  []string          `json:"pulled"`
		Skipped []string          `json:"skipped"`
		Failed  map[string]string `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Pulled) != 2 {
		t.Fatalf("expected 2 pulled items, got %v", resp.Pulled)
	}
	if len(resp.Failed) != 0 {
		t.Fatalf("expected no failures, got %v", resp.Failed)
	}

	// Verify both exist in their respective source dirs.
	if _, err := os.Stat(filepath.Join(sourceDir, "my-skill", "SKILL.md")); err != nil {
		t.Fatalf("skill not found in source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsSource, "reviewer.md")); err != nil {
		t.Fatalf("agent not found in agents source: %v", err)
	}
}

func TestHandleCollectScan_AgentKind_NoSource(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Write config without agents_source — should return 0 agents, no error.
	cfgPath := os.Getenv("SKILLSHARE_CONFIG")
	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n  claude:\n    path: " + tgtPath + "\n"
	if err := os.WriteFile(cfgPath, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan?kind=agent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.TotalCount != 0 {
		t.Fatalf("expected totalCount=0 when no agents source, got %d", resp.TotalCount)
	}
}
