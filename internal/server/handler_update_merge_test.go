package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

// installEditedSkill installs skills/foo from a new remote into src, changes
// the installed SKILL.md's first body line to "mine", and commits upstream
// as the remote's new body.
func installEditedSkill(t *testing.T, s *Server, src, upstream string) {
	t.Helper()
	remote := t.TempDir()
	initGitRepo(t, remote)
	file := filepath.Join(remote, "skills", "foo", "SKILL.md")
	write := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(file, "---\nname: foo\n---\nfirst\nmiddle\nlast\n")
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", "v1")

	source, err := install.ParseSource("file://" + remote + "//skills/foo")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(src, "foo")
	if _, err := install.Install(source, dest, install.InstallOptions{SourceDir: src, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	if s.skillsStore, err = install.LoadMetadata(src); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(dest, "SKILL.md"), "---\nname: foo\n---\nmine\nmiddle\nlast\n")
	write(file, "---\nname: foo\n---\n"+upstream)
	runGit(t, remote, "commit", "-am", "v2")
}

func TestHandleUpdate_MergeKeepsLocalEdits(t *testing.T) {
	s, src := newTestServer(t)
	installEditedSkill(t, s, src, "first\nmiddle\ntheirs\n")

	results := updateResults(t, postUpdate(s, `{"name":"foo","merge":true,"skipAudit":true}`), false)
	if len(results) != 1 || results[0].Action != "updated" || results[0].Message != "merged local changes" {
		t.Fatalf("results = %+v, want one merged update", results)
	}
	got, _ := os.ReadFile(filepath.Join(src, "foo", "SKILL.md"))
	if string(got) != "---\nname: foo\n---\nmine\nmiddle\ntheirs\n" {
		t.Errorf("SKILL.md = %q, want both edits", got)
	}
}

func TestHandleUpdate_MergeConflictOffersForce(t *testing.T) {
	s, src := newTestServer(t)
	installEditedSkill(t, s, src, "theirs\nmiddle\nlast\n")

	results := updateResults(t, postUpdate(s, `{"name":"foo","merge":true,"skipAudit":true}`), false)
	if len(results) != 1 || results[0].Action != "error" ||
		!strings.Contains(results[0].Message, "SKILL.md") || !strings.Contains(results[0].Message, "(try force update)") {
		t.Fatalf("results = %+v, want a conflict naming SKILL.md with a force hint", results)
	}
	got, _ := os.ReadFile(filepath.Join(src, "foo", "SKILL.md"))
	if string(got) != "---\nname: foo\n---\nmine\nmiddle\nlast\n" {
		t.Errorf("SKILL.md changed on conflict: %q", got)
	}
}

// A branch switch replaces the skill with another branch, so there is no
// upstream of the installed version to merge into.
func TestHandleUpdate_MergeExcludesForceAndBranch(t *testing.T) {
	s, _ := newTestServer(t)
	for _, body := range []string{
		`{"name":"foo","force":true,"merge":true}`,
		`{"name":"foo","branch":"main","merge":true}`,
	} {
		if rr := postUpdate(s, body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", body, rr.Code)
		}
	}
}

func TestUpdateAgent_MergeKeepsLocalEdits(t *testing.T) {
	s, _ := newTestServer(t)
	remote := t.TempDir()
	initGitRepo(t, remote)
	upstream := filepath.Join(remote, "agents", "edited.md")
	if err := os.MkdirAll(filepath.Dir(upstream), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upstream, []byte("# agent\nfirst\nmiddle\nlast\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "add", ".")
	runGit(t, remote, "commit", "-m", "v1")
	source, err := install.ParseSource("file://" + remote)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		t.Fatal(err)
	}
	defer install.CleanupDiscovery(discovery)
	agentsDir := s.agentsSource()
	if _, err := install.InstallAgentFromDiscovery(discovery, discovery.Agents[0], agentsDir, install.InstallOptions{Kind: "agent", SourceDir: agentsDir, SkipAudit: true}); err != nil {
		t.Fatal(err)
	}
	if s.agentsStore, err = install.LoadMetadataWithMigration(agentsDir, install.MetadataKindAgent); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(agentsDir, "edited.md")
	if err := os.WriteFile(local, []byte("# agent\nmine\nmiddle\nlast\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(upstream, []byte("# agent\nfirst\nmiddle\ntheirs\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, remote, "commit", "-am", "v2")

	result := s.updateAgent("edited", false, true, true)
	if result.Action != "updated" || result.Message != "merged local changes" {
		t.Fatalf("got %q: %s, want a merged update", result.Action, result.Message)
	}
	if got, _ := os.ReadFile(local); string(got) != "# agent\nmine\nmiddle\ntheirs\n" {
		t.Errorf("agent = %q, want both edits", got)
	}
}
