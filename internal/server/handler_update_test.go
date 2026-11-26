package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

// initGitRepo creates a minimal git repo with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}
	// Create initial commit
	f := filepath.Join(dir, "README.md")
	os.WriteFile(f, []byte("# init"), 0644)
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}
}

func TestUpdateSingle_NestedTrackedRepo_Resolves(t *testing.T) {
	s, src := newTestServer(t)

	// Create a nested tracked repo with a real git init
	repoDir := filepath.Join(src, "org", "_team-skills")
	os.MkdirAll(repoDir, 0755)
	initGitRepo(t, repoDir)

	result := s.updateSingle("org/_team-skills", false, true)
	// Should resolve (may be up-to-date or updated, but NOT "error"/"not found")
	if result.Action == "error" && strings.Contains(result.Message, "not found") {
		t.Fatalf("updateSingle failed to resolve nested repo: %s", result.Message)
	}
}

func TestUpdateSingle_BasenameFallback(t *testing.T) {
	s, src := newTestServer(t)

	// Create a top-level tracked repo with _ prefix
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(repoDir, 0755)
	initGitRepo(t, repoDir)

	// Call with unprefixed basename — should resolve to _team-skills
	result := s.updateSingle("team-skills", false, true)
	if result.Action == "error" && strings.Contains(result.Message, "not found") {
		t.Fatalf("updateSingle failed to resolve basename: %s", result.Message)
	}
	if result.Name != "_team-skills" && result.Name != "team-skills" {
		// Name should reflect the resolved repo
		t.Logf("resolved name: %s, action: %s", result.Name, result.Action)
	}
}

func TestUpdateSingle_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	result := s.updateSingle("nonexistent-repo", false, true)
	if result.Action != "error" {
		t.Fatalf("expected action=error for not-found, got %q", result.Action)
	}
	if !strings.Contains(result.Message, "not found") && !strings.Contains(result.Message, "no update source") {
		t.Fatalf("expected not-found message, got %q", result.Message)
	}
}

func TestUpdateRegularSkill_NestedSkillRefreshesRootMetadataStore(t *testing.T) {
	s, src := newTestServer(t)

	remote := t.TempDir()
	initGitRepo(t, remote)
	remoteSkill := filepath.Join(remote, "skills", "agent-browser")
	if err := os.MkdirAll(remoteSkill, 0755); err != nil {
		t.Fatalf("create remote skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remoteSkill, "SKILL.md"), []byte("---\nname: agent-browser\n---\n# Agent Browser\n"), 0644); err != nil {
		t.Fatalf("write remote skill: %v", err)
	}
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", "add skill")
	latestCommit := strings.TrimSpace(string(runGit(t, remote, "rev-parse", "--short", "HEAD")))

	localSkill := filepath.Join(src, "tools", "agent-browser")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatalf("create local skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("---\nname: agent-browser\n---\n# Old\n"), 0644); err != nil {
		t.Fatalf("write local skill: %v", err)
	}

	store := install.NewMetadataStore()
	store.Set("tools/agent-browser", &install.MetadataEntry{
		Source:  "file://" + remote + "//skills/agent-browser",
		RepoURL: "file://" + remote,
		Subdir:  "skills/agent-browser",
		Version: "old-version",
	})
	if err := store.Save(src); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	s.skillsStore = store

	result := s.updateRegularSkill("tools/agent-browser", localSkill, false, true, false)
	if result.Action != "updated" {
		t.Fatalf("expected updated action, got %q: %s", result.Action, result.Message)
	}

	rootStore, err := install.LoadMetadata(src)
	if err != nil {
		t.Fatalf("load root metadata: %v", err)
	}
	entry := rootStore.GetByPath("tools/agent-browser")
	if entry == nil {
		t.Fatal("expected root metadata entry for nested skill")
	}
	if entry.Version != latestCommit {
		t.Fatalf("root metadata version = %q, want %q", entry.Version, latestCommit)
	}
	if cached := s.skillsStore.GetByPath("tools/agent-browser"); cached == nil || cached.Version != latestCommit {
		t.Fatalf("server metadata cache was not refreshed, got %#v", cached)
	}
	if _, err := os.Stat(filepath.Join(src, "tools", install.MetadataFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no nested metadata store, stat err=%v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s %v", args, out, err)
	}
	return out
}

func TestUpdateRegularSkill_LocallyModifiedIsSkippedWithoutForce(t *testing.T) {
	s, src := newTestServer(t)
	skillPath := filepath.Join(src, "edited")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("edited", &install.MetadataEntry{
		Source:     "github.com/org/repo/edited",
		RepoURL:    "https://github.com/org/repo.git",
		FileHashes: map[string]string{"SKILL.md": "sha256:original"},
	})

	result := s.updateRegularSkill("edited", skillPath, false, true, false)
	if result.Action != "skipped" || !strings.Contains(result.Message, "(try force update)") {
		t.Fatalf("got %q: %s, want skipped with a force hint", result.Action, result.Message)
	}
}

// addEditedAgent installs agents/edited.md from a new local remote at version,
// edits the installed file, and returns its path and the remote's commit.
func addEditedAgent(t *testing.T, s *Server, version string) (file, head string) {
	t.Helper()
	remote := t.TempDir()
	initGitRepo(t, remote)
	if err := os.MkdirAll(filepath.Join(remote, "agents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remote, "agents", "edited.md"), []byte("# upstream\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", "add edited")
	head = strings.TrimSpace(string(runGit(t, remote, "rev-parse", "--short", "HEAD")))
	if version == "" {
		version = head
	}

	agentsDir := s.agentsSource()
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(agentsDir, "edited.md")
	if err := os.WriteFile(file, []byte("# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s.agentsStore = install.NewMetadataStore()
	s.agentsStore.Set("edited", &install.MetadataEntry{
		Source:     "file://" + remote + "//agents/edited.md",
		Kind:       "agent",
		RepoURL:    "file://" + remote,
		Subdir:     "agents/edited.md",
		Version:    version,
		FileHashes: map[string]string{"edited.md": "sha256:original"},
	})
	return file, head
}

func TestUpdateAgent_LocallyModifiedIsSkippedWithoutForce(t *testing.T) {
	s, _ := newTestServer(t)
	file, _ := addEditedAgent(t, s, "outdated")

	result := s.updateAgent("edited", false, true, false)
	if result.Action != "skipped" || !strings.Contains(result.Message, "(try force update)") {
		t.Fatalf("got %q: %s, want skipped with a force hint", result.Action, result.Message)
	}
	if got, _ := os.ReadFile(file); string(got) != "# my edit\n" {
		t.Errorf("the local edit was overwritten: %q", got)
	}
}

// Force has nothing to apply at the installed version, so it is not offered.
func TestUpdateAgent_LocallyModifiedAtTheRemoteVersionIsUpToDate(t *testing.T) {
	s, _ := newTestServer(t)
	addEditedAgent(t, s, "")

	if result := s.updateAgent("edited", false, true, false); result.Action != "up-to-date" {
		t.Fatalf("got %q: %s, want up-to-date", result.Action, result.Message)
	}
}

// A later commit that left the agent's file alone has nothing to apply either:
// its blob hash matches the one recorded at install (#1041).
func TestUpdateAgent_UnchangedFileAtANewerCommitIsUpToDate(t *testing.T) {
	s, _ := newTestServer(t)
	addEditedAgent(t, s, "outdated")
	entry := s.agentsStore.Get("edited")
	remote := strings.TrimPrefix(entry.RepoURL, "file://")
	entry.TreeHash = strings.TrimSpace(string(runGit(t, remote, "rev-parse", "HEAD:agents/edited.md")))

	if result := s.updateAgent("edited", false, true, false); result.Action != "up-to-date" {
		t.Fatalf("got %q: %s, want up-to-date", result.Action, result.Message)
	}
}

func TestUpdateSingle_RegularSkillPriority(t *testing.T) {
	s, src := newTestServer(t)

	// Create both a regular skill with meta AND a tracked repo with _ prefix
	skillDir := filepath.Join(src, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n# Skill"), 0644)
	addSkillMeta(t, src, "my-skill", "https://github.com/example/my-skill")

	repoDir := filepath.Join(src, "_my-skill")
	os.MkdirAll(repoDir, 0755)
	initGitRepo(t, repoDir)

	// updateSingle("my-skill") should try regular skill first (has meta).
	// It will fail to clone (fake URL) but the point is it chose the skill path,
	// not the tracked repo path. A "not found" would mean it tried the repo instead.
	result := s.updateSingle("my-skill", false, true)
	// The error should be about cloning (skill path), not "not found" (repo path)
	if result.Action == "error" && strings.Contains(result.Message, "not found") && !strings.Contains(result.Message, "clone") {
		t.Fatalf("expected regular skill to be tried first, got repo-like not-found: %s", result.Message)
	}
}

func TestUpdateAgent_AzureOnPrem_UsesParseOpts(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.AzureHosts = []string{"azuredevops.corp.com"}

	// Verify that server.parseOpts() + ParseSourceWithOptions produces
	// the correct CloneURL for an on-prem Azure agent source.
	// This is the exact parse path used inside updateAgent.
	source, err := install.ParseSourceWithOptions(
		"https://azuredevops.corp.com/Org/Project/_git/Repo/agents/my-agent.md",
		s.parseOpts(),
	)
	if err != nil {
		t.Fatalf("ParseSourceWithOptions error: %v", err)
	}
	wantURL := "https://azuredevops.corp.com/Org/Project/_git/Repo"
	if source.CloneURL != wantURL {
		t.Errorf("CloneURL = %q, want %q", source.CloneURL, wantURL)
	}
	if source.Subdir != "agents/my-agent.md" {
		t.Errorf("Subdir = %q, want %q", source.Subdir, "agents/my-agent.md")
	}
}

// writeRemoteSkill commits skills/<name>/SKILL.md with body to remote.
func writeRemoteSkill(t *testing.T, remote, name, body string) {
	t.Helper()
	dir := filepath.Join(remote, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"+body+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", name+" "+body)
}

// recordRemoteSkills creates a remote holding the named skills and records an
// outdated local copy of each in the server's store.
func recordRemoteSkills(t *testing.T, s *Server, src string, names ...string) string {
	t.Helper()
	remote := t.TempDir()
	initGitRepo(t, remote)
	for _, name := range names {
		writeRemoteSkill(t, remote, name, "v1")
		if err := os.MkdirAll(filepath.Join(src, name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, name, "SKILL.md"), []byte("---\nname: "+name+"\n---\nold\n"), 0644); err != nil {
			t.Fatal(err)
		}
		s.skillsStore.Set(name, &install.MetadataEntry{
			Source:  "file://" + remote + "//skills/" + name,
			RepoURL: "file://" + remote,
			Subdir:  "skills/" + name,
		})
	}
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	return remote
}

func streamUpdate(t *testing.T, s *Server, names string) []updateResultItem {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handleUpdateStream(rr, httptest.NewRequest(http.MethodGet, "/api/update/stream?skipAudit=true&names="+names, nil))
	return updateResults(t, rr, true)
}

// Skills of one repository share a fetch, so the stream must still answer in
// request order (the dashboard pairs results with names by position) and
// report a skill that already matches the remote as up to date.
func TestHandleUpdateStream_SameRepoSkillsReportInRequestOrder(t *testing.T) {
	s, src := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	shared := recordRemoteSkills(t, s, src, "a1", "a2")
	recordRemoteSkills(t, s, src, "b")

	run := func() string {
		var got []string
		for _, item := range streamUpdate(t, s, "a1,b,a2") {
			got = append(got, item.Name+"="+item.Action)
		}
		return strings.Join(got, " ")
	}

	if got, want := run(), "a1=updated b=updated a2=updated"; got != want {
		t.Fatalf("first update: got %q, want %q", got, want)
	}
	writeRemoteSkill(t, shared, "a2", "v2")
	if got, want := run(), "a1=up-to-date b=up-to-date a2=updated"; got != want {
		t.Fatalf("after a2 changed upstream: got %q, want %q", got, want)
	}
}

// One skill's outcome must not change its repo-mates', and the messages the
// dashboard matches for Force Retry and Purge must survive the shared fetch.
func TestHandleUpdate_AllReportsEachSkillOfSharedRepo(t *testing.T) {
	s, src := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	remote := recordRemoteSkills(t, s, src, "current", "edited", "gone")
	streamUpdate(t, s, "current,edited,gone")

	if err := os.WriteFile(filepath.Join(src, "edited", "SKILL.md"), []byte("# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "rm", "-r", "skills/gone")
	runGit(t, remote, "commit", "-m", "drop gone")

	got := map[string]updateResultItem{}
	for _, item := range updateResults(t, postUpdate(s, `{"all":true,"skipAudit":true}`), false) {
		got[item.Name] = item
	}
	for name, want := range map[string]struct{ action, message string }{
		"current": {"up-to-date", ""},
		"edited":  {"skipped", "(try force update)"},
		"gone":    {"error", "not found in repository"},
	} {
		if item := got[name]; item.Action != want.action || !strings.Contains(item.Message, want.message) {
			t.Errorf("%s: got %q (%s), want %q containing %q", name, item.Action, item.Message, want.action, want.message)
		}
	}
}

// A root skill kept as a Git checkout updates by pulling, so the shared fetch
// must not swap a plain copy over it and drop its .git directory.
func TestHandleUpdateStream_RootSkillCheckoutKeepsItsRepository(t *testing.T) {
	s, src := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	remote := t.TempDir()
	initGitRepo(t, remote)
	writeRootSkill := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(remote, "SKILL.md"), []byte("---\nname: solo\n---\n"+body+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, remote, "add", ".")
		runGit(t, remote, "commit", "-m", body)
	}
	writeRootSkill("v1")
	runGit(t, src, "clone", "file://"+remote, "solo")
	s.skillsStore.Set("solo", &install.MetadataEntry{Source: "file://" + remote, RepoURL: "file://" + remote})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	writeRootSkill("v2")

	if got := streamUpdate(t, s, "solo"); len(got) != 1 || got[0].Action != "updated" {
		t.Fatalf("update: got %+v, want one updated item", got)
	}
	if !install.IsGitRepo(filepath.Join(src, "solo")) {
		t.Fatal("the checkout lost its .git directory")
	}
}

// The shared fetch records the source in the repository's canonical spelling.
// A project declares the skill by the spelling it was installed with, so the
// record must keep that spelling or the declaration stops matching.
func TestHandleUpdateStream_KeepsRecordedSourceSpelling(t *testing.T) {
	s, src := newTestServer(t)
	s.skillsStore = install.NewMetadataStore()
	remote := recordRemoteSkills(t, s, src, "demo")
	spelling := "file://" + remote + "/.//skills/demo"
	s.skillsStore.GetByPath("demo").Source = spelling
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}

	if got := streamUpdate(t, s, "demo"); len(got) != 1 || got[0].Action != "updated" {
		t.Fatalf("update: got %+v, want one updated item", got)
	}
	if got := s.skillsStore.GetByPath("demo").Source; got != spelling {
		t.Fatalf("recorded source = %q, want %q", got, spelling)
	}
}
