package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nestedRepoSkillRel = "group/sub/_repo/nested"

// addNestedRepoSkill creates a committed git repo at group/sub/_repo, where
// install --track --into group/sub puts it, holding one skill whose
// frontmatter targets cursor.
func addNestedRepoSkill(t *testing.T, sourceDir string) string {
	t.Helper()
	repoDir := filepath.Join(sourceDir, "group", "sub", "_repo")
	skillDir := filepath.Join(sourceDir, filepath.FromSlash(nestedRepoSkillRel))
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: nested\nmetadata:\n  targets:\n    - cursor\n---\n# nested"), 0644)
	initGitRepo(t, repoDir)
	return repoDir
}

func TestHandleBatchSetTargets_NestedRepoSkillOverrides(t *testing.T) {
	s, src := newTestServer(t)
	repoDir := addNestedRepoSkill(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(`{"folder":"group/sub","target":"claude"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if got := effectiveTargets(t, src, nestedRepoSkillRel); len(got) != 1 || got[0] != "claude" {
		t.Errorf("expected effective targets [claude], got %v", got)
	}
	if out := runGit(t, repoDir, "status", "--porcelain"); len(out) != 0 {
		t.Errorf("expected clean tracked repo, got:\n%s", out)
	}
}

func TestHandleBatchUninstall_NestedRepoSkillRefused(t *testing.T) {
	s, src := newTestServer(t)
	addNestedRepoSkill(t, src)

	b, _ := json.Marshal(batchUninstallRequest{Names: []string{"nested"}, Force: true})
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	s.handleBatchUninstall(rr, req)

	if !strings.Contains(rr.Body.String(), "skill is inside a tracked repo") {
		t.Fatalf("expected tracked-repo refusal, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(src, filepath.FromSlash(nestedRepoSkillRel))); err != nil {
		t.Fatalf("expected nested repo skill kept: %v", err)
	}
}

func TestHandleOverview_NestedRepoSkillCount(t *testing.T) {
	s, src := newTestServer(t)
	addNestedRepoSkill(t, src)

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TrackedRepos []trackedRepoItem `json:"trackedRepos"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.TrackedRepos) != 1 || resp.TrackedRepos[0].Name != "group/sub/_repo" || resp.TrackedRepos[0].SkillCount != 1 {
		t.Errorf("expected group/sub/_repo with 1 skill, got %+v", resp.TrackedRepos)
	}
}
