package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/testutil"
)

func makeFollowedLink(t *testing.T, source, target, name string) *sourcewalk.FollowSet {
	t.Helper()
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(source, name)); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte(name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	return &set
}

func TestFollowedLinksStagedReachability(t *testing.T) {
	for _, layout := range []string{"skills", "root", "agents", "extras", "custom-root", "source-linked-out", "source-alias-in", "linked-parent-out"} {
		t.Run(layout, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "root")
			source := filepath.Join(root, "skills")
			staging := root
			want := true
			if err := os.MkdirAll(source, 0755); err != nil {
				t.Fatal(err)
			}
			switch layout {
			case "skills":
				staging = source
			case "agents", "extras":
				staging = filepath.Join(root, layout)
				want = false
			case "custom-root":
				staging = base
			case "source-linked-out":
				external := filepath.Join(base, "source-out")
				if err := os.Mkdir(external, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(source); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, source); err != nil {
					t.Skip(err)
				}
				want = false
			case "source-alias-in":
				alias := filepath.Join(base, "alias")
				if err := os.Symlink(source, alias); err != nil {
					t.Skip(err)
				}
				source = alias
			case "linked-parent-out":
				external := filepath.Join(base, "parent-out")
				if err := os.Mkdir(external, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(root, "parent")); err != nil {
					t.Skip(err)
				}
				source = filepath.Join(root, "parent", "skills")
				want = false
			}
			if err := os.MkdirAll(staging, 0755); err != nil {
				t.Fatal(err)
			}
			follow := makeFollowedLink(t, source, filepath.Join(t.TempDir(), "target"), "_dev")
			links, err := FollowedLinksStaged(staging, follow)
			if err != nil {
				t.Fatal(err)
			}
			if (len(links) == 1) != want {
				t.Fatalf("reachable links=%+v want=%v", links, want)
			}
		})
	}
}

func TestFollowedStagingIndexedAndIgnored(t *testing.T) {
	for _, condition := range []string{"unignored", "directory-pattern", "ignored", "indexed"} {
		t.Run(condition, func(t *testing.T) {
			source := t.TempDir()
			follow := makeFollowedLink(t, source, t.TempDir(), "_dev")
			testutil.RunGit(t, source, "init")
			if condition == "indexed" {
				testutil.RunGit(t, source, "add", "--", "_dev")
			}
			if condition != "unignored" {
				pattern := "/_dev\n"
				if condition == "directory-pattern" {
					pattern = "/_dev/\n"
				}
				if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte(pattern), 0644); err != nil {
					t.Fatal(err)
				}
			}
			err := StageAll(source, follow)
			if condition == "ignored" {
				if err != nil {
					t.Fatal(err)
				}
				if indexed, _ := IsPathIndexed(source, "_dev"); indexed {
					t.Fatal("ignored link was staged")
				}
			} else {
				var guard *FollowedLinksError
				if !errors.As(err, &guard) {
					t.Fatalf("expected refusal, got %v", err)
				}
				if !strings.Contains(err.Error(), "/_dev") || !strings.Contains(err.Error(), filepath.Join(source, ".gitignore")) {
					t.Fatalf("missing exact ignore guidance: %v", err)
				}
				if condition == "indexed" && !strings.Contains(err.Error(), "git rm --cached --") {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestFollowedIgnorePatternLiteral(t *testing.T) {
	for _, name := range []string{"_with spaces", "_a#b", "_a[b]", "_a*b", "_a?b"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testutil.RunGit(t, root, "init")
			if err := os.Symlink(t.TempDir(), filepath.Join(root, name)); err != nil {
				t.Skip(err)
			}
			line := install.FollowedIgnoreLine(name)
			if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(line+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
			ignored, err := IsPathIgnored(root, name)
			if err != nil || !ignored {
				t.Fatalf("literal ignore %q failed: %v", line, err)
			}
		})
	}
}

func TestDiscardKeepsIgnoredFollowedLink(t *testing.T) {
	source := t.TempDir()
	follow := makeFollowedLink(t, source, t.TempDir(), "_dev")
	testutil.RunGit(t, source, "init")
	testutil.ConfigureGitUser(t, source)
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("/_dev\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, source, "add", ".")
	testutil.RunGit(t, source, "commit", "-m", "initial")
	before, err := os.Readlink(filepath.Join(source, "_dev"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckFollowedLinks(source, follow); err != nil {
		t.Fatal(err)
	}
	if err := DiscardChanges(source, false); err != nil {
		t.Fatal(err)
	}
	after, err := os.Readlink(filepath.Join(source, "_dev"))
	if err != nil || after != before {
		t.Fatalf("discard changed link: %s %v", after, err)
	}
}

func TestUntrackCommandQuotesLiteralName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell execution test")
	}
	source := t.TempDir()
	testutil.RunGit(t, source, "init")
	name := "_$(touch marker)'quoted"
	if err := os.Symlink(t.TempDir(), filepath.Join(source, name)); err != nil {
		t.Fatal(err)
	}
	testutil.RunGit(t, source, "add", "--", name)
	cmd := exec.Command("sh", "-c", UntrackCommand("", name))
	cmd.Dir = source
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("literal untrack failed: %s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(source, "marker")); !os.IsNotExist(err) {
		t.Fatal("remediation command executed filename content")
	}
	if indexed, err := IsPathIndexed(source, name); err != nil || indexed {
		t.Fatalf("literal name remains indexed: %v", err)
	}
}

func TestFollowedLinksUnreadableDeclarationFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix permission enforcement")
	}
	source := t.TempDir()
	testutil.RunGit(t, source, "init")
	if err := os.Symlink(t.TempDir(), filepath.Join(source, "_dev")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".gitignore"), []byte("/.skillfollow.local\n"), 0644); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(source, ".skillfollow.local")
	if err := os.WriteFile(local, []byte("_dev\n"), 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(local, 0600) })
	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	if len(set.ParsedEntries()) != 0 || set.Err() == nil {
		t.Fatalf("fixture: parsed %v, err %v", set.ParsedEntries(), set.Err())
	}
	if links, err := FollowedLinksStaged(source, &set); err == nil {
		t.Fatalf("FollowedLinksStaged accepted an unread declaration: %+v", links)
	}
	if err := CheckFollowedLinks(source, &set); err == nil || !strings.Contains(err.Error(), ".skillfollow.local") {
		t.Fatalf("CheckFollowedLinks = %v", err)
	}
	if err := StageAll(source, &set); err == nil {
		t.Fatal("StageAll accepted an unread declaration")
	}
	if got := testutil.RunGit(t, source, "ls-files"); got != "" {
		t.Fatalf("guard staged files: %s", got)
	}
}
