//go:build !online

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

func TestCheck_TrackedRepo_TokenEnvDoesNotBreakFileFetch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	remoteRepo := filepath.Join(sb.Root, "remote-repo.git")
	gitInit(t, remoteRepo, true)

	trackedPath := filepath.Join(sb.SourcePath, "_auth-check-repo")
	gitClone(t, remoteRepo, trackedPath)
	os.MkdirAll(filepath.Join(trackedPath, "my-skill"), 0755)
	os.WriteFile(filepath.Join(trackedPath, "my-skill", "SKILL.md"), []byte("# Test"), 0644)
	gitAddCommit(t, trackedPath, "initial")
	gitPush(t, trackedPath)

	t.Setenv("GITHUB_TOKEN", "ghp_fake_token_12345")
	t.Setenv("GITLAB_TOKEN", "glpat-fake-token")
	t.Setenv("BITBUCKET_TOKEN", "bb-fake-token")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "generic-fake-token")

	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "up to date")
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
	// Unfiltered check summarizes local skills instead of listing each one
	result.AssertOutputContains(t, "1 local skill(s) skipped")
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

// ── Filtered check tests (multi-name + --group) ──────────

func TestCheck_SingleName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	writeMeta(t, d1)
	d2 := sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	writeMeta(t, d2)

	result := sb.RunCLI("check", "alpha")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "alpha")
	result.AssertOutputNotContains(t, "beta")
}

func TestCheck_MultipleNames(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	writeMeta(t, d1)
	d2 := sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	writeMeta(t, d2)
	d3 := sb.CreateSkill("gamma", map[string]string{"SKILL.md": "# Gamma"})
	writeMeta(t, d3)

	result := sb.RunCLI("check", "alpha", "gamma")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "alpha")
	result.AssertAnyOutputContains(t, "gamma")
	result.AssertOutputNotContains(t, "beta")
}

func TestCheck_Group(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("frontend/react", map[string]string{"SKILL.md": "# React"})
	writeMeta(t, d1)
	d2 := sb.CreateNestedSkill("frontend/vue", map[string]string{"SKILL.md": "# Vue"})
	writeMeta(t, d2)

	result := sb.RunCLI("check", "--group", "frontend")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "react")
	result.AssertAnyOutputContains(t, "vue")
}

func TestCheck_Group_SkipsLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("backend/api", map[string]string{"SKILL.md": "# API"})
	writeMeta(t, d1)
	// local-only has no metadata, so it's skipped by resolveGroupUpdatable
	sb.CreateNestedSkill("backend/local-only", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("check", "--group", "backend")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "api")
	result.AssertOutputNotContains(t, "local-only")
}

func TestCheck_GroupNotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	result := sb.RunCLI("check", "--group", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "nonexistent")
}

func TestCheck_PositionalGroupAutoDetect(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateNestedSkill("mygroup/s1", map[string]string{"SKILL.md": "# S1"})
	writeMeta(t, d1)

	result := sb.RunCLI("check", "mygroup")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "is a group")
	result.AssertAnyOutputContains(t, "s1")
}

func TestCheck_Mixed_NamesAndGroup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("standalone", map[string]string{"SKILL.md": "# Standalone"})
	writeMeta(t, d1)

	d2 := sb.CreateNestedSkill("frontend/react", map[string]string{"SKILL.md": "# React"})
	writeMeta(t, d2)

	result := sb.RunCLI("check", "standalone", "--group", "frontend")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "standalone")
	result.AssertAnyOutputContains(t, "react")
}

func TestCheck_SingleName_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	d1 := sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	writeMeta(t, d1)
	d2 := sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})
	writeMeta(t, d2)

	result := sb.RunCLI("check", "alpha", "--json")
	result.AssertSuccess(t)

	var output checkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 1 {
		t.Errorf("expected 1 skill in JSON, got %d", len(output.Skills))
	}
	if len(output.Skills) > 0 && output.Skills[0].Name != "alpha" {
		t.Errorf("expected skill name 'alpha', got %q", output.Skills[0].Name)
	}
}

func TestCheckProject_MultiNames(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	projectRoot := sb.SetupProjectDir("claude")
	d1 := sb.CreateProjectSkill(projectRoot, "alpha", map[string]string{"SKILL.md": "# Alpha"})
	writeMeta(t, d1)
	d2 := sb.CreateProjectSkill(projectRoot, "beta", map[string]string{"SKILL.md": "# Beta"})
	writeMeta(t, d2)
	d3 := sb.CreateProjectSkill(projectRoot, "gamma", map[string]string{"SKILL.md": "# Gamma"})
	writeMeta(t, d3)

	result := sb.RunCLIInDir(projectRoot, "check", "alpha", "gamma")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "alpha")
	result.AssertAnyOutputContains(t, "gamma")
	result.AssertOutputNotContains(t, "beta")
}

// ── Tree hash tests ──────────────────────────────────────

// TestCheck_TreeHash_SkillUnchanged verifies that when a registry repo gets a
// new commit in a different subdir, the unchanged skill stays "up_to_date".
func TestCheck_TreeHash_SkillUnchanged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create bare remote with two skill subdirs
	remoteRepo := filepath.Join(sb.Root, "registry.git")
	workClone := filepath.Join(sb.Root, "work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "skill-a"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "skill-a", "SKILL.md"), []byte("# Skill A"), 0644)
	os.MkdirAll(filepath.Join(workClone, "skills", "skill-b"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "skill-b", "SKILL.md"), []byte("# Skill B"), 0644)
	gitAddCommit(t, workClone, "add both skills")
	gitPush(t, workClone)

	// Get current commit hash and tree hash for skill-a
	commitHash := gitRevParse(t, workClone, "HEAD")
	treeHash := gitRevParse(t, workClone, "HEAD:skills/skill-a")

	// Install skill-a with tree hash metadata
	skillDir := sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Skill A",
	})
	writeMetaWithTreeHash(t, skillDir, "file://"+remoteRepo, commitHash, treeHash, "skills/skill-a")

	// Now push a change to skill-b (skill-a is untouched)
	os.WriteFile(filepath.Join(workClone, "skills", "skill-b", "SKILL.md"), []byte("# Skill B v2"), 0644)
	gitAddCommit(t, workClone, "update skill-b only")
	gitPush(t, workClone)

	// Check: skill-a should be up_to_date (tree hash matches)
	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output checkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(output.Skills))
	}
	if output.Skills[0].Status != "up_to_date" {
		t.Errorf("expected up_to_date, got %q", output.Skills[0].Status)
	}
}

// TestCheck_TreeHash_SkillChanged verifies that when a skill's subdir changes,
// it correctly reports "update_available".
func TestCheck_TreeHash_SkillChanged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	remoteRepo := filepath.Join(sb.Root, "registry.git")
	workClone := filepath.Join(sb.Root, "work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "my-skill", "SKILL.md"), []byte("# V1"), 0644)
	gitAddCommit(t, workClone, "add skill")
	gitPush(t, workClone)

	commitHash := gitRevParse(t, workClone, "HEAD")
	treeHash := gitRevParse(t, workClone, "HEAD:skills/my-skill")

	skillDir := sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# V1",
	})
	writeMetaWithTreeHash(t, skillDir, "file://"+remoteRepo, commitHash, treeHash, "skills/my-skill")

	// Modify the skill's subdir and push
	os.WriteFile(filepath.Join(workClone, "skills", "my-skill", "SKILL.md"), []byte("# V2"), 0644)
	gitAddCommit(t, workClone, "update skill")
	gitPush(t, workClone)

	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output checkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(output.Skills))
	}
	if output.Skills[0].Status != "update_available" {
		t.Errorf("expected update_available, got %q", output.Skills[0].Status)
	}
}

// TestCheck_TreeHash_FallbackNoTreeHash verifies backward compatibility:
// when meta has no TreeHash, falls back to commit hash comparison.
func TestCheck_TreeHash_FallbackNoTreeHash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	remoteRepo := filepath.Join(sb.Root, "registry.git")
	workClone := filepath.Join(sb.Root, "work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	os.MkdirAll(filepath.Join(workClone, "skills", "old-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "old-skill", "SKILL.md"), []byte("# Old"), 0644)
	gitAddCommit(t, workClone, "add skill")
	gitPush(t, workClone)

	commitHash := gitRevParse(t, workClone, "HEAD")

	// Install with old-style metadata (no tree_hash)
	skillDir := sb.CreateSkill("old-skill", map[string]string{
		"SKILL.md": "# Old",
	})
	writeMetaNoTreeHash(t, skillDir, "file://"+remoteRepo, commitHash, "skills/old-skill")

	// Push an unrelated change (skill untouched)
	os.WriteFile(filepath.Join(workClone, "README.md"), []byte("# Readme"), 0644)
	gitAddCommit(t, workClone, "unrelated change")
	gitPush(t, workClone)

	result := sb.RunCLI("check", "--json")
	result.AssertSuccess(t)

	var output checkOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", err, result.Stdout)
	}

	if len(output.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(output.Skills))
	}
	// Without tree_hash, commit hash differs → update_available (backward compat)
	if output.Skills[0].Status != "update_available" {
		t.Errorf("expected update_available (fallback), got %q", output.Skills[0].Status)
	}
}

// ── Tree hash test helpers ────────────────────────────────

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s failed: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func writeMetaWithTreeHash(t *testing.T, skillDir, repoURL, version, treeHash, subdir string) {
	t.Helper()
	meta := map[string]any{
		"source":       repoURL + "//" + subdir,
		"type":         "github",
		"repo_url":     repoURL,
		"version":      version,
		"tree_hash":    treeHash,
		"subdir":       subdir,
		"installed_at": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatalf("writeMetaWithTreeHash: %v", err)
	}
}

func writeMetaNoTreeHash(t *testing.T, skillDir, repoURL, version, subdir string) {
	t.Helper()
	meta := map[string]any{
		"source":       repoURL + "//" + subdir,
		"type":         "github",
		"repo_url":     repoURL,
		"version":      version,
		"subdir":       subdir,
		"installed_at": "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatalf("writeMetaNoTreeHash: %v", err)
	}
}

// checkOutput mirrors the JSON structure for test parsing
type checkOutput struct {
	TrackedRepos []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"tracked_repos"`
	Skills []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"skills"`
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

func TestCheck_CustomTarget_NoUnknownWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	customPath := filepath.Join(sb.Home, ".custom-tool", "skills")
	os.MkdirAll(customPath, 0755)

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\ntargets: [claude, custom-tool]\n---\n# My Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + sb.CreateTarget("claude") + `
  custom-tool:
    path: ` + customPath + `
`)

	result := sb.RunCLI("check")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "unknown target")
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
