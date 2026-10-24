package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

func TestHandleSkillLocalChanges_DiffsEditedSkill(t *testing.T) {
	s, src := newTestServer(t)
	remote := t.TempDir()
	initGitRepo(t, remote)
	if err := os.WriteFile(filepath.Join(remote, "SKILL.md"), []byte("# v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", "v1")
	source, err := install.ParseSource("file://" + remote)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(src, "tools", "alpha")
	if _, err := install.Install(source, dest, install.InstallOptions{SourceDir: src, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	s.skillsStore = install.LoadMetadataOrNew(src)
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("# mine\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/tools%2Falpha/local-changes", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct{ Files []install.FileChange }
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	want := []install.FileChange{{Path: "SKILL.md", Status: "modified", Diff: "- # v1\n+ # mine\n"}}
	if len(resp.Files) != 1 || resp.Files[0] != want[0] {
		t.Errorf("files = %+v, want %+v", resp.Files, want)
	}
}

// A nested or escaping path must not borrow a top-level skill's record.
func TestHandleSkillLocalChanges_OnlyExactRecord(t *testing.T) {
	s, _ := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("foo", &install.MetadataEntry{Source: "file:///r", RepoURL: "file:///r", Version: "abc1234", FileHashes: map[string]string{"SKILL.md": "sha256:x"}})
	// Each request reloads the store from disk.
	if err := s.skillsStore.Save(s.skillsSource()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nope", "_team%2Ffoo", "x%2F..%2F..%2Ffoo"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/"+name+"/local-changes", nil))
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}
}
