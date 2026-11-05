package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

// branchSwitchFixture installs skills/foo from the dev branch of a remote whose
// main branch has its own skills/foo.
func branchSwitchFixture(t *testing.T) (*Server, string) {
	t.Helper()
	s, src := newTestServer(t)
	remote := t.TempDir()
	initGitRepo(t, remote)
	runGit(t, remote, "branch", "-M", "main")
	writeSkill := func(body string) {
		dir := filepath.Join(remote, "skills", "foo")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: foo\n---\n"+body+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, remote, "add", ".")
		runGit(t, remote, "commit", "-m", body)
	}
	writeSkill("main body")
	runGit(t, remote, "checkout", "-b", "dev")
	writeSkill("dev body")
	runGit(t, remote, "checkout", "main")

	local := filepath.Join(src, "foo")
	if err := os.MkdirAll(local, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("---\nname: foo\n---\ndev body\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("foo", &install.MetadataEntry{
		Source:  "file://" + remote + "//skills/foo",
		RepoURL: "file://" + remote,
		Subdir:  "skills/foo",
		Branch:  "dev",
	})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	return s, src
}

func postUpdate(s *Server, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.handleUpdate(rr, httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(body)))
	return rr
}

func TestHandleUpdate_BranchReinstallsFromThatBranch(t *testing.T) {
	s, src := branchSwitchFixture(t)

	results := updateResults(t, postUpdate(s, `{"name":"foo","branch":"main"}`), false)
	if len(results) != 1 || results[0].Action != "updated" {
		t.Fatalf("results = %+v, want one updated", results)
	}
	got, _ := os.ReadFile(filepath.Join(src, "foo", "SKILL.md"))
	stored, err := install.LoadMetadata(src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "main body") || stored.GetByPath("foo").Branch != "main" {
		t.Fatalf("SKILL.md = %q, recorded branch = %q; want main's content and branch", got, stored.GetByPath("foo").Branch)
	}
}

func TestHandleUpdate_BranchNeedsOneSkill(t *testing.T) {
	s, _ := newTestServer(t)
	for _, body := range []string{
		`{"all":true,"branch":"main"}`,
		`{"name":"foo","kind":"agent","branch":"main"}`,
	} {
		if rr := postUpdate(s, body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, rr.Code)
		}
	}
}
