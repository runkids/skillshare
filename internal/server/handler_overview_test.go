package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func TestDiffNamingWarningIsNotPendingWork(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "invalid")
	if err := os.WriteFile(filepath.Join(src, "invalid", "SKILL.md"), []byte("---\nname: wrong-name\n---\n# Instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := ssync.DiscoverSourceSkills(src)
	if err != nil {
		t.Fatal(err)
	}
	diff := s.computeTargetDiff("claude", config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: t.TempDir(), TargetNaming: "standard"}}, skills, "merge", src, nil)
	if len(diff.Warnings) == 0 || diff.NamingSkippedCount != 1 || len(diff.Items) != 0 {
		t.Fatalf("validation warning becomes a sync operation: %+v", diff)
	}
}

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

func TestHandleOverviewAndTargets_LogicalVariants(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{"claude": filepath.Join(t.TempDir(), "claude"), "universal": filepath.Join(t.TempDir(), "universal")})
	for _, agent := range []string{"claude", "codex"} {
		for _, name := range []string{"fix", "review"} {
			addSkill(t, src, "_repo/skills/"+agent+"/"+name)
		}
	}
	initServerGitRepo(t, filepath.Join(src, "_repo"))
	for _, endpoint := range []string{"/api/overview", "/api/targets"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d: %s", endpoint, rr.Code, rr.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if endpoint == "/api/overview" {
			if resp["skillCount"] != float64(2) || resp["variantCount"] != float64(4) {
				t.Fatalf("overview counts: %+v", resp)
			}
			repo := resp["trackedRepos"].([]any)[0].(map[string]any)
			if repo["skillCount"] != float64(2) || repo["variantCount"] != float64(4) {
				t.Fatalf("repo counts: %+v", repo)
			}
		} else {
			if resp["sourceSkillCount"] != float64(2) || resp["sourceVariantCount"] != float64(4) {
				t.Fatalf("source counts: %+v", resp)
			}
			for _, value := range resp["targets"].([]any) {
				target := value.(map[string]any)
				want := float64(0)
				if target["name"] == "claude" {
					want = 2
				}
				if target["expectedSkillCount"] != want {
					t.Fatalf("target counts: %+v", target)
				}
			}
		}
	}
}

func TestTrackedVariantAmbiguityReachesPreviews(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{"claude": t.TempDir()})
	for _, agent := range []string{"claude", "claude-code", "codex"} {
		addSkill(t, src, "_repo/skills/"+agent+"/fix")
	}
	for _, endpoint := range []string{"/api/diff", "/api/targets"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %s", endpoint, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "ambiguous") || !strings.Contains(rr.Body.String(), "target override") {
			t.Errorf("%s lost selection reason: %s", endpoint, rr.Body.String())
		}
		var response struct {
			Diffs []struct {
				Warnings []string   `json:"warnings"`
				Skipped  int        `json:"skippedCount"`
				Naming   int        `json:"namingSkippedCount"`
				Items    []diffItem `json:"items"`
			} `json:"diffs"`
			Targets []struct {
				Expected int `json:"expectedSkillCount"`
				Skipped  int `json:"skippedSkillCount"`
			}
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if endpoint == "/api/targets" {
			if response.Targets[0].Expected != 0 || response.Targets[0].Skipped != 2 {
				t.Errorf("preview counts = %+v", response.Targets)
			}
		} else {
			preview := response.Diffs[0]
			if preview.Skipped != 2 || preview.Naming != 0 || len(preview.Items) != 2 || len(preview.Warnings) != 0 {
				t.Fatalf("ambiguous preview counts: %+v", preview)
			}
			for _, item := range preview.Items {
				if item.Action != "skip" {
					t.Fatalf("ambiguous preview proposes an operation: %+v", item)
				}
			}
		}
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
