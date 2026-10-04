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

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

func serveSkillfollow(t *testing.T, s *Server, method, body string) (*httptest.ResponseRecorder, skillfollowResponse) {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(method, "/api/skillfollow", bytes.NewBufferString(body)))
	var resp skillfollowResponse
	if rr.Code == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v (body=%s)", err, rr.Body.String())
		}
	}
	return rr, resp
}

// linkFollowedGroup links name in the source to an external group directory.
func linkFollowedGroup(t *testing.T, src, name string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "group")
	if err := os.MkdirAll(filepath.Join(target, "review"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "review", "SKILL.md"), []byte("---\nname: review\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, name)); err != nil {
		t.Skip(err)
	}
	return target
}

func TestHandleGetSkillfollow_NoFiles(t *testing.T) {
	s, src := newTestServer(t)

	rr, resp := serveSkillfollow(t, s, http.MethodGet, "")

	if rr.Code != http.StatusOK || resp.Active || resp.Base.Exists || resp.Local.Exists || len(resp.Entries) != 0 {
		t.Fatalf("expected an inactive empty view, got %d %+v", rr.Code, resp)
	}
	if resp.Base.Path != filepath.Join(src, ".skillfollow") {
		t.Fatalf("base path = %q", resp.Base.Path)
	}
}

func TestHandleGetSkillfollow_BaseOnly(t *testing.T) {
	s, src := newTestServer(t)
	linkFollowedGroup(t, src, "_team")
	os.WriteFile(filepath.Join(src, ".skillfollow"), []byte("# team\n_team\n"), 0644)

	_, resp := serveSkillfollow(t, s, http.MethodGet, "")

	if !resp.Active || resp.LocalActive || resp.Base.Content != "# team\n_team\n" || len(resp.Entries) != 1 || resp.Entries[0].State != sourcewalk.Followed {
		t.Fatalf("expected one followed entry from the base file, got %+v", resp)
	}
}

func TestHandleGetSkillfollow_BaseAndLocal(t *testing.T) {
	s, src := newTestServer(t)
	linkFollowedGroup(t, src, "_team")
	os.WriteFile(filepath.Join(src, ".skillfollow"), []byte("_team\n"), 0644)
	os.WriteFile(filepath.Join(src, ".skillfollow.local"), []byte("_mine\n"), 0644)

	_, resp := serveSkillfollow(t, s, http.MethodGet, "")

	if !resp.LocalActive || !resp.Local.Exists || resp.Local.Content != "_mine\n" || len(resp.Entries) != 2 || resp.Entries[1].Name != "_mine" {
		t.Fatalf("expected base then local entries, got %+v", resp)
	}
}

func TestHandleGetSkillfollow_RejectedEntryPausesPrune(t *testing.T) {
	s, src := newTestServer(t)
	os.WriteFile(filepath.Join(src, ".skillfollow"), []byte("_gone\n"), 0644)

	_, resp := serveSkillfollow(t, s, http.MethodGet, "")

	if len(resp.Entries) != 1 || resp.Entries[0].State != sourcewalk.Missing || len(resp.Warnings) != 1 {
		t.Fatalf("expected one missing entry with a warning, got %+v", resp)
	}
	if len(resp.PrunePaused) != 1 || !strings.HasPrefix(resp.PrunePaused[0], "prune paused: _gone is missing;") {
		t.Fatalf("expected the prune recovery message, got %q", resp.PrunePaused)
	}
}

func TestHandlePutSkillfollow_WritesAndReturnsStates(t *testing.T) {
	s, src := newTestServer(t)
	linkFollowedGroup(t, src, "_team")

	rr, resp := serveSkillfollow(t, s, http.MethodPut, `{"file":"local","content":"_team\n"}`)

	if rr.Code != http.StatusOK || !resp.LocalActive || len(resp.Entries) != 1 || resp.Entries[0].State != sourcewalk.Followed {
		t.Fatalf("expected the written entry to be followed, got %d %s", rr.Code, rr.Body.String())
	}
	if got, _ := os.ReadFile(filepath.Join(src, ".skillfollow.local")); string(got) != "_team\n" {
		t.Fatalf(".skillfollow.local = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(src, "_team")); err != nil {
		t.Fatalf("link was touched: %v", err)
	}
	if e := latestOpsEntry(t, s); e.Command != "skillfollow" || e.Status != "ok" || e.Args["file"] != ".skillfollow.local" {
		t.Fatalf("ops entry = %+v", e)
	}
}

func TestHandlePutSkillfollow_InvalidNameLeavesFile(t *testing.T) {
	s, src := newTestServer(t)
	path := filepath.Join(src, ".skillfollow")
	os.WriteFile(path, []byte("_team\n"), 0644)

	rr, _ := serveSkillfollow(t, s, http.MethodPut, `{"file":"base","content":"_team\n# ok\nnested/dir\n"}`)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `.skillfollow:3: invalid first-level entry \"nested/dir\"`) {
		t.Fatalf("expected a 400 naming line 3, got %d: %s", rr.Code, rr.Body.String())
	}
	if got, _ := os.ReadFile(path); string(got) != "_team\n" {
		t.Fatalf("file changed to %q", got)
	}
}

func TestHandlePutSkillfollow_DeleteNeedsFlag(t *testing.T) {
	s, src := newTestServer(t)
	path := filepath.Join(src, ".skillfollow")
	os.WriteFile(path, []byte("_team\n"), 0644)

	serveSkillfollow(t, s, http.MethodPut, `{"file":"base","content":""}`)
	if got, err := os.ReadFile(path); err != nil || len(got) != 0 {
		t.Fatalf("empty content without delete should leave an empty file: %q %v", got, err)
	}

	rr, resp := serveSkillfollow(t, s, http.MethodPut, `{"file":"base","content":"","delete":true}`)
	if _, err := os.Stat(path); rr.Code != http.StatusOK || resp.Base.Exists || !os.IsNotExist(err) {
		t.Fatalf("expected the file deleted, got %d %v", rr.Code, err)
	}
}

func TestHandlePutSkillfollow_FailedWriteKeepsFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	s, src := newTestServer(t)
	path := filepath.Join(src, ".skillfollow")
	os.WriteFile(path, []byte("_team\n"), 0644)
	if err := os.Chmod(src, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(src, 0755) })

	rr, _ := serveSkillfollow(t, s, http.MethodPut, `{"file":"base","content":"_other\n"}`)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected a write failure, got %d: %s", rr.Code, rr.Body.String())
	}
	if got, _ := os.ReadFile(path); string(got) != "_team\n" {
		t.Fatalf("file changed to %q", got)
	}
	entries, _ := os.ReadDir(src)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".skillshare-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	if e := latestOpsEntry(t, s); e.Command != "skillfollow" || e.Status != "error" {
		t.Fatalf("ops entry = %+v", e)
	}
}

func TestHandlePutSkillfollow_RefusesLinkedFile(t *testing.T) {
	s, src := newTestServer(t)
	shared := linkSourceFile(t, src, ".skillfollow", "_team\n")

	for _, body := range []string{`{"file":"base","content":"_x\n"}`, `{"file":"base","content":"","delete":true}`} {
		rr, _ := serveSkillfollow(t, s, http.MethodPut, body)
		if rr.Code == http.StatusOK || !strings.Contains(rr.Body.String(), "is a link") {
			t.Fatalf("PUT %s: expected a link refusal, got %d: %s", body, rr.Code, rr.Body.String())
		}
		assertLinkKept(t, filepath.Join(src, ".skillfollow"), shared, "_team\n")
	}
}

// The project dashboard edits the project source's declarations, the same
// files its follow set reads, even when the global source is elsewhere.
func TestHandlePutSkillfollow_ProjectSource(t *testing.T) {
	s, _ := newTestServer(t)
	root := t.TempDir()
	source := filepath.Join(root, ".skillshare", "skills")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	pcfg := &config.ProjectConfig{}
	if err := pcfg.Save(root); err != nil {
		t.Fatal(err)
	}
	s = NewProject(s.cfg, pcfg, root, "127.0.0.1:0", "", "")

	rr, resp := serveSkillfollow(t, s, http.MethodPut, `{"file":"base","content":"_gone\n"}`)

	if rr.Code != http.StatusOK || resp.Base.Path != filepath.Join(source, ".skillfollow") || len(resp.Entries) != 1 {
		t.Fatalf("expected the project declaration, got %d %s", rr.Code, rr.Body.String())
	}
}
