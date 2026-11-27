package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/trash"
)

func TestHandleGetSkill_AgentKind(t *testing.T) {
	// A file rule and a folder rule both list the agent as disabled.
	for _, rule := range []string{"demo/reviewer.md\n", "demo/\n"} {
		t.Run(strings.TrimSpace(rule), func(t *testing.T) {
			s, _ := newTestServer(t)
			agentsDir := s.agentsSource()
			if err := os.MkdirAll(agentsDir, 0o755); err != nil {
				t.Fatalf("create agents dir: %v", err)
			}
			addAgent(t, agentsDir, "demo/reviewer.md")
			if err := os.WriteFile(filepath.Join(agentsDir, ".agentignore"), []byte(rule), 0o644); err != nil {
				t.Fatalf("write .agentignore: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/resources/demo__reviewer.md?kind=agent", nil)
			req.SetPathValue("name", "demo__reviewer.md")
			rr := httptest.NewRecorder()
			s.handleGetSkill(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var resp struct {
				Resource struct {
					Kind     string `json:"kind"`
					RelPath  string `json:"relPath"`
					Disabled bool   `json:"disabled"`
				} `json:"resource"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Resource.Kind != "agent" {
				t.Fatalf("expected kind=agent, got %q", resp.Resource.Kind)
			}
			if resp.Resource.RelPath != "demo/reviewer.md" {
				t.Fatalf("expected relPath demo/reviewer.md, got %q", resp.Resource.RelPath)
			}
			if !resp.Resource.Disabled {
				t.Fatal("expected agent detail to report disabled=true")
			}
		})
	}
}

func TestHandleListSkills_AgentDisabled(t *testing.T) {
	// A file rule and a folder rule both list the agent as disabled.
	for _, rule := range []string{"demo/reviewer.md\n", "demo/\n"} {
		t.Run(strings.TrimSpace(rule), func(t *testing.T) {
			s, _ := newTestServer(t)
			agentsDir := s.agentsSource()
			if err := os.MkdirAll(agentsDir, 0o755); err != nil {
				t.Fatalf("create agents dir: %v", err)
			}
			addAgent(t, agentsDir, "demo/reviewer.md")
			if err := os.WriteFile(filepath.Join(agentsDir, ".agentignore"), []byte(rule), 0o644); err != nil {
				t.Fatalf("write .agentignore: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/resources?kind=agent", nil)
			rr := httptest.NewRecorder()
			s.handleListSkills(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var resp struct {
				Resources []struct {
					FlatName string `json:"flatName"`
					Disabled bool   `json:"disabled"`
				} `json:"resources"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(resp.Resources) != 1 {
				t.Fatalf("expected 1 agent, got %d", len(resp.Resources))
			}
			if resp.Resources[0].FlatName != "demo__reviewer.md" {
				t.Fatalf("expected demo__reviewer.md, got %q", resp.Resources[0].FlatName)
			}
			if !resp.Resources[0].Disabled {
				t.Fatal("expected disabled=true in agent list response")
			}
		})
	}
}

func TestHandleToggleSkill_AgentKind(t *testing.T) {
	s, _ := newTestServer(t)
	agentsDir := s.agentsSource()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	addAgent(t, agentsDir, "demo/reviewer.md")

	disableReq := httptest.NewRequest(http.MethodPost, "/api/resources/demo__reviewer.md/disable?kind=agent", nil)
	disableReq.SetPathValue("name", "demo__reviewer.md")
	disableRR := httptest.NewRecorder()
	s.handleDisableSkill(disableRR, disableReq)

	if disableRR.Code != http.StatusOK {
		t.Fatalf("disable expected 200, got %d: %s", disableRR.Code, disableRR.Body.String())
	}

	data, err := os.ReadFile(filepath.Join(agentsDir, ".agentignore"))
	if err != nil {
		t.Fatalf("read .agentignore: %v", err)
	}
	if !strings.Contains(string(data), "demo/reviewer.md") {
		t.Fatalf("expected .agentignore to contain demo/reviewer.md, got %q", string(data))
	}

	enableReq := httptest.NewRequest(http.MethodPost, "/api/resources/demo__reviewer.md/enable?kind=agent", nil)
	enableReq.SetPathValue("name", "demo__reviewer.md")
	enableRR := httptest.NewRecorder()
	s.handleEnableSkill(enableRR, enableReq)

	if enableRR.Code != http.StatusOK {
		t.Fatalf("enable expected 200, got %d: %s", enableRR.Code, enableRR.Body.String())
	}

	data, err = os.ReadFile(filepath.Join(agentsDir, ".agentignore"))
	if err != nil {
		t.Fatalf("read .agentignore after enable: %v", err)
	}
	if strings.Contains(string(data), "demo/reviewer.md") {
		t.Fatalf("expected demo/reviewer.md to be removed from .agentignore, got %q", string(data))
	}
}

func TestHandleUninstallSkill_AgentKind(t *testing.T) {
	s, _ := newTestServer(t)
	agentsDir := s.agentsSource()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	addAgent(t, agentsDir, "demo/reviewer.md")

	store := install.LoadMetadataOrNew(agentsDir)
	store.Set("demo/reviewer", &install.MetadataEntry{
		Source: "file:///tmp/reviewer",
		Kind:   install.MetadataKindAgent,
		Subdir: "demo/reviewer.md",
	})
	if err := store.Save(agentsDir); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	s.agentsStore = store

	req := httptest.NewRequest(http.MethodDelete, "/api/resources/demo__reviewer.md?kind=agent", nil)
	req.SetPathValue("name", "demo__reviewer.md")
	rr := httptest.NewRecorder()
	s.handleUninstallSkill(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(agentsDir, "demo", "reviewer.md")); !os.IsNotExist(err) {
		t.Fatalf("expected agent file removed, stat err=%v", err)
	}
	if entry := trash.FindByName(s.agentTrashBase(), "demo/reviewer"); entry == nil {
		t.Fatal("expected agent to be moved to agent trash")
	}
	if got := s.agentsStore.GetByPath("demo/reviewer"); got != nil {
		t.Fatalf("expected agent metadata removed, got %+v", got)
	}
}

func TestUpdateSingleByKind_Agent(t *testing.T) {
	s, _ := newTestServer(t)
	agentsDir := s.agentsSource()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	addAgent(t, agentsDir, "demo/reviewer.md")

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)
	addAgent(t, repoDir, "demo/reviewer.md")
	if err := os.WriteFile(filepath.Join(repoDir, "demo", "reviewer.md"), []byte("# updated agent\n"), 0o644); err != nil {
		t.Fatalf("write repo agent: %v", err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "add agent"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	source := "file://" + filepath.ToSlash(repoDir) + "//demo/reviewer.md"
	store := install.LoadMetadataOrNew(agentsDir)
	store.Set("demo/reviewer", &install.MetadataEntry{
		Source:  source,
		Kind:    install.MetadataKindAgent,
		Subdir:  "demo/reviewer.md",
		Version: "stale-version",
	})
	if err := store.Save(agentsDir); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	s.agentsStore = store

	result := s.updateSingleByKind("demo__reviewer.md", "agent", false, true, false)
	if result.Action != "updated" {
		t.Fatalf("expected updated, got %+v", result)
	}
	if result.Kind != "agent" {
		t.Fatalf("expected kind=agent, got %+v", result)
	}

	data, err := os.ReadFile(filepath.Join(agentsDir, "demo", "reviewer.md"))
	if err != nil {
		t.Fatalf("read updated agent: %v", err)
	}
	if string(data) != "# updated agent\n" {
		t.Fatalf("expected updated agent content, got %q", string(data))
	}
}
