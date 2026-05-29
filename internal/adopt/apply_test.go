package adopt

import (
	"os"
	"path/filepath"
	"testing"
)

// applyEnv builds an agents dir + source dir + trash base and returns the
// detected candidates ready to feed into Apply.
func applyEnv(t *testing.T) (agents, source, trashBase string) {
	t.Helper()
	tmp := t.TempDir()
	agents = filepath.Join(tmp, "agents")
	source = filepath.Join(tmp, "source")
	trashBase = filepath.Join(tmp, "trash")
	if err := os.MkdirAll(agents, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	return agents, source, trashBase
}

func detect(t *testing.T, agents, source string) []Candidate {
	t.Helper()
	cands, err := DetectAdoptable(agents, source, "merge", nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	return cands
}

func TestApply_DryRunNoMutation(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	cands := detect(t, agents, source)

	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Trashed != 0 || res.PrunedLinks != 0 {
		t.Errorf("dry-run mutated: trashed=%d pruned=%d", res.Trashed, res.PrunedLinks)
	}
	// Original must still exist; source must NOT have received a copy.
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("original removed in dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "web-scraper")); !os.IsNotExist(err) {
		t.Errorf("source mutated in dry-run: %v", err)
	}
}

func TestApply_MigrateAndTrash(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	cands := detect(t, agents, source)

	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "web-scraper" {
		t.Fatalf("adopted = %v, want [web-scraper]", res.Adopted)
	}
	if res.Trashed != 1 {
		t.Errorf("trashed = %d, want 1", res.Trashed)
	}
	// Source now owns the skill; original is gone.
	if _, err := os.Stat(filepath.Join(source, "web-scraper", "SKILL.md")); err != nil {
		t.Errorf("skill not migrated to source: %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("original not trashed: %v", err)
	}
}

func TestApply_ConflictSkippedWithoutForce(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	mkSkill(t, source, "web-scraper") // conflict in source
	cands := detect(t, agents, source)

	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 0 {
		t.Errorf("adopted = %v, want none (conflict)", res.Adopted)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("skipped = %v, want 1", res.Skipped)
	}
	if res.Trashed != 0 {
		t.Errorf("trashed = %d, want 0 (skipped conflict must not be trashed)", res.Trashed)
	}
	// Original must survive — never trashed without a successful copy.
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("conflicting original removed: %v", err)
	}
}

func TestApply_ConflictAdoptedWithForce(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	mkSkill(t, source, "web-scraper")
	cands := detect(t, agents, source)

	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 {
		t.Fatalf("adopted = %v, want 1 (force)", res.Adopted)
	}
	if res.Trashed != 1 {
		t.Errorf("trashed = %d, want 1", res.Trashed)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("original not trashed under force: %v", err)
	}
}

func TestApply_LockfileUntouchedAndWarned(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")

	lockPath := filepath.Join(agents, LockFileName())
	lockData := []byte(`{"web-scraper":{"sourceTool":"firecrawl"}}`)
	if err := os.WriteFile(lockPath, lockData, 0644); err != nil {
		t.Fatal(err)
	}

	cands := detect(t, agents, source)
	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Lockfile must be byte-for-byte unchanged (READ-ONLY).
	after, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("lockfile gone: %v", err)
	}
	if string(after) != string(lockData) {
		t.Errorf("lockfile mutated: %s", after)
	}

	// Adopted skill still in lockfile => a warning.
	if len(res.LockWarnings) != 1 {
		t.Fatalf("lock warnings = %v, want 1", res.LockWarnings)
	}
	if res.LockWarnings[0].Name != "web-scraper" || res.LockWarnings[0].SourceTool != "firecrawl" {
		t.Errorf("unexpected warning: %+v", res.LockWarnings[0])
	}
}

func TestApply_SelectedFilter(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "alpha")
	mkSkill(t, agents, "beta")
	cands := detect(t, agents, source)

	res, err := Apply(cands, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Selected:   []string{"alpha"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "alpha" {
		t.Fatalf("adopted = %v, want [alpha]", res.Adopted)
	}
	// beta must remain untouched in agents.
	if _, err := os.Stat(filepath.Join(agents, "beta")); err != nil {
		t.Errorf("unselected beta was touched: %v", err)
	}
}
