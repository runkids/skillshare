package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/testutil"
)

type sourceMutationFixture struct {
	source, target, remote, seed, name, commit, before string
	follow                                             *sourcewalk.FollowSet
}

func newSourceMutationFixture(t *testing.T, condition string) sourceMutationFixture {
	t.Helper()
	base := t.TempDir()
	remote := testutil.SetupBareRemoteRepo(t, base)
	name := "_dev"
	files := map[string]string{"README.md": "# Source\n", ".gitignore": "/_dev\n"}
	if condition != "undeclared" {
		files[".skillfollow"] = "_dev\n"
	}
	testutil.SeedRemoteBranch(t, base, remote, "main", files)
	source := filepath.Join(base, "source")
	testutil.RunGit(t, "", "clone", remote, source)
	testutil.ConfigureGitUser(t, source)
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "sentinel"), []byte("user owned"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(source, name)); err != nil {
		t.Skip(err)
	}
	if condition == "indexed" {
		testutil.RunGit(t, source, "add", "-f", "--", name)
		testutil.RunGit(t, source, "commit", "-m", "indexed link")
	}
	seed := filepath.Join(base, "seed-main")
	rel := "_dev/incoming\nfile.txt"
	if condition == "indexed" {
		rel = "safe.txt"
	}
	path := filepath.Join(seed, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("remote content"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, seed, "add", "-f", "--", rel)
	testutil.RunGit(t, seed, "commit", "-m", "incoming")
	testutil.RunGit(t, seed, "push", "origin", "HEAD:main", "HEAD:incoming")
	follow := sourcewalk.Follow(source, sourcewalk.FollowOptions{GitRoot: source})
	return sourceMutationFixture{source, target, remote, seed, name, testutil.RunGit(t, seed, "rev-parse", "HEAD"), testutil.RunGit(t, source, "rev-parse", "HEAD"), &follow}
}

func (f sourceMutationFixture) assertKept(t *testing.T) {
	t.Helper()
	if link, err := os.Readlink(filepath.Join(f.source, f.name)); err != nil || link != f.target {
		t.Fatalf("link changed: %s %v", link, err)
	}
	if got := testutil.RunGit(t, f.source, "rev-parse", "HEAD"); got != f.before {
		t.Fatalf("HEAD changed: %s", got)
	}
	data, err := os.ReadFile(filepath.Join(f.target, "sentinel"))
	if err != nil || string(data) != "user owned" {
		t.Fatal("external content changed")
	}
}

func TestSourceMutationRefusesLinks(t *testing.T) {
	for _, route := range []string{"pull", "resolution", "first-pull", "checkout"} {
		for _, condition := range []string{"indexed", "declared", "undeclared"} {
			t.Run(route+"/"+condition, func(t *testing.T) {
				f := newSourceMutationFixture(t, condition)
				var err error
				switch route {
				case "pull":
					_, err = PullWithEnv(f.source, nil, f.follow)
				case "resolution":
					_, err = PullWithResolution(f.source, nil, f.follow)
				case "first-pull":
					testutil.RunGit(t, f.source, "branch", "--unset-upstream")
					_, err = FirstPull(f.source, true, f.follow)
				case "checkout":
					if err = Fetch(f.source); err == nil {
						err = Checkout(f.source, "incoming", f.follow)
					}
				}
				if err == nil || !strings.Contains(err.Error(), f.commit) || !strings.Contains(err.Error(), f.name) || !strings.Contains(err.Error(), "git rm --cached") {
					t.Fatalf("missing refusal guidance: %v", err)
				}
				if condition != "indexed" && !strings.Contains(err.Error(), `incoming\nfile.txt`) {
					t.Fatalf("NUL path was not preserved: %v", err)
				}
				f.assertKept(t)
			})
		}
	}
}

func TestSourceMutationSafePullKeepsLink(t *testing.T) {
	f := newSourceMutationFixture(t, "indexed")
	// The remote only changes safe.txt; an unindexed ignored link is safe.
	testutil.RunGit(t, f.source, "rm", "--cached", "--", f.name)
	testutil.RunGit(t, f.source, "commit", "-m", "untrack link")
	if _, err := PullWithResolution(f.source, nil, f.follow); err != nil {
		t.Fatal(err)
	}
	if link, err := os.Readlink(filepath.Join(f.source, f.name)); err != nil || link != f.target {
		t.Fatalf("safe pull changed link: %s %v", link, err)
	}
}

func TestSourceMutationRefusesMissingIndexedDeclaration(t *testing.T) {
	f := newSourceMutationFixture(t, "indexed")
	if err := os.Remove(filepath.Join(f.source, f.name)); err != nil {
		t.Fatal(err)
	}
	if err := Fetch(f.source); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckSourceMutation(f.source, "origin/main", f.follow); err == nil || !strings.Contains(err.Error(), "is indexed") {
		t.Fatalf("missing indexed declaration was accepted: %v", err)
	}
}
