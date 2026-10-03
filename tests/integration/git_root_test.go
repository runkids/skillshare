//go:build !online

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestGitRoot_CommitCoversAgents verifies that commit with git_root=root
// creates a commit that includes files under agents/ (outside skills/).
func TestGitRoot_CommitCoversAgents(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath) // ~/.config/skillshare in sandbox
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\n  agents: " + agents + "\ntargets: {}\n")

	// Init a repo at root with config.yaml ignored.
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n.DS_Store\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")

	// Add a new agent file, then commit via the CLI.
	grWrite(t, filepath.Join(agents, "my-agent.md"), "# agent\n")
	result := sb.RunCLI("commit", "-m", "add agent")
	result.AssertSuccess(t)

	// The committed tree must include the agents file.
	tree := testutil.RunGit(t, base, "ls-tree", "-r", "--name-only", "HEAD")
	if !strings.Contains(tree, "agents/my-agent.md") {
		t.Errorf("expected agents/my-agent.md in committed tree, got:\n%s", tree)
	}
	if strings.Contains(tree, "config.yaml") {
		t.Errorf("config.yaml must not be tracked at root scope, got:\n%s", tree)
	}
}

// TestGitRoot_MismatchGuidance verifies that when git_root=root but the git
// repo lives under skills/ instead, the CLI prints mismatch guidance and aborts.
func TestGitRoot_MismatchGuidance(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)

	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\ntargets: {}\n")

	// Repo lives at skills/, NOT at the configured root.
	testutil.RunGit(t, skills, "init")
	testutil.ConfigureGitUser(t, skills)

	result := sb.RunCLI("commit", "-m", "x")
	out := result.Stdout + result.Stderr
	if !strings.Contains(out, "Git root mismatch") {
		t.Errorf("expected mismatch guidance, got:\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	}
}

// init --git-root root sets up the repo at BaseDir with config.yaml ignored.
func TestGitRoot_InitRootScope(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-skill", "--git", "--git-root", "root")
	result.AssertSuccess(t)

	base := filepath.Dir(sb.ConfigPath)

	if _, err := os.Stat(filepath.Join(base, ".git")); err != nil {
		t.Errorf("expected .git at root %s: %v", base, err)
	}
	if _, err := os.Stat(filepath.Join(base, "skills", ".git")); err == nil {
		t.Errorf("did not expect .git nested under skills/")
	}

	gi, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), "config.yaml") {
		t.Errorf("root .gitignore must ignore config.yaml, got:\n%s", gi)
	}

	cfgBytes, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(cfgBytes), "git_root: root") {
		t.Errorf("config must persist git_root: root, got:\n%s", cfgBytes)
	}
}

// init --git-root root must add config.yaml to a pre-existing .gitignore that
// lacks it, so a root-scope commit never tracks machine-specific config.
func TestGitRoot_InitRootScope_AppendsConfigToExistingGitignore(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	grMkdir(t, base)
	// A .gitignore already exists at root WITHOUT config.yaml.
	grWrite(t, filepath.Join(base, ".gitignore"), "node_modules/\n")

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-skill", "--git", "--git-root", "root")
	result.AssertSuccess(t)

	gi, err := os.ReadFile(filepath.Join(base, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), "config.yaml") {
		t.Errorf("existing root .gitignore must gain config.yaml, got:\n%s", gi)
	}
	if !strings.Contains(string(gi), "node_modules/") {
		t.Errorf("existing .gitignore entries must be preserved, got:\n%s", gi)
	}
}

// commit with git_root=agents commits changes under agents/ (outside skills/).
func TestGitRoot_CommitAgentsScope(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	sb.WriteConfig("git_root: agents\nsources:\n  skills: " + skills + "\n  agents: " + agents + "\ntargets: {}\n")

	// Repo lives at the agents source.
	testutil.RunGit(t, agents, "init")
	testutil.ConfigureGitUser(t, agents)

	grWrite(t, filepath.Join(agents, "reviewer.md"), "# reviewer\n")
	result := sb.RunCLI("commit", "-m", "add agent")
	result.AssertSuccess(t)

	tree := testutil.RunGit(t, agents, "ls-tree", "-r", "--name-only", "HEAD")
	if !strings.Contains(tree, "reviewer.md") {
		t.Errorf("expected reviewer.md in committed tree, got:\n%s", tree)
	}
}

// push with git_root=root pushes a tree that includes files under agents/
// (outside skills/) to the remote, and never tracks config.yaml.
func TestGitRoot_PushCoversAgents(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\n  agents: " + agents + "\ntargets: {}\n")

	// Bare remote outside the working tree.
	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())

	// Init repo at root, ignore config.yaml, seed initial commit, wire the remote.
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n.DS_Store\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")

	// New agent file (outside skills/), then push via the CLI.
	grWrite(t, filepath.Join(agents, "my-agent.md"), "# agent\n")
	result := sb.RunCLI("push", "-m", "add agent")
	result.AssertSuccess(t)

	// The pushed tree on the remote must include the agents file, not config.yaml.
	tree := testutil.RunGit(t, bareRepo, "ls-tree", "-r", "--name-only", "main")
	if !strings.Contains(tree, "agents/my-agent.md") {
		t.Errorf("expected agents/my-agent.md pushed to remote, got:\n%s", tree)
	}
	if strings.Contains(tree, "config.yaml") {
		t.Errorf("config.yaml must not be pushed at root scope, got:\n%s", tree)
	}
}

// pull with git_root=root brings remote changes under agents/ into the root
// working tree (not just skills/) and syncs them to targets.
func TestGitRoot_PullRootScope(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	claudeAgents := createAgentTarget(t, sb, "claude")
	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\n  agents: " + agents +
		"\ntargets:\n  claude:\n    skills:\n      path: " + sb.CreateTarget("claude") + "\n    agents:\n      path: " + claudeAgents + "\n")

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())

	// Init at root, seed, wire remote, push initial, set remote HEAD to main.
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")
	testutil.RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")

	// Another clone pushes a new agent file to the remote.
	other := filepath.Join(t.TempDir(), "other")
	testutil.RunGit(t, "", "clone", "-b", "main", bareRepo, other)
	testutil.ConfigureGitUser(t, other)
	grWrite(t, filepath.Join(other, "agents", "remote-agent.md"), "# remote agent\n")
	testutil.RunGit(t, other, "add", "-A")
	testutil.RunGit(t, other, "commit", "-m", "add remote agent")
	testutil.RunGit(t, other, "push", "origin", "main")

	// Pull via the CLI must materialize the agents file in the root working tree.
	result := sb.RunCLI("pull")
	result.AssertSuccess(t)

	if _, err := os.Stat(filepath.Join(agents, "remote-agent.md")); err != nil {
		t.Errorf("expected agents/remote-agent.md after pull at root scope: %v", err)
	}
	// The root scope holds agents too, so pull syncs them to targets.
	if _, err := os.Lstat(filepath.Join(claudeAgents, "remote-agent.md")); err != nil {
		t.Errorf("expected remote-agent.md synced to the claude agents target: %v", err)
	}
}

// A remote that tracks config.yaml must not replace this machine's copy: Git
// treats the ignored local file as expendable when merging (#353).
func TestGitRoot_PullKeepsLocalConfigWhenRemoteTracksIt(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsource: " + skills + "\ntargets:\n  claude:\n    skills:\n      path: " + sb.CreateTarget("claude") + "\n")
	localConfig, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")
	testutil.RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")

	other := filepath.Join(t.TempDir(), "other")
	testutil.RunGit(t, "", "clone", "-b", "main", bareRepo, other)
	testutil.ConfigureGitUser(t, other)
	grWrite(t, filepath.Join(other, "config.yaml"), "source: /other/machine/skills\n")
	testutil.RunGit(t, other, "add", "-f", "config.yaml")
	testutil.RunGit(t, other, "commit", "-m", "track config by mistake")
	testutil.RunGit(t, other, "push", "origin", "main")

	result := sb.RunCLI("pull")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "remote tracks config.yaml")

	if got, err := os.ReadFile(sb.ConfigPath); err != nil || string(got) != string(localConfig) {
		t.Fatalf("config.yaml = %q, %v; want this machine's copy kept", got, err)
	}

	// The suggested push removes config.yaml from the remote, keeping it on disk.
	sb.RunCLI("push").AssertSuccess(t)
	if out := testutil.RunGit(t, bareRepo, "ls-tree", "--name-only", "main"); strings.Contains(out, "config.yaml") {
		t.Fatalf("remote still tracks config.yaml after push: %q", out)
	}
	if got, err := os.ReadFile(sb.ConfigPath); err != nil || string(got) != string(localConfig) {
		t.Fatalf("config.yaml = %q, %v; want this machine's copy kept after push", got, err)
	}
}

// push --pull at root scope: a pull that brings in a remote-tracked config.yaml
// keeps this machine's copy, and the same push removes it from the remote.
func TestGitRoot_PushPullUntracksConfigBroughtInByPull(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsource: " + skills + "\ntargets:\n  claude:\n    skills:\n      path: " + sb.CreateTarget("claude") + "\n")
	localConfig, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")
	testutil.RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")

	other := filepath.Join(t.TempDir(), "other")
	testutil.RunGit(t, "", "clone", "-b", "main", bareRepo, other)
	testutil.ConfigureGitUser(t, other)
	grWrite(t, filepath.Join(other, "config.yaml"), "source: /other/machine/skills\n")
	testutil.RunGit(t, other, "add", "-f", "config.yaml")
	testutil.RunGit(t, other, "commit", "-m", "track config by mistake")
	testutil.RunGit(t, other, "push", "origin", "main")

	sb.RunCLI("push", "--pull").AssertSuccess(t)

	if out := testutil.RunGit(t, bareRepo, "ls-tree", "--name-only", "main"); strings.Contains(out, "config.yaml") {
		t.Fatalf("remote still tracks config.yaml after push --pull: %q", out)
	}
	if got, err := os.ReadFile(sb.ConfigPath); err != nil || string(got) != string(localConfig) {
		t.Fatalf("config.yaml = %q, %v; want this machine's copy kept", got, err)
	}
	if out := testutil.RunGit(t, base, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Fatalf("working tree not clean after push --pull: %q", out)
	}
}

// push --pull at root scope repairs a pulled .gitignore that un-ignores
// config.yaml and pushes the repair, leaving a clean tree.
func TestGitRoot_PushPullCommitsGitignoreRepair(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsource: " + skills + "\ntargets:\n  claude:\n    skills:\n      path: " + sb.CreateTarget("claude") + "\n")

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")
	testutil.RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")

	other := filepath.Join(t.TempDir(), "other")
	testutil.RunGit(t, "", "clone", "-b", "main", bareRepo, other)
	testutil.ConfigureGitUser(t, other)
	grWrite(t, filepath.Join(other, ".gitignore"), "config.yaml\n!config.yaml\n")
	testutil.RunGit(t, other, "commit", "-am", "un-ignore config by mistake")
	testutil.RunGit(t, other, "push", "origin", "main")

	sb.RunCLI("push", "--pull").AssertSuccess(t)

	if out := testutil.RunGit(t, base, "status", "--porcelain"); strings.TrimSpace(out) != "" {
		t.Fatalf("working tree not clean after push --pull: %q", out)
	}
	if out := testutil.RunGit(t, bareRepo, "show", "main:.gitignore"); !strings.HasSuffix(out, "!config.yaml\nconfig.yaml") {
		t.Fatalf("remote .gitignore = %q; want the repair that ignores config.yaml again", out)
	}
}

// push --dry-run previews the .gitignore repair a real push makes when a later
// rule un-ignores config.yaml, instead of listing config.yaml to commit.
func TestGitRoot_PushDryRunPreviewsGitignoreRepair(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsource: " + skills + "\ntargets:\n  claude:\n    skills:\n      path: " + sb.CreateTarget("claude") + "\n")

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	gitignore := "config.yaml\n!config.yaml\n"
	grWrite(t, filepath.Join(base, ".gitignore"), gitignore)
	testutil.RunGit(t, base, "add", ".gitignore")
	testutil.RunGit(t, base, "commit", "-m", "initial")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, base, "push", "-u", "origin", "main")

	result := sb.RunCLI("push", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Would add config.yaml to .gitignore")
	result.AssertAnyOutputContains(t, "~ .gitignore")
	result.AssertOutputNotContains(t, "+ config.yaml")
	if got, _ := os.ReadFile(filepath.Join(base, ".gitignore")); string(got) != gitignore {
		t.Fatalf(".gitignore = %q; dry run must not change it", got)
	}
}

// init --git-root <scope> on an already-initialized setup switches the scope
// headlessly: it inits a repo at the new scope dir and persists git_root,
// without prompting or erroring with "already initialized".
func TestGitRoot_SwitchScopeAfterInit(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Fresh init at the default skills scope.
	sb.RunCLI("init", "--no-copy", "--no-targets", "--no-skill", "--git").AssertSuccess(t)

	base := filepath.Dir(sb.ConfigPath)
	if _, err := os.Stat(filepath.Join(base, "skills", ".git")); err != nil {
		t.Fatalf("expected skills repo after fresh init: %v", err)
	}

	// Headless switch to the agents scope on the already-initialized setup.
	result := sb.RunCLI("init", "--git-root", "agents")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "Git root", "agents")

	if _, err := os.Stat(filepath.Join(base, "agents", ".git")); err != nil {
		t.Errorf("expected agents repo after scope switch: %v", err)
	}
	cfgBytes, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfgBytes), "git_root: agents") {
		t.Errorf("config must persist git_root: agents, got:\n%s", cfgBytes)
	}
}

// init --git-root <scope> --remote <url> switches the scope and wires the
// remote on the new scope's repo in one headless command.
func TestGitRoot_SwitchScopeWithRemote(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.RunCLI("init", "--no-copy", "--no-targets", "--no-skill", "--git").AssertSuccess(t)

	base := filepath.Dir(sb.ConfigPath)
	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())

	// Switch to root scope AND wire a remote in one command.
	result := sb.RunCLI("init", "--git-root", "root", "--remote", bareRepo)
	result.AssertSuccess(t)

	cfgBytes, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfgBytes), "git_root: root") {
		t.Errorf("expected git_root: root, got:\n%s", cfgBytes)
	}
	// The root-scope repo must carry the configured remote.
	remote := testutil.RunGit(t, base, "remote", "get-url", "origin")
	if remote != bareRepo {
		t.Errorf("expected origin %q at root scope, got %q", bareRepo, remote)
	}
}

// A nested git repo under a root-scope source must block commit: committing it
// would record an empty submodule (gitlink) and silently drop its files. The
// CLI prints the remediation box and aborts with a non-zero exit.
func TestGitRoot_NestedRepoBlocksCommit(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\ntargets: {}\n")

	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", "-A")
	testutil.RunGit(t, base, "commit", "-m", "initial")

	// Plant a nested repo with uncommitted work to stage.
	grMkdir(t, filepath.Join(skills, "vendored", ".git"))
	grWrite(t, filepath.Join(skills, "vendored", "SKILL.md"), "# vendored\n")

	result := sb.RunCLI("commit", "-m", "x")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "Nested git repositories")

	// Nothing must have been committed beyond the initial commit.
	count := testutil.RunGit(t, base, "rev-list", "--count", "HEAD")
	if count != "1" {
		t.Errorf("commit count = %q, want 1 (nested repo must abort commit)", count)
	}
}

// A root-scope dry-run must be strictly read-only: it must not run
// `git rm --cached config.yaml`. Regression test for the sweep mutating the
// repo before the dry-run check.
func TestGitRoot_DryRunDoesNotUntrackConfig(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	grMkdir(t, skills)
	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\ntargets: {}\n")

	// Init a repo that (wrongly) tracks config.yaml — the state the sweep is
	// meant to clean up on a real run, but must leave untouched on dry-run.
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	testutil.RunGit(t, base, "add", "config.yaml")
	testutil.RunGit(t, base, "commit", "-m", "initial with config")

	// Make a stageable change so the dry-run reaches the preview path.
	grWrite(t, filepath.Join(skills, "a.md"), "# a\n")

	result := sb.RunCLI("commit", "-m", "x", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Would remove config.yaml")

	// config.yaml must still be tracked — dry-run made no changes.
	tracked := testutil.RunGit(t, base, "ls-files", "--", "config.yaml")
	if !strings.Contains(tracked, "config.yaml") {
		t.Errorf("dry-run must not untrack config.yaml, but it is no longer tracked")
	}
}

// requireWorkingGit skips the test when the host/container git environment
// cannot init a repo and create a commit (e.g. read-only HOME, an identity that
// cannot be written, or an owner/permission mismatch). Stripped one-shot
// containers that lack the devcontainer's setup can hit this; without the probe
// the failure surfaces deep inside a test as a misleading assertion error. The
// probe writes nothing outside its own temp dir and never touches git_root.
func requireWorkingGit(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v (%s)", args, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := os.WriteFile(filepath.Join(dir, "probe.txt"), []byte("x"), 0o644); err != nil {
		t.Skipf("git environment probe: cannot write temp file: %v", err)
	}
	for _, step := range [][]string{
		{"init"},
		{"config", "user.email", "probe@test.local"},
		{"config", "user.name", "Probe"},
		{"add", "-A"},
		{"commit", "-m", "probe"},
	} {
		if err := run(step...); err != nil {
			t.Skipf("git environment cannot create commits, skipping git_root test: %v", err)
		}
	}
}

func grMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func grWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitRoot_FirstPull_RemoteTracksConfig_Refused(t *testing.T) {
	requireWorkingGit(t)

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	localConfig := "git_root: root\nsources:\n  skills: " + skills + "\n  agents: " + agents + "\ntargets: {}\n"
	sb.WriteConfig(localConfig)

	bareRepo := testutil.SetupBareRemoteRepo(t, t.TempDir())

	// Remote tracks config.yaml and a skill
	seed := filepath.Join(t.TempDir(), "seed")
	testutil.RunGit(t, "", "init", seed)
	testutil.ConfigureGitUser(t, seed)
	grWrite(t, filepath.Join(seed, "config.yaml"), "git_root: root\nsources:\n  skills: /other/skills\n")
	grMkdir(t, filepath.Join(seed, "skills"))
	grWrite(t, filepath.Join(seed, "skills", "remote.md"), "# remote\n")
	testutil.RunGit(t, seed, "add", "-A")
	testutil.RunGit(t, seed, "commit", "-m", "remote tracking config")
	testutil.RunGit(t, seed, "branch", "-M", "main")
	testutil.RunGit(t, seed, "remote", "add", "origin", bareRepo)
	testutil.RunGit(t, seed, "push", "-u", "origin", "main")
	testutil.RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/main")

	// Local repo initialized at root, wire remote without upstream
	testutil.RunGit(t, base, "init")
	testutil.ConfigureGitUser(t, base)
	grWrite(t, filepath.Join(base, ".gitignore"), "config.yaml\n")
	testutil.RunGit(t, base, "add", ".gitignore")
	testutil.RunGit(t, base, "commit", "-m", "initial scaffold")
	testutil.RunGit(t, base, "branch", "-M", "main")
	testutil.RunGit(t, base, "remote", "add", "origin", bareRepo)

	// pull must fail and preserve local config.yaml
	result := sb.RunCLI("pull")
	result.AssertFailure(t)
	result.AssertOutputContains(t, "Remote tracks config.yaml")

	cfgBytes, err := os.ReadFile(sb.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(cfgBytes) != localConfig {
		t.Fatalf("local config.yaml was overwritten: got %q, want %q", string(cfgBytes), localConfig)
	}
}
