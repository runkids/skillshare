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

func TestHandlePutSkillContent_WritesFile(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := skillContentRequest{Content: "---\nname: my-skill\ndescription: edited\n---\n# Updated"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp skillContentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response decode: %v", err)
	}
	if resp.BytesWritten != len(body.Content) {
		t.Errorf("expected %d bytes written, got %d", len(body.Content), resp.BytesWritten)
	}

	on := filepath.Join(src, "my-skill", "SKILL.md")
	got, err := os.ReadFile(on)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(got) != body.Content {
		t.Errorf("file content mismatch:\nwant: %q\ngot:  %q", body.Content, got)
	}
}

func TestHandlePutSkillContent_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	body := skillContentRequest{Content: "foo"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/nope/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutSkillContent_InvalidJSON(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", strings.NewReader("not-json"))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutSkillContent_TooLarge(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// Assemble a 3 MiB body — exceeds the 2 MiB ceiling.
	huge := make([]byte, 3*1024*1024)
	for i := range huge {
		huge[i] = 'x'
	}
	body := skillContentRequest{Content: string(huge)}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
}

func TestHandlePutSkillContent_AtomicTempCleanup(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := skillContentRequest{Content: "new content"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/my-skill/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected %d", rr.Code)
	}

	entries, err := os.ReadDir(filepath.Join(src, "my-skill"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".skillshare-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestHandlePutSkillContent_RejectsTraversal(t *testing.T) {
	// The route only exposes {name}; Go's mux rejects a literal "/" inside
	// {name}, but we still want to confirm `..` as a name doesn't escape.
	s, src := newTestServer(t)
	addSkill(t, src, "victim")

	body := skillContentRequest{Content: "boom"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/resources/..%2Fescape/content", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	// Any of: 400 / 404 is acceptable — what matters is that the victim's file
	// is untouched.
	got, err := os.ReadFile(filepath.Join(src, "victim", "SKILL.md"))
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if !strings.Contains(string(got), "# victim") {
		t.Errorf("victim content was modified: %q", got)
	}
}

func TestHandlePatchSkillSource_RemoteUpdateFailureDoesNotPersistMetadata(t *testing.T) {
	s, src := newTestServer(t)

	repoDir := filepath.Join(src, "_team-repo")
	skillDir := filepath.Join(repoDir, "skill-a")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("create .git dir: %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: skill-a\n---\n# skill-a"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("_team-repo/skill-a", &install.MetadataEntry{
		Source:  "https://github.com/old/repo/skill-a",
		RepoURL: "https://github.com/old/repo.git",
		Tracked: true,
	})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	body := patchSourceRequest{Source: "https://github.com/new/repo/skill-a"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/_team-repo__skill-a/source", bytes.NewReader(raw))
	req.SetPathValue("name", "_team-repo__skill-a")
	rr := httptest.NewRecorder()
	s.handlePatchSkillSource(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "failed to update git remote URL") {
		t.Fatalf("response %q does not mention remote update failure", rr.Body.String())
	}

	store, err := install.LoadMetadata(src)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	entry := store.Get("_team-repo/skill-a")
	if entry == nil {
		t.Fatal("metadata entry missing")
	}
	if entry.Source != "https://github.com/old/repo/skill-a" {
		t.Fatalf("metadata Source = %q, want old source", entry.Source)
	}
	if entry.RepoURL != "https://github.com/old/repo.git" {
		t.Fatalf("metadata RepoURL = %q, want old repo URL", entry.RepoURL)
	}
}

func TestHandlePatchSkillSource_UpdatesRemoteAndMetadata(t *testing.T) {
	s, src := newTestServer(t)

	repoDir := filepath.Join(src, "_team-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	initGitRepo(t, repoDir)
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/old/repo.git")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add failed: %v\n%s", err, out)
	}

	skillDir := filepath.Join(repoDir, "skill-a")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: skill-a\n---\n# skill-a"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("_team-repo/skill-a", &install.MetadataEntry{
		Source:  "https://github.com/old/repo/skill-a",
		RepoURL: "https://github.com/old/repo.git",
		Tracked: true,
	})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	body := patchSourceRequest{Source: "https://github.com/new/repo/skill-a"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, "/api/resources/_team-repo__skill-a/source", bytes.NewReader(raw))
	req.SetPathValue("name", "_team-repo__skill-a")
	rr := httptest.NewRecorder()
	s.handlePatchSkillSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cmd = exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git remote get-url failed: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "https://github.com/new/repo.git" {
		t.Fatalf("remote URL = %q, want %q", got, "https://github.com/new/repo.git")
	}

	store, err := install.LoadMetadata(src)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	entry := store.Get("_team-repo/skill-a")
	if entry == nil {
		t.Fatal("metadata entry missing")
	}
	if entry.Source != "https://github.com/new/repo/skill-a" {
		t.Fatalf("metadata Source = %q, want new source", entry.Source)
	}
	if entry.RepoURL != "https://github.com/new/repo.git" {
		t.Fatalf("metadata RepoURL = %q, want new repo URL", entry.RepoURL)
	}
}
