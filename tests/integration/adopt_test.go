//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// adoptWriteSkill creates a real skill directory (with SKILL.md) at base/name.
func adoptWriteSkill(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill %s: %v", name, err)
	}
	body := "---\nname: " + name + "\ndescription: external skill\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

// adoptWriteLock writes a fake ~/.agents/.skill-lock.json claiming the given
// skill names, each owned by sourceTool.
func adoptWriteLock(t *testing.T, agentsDir, sourceTool string, names ...string) {
	t.Helper()
	skills := map[string]any{}
	for _, n := range names {
		skills[n] = map[string]any{"sourceTool": sourceTool}
	}
	lock := map[string]any{"skills": skills}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	// The lockfile lives one level up beside the skills dir
	// (~/.agents/.skill-lock.json), not inside ~/.agents/skills.
	if err := os.WriteFile(filepath.Join(filepath.Dir(agentsDir), ".skill-lock.json"), data, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
}

// setupAdoptGlobal wires a sandbox where the universal/agents target
// (~/.agents/skills) holds a real external skill <X>, the claude target has an
// orphan symlink to it, and a lockfile claims <X>. Returns (agentsPath, claudePath).
func setupAdoptGlobal(t *testing.T, sb *testutil.Sandbox, skillName, sourceTool string) (string, string) {
	t.Helper()

	agentsPath := filepath.Join(sb.Home, ".agents", "skills")
	if err := os.MkdirAll(agentsPath, 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	claudePath := sb.CreateTarget("claude")

	// Real external skill dropped by the CLI tool.
	skillPath := adoptWriteSkill(t, agentsPath, skillName)

	// External symlink in the claude target pointing into the agents dir.
	link := filepath.Join(claudePath, skillName)
	if err := os.Symlink(skillPath, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Lockfile claims the skill.
	adoptWriteLock(t, agentsPath, sourceTool, skillName)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  universal:
    path: ` + agentsPath + `
  claude:
    path: ` + claudePath + `
`)

	return agentsPath, claudePath
}

func TestAdopt_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsPath, claudePath := setupAdoptGlobal(t, sb, "firecrawl", "firecrawl")

	result := sb.RunCLI("adopt", "-g", "--all", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "firecrawl")

	// Source untouched.
	if sb.FileExists(filepath.Join(sb.SourcePath, "firecrawl")) {
		t.Error("dry-run copied skill into source")
	}
	// Original untouched.
	if !sb.FileExists(filepath.Join(agentsPath, "firecrawl", "SKILL.md")) {
		t.Error("dry-run removed original from agents dir")
	}
	// Orphan symlink untouched.
	if !sb.FileExists(filepath.Join(claudePath, "firecrawl")) {
		t.Error("dry-run removed orphan symlink")
	}
}

func TestAdopt_Apply_MigratesAndResyncs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsPath, claudePath := setupAdoptGlobal(t, sb, "firecrawl", "firecrawl")

	result := sb.RunCLI("adopt", "-g", "--all", "--force")
	result.AssertSuccess(t)

	// Canonical files now in source.
	if !sb.FileExists(filepath.Join(sb.SourcePath, "firecrawl", "SKILL.md")) {
		t.Errorf("skill not migrated into source\n%s", result.Output())
	}

	// The original real directory was moved out of the agents dir; whatever
	// remains at that path must be a symlink into source (re-synced), not the
	// original real directory.
	agentsEntry := filepath.Join(agentsPath, "firecrawl")
	if sb.FileExists(agentsEntry) && !sb.IsSymlink(agentsEntry) {
		t.Error("original real directory still present in agents dir after adopt")
	}

	// Trash holds the migrated original (restorable).
	trashBase := filepath.Join(sb.Home, ".local", "share", "skillshare", "trash")
	entries, _ := os.ReadDir(trashBase)
	if len(entries) == 0 {
		t.Error("trash is empty, expected the migrated original")
	}

	// After re-sync the claude target has a symlink for firecrawl pointing into
	// source (the old orphan symlink into the agents dir is gone).
	claudeLink := filepath.Join(claudePath, "firecrawl")
	if !sb.IsSymlink(claudeLink) {
		t.Fatalf("expected a symlink for firecrawl in claude target\n%s", result.Output())
	}
	tgt := sb.SymlinkTarget(claudeLink)
	if want := filepath.Join(sb.SourcePath, "firecrawl"); tgt != want {
		t.Errorf("claude symlink target = %q, want %q (should point into source, not agents)", tgt, want)
	}
}

func TestAdopt_LockfileUnchanged_WithWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsPath, _ := setupAdoptGlobal(t, sb, "firecrawl", "firecrawl")

	lockPath := filepath.Join(filepath.Dir(agentsPath), ".skill-lock.json")
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock before: %v", err)
	}

	result := sb.RunCLI("adopt", "-g", "--all", "--force")
	result.AssertSuccess(t)

	// Output warns about the lingering lock entry.
	result.AssertAnyOutputContains(t, ".skill-lock.json")

	// Lockfile is byte-for-byte unchanged on disk.
	after, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("lockfile was modified:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestAdopt_SameNameConflict_NotOverwrittenWithoutForce(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Source already has a skill with the same name.
	sb.CreateSkill("firecrawl", map[string]string{"SKILL.md": "# Source Version"})

	agentsPath, _ := setupAdoptGlobal(t, sb, "firecrawl", "firecrawl")

	// --all without --force must not silently overwrite the source copy.
	result := sb.RunCLI("adopt", "-g", "--all")
	result.AssertSuccess(t)

	// Source content preserved.
	content, err := os.ReadFile(filepath.Join(sb.SourcePath, "firecrawl", "SKILL.md"))
	if err != nil {
		t.Fatalf("read source skill: %v", err)
	}
	if string(content) != "# Source Version" {
		t.Errorf("source overwritten without --force, got: %q", string(content))
	}

	// Original must remain in the agents dir since it was not adopted.
	if !sb.FileExists(filepath.Join(agentsPath, "firecrawl", "SKILL.md")) {
		t.Error("conflicting original was trashed even though it was skipped")
	}
}

func TestAdopt_MissingAgentsDir_NoOp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Universal target configured, but the agents skills dir does not exist.
	agentsPath := filepath.Join(sb.Home, ".agents", "skills")
	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  universal:
    path: ` + agentsPath + `
  claude:
    path: ` + claudePath + `
`)

	result := sb.RunCLI("adopt", "-g", "--all", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No adoptable skills")
}

func TestAdopt_NonInteractive_BareRefuses(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsPath, claudePath := setupAdoptGlobal(t, sb, "firecrawl", "firecrawl")

	// A bare run in a non-interactive terminal (the test harness has no TTY)
	// must NOT migrate or trash anything: it refuses and points at a flag.
	result := sb.RunCLI("adopt", "-g")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Non-interactive terminal")

	// Nothing migrated into source.
	if sb.FileExists(filepath.Join(sb.SourcePath, "firecrawl")) {
		t.Error("bare non-interactive adopt migrated a skill")
	}
	// Original left untouched in the agents dir.
	if !sb.FileExists(filepath.Join(agentsPath, "firecrawl", "SKILL.md")) {
		t.Error("bare non-interactive adopt trashed the original")
	}
	// Orphan symlink left untouched.
	if !sb.FileExists(filepath.Join(claudePath, "firecrawl")) {
		t.Error("bare non-interactive adopt removed the orphan symlink")
	}
}

func TestAdoptProject_MigratesAndResyncs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Project with claude + universal targets (both resolve under the project).
	projectRoot := sb.SetupProjectDir("claude")
	sb.WriteProjectConfig(projectRoot, "targets:\n  - claude\n  - universal\n")

	// Project-level agents skills dir (.agents/skills) with a real external skill.
	agentsPath := filepath.Join(projectRoot, ".agents", "skills")
	if err := os.MkdirAll(agentsPath, 0o755); err != nil {
		t.Fatalf("mkdir project agents: %v", err)
	}
	skillPath := adoptWriteSkill(t, agentsPath, "firecrawl")

	// Orphan symlink in the claude target.
	claudeDir := filepath.Join(projectRoot, ".claude", "skills")
	os.MkdirAll(claudeDir, 0o755)
	if err := os.Symlink(skillPath, filepath.Join(claudeDir, "firecrawl")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Lockfile claims the skill.
	adoptWriteLock(t, agentsPath, "firecrawl", "firecrawl")

	result := sb.RunCLIInDir(projectRoot, "adopt", "-p", "--all", "--force")
	result.AssertSuccess(t)

	// Migrated into the project source.
	projectSource := filepath.Join(projectRoot, ".skillshare", "skills")
	if !sb.FileExists(filepath.Join(projectSource, "firecrawl", "SKILL.md")) {
		t.Errorf("skill not migrated into project source\n%s", result.Output())
	}

	// The original real directory was moved out of the project agents dir; any
	// remaining entry must be a re-synced symlink into source, not the original.
	agentsEntry := filepath.Join(agentsPath, "firecrawl")
	if sb.FileExists(agentsEntry) && !sb.IsSymlink(agentsEntry) {
		t.Error("original real directory still present in project agents dir after adopt")
	}

	// Trashed into the project trash dir (ProjectTrashDir).
	projectTrash := filepath.Join(projectRoot, ".skillshare", "trash")
	if entries, _ := os.ReadDir(projectTrash); len(entries) == 0 {
		t.Error("project trash is empty, expected the migrated original")
	}

	// After re-sync the claude target has a symlink for firecrawl into source.
	claudeLink := filepath.Join(claudeDir, "firecrawl")
	if !sb.IsSymlink(claudeLink) {
		t.Errorf("expected re-synced symlink for firecrawl in claude target\n%s", result.Output())
	} else if tgt := sb.SymlinkTarget(claudeLink); tgt != filepath.Join(projectSource, "firecrawl") {
		t.Errorf("claude symlink target = %q, want into project source", tgt)
	}

	result.AssertAnyOutputContains(t, "firecrawl")
}
