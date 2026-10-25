//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
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
	})
	writeMetaEntry(t, filepath.Join(sb.SourcePath, "my-skill"), &install.MetadataEntry{
		Source: "/local/path",
		Type:   "local",
	})

	result := sb.RunCLI("check")
	result.AssertSuccess(t)
	// Unfiltered check summarizes local skills instead of listing each one
	result.AssertOutputContains(t, "1 local skill skipped")
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
	})
	writeMetaEntry(t, filepath.Join(sb.SourcePath, "json-skill"), &install.MetadataEntry{
		Source: "/local/path",
		Type:   "local",
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

func TestCheck_LocalSource_DetectsChangeUntilUpdated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	external := filepath.Join(sb.Root, "app-bundle", "app-skill")
	sb.WriteFile(filepath.Join(external, "SKILL.md"), "---\nname: app-skill\n---\n# v1")
	sb.RunCLI("install", external).AssertSuccess(t)

	checkStatus := func() string {
		t.Helper()
		result := sb.RunCLI("check", "--json")
		result.AssertSuccess(t)
		var output checkOutput
		if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
			t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, result.Stdout)
		}
		for _, s := range output.Skills {
			if s.Name == "app-skill" {
				return s.Status
			}
		}
		t.Fatalf("app-skill missing from check output: %s", result.Stdout)
		return ""
	}

	if got := checkStatus(); got != "up_to_date" {
		t.Fatalf("after install: status = %q, want up_to_date", got)
	}

	sb.WriteFile(filepath.Join(external, "SKILL.md"), "---\nname: app-skill\n---\n# v2")
	if got := checkStatus(); got != "update_available" {
		t.Fatalf("after source change: status = %q, want update_available", got)
	}

	sb.RunCLI("update", "app-skill").AssertSuccess(t)
	if got := checkStatus(); got != "up_to_date" {
		t.Fatalf("after update: status = %q, want up_to_date", got)
	}
}

func TestCheck_LocalSource_Missing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	external := filepath.Join(sb.Root, "app-bundle", "gone-skill")
	sb.WriteFile(filepath.Join(external, "SKILL.md"), "---\nname: gone-skill\n---\n# v1")
	sb.RunCLI("install", external).AssertSuccess(t)
	os.RemoveAll(external)

	result := sb.RunCLI("check", "gone-skill")
	result.AssertExitCode(t, 1)
	result.AssertAnyOutputContains(t, "local source not found")

	for _, args := range [][]string{{"check", "--json"}, {"check", "gone-skill", "--json"}} {
		result := sb.RunCLI(args...)
		result.AssertExitCode(t, 1)
		if result.Stderr != "" {
			t.Errorf("JSON failure wrote to stderr: %s", result.Stderr)
		}
		var output checkOutput
		if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
			t.Fatalf("invalid JSON: %v: %s", err, result.Stdout)
		}
		if len(output.Skills) != 1 || output.Skills[0].Status != "error" {
			t.Fatalf("expected failed skill in JSON: %s", result.Stdout)
		}
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

// writeMetaEntry writes a single metadata entry to .metadata.json in the source root.
func writeMetaEntry(t *testing.T, skillDir string, entry *install.MetadataEntry) {
	t.Helper()
	sourceDir := findSourceRoot(skillDir)
	rel, _ := filepath.Rel(sourceDir, skillDir)

	store, err := install.LoadMetadata(sourceDir)
	if err != nil {
		t.Fatalf("writeMetaEntry: load: %v", err)
	}
	store.Set(rel, entry)
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("writeMetaEntry: save: %v", err)
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
	sourceDir := findSourceRoot(skillDir)
	rel, _ := filepath.Rel(sourceDir, skillDir)

	store, err := install.LoadMetadata(sourceDir)
	if err != nil {
		t.Fatalf("writeMetaWithTreeHash: load: %v", err)
	}
	store.Set(rel, &install.MetadataEntry{
		Source:   repoURL + "//" + subdir,
		Type:     "github",
		RepoURL:  repoURL,
		Version:  version,
		TreeHash: treeHash,
		Subdir:   subdir,
	})
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("writeMetaWithTreeHash: save: %v", err)
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

func TestCheckProject_RelativeLocalSource_DetectsChangeUntilUpdated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	skillFile := filepath.Join(projectRoot, "vendor", "team-skill", "SKILL.md")
	sb.WriteFile(skillFile, "---\nname: team-skill\n---\n# v1")
	sb.RunCLIInDir(projectRoot, "install", "./vendor/team-skill", "-p").AssertSuccess(t)

	checkStatus := func() string {
		t.Helper()
		result := sb.RunCLIInDir(projectRoot, "check", "-p", "--json")
		result.AssertSuccess(t)
		var output checkOutput
		if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
			t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, result.Stdout)
		}
		for _, s := range output.Skills {
			if s.Name == "team-skill" {
				return s.Status
			}
		}
		t.Fatalf("team-skill missing from check output: %s", result.Stdout)
		return ""
	}

	if got := checkStatus(); got != "up_to_date" {
		t.Fatalf("after install: status = %q, want up_to_date", got)
	}

	sb.WriteFile(skillFile, "---\nname: team-skill\n---\n# v2")
	if got := checkStatus(); got != "update_available" {
		t.Fatalf("after source change: status = %q, want update_available", got)
	}

	sb.RunCLIInDir(projectRoot, "update", "team-skill", "-p").AssertSuccess(t)
	if got := checkStatus(); got != "up_to_date" {
		t.Fatalf("after update: status = %q, want up_to_date", got)
	}
}
