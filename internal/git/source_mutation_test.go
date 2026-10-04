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

func TestSourceMutationRefusesPathsBelowMissingDeclaration(t *testing.T) {
	for _, c := range []struct {
		name, prefix string
		existing     string // "", "link", "file", "indexed-file" or "dir" at the declared path
		incoming     []string
		refusal      string
	}{
		{name: "missing", incoming: []string{"group/a/SKILL.md"}, refusal: `is inside declared entry "group"`},
		{name: "nested-source", prefix: "skills/", incoming: []string{"skills/group/a/SKILL.md"}, refusal: `is inside declared entry "group"`},
		{name: "live-ignored", existing: "link", incoming: []string{"group/a/SKILL.md"}, refusal: `touches link "group"`},
		// A regular file is an invalid target, not an intentional real directory,
		// so the declared prefix still guards it against becoming a directory.
		{name: "regular-file", existing: "file", incoming: []string{"group/a/SKILL.md"}, refusal: `is inside declared entry "group"`},
		{name: "indexed-file", existing: "indexed-file", incoming: []string{"group/a/SKILL.md"}, refusal: `declared entry "group" is indexed`},
		{name: "real-directory", existing: "dir", incoming: []string{"group/a/SKILL.md"}},
		{name: "sibling", incoming: []string{"group-other/a/SKILL.md", "groupx/a/SKILL.md"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{
				"README.md": "# Source\n", c.prefix + ".gitignore": "/group\n", c.prefix + ".skillfollow": "group\n",
			})
			root := filepath.Join(base, "source")
			testutil.RunGit(t, "", "clone", remote, root)
			source := filepath.Join(root, filepath.FromSlash(c.prefix))
			entry := filepath.Join(source, "group")
			switch c.existing {
			case "link":
				if err := os.Symlink(t.TempDir(), entry); err != nil {
					t.Skip(err)
				}
			case "file", "indexed-file":
				if err := os.WriteFile(entry, []byte("user file\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if c.existing == "indexed-file" {
					testutil.ConfigureGitUser(t, root)
					testutil.RunGit(t, root, "add", "-f", "--", filepath.Join(filepath.FromSlash(c.prefix), "group"))
					testutil.RunGit(t, root, "commit", "-m", "indexed file")
				}
			case "dir":
				if err := os.Mkdir(entry, 0755); err != nil {
					t.Fatal(err)
				}
			}
			seed := filepath.Join(base, "seed-main")
			for _, rel := range c.incoming {
				path := filepath.Join(seed, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("# Remote\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			testutil.RunGit(t, seed, append([]string{"add", "-f", "--"}, c.incoming...)...)
			testutil.RunGit(t, seed, "commit", "-m", "incoming")
			testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
			if err := Fetch(root); err != nil {
				t.Fatal(err)
			}
			before := testutil.RunGit(t, root, "rev-parse", "HEAD")
			follow := sourcewalk.Follow(source, sourcewalk.FollowOptions{GitRoot: root})
			_, err := CheckSourceMutation(source, "origin/main", &follow)
			if c.refusal == "" {
				if err != nil {
					t.Fatalf("path outside a guarded entry was refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.refusal) {
				t.Fatalf("err = %v, want %q", err, c.refusal)
			}
			if got := testutil.RunGit(t, root, "rev-parse", "HEAD"); got != before {
				t.Fatalf("HEAD changed: %s", got)
			}
			if c.existing == "file" || c.existing == "indexed-file" {
				if data, err := os.ReadFile(entry); err != nil || string(data) != "user file\n" {
					t.Fatalf("declared file changed: %q %v", data, err)
				}
			} else if info, err := os.Lstat(entry); (c.existing == "link") != (err == nil) || (c.existing == "link" && info.Mode()&os.ModeSymlink == 0) {
				t.Fatalf("declared entry changed: %v %v", info, err)
			}
		})
	}
}
