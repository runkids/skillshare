package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestGitUpdateCandidatePreservesLocalChanges(t *testing.T) {
	for _, autocrlf := range []string{"false", "true"} {
		t.Run("autocrlf="+autocrlf, func(t *testing.T) {
			for _, worktree := range []string{"unstaged notes\n", "original notes\n"} {
				t.Run(strings.TrimSpace(worktree), func(t *testing.T) {
					testGitUpdateCandidatePreservesLocalChanges(t, autocrlf, worktree)
				})
			}
		})
	}
}

func testGitUpdateCandidatePreservesLocalChanges(t *testing.T, autocrlf, worktree string) {
	t.Helper()
	_, _, repo, _, _ := setupInstallUpdateFollowedCheckout(t)
	testutil.RunGit(t, repo, "config", "core.autocrlf", autocrlf)
	testutil.RunGit(t, repo, "reset", "--hard", "HEAD")
	// The legacy install-update path allows a compatible tracked edit.
	notes := filepath.Join(repo, "notes.txt")
	if err := os.WriteFile(notes, []byte("staged notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "staged-only.txt"), []byte("staged only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, repo, "add", "notes.txt", "staged-only.txt")
	if err := os.WriteFile(notes, []byte(worktree), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("untracked notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := testutil.RunGit(t, repo, "status", "--porcelain")
	candidate, err := StageGitUpdate(repo, false)
	if err != nil {
		t.Fatal(err)
	}
	defer candidate.Close()
	for _, name := range []string{"notes.txt", "staged-only.txt", "untracked.txt"} {
		want, _ := os.ReadFile(filepath.Join(repo, name))
		got, err := os.ReadFile(filepath.Join(candidate.Path, name))
		if err != nil || string(got) != string(want) {
			t.Errorf("candidate lost %s: %q, %v", name, got, err)
		}
	}
	if got := testutil.RunGit(t, candidate.Path, "show", ":notes.txt"); got != "staged notes" {
		t.Errorf("candidate lost index contents: %q", got)
	}
	// Use a fast-forward candidate; git itself refuses divergent merges with
	// staged local changes, just as it did before staging was introduced.
	if err := gitPull(candidate.Path, nil); err != nil {
		t.Fatal(err)
	}
	revision, err := getGitFullHash(candidate.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Publish(revision); err != nil {
		t.Fatal(err)
	}
	if got := testutil.RunGit(t, repo, "status", "--porcelain"); got != before {
		t.Errorf("publication changed local state: %q, want %q", got, before)
	}
	if got, _ := os.ReadFile(notes); string(got) != worktree {
		t.Errorf("publication lost local edit: %q", got)
	}
	candidate.Close()
	if _, err := os.Stat(candidate.tempDir); !os.IsNotExist(err) {
		t.Errorf("candidate was not removed: %v", err)
	}
}

func TestGitUpdateCandidateFailedPublicationPreservesCheckout(t *testing.T) {
	if os.Getuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("test requires POSIX directory permissions and non-root user")
	}
	for _, mode := range []string{"clean", "local-edits", "force", "force-readonly-local-edits", "force-deleted", "force-renamed", "force-sparse", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{
				"a.txt": "a1\n", "a[1].txt": "a1\n", "sub/b.txt": "b1\n", "notes.txt": "notes\n", "excluded/c.txt": "c1\n",
			})
			repo, seed := filepath.Join(base, "repo"), filepath.Join(base, "seed-main")
			testutil.RunGit(t, "", "clone", remote, repo)
			for name, content := range map[string]string{"a.txt": "a2\n", "a[1].txt": "a2\n", "added.txt": "new\n", "added[1].txt": "new\n", "sub/b.txt": "b2\n", "excluded/c.txt": "c2\n"} {
				if err := os.WriteFile(filepath.Join(seed, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "symlink" {
				if err := os.Remove(filepath.Join(seed, "a.txt")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("a[1].txt", filepath.Join(seed, "a.txt")); err != nil {
					t.Skip(err)
				}
			}
			testutil.RunGit(t, seed, "add", "-A")
			testutil.RunGit(t, seed, "commit", "-m", "remote changes")
			testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
			force := strings.HasPrefix(mode, "force")
			if force {
				paths := []string{"a.txt"}
				if mode == "force-readonly-local-edits" {
					paths = append(paths, "sub/b.txt")
				}
				for _, name := range paths {
					if err := os.WriteFile(filepath.Join(repo, name), []byte("my staged edit\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					testutil.RunGit(t, repo, "add", name)
					if err := os.WriteFile(filepath.Join(repo, name), []byte("my unstaged edit\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "force-deleted" {
				testutil.RunGit(t, repo, "rm", "-f", "a.txt")
			} else if mode == "force-renamed" {
				testutil.RunGit(t, repo, "mv", "a.txt", "renamed.txt")
			}
			if mode != "clean" {
				if err := os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("my local notes\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "force-sparse" {
				testutil.RunGit(t, repo, "sparse-checkout", "set", "sub")
			}
			beforeIndex := testutil.RunGit(t, repo, "ls-files", "-t")
			before, err := gitCandidateState(repo)
			if err != nil {
				t.Fatal(err)
			}
			candidate, err := StageGitUpdate(repo, force)
			if err != nil {
				t.Fatal(err)
			}
			defer candidate.Close()
			if err := gitPull(candidate.Path, nil); err != nil {
				t.Fatal(err)
			}
			revision := testutil.RunGit(t, candidate.Path, "rev-parse", "HEAD")
			sub := filepath.Join(repo, "sub")
			if err := os.Chmod(sub, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
			if err := candidate.Publish(revision); err == nil {
				t.Fatal("publication should fail on the read-only directory")
			}
			if after, err := gitCandidateState(repo); err != nil || after != before {
				t.Fatalf("partial publication changed checkout: %q, want %q (%v)", after, before, err)
			}
			if afterIndex := testutil.RunGit(t, repo, "ls-files", "-t"); afterIndex != beforeIndex {
				t.Errorf("publication changed index flags: %q, want %q", afterIndex, beforeIndex)
			}
			if mode == "force-sparse" {
				if _, err := os.Lstat(filepath.Join(repo, "excluded/c.txt")); !os.IsNotExist(err) {
					t.Errorf("publication materialized sparse-excluded file: %v", err)
				}
			}
			if head := testutil.RunGit(t, repo, "rev-parse", "HEAD"); head != candidate.BeforeHash {
				t.Fatalf("failed publication moved HEAD to %s", head)
			}
		})
	}
}

func TestInstallTrackedUpdatePublishesExactCandidate(t *testing.T) {
	source, _, repo, remote, _ := setupInstallUpdateFollowedCheckout(t)
	approved := testutil.RunGit(t, filepath.Join(filepath.Dir(repo), "seed-main"), "rev-parse", "HEAD")
	barrier := false
	result := &TrackedRepoResult{}
	_, err := updateTrackedRepo(repo, result, InstallOptions{
		SourceDir: source, AuditOverride: true,
		OnProgress: func(message string) {
			if message != "Auditing update candidate..." {
				return
			}
			barrier = true
			seed := filepath.Join(filepath.Dir(repo), "seed-main")
			if err := os.WriteFile(filepath.Join(seed, "SKILL.md"), []byte("Later unscanned revision\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			testutil.RunGit(t, seed, "add", "SKILL.md")
			testutil.RunGit(t, seed, "commit", "-m", "advance upstream")
			testutil.RunGit(t, seed, "push", remote, "HEAD:main")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !barrier {
		t.Fatal("audit barrier was not reached")
	}
	if got := testutil.RunGit(t, repo, "rev-parse", "HEAD"); got != approved {
		t.Errorf("published %s, want audited %s", got, approved)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, "SKILL.md")); strings.Contains(string(got), "Later unscanned") {
		t.Errorf("published unscanned upstream content: %q", got)
	}
}
