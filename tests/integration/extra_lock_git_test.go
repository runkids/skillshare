//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestExtraRootLock_CommitExcludesOnlyKnownEmptyRootFile(t *testing.T) {
	requireWorkingGit(t)
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	// A pathspec metacharacter must remain an exact path, not a glob.
	extras := filepath.Join(base, "custom-extras[1]")
	for _, dir := range []string{skills, extras} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\n  extras: " + extras + "\ntargets: {}\n")
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n.DS_Store\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	lock := filepath.Join(extras, ".skillshare-extras.lock")
	grWrite(t, lock, "")
	before := testutil.RunGit(t, base, "rev-parse", "HEAD")
	preview := sb.RunCLI("commit", "--dry-run", "-m", "nothing but admin lock")
	preview.AssertSuccess(t)
	preview.AssertOutputContains(t, "Nothing to commit")
	sb.RunCLI("commit", "-m", "nothing but admin lock").AssertSuccess(t)
	if after := testutil.RunGit(t, base, "rev-parse", "HEAD"); after != before {
		t.Error("empty administrative lock created a commit")
	}
	controls := []string{"elsewhere/.skillshare-extras.lock", "custom-extras[1]/team/.skillshare-extras.lock", "custom-extras1/.skillshare-extras.lock", "note.md"}
	for _, rel := range controls {
		grWrite(t, filepath.Join(base, filepath.FromSlash(rel)), "")
	}
	sb.RunCLI("commit", "-m", "preserve same-name upstream files").AssertSuccess(t)
	tree := strings.Split(testutil.RunGit(t, base, "ls-tree", "-r", "--name-only", "HEAD"), "\n")
	for _, rel := range controls {
		found := false
		for _, file := range tree {
			found = found || file == rel
		}
		if !found {
			t.Errorf("same-name ordinary content was hidden from staging: %s, tree=%v", rel, tree)
		}
	}
	for _, file := range tree {
		if file == "custom-extras[1]/.skillshare-extras.lock" {
			t.Error("known empty admin lock was staged")
		}
	}
	// A nonempty file is not the administrative lock, even at the exact path.
	grWrite(t, lock, "upstream content must remain visible\n")
	sb.RunCLI("commit", "-m", "nonempty same-name content").AssertSuccess(t)
	if got := testutil.RunGit(t, base, "show", "HEAD:custom-extras[1]/.skillshare-extras.lock"); got != "upstream content must remain visible" {
		t.Errorf("nonempty exact-path file not tracked: %q", got)
	}
	ignore, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil || string(ignore) != "config.yaml\n.DS_Store\n" {
		t.Errorf("transient filtering changed ignore config: %q %v", ignore, err)
	}
}
