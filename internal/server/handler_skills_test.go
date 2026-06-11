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
	"skillshare/internal/install"
	"skillshare/internal/trash"
)

func TestHandleListSkills_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Resources []any `json:"resources"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resp.Resources))
	}
}

func TestHandleListSkills_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	addSkill(t, src, "beta")

	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Resources []map[string]any `json:"resources"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(resp.Resources))
	}
}

func TestHandleGetSkill_Found(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	res := resp["resource"].(map[string]any)
	if res["flatName"] != "my-skill" {
		t.Errorf("expected flatName 'my-skill', got %v", res["flatName"])
	}
}

func TestHandleGetSkill_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/resources/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUninstallSkill_DisabledSkill(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "disabled-skill")
	if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("disabled-skill\n"), 0644); err != nil {
		t.Fatalf("disable skill: %v", err)
	}
	s.skillsStore = install.NewMetadataStore()

	req := httptest.NewRequest(http.MethodDelete, "/api/resources/disabled-skill", nil)
	req.SetPathValue("name", "disabled-skill")
	rr := httptest.NewRecorder()
	s.handleUninstallSkill(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(src, "disabled-skill")); !os.IsNotExist(err) {
		t.Fatalf("expected disabled-skill directory to be removed from source, stat err=%v", err)
	}
}

func TestHandleGetSkillFile_PathTraversal(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// Go's HTTP mux cleans ".." from URL paths before routing, so we need
	// to bypass mux and call the handler directly with a crafted PathValue.
	// Instead, test that a valid-looking but still-traversal path is rejected.
	// The handler checks strings.Contains(fp, "..").
	req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill/files/sub%2F..%2F..%2Fetc%2Fpasswd", nil)
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

	// Seed store with entries belonging to this repo
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("team-skills/vue-best-practices", &install.MetadataEntry{Group: "team-skills", Tracked: true})
	s.skillsStore.Set("team-skills/react-patterns", &install.MetadataEntry{Group: "team-skills", Tracked: true})
	s.skillsStore.Set("unrelated-skill", &install.MetadataEntry{})

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/_team-skills", nil)
	req.SetPathValue("name", "_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Store should not contain any team-skills entries
	for _, name := range s.skillsStore.List() {
		entry := s.skillsStore.Get(name)
		if entry != nil && entry.Group == "team-skills" {
			t.Fatalf("expected team-skills entries to be pruned, but found %q", name)
		}
	}
}

func TestHandleUninstallRepo_NestedPruneDoesNotAffectSibling(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))
	addTrackedRepo(t, src, filepath.Join("dept", "_team-skills"))

	// Seed store: entries from both nested repos + an exact-group entry
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("org/_team-skills/vue", &install.MetadataEntry{Group: "org/_team-skills", Tracked: true})
	s.skillsStore.Set("dept/_team-skills/react", &install.MetadataEntry{Group: "dept/_team-skills", Tracked: true})
	s.skillsStore.Set("unrelated", &install.MetadataEntry{})

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// org/_team-skills entries should be pruned
	for _, name := range s.skillsStore.List() {
		entry := s.skillsStore.Get(name)
		if entry != nil && entry.Group == "org/_team-skills" {
			t.Fatalf("expected org/_team-skills entries to be pruned, but found %q", name)
		}
	}

	// dept/_team-skills entries must survive
	var found bool
	for _, name := range s.skillsStore.List() {
		entry := s.skillsStore.Get(name)
		if entry != nil && entry.Group == "dept/_team-skills" {
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
	s.projectCfg = &config.ProjectConfig{}
	s.cfg.Source = projectSkillsDir

	// Create tracked repo inside project skills dir
	addTrackedRepo(t, projectSkillsDir, "_team-skills")
	_ = src // global source unused in this test

	// Write a gitignore entry the way project install does: in .skillshare/.gitignore
	gitignoreDir := filepath.Join(projectRoot, ".skillshare")
	gitignorePath := filepath.Join(gitignoreDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/_team-skills/\n# END SKILLSHARE MANAGED\n"), 0644)

	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("_team-skills", &install.MetadataEntry{Tracked: true})

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

	// Store with entries using full nested path as Group (new reconcile format)
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("org/_team-skills/vue", &install.MetadataEntry{Group: "org/_team-skills", Tracked: true})
	s.skillsStore.Set("org/_team-skills/react", &install.MetadataEntry{Group: "org/_team-skills", Tracked: true})
	s.skillsStore.Set("unrelated", &install.MetadataEntry{})

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// All org/_team-skills entries should be pruned, unrelated survives
	names := s.skillsStore.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d", len(names))
	}
	if !s.skillsStore.Has("unrelated") {
		t.Fatalf("expected 'unrelated' to survive")
	}
}

func TestHandleUninstallRepo_PrunesNestedMembersByPrefix(t *testing.T) {
	s, src := newTestServer(t)
	addTrackedRepo(t, src, filepath.Join("org", "_team-skills"))

	// Store with the repo's own entry + a sub-skill using name prefix
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("org/_team-skills", &install.MetadataEntry{Tracked: true})                                      // repo entry
	s.skillsStore.Set("org/_team-skills/sub-skill", &install.MetadataEntry{Group: "org/_team-skills", Tracked: true}) // member
	s.skillsStore.Set("standalone", &install.MetadataEntry{})

	req := httptest.NewRequest(http.MethodDelete, "/api/repos/org/_team-skills", nil)
	req.SetPathValue("name", "org/_team-skills")
	rr := httptest.NewRecorder()
	s.handleUninstallRepo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	names := s.skillsStore.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d", len(names))
	}
	if !s.skillsStore.Has("standalone") {
		t.Fatalf("expected 'standalone' to survive")
	}
}
