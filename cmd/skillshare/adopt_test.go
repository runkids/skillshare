package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/adopt"
)

// writeSkill creates a minimal skill directory with a SKILL.md file.
func writeSkill(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	body := "---\nname: " + name + "\ndescription: test\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

// newAdoptTestContext builds an adoptContext rooted in a temp dir, with an
// agents target dir, a (separate) target dir to host orphan symlinks, a source
// dir, and a trash dir.
func newAdoptTestContext(t *testing.T) (adoptContext, string) {
	t.Helper()
	root := t.TempDir()
	agentsPath := filepath.Join(root, "agents", "skills")
	sourcePath := filepath.Join(root, "source")
	otherTarget := filepath.Join(root, "claude", "skills")
	trashBase := filepath.Join(root, "trash")

	for _, d := range []string{agentsPath, sourcePath, otherTarget} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	actx := adoptContext{
		agentsPath: agentsPath,
		sourcePath: sourcePath,
		syncMode:   "merge",
		allTargets: map[string]string{
			"universal": agentsPath,
			"claude":    otherTarget,
		},
		trashBase:  trashBase,
		configPath: filepath.Join(root, "config.yaml"),
	}
	return actx, root
}

func TestRunAdopt_MigratesSkillAndTrashesOriginal(t *testing.T) {
	actx, _ := newAdoptTestContext(t)

	skillPath := writeSkill(t, actx.agentsPath, "firecrawl")
	// Orphan symlink in the claude target pointing into the agents dir.
	link := filepath.Join(actx.allTargets["claude"], "firecrawl")
	if err := os.Symlink(skillPath, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	res, err := runAdopt(actx, adoptOptions{all: true, force: true})
	if err != nil {
		t.Fatalf("runAdopt: %v", err)
	}

	if len(res.Adopted) != 1 || res.Adopted[0] != "firecrawl" {
		t.Fatalf("adopted = %v, want [firecrawl]", res.Adopted)
	}

	// Canonical file copied into source.
	if _, err := os.Stat(filepath.Join(actx.sourcePath, "firecrawl", "SKILL.md")); err != nil {
		t.Fatalf("skill not copied to source: %v", err)
	}

	// Original removed from agents dir (moved to trash).
	if _, err := os.Stat(filepath.Join(actx.agentsPath, "firecrawl")); !os.IsNotExist(err) {
		t.Fatalf("original still present in agents dir: err=%v", err)
	}

	// Trash contains the skill.
	entries, _ := os.ReadDir(actx.trashBase)
	if len(entries) == 0 {
		t.Fatalf("trash is empty, expected trashed skill")
	}

	if res.Trashed != 1 {
		t.Errorf("trashed = %d, want 1", res.Trashed)
	}
}

func TestRunAdopt_DryRunMakesNoChanges(t *testing.T) {
	actx, _ := newAdoptTestContext(t)
	writeSkill(t, actx.agentsPath, "gws")

	res, err := runAdopt(actx, adoptOptions{all: true, force: true, dryRun: true})
	if err != nil {
		t.Fatalf("runAdopt: %v", err)
	}

	if len(res.Adopted) != 1 {
		t.Fatalf("adopted = %v, want 1 entry", res.Adopted)
	}
	// Source untouched.
	if _, err := os.Stat(filepath.Join(actx.sourcePath, "gws")); !os.IsNotExist(err) {
		t.Errorf("dry-run copied to source: %v", err)
	}
	// Original untouched.
	if _, err := os.Stat(filepath.Join(actx.agentsPath, "gws")); err != nil {
		t.Errorf("dry-run removed original: %v", err)
	}
	if res.Trashed != 0 {
		t.Errorf("dry-run trashed = %d, want 0", res.Trashed)
	}
}

func TestRunAdopt_WarnsOnLingeringLockfile(t *testing.T) {
	actx, _ := newAdoptTestContext(t)
	writeSkill(t, actx.agentsPath, "firecrawl")

	// Lockfile still references the adopted skill.
	lock := map[string]any{
		"skills": map[string]any{
			"firecrawl": map[string]any{"sourceTool": "firecrawl"},
		},
	}
	data, _ := json.Marshal(lock)
	// The lockfile lives beside the skills dir (~/.agents/.skill-lock.json),
	// one level up — matching where production ReadLock looks.
	lockPath := adopt.LockPath(actx.agentsPath)
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	res, err := runAdopt(actx, adoptOptions{all: true, force: true})
	if err != nil {
		t.Fatalf("runAdopt: %v", err)
	}

	if len(res.LockWarnings) != 1 || res.LockWarnings[0].Name != "firecrawl" {
		t.Fatalf("LockWarnings = %v, want one entry for firecrawl", res.LockWarnings)
	}
	if res.LockWarnings[0].SourceTool != "firecrawl" {
		t.Errorf("SourceTool = %q, want firecrawl", res.LockWarnings[0].SourceTool)
	}

	// Lockfile must NOT be modified.
	raw, _ := os.ReadFile(lockPath)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("lockfile became unreadable: %v", err)
	}
	if _, ok := got["skills"]; !ok {
		t.Errorf("lockfile was mutated: %s", raw)
	}
}

func TestRunAdopt_NoCandidates(t *testing.T) {
	actx, _ := newAdoptTestContext(t)
	res, err := runAdopt(actx, adoptOptions{all: true, force: true})
	if err != nil {
		t.Fatalf("runAdopt: %v", err)
	}
	if len(res.Adopted) != 0 {
		t.Errorf("adopted = %v, want empty", res.Adopted)
	}
}

// Ensures Provenance wiring uses the adopt package as the source of truth.
func TestAdoptProvenance(t *testing.T) {
	entries := map[string]adopt.LockEntry{
		"x": {Name: "x", SourceTool: "firecrawl"},
	}
	if got := adopt.Provenance(entries, "x"); got != "firecrawl" {
		t.Errorf("Provenance = %q, want firecrawl", got)
	}
}
