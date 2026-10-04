//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// followEnv is a global or project skills source inside a Git work tree,
// with an external directory to follow and one active skills target.
type followEnv struct {
	sb       *testutil.Sandbox
	project  bool
	root     string // project root, or the source in global mode
	source   string
	external string
	target   string
}

func newFollowEnv(t *testing.T, project bool) *followEnv {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	e := &followEnv{sb: sb, project: project, external: filepath.Join(sb.Root, "external")}
	if project {
		e.root = sb.SetupProjectDir("claude")
		e.source = filepath.Join(e.root, ".skillshare", "skills")
		e.target = filepath.Join(e.root, ".claude", "skills")
	} else {
		e.root = sb.SourcePath
		e.source = sb.SourcePath
		e.target = filepath.Join(sb.Home, ".claude", "skills")
		sb.WriteConfig("source: " + e.source + "\nmode: merge\ntargets:\n  claude:\n    path: " + e.target + "\n")
	}
	for _, dir := range []string{e.source, e.target, filepath.Join(e.external, "a")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	sb.WriteFile(filepath.Join(e.external, "a", "SKILL.md"), "---\nname: a\ndescription: Fixture\n---\n# A\n")
	testutil.RunGit(t, e.root, "init", "-q")
	return e
}

func (e *followEnv) run(args ...string) *testutil.Result {
	if e.project {
		return e.sb.RunCLIInDir(e.root, append(args, "-p")...)
	}
	return e.sb.RunCLI(append(args, "-g")...)
}

func (e *followEnv) read(name string) string {
	data, err := os.ReadFile(filepath.Join(e.source, name))
	if err != nil && !os.IsNotExist(err) {
		e.sb.T.Fatal(err)
	}
	return string(data)
}

func (e *followEnv) write(name, content string) {
	e.sb.WriteFile(filepath.Join(e.source, name), content)
}

func (e *followEnv) link(name string) {
	e.sb.T.Helper()
	if err := os.Symlink(e.external, filepath.Join(e.source, name)); err != nil {
		e.sb.T.Fatal(err)
	}
}

func forBothModes(t *testing.T, test func(t *testing.T, e *followEnv)) {
	for _, project := range []bool{false, true} {
		name := "global"
		if project {
			name = "project"
		}
		t.Run(name, func(t *testing.T) { test(t, newFollowEnv(t, project)) })
	}
}

func TestFollow_ToCreatesLinkDeclaresAndIgnores(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.write(".skillfollow", "# team links\ngroup\n")
		result := e.run("follow", "_dev", "--to", e.external)
		result.AssertSuccess(t)
		result.AssertRowContains(t, "_dev", "linked to "+e.external)
		result.AssertRowContains(t, "_dev", "added to .skillfollow")
		result.AssertRowContains(t, ".gitignore", "added /_dev")
		result.AssertRowContains(t, "_dev", "followed")
		result.AssertOutputContains(t, "skillshare sync")

		if got := e.sb.SymlinkTarget(filepath.Join(e.source, "_dev")); got != e.external {
			t.Fatalf("link target = %q, want %q", got, e.external)
		}
		if got := e.read(".skillfollow"); got != "# team links\ngroup\n_dev\n" {
			t.Fatalf(".skillfollow = %q", got)
		}
		if got := e.read(".gitignore"); !strings.Contains(got, "\n/_dev\n") {
			t.Fatalf(".gitignore = %q", got)
		}
		if out := testutil.RunGit(t, e.source, "check-ignore", "_dev"); out != "_dev" {
			t.Fatalf("git does not ignore the link: %q", out)
		}
	})
}

func TestFollow_ExistingLinkIsIdempotent(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.link("group")
		e.run("follow", "group").AssertSuccess(t)
		before := e.read(".gitignore")

		result := e.run("follow", "group")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "group", "already in .skillfollow")
		result.AssertOutputNotContains(t, "skillshare sync")
		if got := e.read(".skillfollow"); got != "group\n" {
			t.Fatalf(".skillfollow = %q", got)
		}
		if got := e.read(".gitignore"); got != before {
			t.Fatalf(".gitignore changed:\n%s", got)
		}
	})
}

func TestFollow_LocalWritesLocalFileAndIgnore(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.link("_dev")
		result := e.run("follow", "_dev", "--local")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "_dev", "added to .skillfollow.local")
		result.AssertRowContains(t, ".gitignore", "added /.skillfollow.local")
		if got := e.read(".skillfollow.local"); got != "_dev\n" {
			t.Fatalf(".skillfollow.local = %q", got)
		}
		if e.sb.FileExists(filepath.Join(e.source, ".skillfollow")) {
			t.Fatal("--local wrote .skillfollow")
		}
		testutil.RunGit(t, e.source, "check-ignore", ".skillfollow.local")
	})
}

func TestFollow_PrintsRejectedState(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		result := e.run("follow", "out", "--to", filepath.Dir(e.target))
		result.AssertSuccess(t)
		result.AssertRowContains(t, "out", "target-overlap — target overlaps active skills target")

		jsonResult := e.run("follow", "out", "--json")
		jsonResult.AssertSuccess(t)
		var out struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
			Added  bool   `json:"added"`
		}
		if err := json.Unmarshal([]byte(jsonResult.Stdout), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, jsonResult.Stdout)
		}
		if out.State != "target-overlap" || out.Added || !strings.Contains(out.Reason, "active skills target") {
			t.Fatalf("unexpected JSON: %s", jsonResult.Stdout)
		}
		jsonResult.AssertOutputContains(t, `"ignore_lines_added": []`)
	})
}

func TestFollow_IndexedLinkIsNotUntracked(t *testing.T) {
	e := newFollowEnv(t, false)
	e.link("_dev")
	testutil.RunGit(t, e.source, "add", "_dev")

	result := e.run("follow", "_dev")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "_dev", "indexed in Git; run git -C '"+e.source+"' rm --cached -- '_dev'")
	if out := testutil.RunGit(t, e.source, "ls-files", "_dev"); out != "_dev" {
		t.Fatalf("follow untracked the link: %q", out)
	}
}

func TestFollow_Refusals(t *testing.T) {
	e := newFollowEnv(t, false)
	e.link("_dev")
	if err := os.MkdirAll(filepath.Join(e.source, "inner"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"path name":       {[]string{"follow", "a/b"}, "not a first-level entry name"},
		"glob name":       {[]string{"follow", "dev*"}, "not a first-level entry name"},
		"missing entry":   {[]string{"follow", "nope"}, "pass --to <dir> to create the link"},
		"other link":      {[]string{"follow", "_dev", "--to", e.sb.Home}, "already exists and does not link to"},
		"inside source":   {[]string{"follow", "x", "--to", filepath.Join(e.source, "inner")}, "is inside the skills source"},
		"missing --to":    {[]string{"follow", "x", "--to", filepath.Join(e.sb.Root, "absent")}, "--to:"},
		"unfollow a path": {[]string{"unfollow", "../x"}, "not a first-level entry name"},
	} {
		t.Run(name, func(t *testing.T) {
			result := e.run(tc.args...)
			result.AssertFailure(t)
			result.AssertAnyOutputContains(t, tc.want)
		})
	}
	if e.sb.FileExists(filepath.Join(e.source, ".skillfollow")) {
		t.Fatal("a refused follow wrote .skillfollow")
	}
}

func TestUnfollow_RemovesLinkAndEveryDeclaration(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.run("follow", "_dev", "--to", e.external).AssertSuccess(t)
		e.write(".skillfollow", "# keep me\n_dev\ngroup\n")
		e.write(".skillfollow.local", "_dev\n# local note\n")

		result := e.run("unfollow", "_dev")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "_dev", "removed from .skillfollow")
		result.AssertRowContains(t, "_dev", "removed from .skillfollow.local")
		result.AssertRowContains(t, "_dev", "link removed; its target was not touched")
		result.AssertRowContains(t, ".gitignore", "removed /_dev")
		result.AssertOutputContains(t, "prune the entry's managed links")

		if got := e.read(".skillfollow"); got != "# keep me\ngroup\n" {
			t.Fatalf(".skillfollow = %q", got)
		}
		if got := e.read(".skillfollow.local"); got != "# local note\n" {
			t.Fatalf(".skillfollow.local = %q", got)
		}
		if _, err := os.Lstat(filepath.Join(e.source, "_dev")); !os.IsNotExist(err) {
			t.Fatalf("link still exists: %v", err)
		}
		if !e.sb.FileExists(filepath.Join(e.external, "a", "SKILL.md")) {
			t.Fatal("unfollow touched the link target")
		}
		if strings.Contains(e.read(".gitignore"), "/_dev") {
			t.Fatalf("ignore line kept: %q", e.read(".gitignore"))
		}
	})
}

func TestUnfollow_KeepLink(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.run("follow", "_dev", "--to", e.external).AssertSuccess(t)
		result := e.run("unfollow", "_dev", "--keep-link")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "_dev", "link kept: --keep-link")
		result.AssertRowContains(t, ".gitignore", "kept /_dev")
		if !e.sb.IsSymlink(filepath.Join(e.source, "_dev")) {
			t.Fatal("--keep-link removed the link")
		}
		if !strings.Contains(e.read(".gitignore"), "/_dev") {
			t.Fatal("--keep-link removed the ignore line")
		}
	})
}

func TestUnfollow_LocalReportsStillFollowed(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.link("_dev")
		e.write(".skillfollow", "_dev\n")
		e.write(".skillfollow.local", "_dev\n")

		result := e.run("unfollow", "_dev", "--local")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "_dev", "removed from .skillfollow.local")
		result.AssertRowContains(t, "_dev", "still declared in .skillfollow; it remains followed")
		result.AssertOutputNotContains(t, "prune the entry's managed links")
		if got := e.read(".skillfollow"); got != "_dev\n" {
			t.Fatalf("--local edited .skillfollow: %q", got)
		}
		if !e.sb.IsSymlink(filepath.Join(e.source, "_dev")) {
			t.Fatal("--local removed a link that is still declared")
		}
	})
}

func TestUnfollow_RealDirectoryIsKept(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.sb.WriteFile(filepath.Join(e.source, "real", "s", "SKILL.md"), "---\nname: s\ndescription: Fixture\n---\n")
		result := e.run("follow", "real")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "real", "not-link")

		result = e.run("unfollow", "real")
		result.AssertSuccess(t)
		result.AssertRowContains(t, "real", "link kept: not a link")
		if !e.sb.FileExists(filepath.Join(e.source, "real", "s", "SKILL.md")) {
			t.Fatal("unfollow removed a real directory")
		}
	})
}

func TestUnfollow_JSON(t *testing.T) {
	forBothModes(t, func(t *testing.T, e *followEnv) {
		e.run("follow", "_dev", "--to", e.external, "--local").AssertSuccess(t)
		result := e.run("unfollow", "_dev", "--json")
		result.AssertSuccess(t)
		var out struct {
			FilesEdited       []string `json:"files_edited"`
			StillDeclaredIn   []string `json:"still_declared_in"`
			LinkRemoved       bool     `json:"link_removed"`
			IgnoreLineRemoved bool     `json:"ignore_line_removed"`
		}
		if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, result.Stdout)
		}
		if len(out.FilesEdited) != 1 || out.FilesEdited[0] != ".skillfollow.local" || len(out.StillDeclaredIn) != 0 || !out.LinkRemoved || !out.IgnoreLineRemoved {
			t.Fatalf("unexpected JSON: %s", result.Stdout)
		}
		result.AssertOutputContains(t, `"still_declared_in": []`)
		again := e.run("unfollow", "_dev", "--json")
		again.AssertSuccess(t)
		again.AssertOutputContains(t, `"files_edited": []`)

		failed := e.run("unfollow", "a/b", "--json")
		failed.AssertFailure(t)
		if !strings.Contains(failed.Stdout, `"error"`) {
			t.Fatalf("missing JSON error: %s", failed.Stdout)
		}
	})
}

// A refused declaration write must remove the link follow --to just made.
func TestFollow_ToRemovesNewLinkWhenDeclarationFails(t *testing.T) {
	e := newFollowEnv(t, false)
	outside := filepath.Join(e.sb.Root, "decl")
	e.sb.WriteFile(outside, "# team\n")
	if err := os.Symlink(outside, filepath.Join(e.source, ".skillfollow")); err != nil {
		t.Fatal(err)
	}

	result := e.run("follow", "x", "--to", e.external, "--json")
	result.AssertFailure(t)
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || !strings.Contains(out.Error, "failed to update .skillfollow") {
		t.Fatalf("unexpected JSON error (%v): %s", err, result.Stdout)
	}
	if _, err := os.Lstat(filepath.Join(e.source, "x")); !os.IsNotExist(err) {
		t.Fatalf("the new link was left in place: %v", err)
	}
	if got := e.sb.ReadFile(outside); got != "# team\n" {
		t.Fatalf("write went through the link: %q", got)
	}
}

// A refused second write must fail the command without claiming an unfollow.
func TestUnfollow_PartialWriteFailureReportsFailure(t *testing.T) {
	e := newFollowEnv(t, false)
	e.link("_dev")
	e.write(".skillfollow", "_dev\n")
	outside := filepath.Join(e.sb.Root, "local-decl")
	e.sb.WriteFile(outside, "_dev\n")
	if err := os.Symlink(outside, filepath.Join(e.source, ".skillfollow.local")); err != nil {
		t.Fatal(err)
	}

	result := e.run("unfollow", "_dev")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "failed to update .skillfollow.local")
	result.AssertAnyOutputContains(t, ".skillfollow was already edited, and _dev is still declared")
	result.AssertOutputNotContains(t, "link removed")
	if !e.sb.IsSymlink(filepath.Join(e.source, "_dev")) {
		t.Fatal("link removed after a failed write")
	}
	if got := e.sb.ReadFile(outside); got != "_dev\n" {
		t.Fatalf("write went through the link: %q", got)
	}
}
