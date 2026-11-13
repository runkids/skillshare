//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// lockRemote seeds a bare repo with one skill in a subdirectory and returns
// its source string plus a function that pushes a new version of the skill.
func lockRemote(t *testing.T, sb *testutil.Sandbox) (source string, bump func(body string) string) {
	t.Helper()
	remote := testutil.SetupBareRemoteRepo(t, sb.Root)
	testutil.SeedRemoteBranch(t, sb.Root, remote, "main", map[string]string{
		"skills/demo/SKILL.md":  "---\nname: demo\n---\n# v1\n",
		"skills/other/SKILL.md": "---\nname: other\n---\n# other\n",
	})
	seed := filepath.Join(sb.Root, "seed-main")
	bump = func(body string) string {
		if err := os.WriteFile(filepath.Join(seed, "skills/demo/SKILL.md"), []byte("---\nname: demo\n---\n# "+body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		testutil.RunGit(t, seed, "commit", "-am", body)
		testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
		return strings.TrimSpace(testutil.RunGit(t, seed, "rev-parse", "HEAD"))
	}
	return "file://" + remote + "//skills/demo", bump
}

func readLock(t *testing.T, project string) install.Lock {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".skillshare", install.LockFileName))
	if err != nil {
		t.Fatalf("lockfile: %v", err)
	}
	var lock install.Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	return lock
}

// teammate copies what a clone of the project would carry: config and lock,
// not the gitignored skill folders.
func teammate(t *testing.T, sb *testutil.Sandbox, project string) string {
	t.Helper()
	other := filepath.Join(sb.Root, "teammate")
	if err := os.MkdirAll(filepath.Join(other, ".skillshare", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.yaml", install.LockFileName} {
		data, err := os.ReadFile(filepath.Join(project, ".skillshare", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, ".skillshare", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return other
}

func skillBody(t *testing.T, project string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".skillshare", "skills", "demo", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLock_InstallWritesFullCommit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	head := bump("v2")
	project := sb.SetupProjectDir("claude")

	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)

	if got := readLock(t, project).Skills["demo"].Commit; got != head {
		t.Fatalf("locked commit = %q, want %q", got, head)
	}
}

func TestLock_TeammateGetsLockedCommitAfterUpstreamMoves(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)
	other := teammate(t, sb, project)
	bump("v2")

	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)

	if body := skillBody(t, other); !strings.Contains(body, "# v1") {
		t.Fatalf("teammate got upstream head instead of the locked commit:\n%s", body)
	}
}

func TestLock_InstallFollowsALockThatMovedOn(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)
	other := teammate(t, sb, project)
	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)

	// The first developer updates and commits the new lock; the teammate pulls it.
	bump("v2")
	sb.RunCLIInDir(project, "update", "demo", "-p").AssertSuccess(t)
	lock, err := os.ReadFile(filepath.Join(project, ".skillshare", install.LockFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".skillshare", install.LockFileName), lock, 0o644); err != nil {
		t.Fatal(err)
	}

	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)

	if body := skillBody(t, other); !strings.Contains(body, "# v2") {
		t.Fatalf("existing copy did not follow the lock:\n%s", body)
	}
}

func TestLock_JSONInstallRequiresOverwriteConsent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)
	other := teammate(t, sb, project)
	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)

	bump("v2")
	sb.RunCLIInDir(project, "update", "demo", "-p").AssertSuccess(t)
	lockPath := filepath.Join(other, ".skillshare", install.LockFileName)
	lock := sb.ReadFile(filepath.Join(project, ".skillshare", install.LockFileName))
	sb.WriteFile(lockPath, lock)
	const edited = "---\nname: demo\n---\n# Local edit\n"
	sb.WriteFile(filepath.Join(other, ".skillshare", "skills", "demo", "SKILL.md"), edited)

	result := sb.RunCLIInDir(other, "install", "-p", "--json")
	if body := skillBody(t, other); body != edited {
		t.Fatalf("JSON install followed a changed lock without overwrite consent:\n%s", body)
	}
	result.AssertSuccess(t)
	out := parseJSON(t, result.Stdout)
	if skipped, ok := out["skipped"].([]any); !ok || len(skipped) != 1 || skipped[0] != "demo" {
		t.Fatalf("missing skipped locked skill: %s", result.Stdout)
	}
	if got := sb.ReadFile(lockPath); got != lock {
		t.Fatal("skipping an installed skill changed the teammate's lock")
	}

	forced := sb.RunCLIInDir(other, "install", "-p", "--json", "--force")
	forced.AssertSuccess(t)
	if body := skillBody(t, other); !strings.Contains(body, "# v2") {
		t.Fatalf("explicit force did not follow the new lock:\n%s", body)
	}
	out = parseJSON(t, forced.Stdout)
	if installed, ok := out["skills"].([]any); !ok || len(installed) != 1 || installed[0] != "demo" {
		t.Fatalf("forced relock not reported: %s", forced.Stdout)
	}
}

func TestLock_UninstallDropsThePin(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, _ := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)

	sb.RunCLIInDir(project, "uninstall", "demo", "-p", "--force").AssertSuccess(t)

	if _, err := os.Stat(filepath.Join(project, ".skillshare", install.LockFileName)); !os.IsNotExist(err) {
		t.Fatalf("lockfile should be gone once nothing is pinned, stat err = %v", err)
	}
}

func TestLock_TrackedRepoIsPinnedButStaysOnItsBranch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	repo := strings.TrimSuffix(source, "//skills/demo")
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", repo, "--track", "-p").AssertSuccess(t)
	other := teammate(t, sb, project)
	bump("v2")

	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)

	clone := filepath.Join(other, ".skillshare", "skills", "_remote.git")
	data, err := os.ReadFile(filepath.Join(clone, "skills", "demo", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# v1") {
		t.Fatalf("tracked repo is not at the locked commit:\n%s", data)
	}
	if branch := strings.TrimSpace(testutil.RunGit(t, clone, "rev-parse", "--abbrev-ref", "HEAD")); branch != "main" {
		t.Fatalf("HEAD = %q, want main so update can still pull", branch)
	}
}

// A copy that is merely behind must not drag the pin back when some other
// command rewrites the lockfile.
func TestLock_BehindCopyKeepsTheNewerPin(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, bump := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "-p").AssertSuccess(t)
	other := teammate(t, sb, project)
	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)
	v2 := bump("v2")
	sb.RunCLIInDir(project, "update", "demo", "-p").AssertSuccess(t)
	lock, err := os.ReadFile(filepath.Join(project, ".skillshare", install.LockFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".skillshare", install.LockFileName), lock, 0o644); err != nil {
		t.Fatal(err)
	}

	// The teammate pulled the lock but installs something else first.
	sb.RunCLIInDir(other, "install", strings.Replace(source, "skills/demo", "skills/other", 1), "-p").AssertSuccess(t)

	if got := readLock(t, other).Skills["demo"].Commit; got != v2 {
		t.Fatalf("pin went back to %q, want %q", got, v2)
	}
}

func trackedPair(t *testing.T, sb *testutil.Sandbox) (project, other string, bump func(string) string) {
	t.Helper()
	source, bump := lockRemote(t, sb)
	project = sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", strings.TrimSuffix(source, "//skills/demo"), "--track", "-p").AssertSuccess(t)
	return project, teammate(t, sb, project), bump
}

// Resetting a depth-1 clone to an older commit must not strand it: the next
// update still has to fast-forward.
func TestLock_PinnedTrackedRepoCanStillUpdate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	_, other, bump := trackedPair(t, sb)
	bump("v2")
	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)
	bump("v3")

	sb.RunCLIInDir(other, "update", "_remote.git", "-p").AssertSuccess(t)

	data, err := os.ReadFile(filepath.Join(other, ".skillshare", "skills", "_remote.git", "skills", "demo", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# v3") {
		t.Fatalf("tracked repo did not update past its pin:\n%s", data)
	}
}

func TestLock_RelockKeepsUncommittedEdits(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	project, other, bump := trackedPair(t, sb)
	sb.RunCLIInDir(other, "install", "-p").AssertSuccess(t)
	bump("v2")
	sb.RunCLIInDir(project, "update", "_remote.git", "-p").AssertSuccess(t)
	lock, err := os.ReadFile(filepath.Join(project, ".skillshare", install.LockFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, ".skillshare", install.LockFileName), lock, 0o644); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(other, ".skillshare", "skills", "_remote.git", "skills", "demo", "SKILL.md")
	if err := os.WriteFile(edited, []byte("---\nname: demo\n---\n# my edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sb.RunCLIInDir(other, "install", "-p")

	data, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# my edit") {
		t.Fatalf("install -p discarded an uncommitted edit:\n%s", data)
	}
}

func TestLock_GroupUninstallDropsItsPins(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	source, _ := lockRemote(t, sb)
	project := sb.SetupProjectDir("claude")
	sb.RunCLIInDir(project, "install", source, "--into", "frontend", "-p").AssertSuccess(t)
	if _, ok := readLock(t, project).Skills["frontend/demo"]; !ok {
		t.Fatalf("expected a pin for frontend/demo, got %v", readLock(t, project).Skills)
	}

	sb.RunCLIInDir(project, "uninstall", "--group", "frontend", "-p", "--force").AssertSuccess(t)

	if _, err := os.Stat(filepath.Join(project, ".skillshare", install.LockFileName)); !os.IsNotExist(err) {
		t.Fatalf("pins of the removed group are still in the lockfile: %v", readLock(t, project).Skills)
	}
}
