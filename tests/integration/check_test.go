package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestCheck_NoItems(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No tracked repositories or updatable skills found")
}

func TestCheck_Help(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("check", "--help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Check for available updates")
}

func TestCheck_TrackedRepo_UpToDate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a "remote" bare repo
	remoteRepo := filepath.Join(sb.Root, "remote-repo.git")
	gitInit(t, remoteRepo, true) // bare repo

	// Clone it as a tracked repo
	trackedPath := filepath.Join(sb.SourcePath, "_test-repo")
	gitClone(t, remoteRepo, trackedPath)

	// Create a skill inside so it's discovered
	os.MkdirAll(filepath.Join(trackedPath, "my-skill"), 0755)
	os.WriteFile(filepath.Join(trackedPath, "my-skill", "SKILL.md"), []byte("# Test"), 0644)
	gitAddCommit(t, trackedPath, "add skill")

	// Push to remote so origin is in sync
	gitPush(t, trackedPath)

	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "up to date")
}

func TestCheck_TrackedRepo_Behind(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a "remote" bare repo
	remoteRepo := filepath.Join(sb.Root, "remote-repo.git")
	gitInit(t, remoteRepo, true)

	// Clone it as a tracked repo
	trackedPath := filepath.Join(sb.SourcePath, "_test-repo")
	gitClone(t, remoteRepo, trackedPath)

	// Create initial content and push
	os.MkdirAll(filepath.Join(trackedPath, "my-skill"), 0755)
	os.WriteFile(filepath.Join(trackedPath, "my-skill", "SKILL.md"), []byte("# Test"), 0644)
	gitAddCommit(t, trackedPath, "initial")
	gitPush(t, trackedPath)

	// Clone another working copy, make commits, push them
	otherClone := filepath.Join(sb.Root, "other-clone")
	gitClone(t, remoteRepo, otherClone)
	os.WriteFile(filepath.Join(otherClone, "my-skill", "SKILL.md"), []byte("# Updated"), 0644)
	gitAddCommit(t, otherClone, "update from other")
	gitPush(t, otherClone)

	// Now tracked repo is behind
	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "behind")
}

func TestCheck_RegularSkill_ShowsMeta(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a skill with metadata (but local source, so check will show "local source")
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill",
		".skillshare-meta.json": `{
			"source": "/local/path",
			"type": "local",
			"installed_at": "2024-01-01T00:00:00Z"
		}`,
	})

	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "my-skill")
	result.AssertOutputContains(t, "local")
}

func TestCheck_JsonOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a skill with metadata
	sb.CreateSkill("json-skill", map[string]string{
		"SKILL.md": "# JSON Skill",
		".skillshare-meta.json": `{
			"source": "/local/path",
			"type": "local",
			"installed_at": "2024-01-01T00:00:00Z"
		}`,
	})

	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	// Verify JSON can be parsed
	var output struct {
		TrackedRepos []json.RawMessage `json:"tracked_repos"`
		Skills       []json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 1 {
		t.Errorf("expected 1 skill in JSON, got %d", len(output.Skills))
	}
}

func TestCheck_JsonOutput_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output struct {
		TrackedRepos []json.RawMessage `json:"tracked_repos"`
		Skills       []json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.TrackedRepos) != 0 {
		t.Errorf("expected 0 tracked_repos, got %d", len(output.TrackedRepos))
	}
	if len(output.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(output.Skills))
	}
}

// ── Git helpers ───────────────────────────────────────────

func gitInit(t *testing.T, dir string, bare bool) {
	t.Helper()
	if bare {
		os.MkdirAll(dir, 0755)
		run(t, dir, "git", "init", "--bare")
	} else {
		os.MkdirAll(dir, 0755)
		run(t, dir, "git", "init")
		run(t, dir, "git", "config", "user.email", "test@test.com")
		run(t, dir, "git", "config", "user.name", "test")
	}
}

func gitClone(t *testing.T, remote, dest string) {
	t.Helper()
	run(t, "", "git", "clone", remote, dest)
	run(t, dest, "git", "config", "user.email", "test@test.com")
	run(t, dest, "git", "config", "user.name", "test")
}

func gitAddCommit(t *testing.T, dir, msg string) {
	t.Helper()
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", msg)
}

func gitPush(t *testing.T, dir string) {
	t.Helper()
	// Get current branch name
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse failed: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	run(t, dir, "git", "push", "-u", "origin", branch)
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %s\n%s", name, args, err, out)
	}
}
