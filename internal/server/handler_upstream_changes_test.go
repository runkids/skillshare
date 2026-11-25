package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/git"
	"skillshare/internal/install"
)

func TestHandleSkillUpstreamChanges_NoInstallRecord(t *testing.T) {
	s, _ := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("local", &install.MetadataEntry{})
	if err := s.skillsStore.Save(s.skillsSource()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nope", "local", "x%2F..%2F..%2Flocal"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/"+name+"/upstream-changes", nil))
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}
}

func TestHandleSkillUpstreamChanges_DiffsLatest(t *testing.T) {
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
	if _, err := install.Install(source, filepath.Join(src, "alpha"), install.InstallOptions{SourceDir: src, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	s.skillsStore = install.LoadMetadataOrNew(src)
	if err := os.WriteFile(filepath.Join(remote, "SKILL.md"), []byte("# v2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "commit", "-am", "v2")

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/resources/alpha/upstream-changes", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp install.UpstreamDiff
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Status != "modified" || resp.From == resp.To {
		t.Errorf("resp = %+v", resp)
	}
}

func TestHandleRepoUpstreamChanges(t *testing.T) {
	s, src := newTestServer(t)
	seed := t.TempDir()
	initGitRepo(t, seed)
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, seed, "clone", "--bare", seed, origin)
	runGit(t, src, "clone", origin, "_team")
	// Origin moves ahead and the clone fetches, as an update check does.
	if err := os.WriteFile(filepath.Join(seed, "new.md"), []byte("a\nb\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "add new")
	runGit(t, seed, "push", origin, "HEAD")
	runGit(t, filepath.Join(src, "_team"), "fetch")

	get := func(name string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/repos/"+name+"/upstream-changes", nil))
		return rr
	}

	rr := get("_team")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		TotalCommits int `json:"total_commits"`
		Commits      []git.UpstreamCommit
		Files        []git.UpstreamFile
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TotalCommits != 1 || len(resp.Commits) != 1 || resp.Commits[0].Subject != "add new" {
		t.Errorf("commits = %+v (total %d)", resp.Commits, resp.TotalCommits)
	}
	if len(resp.Files) != 1 || resp.Files[0] != (git.UpstreamFile{Path: "new.md", Added: 2}) {
		t.Errorf("files = %+v", resp.Files)
	}

	for _, name := range []string{"nope", "x%2F..%2F_team"} {
		if rr := get(name); rr.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d: %s", name, rr.Code, rr.Body.String())
		}
	}
}

func TestHandleRepoUpstreamChanges_NoUpstream(t *testing.T) {
	s, src := newTestServer(t)
	repo := filepath.Join(src, "_local")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/repos/_local/upstream-changes", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}
