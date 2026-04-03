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
	"skillshare/internal/trash"
)

func TestHandleListSkills_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Skills []any `json:"skills"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(resp.Skills))
	}
}

func TestHandleListSkills_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	addSkill(t, src, "beta")

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(resp.Skills))
	}
}

func TestHandleGetSkill_Found(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/skills/my-skill", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	skill := resp["skill"].(map[string]any)
	if skill["flatName"] != "my-skill" {
		t.Errorf("expected flatName 'my-skill', got %v", skill["flatName"])
	}
}

func TestHandleGetSkill_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skills/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetSkillFile_PathTraversal(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// Go's HTTP mux cleans ".." from URL paths before routing, so we need
	// to bypass mux and call the handler directly with a crafted PathValue.
	// Instead, test that a valid-looking but still-traversal path is rejected.
	// The handler checks strings.Contains(fp, "..").
	req := httptest.NewRequest(http.MethodGet, "/api/skills/my-skill/files/sub%2F..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	// The mux will decode %2F as / and clean the path, which may result in
	// 404 or the handler never seeing "..". Either non-200 is acceptable.
	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for path traversal attempt")
	}
}

func TestHandleUninstallRepo_NestedRepoPath(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(src, "org", "_team-skills")); !os.IsNotExist(err) {
		t.Fatalf("expected nested tracked repo to be removed from source, stat err=%v", err)
	}

	entries, err := filepath.Glob(filepath.Join(trash.TrashDir(), "org", "_team-skills_*"))
	if err != nil {
		t.Fatalf("failed to inspect trash: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected nested tracked repo to be moved to trash, got %d matches", len(entries))
	}

	// Verify List() can find nested trash entries
	items := trash.List(trash.TrashDir())
	if len(items) != 1 {
		t.Fatalf("expected 1 trash item from List, got %d", len(items))
	}
	if items[0].Name != "org/_team-skills" {
		t.Fatalf("expected Name 'org/_team-skills', got %q", items[0].Name)
	}
}

func TestHandleUninstallRepo_AmbiguousBasenameRequiresFullPath(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))
	addTrackedRepo(t, src, filepath.Join("dept", "_team-skills"))

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/team-skills", nil)
	req.SetPathValue("name", "team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "multiple tracked repositories match") {
		t.Fatalf("expected ambiguous repo error, got %s", rr.Body.String())
	}
}

func TestHandleUninstallRepo_RejectsPathTraversal(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/../../evil", nil)
	req.SetPathValue("name", "../evil")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid or missing tracked repository name") {
		t.Fatalf("expected invalid name error, got %s", rr.Body.String())
	}
}

func TestHandleUninstallRepo_PrunesRegistry(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, "_team-skills")
	addSkill(t, src, "unrelated-skill") // must exist on disk to survive reconcile

	// Seed registry with entries belonging to this repo
	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "vue-best-practices", Group: "team-skills", Tracked: true},
			{Name: "react-patterns", Group: "team-skills", Tracked: true},
			{Name: "unrelated-skill", Group: ""},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/_team-skills", nil)
	req.SetPathValue("name", "_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Registry should not contain any team-skills entries
	for _, entry := range s.registry.Skills {
		if entry.Group == "team-skills" {
			t.Fatalf("expected team-skills entries to be pruned, but found %q", entry.Name)
		}
	}
}

func TestHandleUninstallRepo_NestedPruneDoesNotAffectSibling(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))
	addTrackedRepo(t, src, filepath.Join("dept", "_team-skills"))

	// Seed registry: entries from both nested repos + an exact-group entry
	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "vue", Group: "org/_team-skills", Tracked: true},
			{Name: "react", Group: "dept/_team-skills", Tracked: true},
			{Name: "unrelated", Group: ""},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// org/_team-skills entries should be pruned
	for _, entry := range s.registry.Skills {
		if entry.Group == "org/_team-skills" {
			t.Fatalf("expected org/_team-skills entries to be pruned, but found %q", entry.Name)
		}
	}

	// dept/_team-skills entries must survive
	var found bool
	for _, entry := range s.registry.Skills {
		if entry.Group == "dept/_team-skills" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected dept/_team-skills entries to survive, but they were pruned")
	}
}

func TestHandleUninstallRepo_ProjectMode_GitignorePath(t *testing.T) {
	s, src := newTestServer(t)

	// Simulate project mode: set projectRoot to a temp dir containing .skillshare/
	projectRoot := t.TempDir()
	projectSkillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	os.MkdirAll(projectSkillsDir, 0755)
	s.projectRoot = projectRoot
	s.cfg.Source = projectSkillsDir

	// Create tracked repo inside project skills dir
	addTrackedRepo(t, projectSkillsDir, "_team-skills")
	_ = src // global source unused in this test

	// Write a gitignore entry the way project install does: in .skillshare/.gitignore
	gitignoreDir := filepath.Join(projectRoot, ".skillshare")
	gitignorePath := filepath.Join(gitignoreDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/_team-skills/\n# END SKILLSHARE MANAGED\n"), 0644)

	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "_team-skills", Tracked: true},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/_team-skills", nil)
	req.SetPathValue("name", "_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify gitignore entry was removed from .skillshare/.gitignore
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read gitignore: %v", err)
	}
	if strings.Contains(string(data), "skills/_team-skills/") {
		t.Fatal("expected skills/_team-skills/ to be removed from .skillshare/.gitignore, but it still exists")
	}
}

// --- resolveTrackedRepo tests ---

func TestResolveTrackedRepo_AutoPrefixUnderscore(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, "_team-skills")

	name, path, err := s.resolveTrackedRepo("team-skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "_team-skills" {
		t.Errorf("expected name '_team-skills', got %q", name)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestResolveTrackedRepo_NestedAutoPrefix(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	name, path, err := s.resolveTrackedRepo("org/team-skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != filepath.Join("org", "_team-skills") {
		t.Errorf("expected 'org/_team-skills', got %q", name)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestResolveTrackedRepo_AlreadyPrefixed(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, "_team-skills")

	name, path, err := s.resolveTrackedRepo("_team-skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "_team-skills" {
		t.Errorf("expected '_team-skills', got %q", name)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestResolveTrackedRepo_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	name, path, err := s.resolveTrackedRepo("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error for not-found: %v", err)
	}
	if name != "" || path != "" {
		t.Errorf("expected empty results for not-found, got name=%q path=%q", name, path)
	}
}

func TestResolveTrackedRepo_BasenameFallback(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	// Search by basename only — should find via fallback
	name, path, err := s.resolveTrackedRepo("team-skills")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != filepath.Join("org", "_team-skills") {
		t.Errorf("expected 'org/_team-skills', got %q", name)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

// --- Additional registry prune tests ---

func TestHandleUninstallRepo_PrunesNestedFullPathGroup(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	// Registry with entries using full nested path as Group (new reconcile format)
	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "vue", Group: "org/_team-skills", Tracked: true},
			{Name: "react", Group: "org/_team-skills", Tracked: true},
			{Name: "unrelated", Group: ""},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// All org/_team-skills entries should be pruned, unrelated survives
	if len(s.registry.Skills) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d", len(s.registry.Skills))
	}
	if s.registry.Skills[0].Name != "unrelated" {
		t.Fatalf("expected 'unrelated' to survive, got %q", s.registry.Skills[0].Name)
	}
}

func TestHandleUninstallRepo_PrunesNestedMembersByPrefix(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	// Registry with the repo's own entry + a sub-skill using FullName prefix
	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "org/_team-skills", Group: "", Tracked: true},          // repo entry
			{Name: "sub-skill", Group: "org/_team-skills", Tracked: true}, // member
			{Name: "standalone", Group: ""},
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if len(s.registry.Skills) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d", len(s.registry.Skills))
	}
	if s.registry.Skills[0].Name != "standalone" {
		t.Fatalf("expected 'standalone' to survive, got %q", s.registry.Skills[0].Name)
	}
}
