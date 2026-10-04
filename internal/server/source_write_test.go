package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// linkSourceFile replaces the file at rel in the skills source with a link to
// a shared file outside it and returns the shared file.
func linkSourceFile(t *testing.T, src, rel, content string) string {
	t.Helper()
	shared := filepath.Join(t.TempDir(), "shared")
	if err := os.WriteFile(shared, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, rel)
	os.Remove(path)
	if err := os.Symlink(shared, path); err != nil {
		t.Fatal(err)
	}
	return shared
}

func assertLinkKept(t *testing.T, path, shared, content string) {
	t.Helper()
	if fi, err := os.Lstat(path); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a link: %v", path, err)
	}
	if got, _ := os.ReadFile(shared); string(got) != content {
		t.Fatalf("shared file was modified: %q", got)
	}
}

func TestHandleSetSkillTargets_RefusesLinkedSkillFile(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")
	content := "---\nname: my-skill\n---\n# Shared"
	shared := linkSourceFile(t, src, filepath.Join("my-skill", "SKILL.md"), content)

	req := httptest.NewRequest(http.MethodPatch, "/api/resources/my-skill/targets", bytes.NewBufferString(`{"target":"claude"}`))
	req.SetPathValue("name", "my-skill")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code == http.StatusOK || !strings.Contains(rr.Body.String(), "is a link") {
		t.Fatalf("expected a link refusal, got %d: %s", rr.Code, rr.Body.String())
	}
	assertLinkKept(t, filepath.Join(src, "my-skill", "SKILL.md"), shared, content)
}

func TestHandleDisableSkill_RefusesLinkedSkillignore(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")
	shared := linkSourceFile(t, src, ".skillignore", "# shared\n")

	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/disable", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK || !strings.Contains(rr.Body.String(), "is a link") {
		t.Fatalf("expected a link refusal, got %d: %s", rr.Code, rr.Body.String())
	}
	assertLinkKept(t, filepath.Join(src, ".skillignore"), shared, "# shared\n")
}

func TestHandlePutSkillignore_RefusesLinkedFile(t *testing.T) {
	s, src := newTestServer(t)
	shared := linkSourceFile(t, src, ".skillignore", "# shared\n")

	for _, body := range []string{`{"raw":"replaced\n"}`, `{"raw":""}`} {
		req := httptest.NewRequest(http.MethodPut, "/api/skillignore", bytes.NewBufferString(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		if rr.Code == http.StatusOK || !strings.Contains(rr.Body.String(), "is a link") {
			t.Fatalf("PUT %s: expected a link refusal, got %d: %s", body, rr.Code, rr.Body.String())
		}
		assertLinkKept(t, filepath.Join(src, ".skillignore"), shared, "# shared\n")
	}
}

func TestHandleCreateSkill_RefusesIntoThroughLink(t *testing.T) {
	s, src := newTestServer(t)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(src, "linked")); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"my-tool","pattern":"none","into":"linked"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusCreated || !strings.Contains(rr.Body.String(), "is a link") {
		t.Fatalf("expected a link refusal, got %d: %s", rr.Code, rr.Body.String())
	}
	if entries, _ := os.ReadDir(external); len(entries) != 0 {
		t.Fatalf("external tree changed: %v", entries)
	}
}
