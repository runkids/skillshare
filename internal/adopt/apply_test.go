package adopt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
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

func TestApply_ForceRefusesManagedSourceConflict(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	candidate := mkSkill(t, agents, "managed")
	sourceSkill := mkSkill(t, source, "managed")
	if err := os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("source original"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("managed", &install.MetadataEntry{Source: "github.com/example/managed"})
	if err := store.Save(source); err != nil {
		t.Fatal(err)
	}

	res, err := Apply([]Candidate{{Name: "managed", Path: candidate, Conflict: true}}, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 0 || res.Failed["managed"] == "" {
		t.Fatalf("result = %+v, want managed ownership failure", res)
	}
	data, readErr := os.ReadFile(filepath.Join(sourceSkill, "SKILL.md"))
	if readErr != nil || string(data) != "source original" {
		t.Fatalf("managed source was replaced: data=%q err=%v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(candidate, "SKILL.md")); statErr != nil {
		t.Fatalf("external original was moved despite refusal: %v", statErr)
	}
}

func TestApply_RefusesAbsentSourceWithLingeringInstallMetadata(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	candidate := mkSkill(t, agents, "managed")
	store := install.NewMetadataStore()
	store.Set("managed", &install.MetadataEntry{Source: "github.com/example/managed"})
	if err := store.Save(source); err != nil {
		t.Fatal(err)
	}

	res, err := Apply([]Candidate{{Name: "managed", Path: candidate}}, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 0 || res.Failed["managed"] == "" {
		t.Fatalf("result = %+v, want lingering ownership failure", res)
	}
	if _, statErr := os.Stat(filepath.Join(source, "managed")); !os.IsNotExist(statErr) {
		t.Fatalf("managed destination was created: %v", statErr)
	}
}

func TestApply_RefusesProjectReplayEntryWithoutInstallMetadata(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	candidate := mkSkill(t, agents, "managed")

	res, err := Apply([]Candidate{{Name: "managed", Path: candidate}}, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Force:      true,
		ProjectSkills: []config.SkillEntry{{
			Name:   "managed",
			Source: "github.com/example/managed",
		}},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 0 || res.Failed["managed"] == "" {
		t.Fatalf("result = %+v, want project replay ownership failure", res)
	}
	if _, statErr := os.Stat(filepath.Join(source, "managed")); !os.IsNotExist(statErr) {
		t.Fatalf("project-managed destination was created: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(candidate, "SKILL.md")); statErr != nil {
		t.Fatalf("external original was moved despite refusal: %v", statErr)
	}
}

func TestApply_DryRunReadsLegacyMetadataWithoutMigratingIt(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	candidate := mkSkill(t, agents, "managed")
	sourceSkill := mkSkill(t, source, "managed")
	if err := install.WriteMeta(sourceSkill, &install.SkillMeta{Source: "github.com/example/managed"}); err != nil {
		t.Fatal(err)
	}
	sidecarPath := filepath.Join(sourceSkill, install.MetaFileName)
	before, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatal(err)
	}

	res, err := Apply([]Candidate{{Name: "managed", Path: candidate, Conflict: true}}, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		DryRun:     true,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 0 || res.Failed["managed"] == "" {
		t.Fatalf("result = %+v, want legacy ownership failure", res)
	}
	after, readErr := os.ReadFile(sidecarPath)
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("legacy sidecar changed during dry-run: data=%q err=%v", after, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(source, install.MetadataFileName)); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run created centralized metadata: %v", statErr)
	}
}

func TestApply_LockfileUntouchedAndWarned(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")

	lockPath := LockPath(agents) // ~/.agents/.skill-lock.json — beside the skills dir, not inside it
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

// TestApply_ReSyncHonorsCopyMode guards the re-sync mode dispatch: a copy-mode
// target must receive a real directory copy, not a merge-mode symlink.
func TestApply_ReSyncHonorsCopyMode(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")

	// A separate target configured for copy mode, beside the agents dir.
	copyTargetPath := filepath.Join(filepath.Dir(agents), "copytarget")
	if err := os.MkdirAll(copyTargetPath, 0755); err != nil {
		t.Fatal(err)
	}
	targets := map[string]config.TargetConfig{
		"copytarget": {Path: copyTargetPath, Mode: "copy"},
	}

	cands := detect(t, agents, source)
	res, err := Apply(cands, Request{
		AgentsPath:  agents,
		SourcePath:  source,
		TrashBase:   trashBase,
		Targets:     targets,
		DefaultMode: "merge", // config default is merge; the target's own copy mode must win
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 {
		t.Fatalf("adopted = %v, want 1", res.Adopted)
	}

	// Copy mode must produce a REAL entry in the target, never a symlink.
	entry := filepath.Join(copyTargetPath, "web-scraper")
	info, err := os.Lstat(entry)
	if err != nil {
		t.Fatalf("copy-mode target entry missing: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("copy mode produced a symlink; want a real copy")
	}
	if _, err := os.Stat(filepath.Join(entry, "SKILL.md")); err != nil {
		t.Errorf("copied skill missing SKILL.md: %v", err)
	}
}

// Adopt's --force flag only permits replacing the selected skill in the source
// tree. It must not broaden a later copy-mode sync into overwriting unrelated
// unmanaged target entries.
func TestApply_ForcePreservesUnrelatedCopyTargetEntry(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")
	mkSkill(t, source, "unrelated")

	copyTargetPath := filepath.Join(filepath.Dir(agents), "copytarget")
	localSkill := mkSkill(t, copyTargetPath, "unrelated")
	localContent := []byte("local unmanaged content")
	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), localContent, 0644); err != nil {
		t.Fatal(err)
	}

	targets := map[string]config.TargetConfig{
		"copytarget": {Path: copyTargetPath, Mode: "copy"},
	}
	res, err := Apply(detect(t, agents, source), Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Targets:    targets,
		Force:      true,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "web-scraper" {
		t.Fatalf("adopted = %v, want [web-scraper]", res.Adopted)
	}

	after, err := os.ReadFile(filepath.Join(localSkill, "SKILL.md"))
	if err != nil {
		t.Fatalf("read unrelated target skill: %v", err)
	}
	if string(after) != string(localContent) {
		t.Fatalf("unrelated copy-target entry was overwritten: got %q, want %q", after, localContent)
	}
}

func TestApply_ReSyncHonorsFileIgnorePatterns(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	externalSkill := mkSkill(t, agents, "web-scraper")
	if err := os.WriteFile(filepath.Join(externalSkill, "local.cache"), []byte("transient"), 0644); err != nil {
		t.Fatal(err)
	}

	copyTargetPath := filepath.Join(filepath.Dir(agents), "copytarget")
	targets := map[string]config.TargetConfig{
		"copytarget": {Path: copyTargetPath, Mode: "copy"},
	}
	_, err := Apply(detect(t, agents, source), Request{
		AgentsPath:         agents,
		SourcePath:         source,
		TrashBase:          trashBase,
		Targets:            targets,
		FileIgnorePatterns: []string{"local.cache"},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	targetSkill := filepath.Join(copyTargetPath, "web-scraper")
	if _, err := os.Stat(filepath.Join(targetSkill, "SKILL.md")); err != nil {
		t.Fatalf("copy target missing adopted skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetSkill, "local.cache")); !os.IsNotExist(err) {
		t.Fatalf("ignored file was copied during adopt re-sync: %v", err)
	}
}

func TestApply_PrunesOnlyLinksRecordedForAdoptedSkills(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	targetPath := filepath.Join(filepath.Dir(agents), "target")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}

	adoptedLink := filepath.Join(targetPath, "web-scraper")
	if err := os.Symlink(skillDir, adoptedLink); err != nil {
		t.Fatal(err)
	}
	unrelatedLink := filepath.Join(targetPath, "keep-me")
	if err := os.Symlink(filepath.Join(filepath.Dir(agents), "missing-external-skill"), unrelatedLink); err != nil {
		t.Fatal(err)
	}

	allTargets := map[string]string{"target": targetPath}
	candidates, err := DetectAdoptable(agents, source, "merge", allTargets)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	res, err := Apply(candidates, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.PrunedLinks != 1 {
		t.Fatalf("pruned links = %d, want only the adopted skill link", res.PrunedLinks)
	}
	if _, err := os.Lstat(adoptedLink); !os.IsNotExist(err) {
		t.Fatalf("recorded adopted link was not removed: %v", err)
	}
	if _, err := os.Lstat(unrelatedLink); err != nil {
		t.Fatalf("unrelated broken link was removed: %v", err)
	}
}

func TestApply_DeduplicatesAliasedCleanupLinks(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	skillDir := mkSkill(t, agents, "web-scraper")
	targetPath := filepath.Join(filepath.Dir(agents), "target")
	targetAlias := filepath.Join(filepath.Dir(agents), "target-alias")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, targetAlias); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(targetPath, "web-scraper")
	if err := os.Symlink(skillDir, linkPath); err != nil {
		t.Fatal(err)
	}

	candidates, err := DetectAdoptable(agents, source, "merge", map[string]string{"target": targetPath})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %v, want one", candidates)
	}
	// Simulate an older/stale preview that recorded two path aliases for the
	// same physical link. Apply must remain idempotent at the destructive seam.
	candidates[0].ExternalLinks = append(candidates[0].ExternalLinks, filepath.Join(targetAlias, "web-scraper"))

	res, err := Apply(candidates, Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.PrunedLinks != 1 {
		t.Fatalf("pruned links = %d, want one physical link", res.PrunedLinks)
	}
	if len(res.Failed) != 0 {
		t.Fatalf("aliased cleanup produced a false failure: %v", res.Failed)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Fatalf("recorded link still exists: %v", err)
	}
}

func TestApply_ReportsTargetResyncFailures(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")
	blockedTarget := filepath.Join(filepath.Dir(agents), "not-a-directory")
	if err := os.WriteFile(blockedTarget, []byte("occupied"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(detect(t, agents, source), Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Targets: map[string]config.TargetConfig{
			"broken": {Path: blockedTarget, Mode: "merge"},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Adopted) != 1 || res.Adopted[0] != "web-scraper" {
		t.Fatalf("adopted = %v, want [web-scraper]", res.Adopted)
	}
	if res.Failed["sync broken"] == "" {
		t.Fatalf("target re-sync failure was not reported: %v", res.Failed)
	}
}

func TestApply_ReportsAdoptedSkillTargetCollisions(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			agents, source, trashBase := applyEnv(t)
			mkSkill(t, agents, "web-scraper")
			targetPath := filepath.Join(filepath.Dir(agents), mode+"-target")
			localSkill := mkSkill(t, targetPath, "web-scraper")
			localContent := []byte("local target content")
			if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), localContent, 0644); err != nil {
				t.Fatal(err)
			}

			res, err := Apply(detect(t, agents, source), Request{
				AgentsPath: agents,
				SourcePath: source,
				TrashBase:  trashBase,
				Targets: map[string]config.TargetConfig{
					"target": {Path: targetPath, Mode: mode},
				},
			})
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if got := res.Failed["sync target/web-scraper"]; got == "" {
				t.Fatalf("adopted-skill target collision was not reported: %v", res.Failed)
			}
			after, err := os.ReadFile(filepath.Join(localSkill, "SKILL.md"))
			if err != nil {
				t.Fatalf("read preserved local skill: %v", err)
			}
			if string(after) != string(localContent) {
				t.Fatalf("local target collision was overwritten: got %q, want %q", after, localContent)
			}
		})
	}
}

func TestApply_SyncsAliasedTargetPathOnce(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "web-scraper")
	targetPath := filepath.Join(filepath.Dir(agents), "target")
	targetAlias := filepath.Join(filepath.Dir(agents), "target-alias")
	localSkill := mkSkill(t, targetPath, "web-scraper")
	localContent := []byte("local target content")
	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), localContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, targetAlias); err != nil {
		t.Fatal(err)
	}

	res, err := Apply(detect(t, agents, source), Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Targets: map[string]config.TargetConfig{
			"first":  {Path: targetPath, Mode: "merge"},
			"second": {Path: targetAlias, Mode: "merge"},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Failed) != 1 || res.Failed["sync first/web-scraper"] == "" {
		t.Fatalf("aliased target produced duplicate or unstable failures: %v", res.Failed)
	}
	after, err := os.ReadFile(filepath.Join(localSkill, "SKILL.md"))
	if err != nil {
		t.Fatalf("read preserved local skill: %v", err)
	}
	if string(after) != string(localContent) {
		t.Fatalf("local target collision was overwritten: got %q, want %q", after, localContent)
	}
}

func TestApply_ReportsAdoptedSkillExcludedByStandardNameCollision(t *testing.T) {
	agents, source, trashBase := applyEnv(t)
	mkSkill(t, agents, "agent-skill")
	mkSkill(t, filepath.Join(source, "team"), "agent-skill")
	targetPath := filepath.Join(filepath.Dir(agents), "target")

	res, err := Apply(detect(t, agents, source), Request{
		AgentsPath: agents,
		SourcePath: source,
		TrashBase:  trashBase,
		Targets: map[string]config.TargetConfig{
			"target": {
				Skills: &config.ResourceTargetConfig{
					Path:         targetPath,
					Mode:         "merge",
					TargetNaming: "standard",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := res.Failed["sync target/agent-skill"]; got == "" {
		t.Fatalf("standard-name collision was not reported: %v", res.Failed)
	}
	if _, err := os.Lstat(filepath.Join(targetPath, "agent-skill")); !os.IsNotExist(err) {
		t.Fatalf("colliding target entry unexpectedly distributed: %v", err)
	}
}

func TestApply_RejectsOverlappingRootsBeforeMutation(t *testing.T) {
	root := t.TempDir()
	skillPath := mkSkill(t, root, "same-root")
	trashBase := filepath.Join(t.TempDir(), "trash")

	res, err := Apply([]Candidate{{Name: "same-root", Path: skillPath}}, Request{
		AgentsPath: root,
		SourcePath: root,
		TrashBase:  trashBase,
		Force:      true,
	})
	if !errors.Is(err, ErrUnsafePathOverlap) {
		t.Fatalf("Apply error = %v, want ErrUnsafePathOverlap", err)
	}
	if len(res.Adopted) != 0 {
		t.Fatalf("adopted = %v, want none", res.Adopted)
	}
	if _, statErr := os.Stat(filepath.Join(skillPath, "SKILL.md")); statErr != nil {
		t.Fatalf("source skill was mutated despite overlap rejection: %v", statErr)
	}
	if entries, readErr := os.ReadDir(trashBase); readErr == nil && len(entries) != 0 {
		t.Fatalf("trash contains entries after overlap rejection: %v", entries)
	}
}
