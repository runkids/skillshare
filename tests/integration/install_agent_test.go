//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstall_AgentFlag_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// --kind with invalid value should error
	result := sb.RunCLI("install", "--kind", "invalid", "test")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "must be 'skill' or 'agent'")
}

func TestInstall_AgentFlagShort_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// -a without value should error
	result := sb.RunCLI("install", "-a")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "requires agent name")
}

func TestCheck_Agents_EmptyDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)

	result := sb.RunCLI("check", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No agents found")
}

func TestCheck_Agents_LocalAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir with a local agent
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor agent"), 0644)

	result := sb.RunCLI("check", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
	result.AssertAnyOutputContains(t, "local")
}

func TestCheck_Agents_JsonOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor"), 0644)

	result := sb.RunCLI("check", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"name"`)
	result.AssertAnyOutputContains(t, `"status"`)
}

func TestEnable_KindAgent_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)

	// Disable an agent — --kind goes after -g (mode flag)
	result := sb.RunCLI("disable", "-g", "--kind", "agent", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, ".agentignore")

	// Verify .agentignore was created
	agentIgnorePath := filepath.Join(agentsDir, ".agentignore")
	if !sb.FileExists(agentIgnorePath) {
		t.Error(".agentignore should be created")
	}
}

func TestUninstall_AgentsPositional_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Positional "agents" with nonexistent agent — should parse correctly (no "unknown option")
	result := sb.RunCLI("uninstall", "-g", "agents", "nonexistent")
	result.AssertOutputNotContains(t, "unknown option")
}

func TestInstall_MixedRepo_ThenSync_AgentsGoToCorrectTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	windsurf := filepath.Join(sb.Home, ".windsurf", "skills")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
  windsurf:
    skills:
      path: "` + windsurf + `"
`)

	// Create mixed repo with both skills and agents
	repoDir := filepath.Join(sb.Home, "mixed-repo")
	os.MkdirAll(filepath.Join(repoDir, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(repoDir, "skills", "my-skill", "SKILL.md"),
		[]byte("---\nname: my-skill\n---\n# My Skill"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "agents"), 0755)
	os.WriteFile(filepath.Join(repoDir, "agents", "my-agent.md"),
		[]byte("# My Agent"), 0644)
	initGitRepo(t, repoDir)

	// Install
	installResult := sb.RunCLI("install", "file://"+repoDir, "--yes")
	installResult.AssertSuccess(t)

	// Sync all (skills + agents)
	syncResult := sb.RunCLI("sync", "--all")
	syncResult.AssertSuccess(t)

	// Skill in claude skills target
	if !sb.FileExists(filepath.Join(claudeSkills, "my-skill", "SKILL.md")) {
		t.Error("skill should be synced to claude skills dir")
	}

	// Agent in claude agents target
	if !sb.FileExists(filepath.Join(claudeAgents, "my-agent.md")) {
		t.Error("agent should be synced to claude agents dir")
	}

	// Skill in windsurf (skills support)
	if !sb.FileExists(filepath.Join(windsurf, "my-skill", "SKILL.md")) {
		t.Error("skill should be synced to windsurf skills dir")
	}

	// Agent NOT in windsurf skills (no agents path)
	if sb.FileExists(filepath.Join(windsurf, "my-agent.md")) {
		t.Error("agent should NOT be in windsurf skills dir")
	}

	// Warning about skipped target
	syncResult.AssertAnyOutputContains(t, "windsurf")
}

func TestInstall_MixedRepo_InstallsAgentsToAgentsDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a git repo with both skills and agents
	repoDir := filepath.Join(sb.Home, "mixed-repo")
	os.MkdirAll(filepath.Join(repoDir, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(repoDir, "skills", "my-skill", "SKILL.md"),
		[]byte("---\nname: my-skill\n---\n# My Skill"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "agents"), 0755)
	os.WriteFile(filepath.Join(repoDir, "agents", "my-agent.md"),
		[]byte("# My Agent"), 0644)
	initGitRepo(t, repoDir)

	result := sb.RunCLI("install", "file://"+repoDir, "--yes")
	result.AssertSuccess(t)

	// Skill should be in skills source
	skillPath := filepath.Join(sb.SourcePath, "my-skill")
	if !sb.FileExists(filepath.Join(skillPath, "SKILL.md")) {
		t.Error("skill should be installed to skills source dir")
	}

	// Agent should be in agents source (NOT skills source)
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	agentPath := filepath.Join(agentsDir, "my-agent.md")
	if !sb.FileExists(agentPath) {
		t.Errorf("agent should be installed to agents dir (%s), not skills dir", agentsDir)
	}

	// Agent should NOT be in skills source
	wrongPath := filepath.Join(sb.SourcePath, "my-agent.md")
	if sb.FileExists(wrongPath) {
		t.Error("agent should NOT be in skills source dir")
	}
}

// TestInstall_MixedRepo_SkillFlag_SkipsAgents guards issue #183: selecting a
// specific skill with -s/--skill must not drag in the repo's agents.
func TestInstall_MixedRepo_SkillFlag_SkipsAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Mixed repo: two skills, one agent.
	repoDir := filepath.Join(sb.Home, "mixed-skill-flag-repo")
	for _, name := range []string{"skill-one", "skill-two"} {
		os.MkdirAll(filepath.Join(repoDir, "skills", name), 0755)
		os.WriteFile(filepath.Join(repoDir, "skills", name, "SKILL.md"),
			[]byte("---\nname: "+name+"\n---\n# "+name), 0644)
	}
	os.MkdirAll(filepath.Join(repoDir, "agents"), 0755)
	os.WriteFile(filepath.Join(repoDir, "agents", "my-agent.md"),
		[]byte("# My Agent"), 0644)
	initGitRepo(t, repoDir)

	result := sb.RunCLI("install", "file://"+repoDir, "-s", "skill-one")
	result.AssertSuccess(t)

	// Selected skill installed.
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	// Non-selected skill not installed.
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-two")) {
		t.Error("skill-two should NOT be installed")
	}
	// Agent must NOT be installed (issue #183).
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	if sb.FileExists(filepath.Join(agentsDir, "my-agent.md")) {
		t.Error("agent should NOT be installed when scoping to a specific skill via -s")
	}
}

// TestInstall_MixedRepo_SkillAndAgentFlag_InstallsOnlyNamedAgent guards that
// -s skill-one -a agent-one installs only the named agent, not every agent in
// the repo (issue #183 follow-up).
func TestInstall_MixedRepo_SkillAndAgentFlag_InstallsOnlyNamedAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Mixed repo: one skill, two agents.
	repoDir := filepath.Join(sb.Home, "mixed-skill-agent-repo")
	os.MkdirAll(filepath.Join(repoDir, "skills", "skill-one"), 0755)
	os.WriteFile(filepath.Join(repoDir, "skills", "skill-one", "SKILL.md"),
		[]byte("---\nname: skill-one\n---\n# skill-one"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "agents"), 0755)
	os.WriteFile(filepath.Join(repoDir, "agents", "agent-one.md"),
		[]byte("# Agent One"), 0644)
	os.WriteFile(filepath.Join(repoDir, "agents", "agent-two.md"),
		[]byte("# Agent Two"), 0644)
	initGitRepo(t, repoDir)

	result := sb.RunCLI("install", "file://"+repoDir, "-s", "skill-one", "-a", "agent-one")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	// Named agent installed.
	if !sb.FileExists(filepath.Join(agentsDir, "agent-one.md")) {
		t.Error("agent-one should be installed")
	}
	// Unnamed agent NOT installed.
	if sb.FileExists(filepath.Join(agentsDir, "agent-two.md")) {
		t.Error("agent-two should NOT be installed when only agent-one is requested via -a")
	}
}

// TestInstall_MixedRepo_AgentFlag_NotFoundFailsCleanly guards issue #183:
// an unknown -a agent name must fail the whole install (non-zero exit) before
// any skill is installed, rather than leaving a half-success.
func TestInstall_MixedRepo_AgentFlag_NotFoundFailsCleanly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Mixed repo: one skill, one agent.
	repoDir := filepath.Join(sb.Home, "mixed-agent-notfound-repo")
	os.MkdirAll(filepath.Join(repoDir, "skills", "skill-one"), 0755)
	os.WriteFile(filepath.Join(repoDir, "skills", "skill-one", "SKILL.md"),
		[]byte("---\nname: skill-one\n---\n# skill-one"), 0644)
	os.MkdirAll(filepath.Join(repoDir, "agents"), 0755)
	os.WriteFile(filepath.Join(repoDir, "agents", "agent-one.md"),
		[]byte("# Agent One"), 0644)
	initGitRepo(t, repoDir)

	result := sb.RunCLI("install", "file://"+repoDir, "-s", "skill-one", "-a", "missing")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "agents not found: missing")

	// No half-success: the skill must NOT be installed when the agent filter fails.
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-one")) {
		t.Error("skill-one should NOT be installed when the -a agent filter fails")
	}
}

// TestInstall_SkillOnlyRepo_AgentFlag_NotFoundFailsCleanly guards issue #183:
// -a against a repo with zero agents must fail (non-zero exit) before any skill
// is installed — the agent filter asserts the agent exists.
func TestInstall_SkillOnlyRepo_AgentFlag_NotFoundFailsCleanly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Skill-only repo: no agents directory at all.
	repoDir := filepath.Join(sb.Home, "skill-only-repo")
	os.MkdirAll(filepath.Join(repoDir, "skills", "skill-one"), 0755)
	os.WriteFile(filepath.Join(repoDir, "skills", "skill-one", "SKILL.md"),
		[]byte("---\nname: skill-one\n---\n# skill-one"), 0644)
	initGitRepo(t, repoDir)

	result := sb.RunCLI("install", "file://"+repoDir, "-s", "skill-one", "-a", "missing")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "agents not found: missing")

	// No half-success: the skill must NOT be installed.
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-one")) {
		t.Error("skill-one should NOT be installed when the -a agent filter fails")
	}
}

func TestInstall_TrackAgentRepo_UsesTrackedRepoFlow(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	repoDir := filepath.Join(sb.Home, "tracked-agent-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "reviewer.md"), []byte("# Reviewer v1"), 0o644); err != nil {
		t.Fatalf("write agent: %v", err)
	}
	initGitRepo(t, repoDir)

	installResult := sb.RunCLI("install", "file://"+repoDir, "--track", "--kind", "agent")
	installResult.AssertSuccess(t)

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	source, err := install.ParseSource("file://" + repoDir)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	trackedRepoDir := filepath.Join(agentsDir, "_"+source.Name)
	if _, err := os.Stat(filepath.Join(trackedRepoDir, ".git")); err != nil {
		t.Fatalf("expected tracked agent repo .git to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(trackedRepoDir, "reviewer.md")); err != nil {
		t.Fatalf("expected tracked agent file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "_tracked-agent-repo")); !os.IsNotExist(err) {
		t.Fatalf("expected no tracked agent repo in skills source, got err=%v", err)
	}

	checkResult := sb.RunCLI("check", "agents")
	checkResult.AssertSuccess(t)
	checkResult.AssertAnyOutputContains(t, "reviewer")
	checkResult.AssertOutputNotContains(t, "local agent")

	if err := os.WriteFile(filepath.Join(repoDir, "reviewer.md"), []byte("# Reviewer v2"), 0o644); err != nil {
		t.Fatalf("update agent: %v", err)
	}
	for _, args := range [][]string{
		{"add", "reviewer.md"},
		{"commit", "-m", "update reviewer"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	updateResult := sb.RunCLI("update", "agents", "--all")
	updateResult.AssertSuccess(t)
	updateResult.AssertAnyOutputContains(t, "updated")

	content, err := os.ReadFile(filepath.Join(trackedRepoDir, "reviewer.md"))
	if err != nil {
		t.Fatalf("read updated agent: %v", err)
	}
	if string(content) != "# Reviewer v2" {
		t.Fatalf("expected updated tracked agent content, got %q", string(content))
	}
}
