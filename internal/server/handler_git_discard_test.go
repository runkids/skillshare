package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/testutil"
)

func discardRequest(s *Server, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/git/discard", strings.NewReader(body)))
	return rr
}

func TestHandleGitDiscard_Changes(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "discard", true: "dry run"}[dryRun], func(t *testing.T) {
			s, src := newTestServer(t)
			initServerGitRepo(t, src)
			write := func(name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"modified", "deleted", "renamed", ".gitignore"} {
				write(name, "ignored\n")
			}
			testutil.RunGit(t, src, "add", ".")
			testutil.RunGit(t, src, "commit", "-m", "baseline")
			head := testutil.RunGit(t, src, "rev-parse", "HEAD")
			write("modified", "staged")
			testutil.RunGit(t, src, "add", "modified")
			write("modified", "unstaged")
			if err := os.Remove(filepath.Join(src, "deleted")); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, src, "mv", "renamed", "new-name")
			write("added", "staged addition")
			testutil.RunGit(t, src, "add", "added")
			write("untracked", "new")
			write("ignored", "keep")
			before := testutil.RunGit(t, src, "status", "--porcelain")
			body := `{}`
			if dryRun {
				body = `{"dryRun":true}`
			}
			rr := discardRequest(s, body)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
			}
			after := testutil.RunGit(t, src, "status", "--porcelain")
			if dryRun {
				if after != before {
					t.Fatalf("dry run changed status: %q -> %q", before, after)
				}
				if content, err := os.ReadFile(filepath.Join(src, "modified")); err != nil || string(content) != "unstaged" {
					t.Fatalf("dry run changed file: %q, %v", content, err)
				}
			} else {
				if after != "" {
					t.Fatalf("working tree still dirty: %q", after)
				}
				for _, name := range []string{"modified", "deleted", "renamed"} {
					content, err := os.ReadFile(filepath.Join(src, name))
					if err != nil || string(content) != "ignored\n" {
						t.Fatalf("%s not restored: %q, %v", name, content, err)
					}
				}
				for _, name := range []string{"added", "untracked", "new-name"} {
					if _, err := os.Stat(filepath.Join(src, name)); !os.IsNotExist(err) {
						t.Fatalf("%s was not removed: %v", name, err)
					}
				}
			}
			if got := testutil.RunGit(t, src, "rev-parse", "HEAD"); got != head {
				t.Fatalf("HEAD changed: %s -> %s", head, got)
			}
			if content, err := os.ReadFile(filepath.Join(src, "ignored")); err != nil || string(content) != "keep" {
				t.Fatalf("ignored file changed: %q, %v", content, err)
			}
		})
	}
}

func TestHandleGitDiscard_EmptyCommitAndCleanTree(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addSkill(t, src, "untracked-skill")
	for i := 0; i < 2; i++ {
		rr := discardRequest(s, `{}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		if got := testutil.RunGit(t, src, "status", "--porcelain"); got != "" {
			t.Fatalf("working tree still dirty: %s", got)
		}
	}
}

func TestHandleGitDiscard_KeepsLocallyIgnoredFiles(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	ignorePath := filepath.Join(src, ".gitignore")
	if err := os.WriteFile(ignorePath, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, src, "add", ".gitignore")
	testutil.RunGit(t, src, "commit", "-m", "ignore baseline")
	if err := os.WriteFile(ignorePath, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep"), []byte("local ignored file"), 0o644); err != nil {
		t.Fatal(err)
	}
	rr := discardRequest(s, `{}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	if content, err := os.ReadFile(filepath.Join(src, "keep")); err != nil || string(content) != "local ignored file" {
		t.Fatalf("ignored file lost: %q, %v", content, err)
	}
	if content, err := os.ReadFile(ignorePath); err != nil || string(content) != "\n" {
		t.Fatalf(".gitignore not restored: %q, %v", content, err)
	}
}

func TestHandleGitDiscard_RootPreservesConfigAndNestedRepo(t *testing.T) {
	s, src := newTestServer(t)
	setServerGitRoot(t, "root", src)
	root := config.BaseDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	src = filepath.Join(root, "skills")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	setServerGitRoot(t, "root", src)
	initServerGitRepo(t, root)
	testutil.RunGit(t, root, "add", "config.yaml")
	testutil.RunGit(t, root, "commit", "-m", "tracked config")
	setServerGitRoot(t, "root", src)
	f, err := os.OpenFile(config.ConfigPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("# keep local config\n")
	_ = f.Close()
	before, _ := os.ReadFile(config.ConfigPath())
	initServerGitRepo(t, src)
	addSkill(t, src, "nested-skill")
	rr := discardRequest(s, `{}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	after, err := os.ReadFile(config.ConfigPath())
	if err != nil || string(before) != string(after) {
		t.Fatalf("config changed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(src, "nested-skill", "SKILL.md")); err != nil {
		t.Fatalf("nested repo file lost: %v", err)
	}
}

func TestHandleGitDiscard_RejectsUnsafeSource(t *testing.T) {
	for _, scenario := range []string{"not a repo", "ancestor repo", "no commits", "project mode", "invalid scope", "invalid JSON"} {
		t.Run(scenario, func(t *testing.T) {
			s, src := newTestServer(t)
			body := `{}`
			switch scenario {
			case "ancestor repo":
				initServerGitRepo(t, filepath.Dir(src))
			case "no commits":
				testutil.RunGit(t, src, "init")
			case "project mode":
				s, _ = newProjectTargetServer(t, nil)
			case "invalid scope":
				setServerGitRoot(t, "invalid", src)
			case "invalid JSON":
				body = `{`
			}
			addSkill(t, src, "keep-skill")
			rr := discardRequest(s, body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
			}
			if _, err := os.Stat(filepath.Join(src, "keep-skill", "SKILL.md")); err != nil {
				t.Fatalf("file lost after rejected request: %v", err)
			}
		})
	}
}
