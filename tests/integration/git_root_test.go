//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// TestGitRoot_CommitCoversAgents verifies that commit with git_root=root
// creates a commit that includes files under agents/ (outside skills/).
func TestGitRoot_CommitCoversAgents(t *testing.T) {
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
// working tree (not just skills/).
func TestGitRoot_PullRootScope(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	base := filepath.Dir(sb.ConfigPath)
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	grMkdir(t, skills)
	grMkdir(t, agents)

	sb.WriteConfig("git_root: root\nsources:\n  skills: " + skills + "\n  agents: " + agents + "\ntargets: {}\n")

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
