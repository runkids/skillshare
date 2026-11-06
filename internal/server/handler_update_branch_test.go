package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// installFromDev installs skills/foo into src from the dev branch of a remote
// whose main branch has its own skills/foo, and returns the recorded source.
func installFromDev(t *testing.T, s *Server, src string) string {
	t.Helper()
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
	source := "file://" + remote + "//skills/foo"
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("foo", &install.MetadataEntry{
		Source:  source,
		RepoURL: "file://" + remote,
		Subdir:  "skills/foo",
		Branch:  "dev",
	})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	return source
}

func postUpdate(s *Server, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.handleUpdate(rr, httptest.NewRequest(http.MethodPost, "/api/update", strings.NewReader(body)))
	return rr
}

func TestHandleUpdate_BranchReinstallsFromThatBranch(t *testing.T) {
	s, src := newTestServer(t)
	installFromDev(t, s, src)

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

// Only the switched skill's declaration moves; another skill whose local record
// lags the shared config keeps what config.yaml says.
func TestHandleUpdate_BranchMovesOnlyThatProjectDeclaration(t *testing.T) {
	s, root := newTestProjectServerWithExtras(t, nil)
	src := s.cfg.EffectiveSkillsSource()
	source := installFromDev(t, s, src)
	s.skillsStore.Set("bar", &install.MetadataEntry{Source: "github.com/o/r/bar", Branch: "old"})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "bar"), 0755); err != nil {
		t.Fatal(err)
	}
	s.projectCfg.Skills = []config.SkillEntry{
		{Name: "foo", Source: source, Branch: "dev"},
		{Name: "bar", Source: "github.com/o/r/bar", Branch: "new"},
	}
	if err := s.projectCfg.Save(root); err != nil {
		t.Fatal(err)
	}

	updateResults(t, postUpdate(s, `{"name":"foo","branch":"main"}`), false)

	loaded, err := config.LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, skill := range loaded.Skills {
		got[skill.Name] = skill.Branch
	}
	if got["foo"] != "main" || got["bar"] != "new" {
		t.Fatalf("declared branches = %v, want foo on main and bar left on new", got)
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
