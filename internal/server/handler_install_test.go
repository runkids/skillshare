package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestHandleInstallBatch_AgentInstallWritesMetadataToAgentsSource(t *testing.T) {
	s, skillsDir := newTestServer(t)

	agentsDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	s.cfg.AgentsSource = agentsDir
	s.agentsStore = install.NewMetadataStore()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	agentPath := filepath.Join(repoDir, "reviewer.md")
	if err := os.WriteFile(agentPath, []byte("# Reviewer agent"), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
	for _, args := range [][]string{
		{"add", "reviewer.md"},
		{"commit", "-m", "add reviewer agent"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	payload, err := json.Marshal(map[string]any{
		"source": "file://" + repoDir,
		"skills": []map[string]string{
			{"name": "reviewer", "path": "reviewer.md"},
		},
		"kind": "agent",
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/install/batch", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(agentsDir, "reviewer.md")); err != nil {
		t.Fatalf("expected installed agent in agents source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsDir, install.MetadataFileName)); err != nil {
		t.Fatalf("expected metadata written to agents source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, install.MetadataFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no agent metadata written to skills source, got err=%v", err)
	}
}

func TestHandleDiscover_LocalSourceReturnsAgents(t *testing.T) {
	s, _ := newTestServer(t)

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "reviewer.md"), []byte("# Reviewer agent"), 0o644); err != nil {
		t.Fatalf("failed to write local agent: %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"source": sourceDir,
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/discover", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		NeedsSelection bool `json:"needsSelection"`
		Skills         []struct {
			Name string `json:"name"`
		} `json:"skills"`
		Agents []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.NeedsSelection {
		t.Fatal("expected local agent discovery to require selection UI")
	}
	if len(resp.Skills) != 0 {
		t.Fatalf("expected no skills, got %d", len(resp.Skills))
	}
	if len(resp.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "reviewer" || resp.Agents[0].Path != "reviewer.md" || resp.Agents[0].Kind != "agent" {
		t.Fatalf("unexpected agent payload: %+v", resp.Agents[0])
	}
}

func TestHandleInstallBatch_LocalAgentInstallPreservesNestedPath(t *testing.T) {
	s, _ := newTestServer(t)

	agentsDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	s.cfg.AgentsSource = agentsDir
	s.agentsStore = install.NewMetadataStore()

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "demo"), 0o755); err != nil {
		t.Fatalf("failed to create nested agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "demo", "reviewer.md"), []byte("# Reviewer agent"), 0o644); err != nil {
		t.Fatalf("failed to write nested agent: %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"source": sourceDir,
		"skills": []map[string]string{
			{"name": "reviewer", "path": "demo/reviewer.md"},
		},
		"kind": "agent",
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/install/batch", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(agentsDir, "demo", "reviewer.md")); err != nil {
		t.Fatalf("expected installed nested agent in agents source: %v", err)
	}
	if got := s.agentsStore.GetByPath("demo/reviewer"); got == nil {
		t.Fatal("expected nested agent metadata loaded into agentsStore")
	}
}

func TestHandleInstall_ProjectRootRejected(t *testing.T) {
	s, projectRoot := newProjectTargetServer(t, nil)

	for _, tc := range []struct {
		name     string
		endpoint string
		payload  map[string]any
	}{
		{
			name:     "single",
			endpoint: "/api/install",
			payload:  map[string]any{"source": projectRoot, "name": "project-root"},
		},
		{
			name:     "batch",
			endpoint: "/api/install/batch",
			payload: map[string]any{
				"source": projectRoot,
				"skills": []map[string]string{{"name": "project-root", "path": "."}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("failed to marshal payload: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, tc.endpoint, bytes.NewReader(payload))
			rr := httptest.NewRecorder()
			s.mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "cannot install the project root into itself") {
				t.Fatalf("expected rejection message, got %s", rr.Body.String())
			}
			if _, err := os.Stat(filepath.Join(projectRoot, ".skillshare", "skills", "project-root")); !os.IsNotExist(err) {
				t.Fatalf("project root install should not create a skill, got err=%v", err)
			}
		})
	}
}

func TestHandleInstall_TrackPureAgentRepoInstallsIntoAgentsSource(t *testing.T) {
	s, skillsDir := newTestServer(t)

	agentsDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	s.cfg.AgentsSource = agentsDir
	s.agentsStore = install.NewMetadataStore()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	agentPath := filepath.Join(repoDir, "reviewer.md")
	if err := os.WriteFile(agentPath, []byte("# Reviewer v1"), 0o644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
	for _, args := range [][]string{
		{"add", "reviewer.md"},
		{"commit", "-m", "add reviewer agent"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	payload, err := json.Marshal(map[string]any{
		"source": "file://" + repoDir,
		"track":  true,
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/install", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		RepoName string `json:"repoName"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	trackedRepoName := resp.RepoName
	if _, err := os.Stat(filepath.Join(agentsDir, trackedRepoName, ".git")); err != nil {
		agentEntries, _ := os.ReadDir(agentsDir)
		skillEntries, _ := os.ReadDir(skillsDir)
		t.Fatalf("expected tracked agent repo in agents source: %v (agents=%v skills=%v body=%s)", err, entryNames(agentEntries), entryNames(skillEntries), rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(agentsDir, trackedRepoName, "reviewer.md")); err != nil {
		t.Fatalf("expected tracked agent file in agents source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, trackedRepoName)); !os.IsNotExist(err) {
		t.Fatalf("expected no tracked agent repo in skills source, got err=%v", err)
	}
}
