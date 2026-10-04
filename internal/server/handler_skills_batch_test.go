package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// addSkillWithTargets creates a skill with an existing targets field in frontmatter.
func addSkillWithTargets(t *testing.T, sourceDir, name string, targets []string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	os.MkdirAll(skillDir, 0755)
	targetYAML := ""
	if len(targets) > 0 {
		targetYAML = "\nmetadata:\n  targets:\n"
		for _, tgt := range targets {
			targetYAML += "    - " + tgt + "\n"
		}
	}
	content := "---\nname: " + name + targetYAML + "\n---\n# " + name
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
}

// addSkillNested creates a skill in a subdirectory (e.g. "folder/skill-name").
func addSkillNested(t *testing.T, sourceDir, relPath string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, filepath.FromSlash(relPath))
	os.MkdirAll(skillDir, 0755)
	name := filepath.Base(relPath)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
}

// --- Batch endpoint tests ---

func TestHandleBatchSetTargets_SetSingleTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/skill-a")
	addSkillNested(t, src, "frontend/skill-b")

	body := `{"folder":"frontend","target":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2, got %d", resp.Updated)
	}
	if resp.Skipped != 0 {
		t.Errorf("expected skipped=0, got %d", resp.Skipped)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected no errors, got %v", resp.Errors)
	}

	// Verify SKILL.md was modified for both skills
	for _, name := range []string{"frontend/skill-a", "frontend/skill-b"} {
		skillMD := filepath.Join(src, filepath.FromSlash(name), "SKILL.md")
		targets := utils.ParseFrontmatterList(skillMD, "targets")
		if len(targets) != 1 || targets[0] != "claude" {
			t.Errorf("skill %s: expected targets=[claude], got %v", name, targets)
		}
	}
}

func TestHandleBatchSetTargets_RemoveTargets(t *testing.T) {
	s, src := newTestServer(t)
	addSkillWithTargets(t, src, "skill-a", []string{"claude", "cursor"})
	addSkillWithTargets(t, src, "skill-b", []string{"claude"})

	// Set target="" to remove targets (root-level folder)
	body := `{"folder":"","target":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2, got %d", resp.Updated)
	}

	// Verify targets were removed
	for _, name := range []string{"skill-a", "skill-b"} {
		skillMD := filepath.Join(src, name, "SKILL.md")
		targets := utils.ParseFrontmatterList(skillMD, "targets")
		if len(targets) != 0 {
			t.Errorf("skill %s: expected no targets after removal, got %v", name, targets)
		}
	}
}

func TestHandleBatchSetTargets_PathTraversal(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"folder":"../../../etc","target":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBatchSetTargets_EmptyFolder_RootOnly(t *testing.T) {
	s, src := newTestServer(t)
	// Root-level skills
	addSkill(t, src, "root-skill-a")
	addSkill(t, src, "root-skill-b")
	// Nested skill — should NOT be touched
	addSkillNested(t, src, "nested-folder/deep-skill")

	body := `{"folder":"","target":"cursor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2 (root only), got %d", resp.Updated)
	}

	// Nested skill must remain untouched
	nestedMD := filepath.Join(src, "nested-folder", "deep-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(nestedMD, "targets")
	if len(targets) != 0 {
		t.Errorf("nested skill should not be modified, but got targets %v", targets)
	}
}

func TestMatchesFolder(t *testing.T) {
	tests := []struct {
		relPath string
		folder  string
		want    bool
	}{
		// Wildcard matches everything
		{"skill-a", "*", true},
		{"frontend/skill-a", "*", true},
		// Empty folder = root-level only
		{"skill-a", "", true},
		{"frontend/skill-a", "", false},
		// Exact folder match
		{"frontend/skill-a", "frontend", true},
		{"frontend/sub/skill-a", "frontend", true},
		{"backend/skill-a", "frontend", false},
		// No trailing slash confusion
		{"frontend/skill-a", "frontend/", false}, // trailing slash should not match
	}
	for _, tt := range tests {
		got := matchesFolder(tt.relPath, tt.folder)
		if got != tt.want {
			t.Errorf("matchesFolder(%q, %q) = %v, want %v", tt.relPath, tt.folder, got, tt.want)
		}
	}
}

func TestHandleBatchSetTargets_FolderTrailingSlash(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/skill-a")
	addSkillNested(t, src, "frontend/skill-b")

	// Trailing slash should still match (server cleans the input)
	body := `{"folder":"frontend/","target":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2 (trailing slash cleaned), got %d", resp.Updated)
	}
}

// --- Single skill endpoint tests ---

func TestHandleSetSkillTargets_SetTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := `{"target":"claude"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/my-skill/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "my-skill")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}

	// Verify SKILL.md was updated
	skillMD := filepath.Join(src, "my-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(skillMD, "targets")
	if len(targets) != 1 || targets[0] != "claude" {
		t.Errorf("expected targets=[claude], got %v", targets)
	}
}

func TestHandleSetSkillTargets_RemoveTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkillWithTargets(t, src, "my-skill", []string{"claude", "cursor"})

	body := `{"target":""}`
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/my-skill/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "my-skill")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify targets field was removed
	skillMD := filepath.Join(src, "my-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(skillMD, "targets")
	if len(targets) != 0 {
		t.Errorf("expected no targets after removal, got %v", targets)
	}
}

func TestHandleSetSkillTargets_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"target":"claude"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/nonexistent/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Tracked-repo skills ---

const trackedSkillRel = "_caveman/plugins/caveman/skills/cavecrew"

// addTrackedRepoSkill creates a committed git repo "_caveman" holding one skill
// whose frontmatter targets cursor.
func addTrackedRepoSkill(t *testing.T, sourceDir string) string {
	t.Helper()
	repoDir := filepath.Join(sourceDir, "_caveman")
	skillDir := filepath.Join(sourceDir, filepath.FromSlash(trackedSkillRel))
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: cavecrew\nmetadata:\n  targets:\n    - cursor\n---\n# cavecrew"), 0644)
	initGitRepo(t, repoDir)
	return repoDir
}

// effectiveTargets returns the targets discovery reports for relPath.
func effectiveTargets(t *testing.T, sourceDir, relPath string) []string {
	t.Helper()
	skills, err := ssync.DiscoverSourceSkillsAllWithOptions(sourceDir, ssync.DiscoveryOptions{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	for _, d := range skills {
		if d.RelPath == relPath {
			return d.Targets
		}
	}
	t.Fatalf("skill %s not discovered", relPath)
	return nil
}

func setSkillTarget(t *testing.T, s *Server, name, target string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/"+name+"/targets", bytes.NewBufferString(`{"target":"`+target+`"}`))
	req.SetPathValue("name", name)
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetSkillTargets_TrackedRepoSkill_OverridesWithoutDirtyingRepo(t *testing.T) {
	s, src := newTestServer(t)
	repoDir := addTrackedRepoSkill(t, src)

	setSkillTarget(t, s, "cavecrew", "claude")

	if got := effectiveTargets(t, src, trackedSkillRel); len(got) != 1 || got[0] != "claude" {
		t.Errorf("expected effective targets [claude], got %v", got)
	}
	if out := runGit(t, repoDir, "status", "--porcelain"); len(out) != 0 {
		t.Errorf("expected clean tracked repo, got:\n%s", out)
	}
}

func TestHandleSetSkillTargets_TrackedRepoSkill_EmptyTargetMeansAll(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepoSkill(t, src)

	setSkillTarget(t, s, "cavecrew", "")

	// Must not fall back to the repo's frontmatter [cursor].
	if got := effectiveTargets(t, src, trackedSkillRel); got != nil {
		t.Errorf("expected all targets (nil), got %v", got)
	}
}

func TestHandleBatchSetTargets_TrackedRepoSkills(t *testing.T) {
	s, src := newTestServer(t)
	repoDir := addTrackedRepoSkill(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(`{"folder":"_caveman","target":"claude"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 1 || resp.Skipped != 0 {
		t.Errorf("expected updated=1 skipped=0, got updated=%d skipped=%d", resp.Updated, resp.Skipped)
	}
	if got := effectiveTargets(t, src, trackedSkillRel); len(got) != 1 || got[0] != "claude" {
		t.Errorf("expected effective targets [claude], got %v", got)
	}
	if out := runGit(t, repoDir, "status", "--porcelain"); len(out) != 0 {
		t.Errorf("expected clean tracked repo, got:\n%s", out)
	}
}

func TestHandleUninstallRepo_RemovesTargetOverrides(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepoSkill(t, src)
	setSkillTarget(t, s, "cavecrew", "claude")
	s.skillsStore.SetTargetOverride("_other/skill", []string{"cursor"})

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/_caveman", nil)
	req.SetPathValue("name", "_caveman")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	overrides := install.LoadTargetOverrides(src)
	if _, ok := overrides[trackedSkillRel]; ok {
		t.Errorf("expected override for %s to be removed, got %v", trackedSkillRel, overrides)
	}
	if _, ok := overrides["_other/skill"]; !ok {
		t.Errorf("expected override of another repo to survive, got %v", overrides)
	}
}

func TestHandleBatchUninstall_RemovesTargetOverrides(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepoSkill(t, src)
	setSkillTarget(t, s, "cavecrew", "claude")

	b, _ := json.Marshal(batchUninstallRequest{Names: []string{"_caveman"}, Force: true})
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	s.handleBatchUninstall(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if overrides := install.LoadTargetOverrides(src); len(overrides) != 0 {
		t.Errorf("expected no target overrides after uninstall, got %v", overrides)
	}
}

func TestHandleBatchSetTargets_SaveFailureReported(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepoSkill(t, src)
	// A directory at the metadata path makes the atomic rename in Save fail.
	if err := os.MkdirAll(filepath.Join(src, install.MetadataFileName), 0755); err != nil {
		t.Fatalf("block metadata file: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/targets", bytes.NewBufferString(`{"folder":"_caveman","target":"claude"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}
