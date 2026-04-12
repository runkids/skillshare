package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

func TestHandleGetSkillFile_PathTraversal(t *testing.T) {
	t.Run("rejects traversal segment", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill/files/../../etc/passwd", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "../../etc/passwd")
		rr := httptest.NewRecorder()
		s.handleGetSkillFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects symlinked parent directory escape", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")
		skillDir := filepath.Join(src, "my-skill")

		outsideDir := t.TempDir()
		targetPath := filepath.Join(outsideDir, "out.md")
		if err := os.WriteFile(targetPath, []byte("outside"), 0644); err != nil {
			t.Fatalf("failed to seed outside markdown file: %v", err)
		}

		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill/files/linkdir/out.md", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "linkdir/out.md")
		rr := httptest.NewRecorder()
		s.handleGetSkillFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects symlinked parent directory escape when target missing", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")
		skillDir := filepath.Join(src, "my-skill")

		outsideDir := t.TempDir()
		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill/files/linkdir/missing.md", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "linkdir/missing.md")
		rr := httptest.NewRecorder()
		s.handleGetSkillFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects non-regular markdown path", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("unix socket test is not supported on windows")
		}

		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		socketPath := filepath.Join(src, "my-skill", "socket.md")
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Skipf("unix socket not supported in this environment: %v", err)
		}
		defer listener.Close()
		defer os.Remove(socketPath)

		req := httptest.NewRequest(http.MethodGet, "/api/resources/my-skill/files/socket.md", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "socket.md")
		rr := httptest.NewRecorder()
		s.handleGetSkillFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})
}

func TestHandleSaveSkillFile(t *testing.T) {
	t.Run("saves SKILL.md", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/SKILL.md",
			strings.NewReader(`{"content":"# Updated\n\nBody"}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Filename != "SKILL.md" {
			t.Fatalf("filename = %q, want %q", resp.Filename, "SKILL.md")
		}
		if resp.Content != "# Updated\n\nBody" {
			t.Fatalf("content = %q, want %q", resp.Content, "# Updated\n\nBody")
		}

		savedBytes, err := os.ReadFile(filepath.Join(src, "my-skill", "SKILL.md"))
		if err != nil {
			t.Fatalf("failed to read saved file: %v", err)
		}
		if string(savedBytes) != "# Updated\n\nBody" {
			t.Fatalf("saved file content = %q, want %q", string(savedBytes), "# Updated\n\nBody")
		}
	})

	t.Run("allows dotted markdown filename on save and read", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		notesDir := filepath.Join(src, "my-skill", "notes")
		if err := os.MkdirAll(notesDir, 0755); err != nil {
			t.Fatalf("failed to create notes directory: %v", err)
		}
		targetPath := filepath.Join(notesDir, "v1..draft.md")
		if err := os.WriteFile(targetPath, []byte("seed"), 0644); err != nil {
			t.Fatalf("failed to seed dotted markdown file: %v", err)
		}

		putReq := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/notes/v1..draft.md",
			strings.NewReader(`{"content":"# Dotted\n\nUpdated"}`),
		)
		putRR := httptest.NewRecorder()
		s.handler.ServeHTTP(putRR, putReq)

		if putRR.Code != http.StatusOK {
			t.Fatalf("expected PUT 200, got %d: %s", putRR.Code, putRR.Body.String())
		}

		getReq := httptest.NewRequest(
			http.MethodGet,
			"/api/resources/my-skill/files/notes/v1..draft.md",
			nil,
		)
		getRR := httptest.NewRecorder()
		s.handler.ServeHTTP(getRR, getReq)

		if getRR.Code != http.StatusOK {
			t.Fatalf("expected GET 200, got %d: %s", getRR.Code, getRR.Body.String())
		}

		var getResp struct {
			Content  string `json:"content"`
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
			t.Fatalf("failed to decode GET response: %v", err)
		}
		if getResp.Filename != "v1..draft.md" {
			t.Fatalf("filename = %q, want %q", getResp.Filename, "v1..draft.md")
		}
		if getResp.Content != "# Dotted\n\nUpdated" {
			t.Fatalf("content = %q, want %q", getResp.Content, "# Dotted\n\nUpdated")
		}
	})

	t.Run("rejects symlinked parent directory escape on save", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")
		skillDir := filepath.Join(src, "my-skill")

		outsideDir := t.TempDir()
		outsidePath := filepath.Join(outsideDir, "out.md")
		if err := os.WriteFile(outsidePath, []byte("outside"), 0644); err != nil {
			t.Fatalf("failed to seed outside markdown file: %v", err)
		}

		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/linkdir/out.md",
			strings.NewReader(`{"content":"updated"}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects symlinked parent directory escape on save when target missing", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")
		skillDir := filepath.Join(src, "my-skill")

		outsideDir := t.TempDir()
		linkDir := filepath.Join(skillDir, "linkdir")
		if err := os.Symlink(outsideDir, linkDir); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/linkdir/missing.md",
			strings.NewReader(`{"content":"updated"}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects non markdown path", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")
		txtPath := filepath.Join(src, "my-skill", "notes.txt")
		if err := os.WriteFile(txtPath, []byte("notes"), 0644); err != nil {
			t.Fatalf("failed to seed notes.txt: %v", err)
		}

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/notes.txt",
			strings.NewReader(`{"content":"updated"}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects unknown JSON field", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/SKILL.md",
			strings.NewReader(`{"content":"updated","extra":true}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid request body") {
			t.Fatalf("expected invalid request body error, got: %s", rr.Body.String())
		}
	})

	t.Run("rejects missing required content field", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		req := httptest.NewRequest(
			http.MethodPut,
			"/api/resources/my-skill/files/SKILL.md",
			strings.NewReader(`{}`),
		)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid request body") {
			t.Fatalf("expected invalid request body error, got: %s", rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "missing content") {
			t.Fatalf("expected missing content message, got: %s", rr.Body.String())
		}
	})
}

func TestHandleOpenSkillFile(t *testing.T) {
	t.Run("opens a local skill file with the configured opener", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		notesPath := filepath.Join(src, "my-skill", "notes.md")
		if err := os.WriteFile(notesPath, []byte("# Notes"), 0644); err != nil {
			t.Fatalf("failed to seed notes.md: %v", err)
		}

		var openedPath string
		s.openPath = func(path string) error {
			openedPath = path
			return nil
		}

		req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-file/notes.md", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "notes.md")
		rr := httptest.NewRecorder()
		s.handleOpenSkillFile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		if openedPath != notesPath {
			t.Fatalf("opened path = %q, want %q", openedPath, notesPath)
		}

		var resp struct {
			Success  bool   `json:"success"`
			Filename string `json:"filename"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Success {
			t.Fatalf("expected success=true, got false")
		}
		if resp.Filename != "notes.md" {
			t.Fatalf("filename = %q, want %q", resp.Filename, "notes.md")
		}
	})

	t.Run("rejects traversal attempts", func(t *testing.T) {
		s, src := newTestServer(t)
		addSkill(t, src, "my-skill")

		req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-file/../../etc/passwd", nil)
		req.SetPathValue("name", "my-skill")
		req.SetPathValue("filepath", "../../etc/passwd")
		rr := httptest.NewRecorder()
		s.handleOpenSkillFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "invalid file path") {
			t.Fatalf("expected invalid file path error, got: %s", rr.Body.String())
		}
	})
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
