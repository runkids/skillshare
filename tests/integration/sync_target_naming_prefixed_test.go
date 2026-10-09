//go:build !online

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

// prefixedFixture creates two tracked repos that both ship "prototype", plus a
// skill outside any tracked repo, and returns the copy-mode target path.
func prefixedFixture(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{
		"SKILL.md": "---\nname: prototype\ndescription: Emil\n---\n# Emil prototype",
	})
	sb.CreateNestedSkill("_mattpocock-skills/skills/engineering/prototype", map[string]string{
		"SKILL.md": "---\nname: prototype\ndescription: Matt\n---\n# Matt prototype",
	})
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Mine",
	})
	return sb.CreateTarget("claude")
}

func writeNamingConfig(sb *testutil.Sandbox, targetPath, naming, mode string) {
	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: ` + naming + `
targets:
  claude:
    path: ` + targetPath + `
    mode: ` + mode + `
`)
}

func readSyncManifest(t *testing.T, sb *testutil.Sandbox, targetPath string) ssync.Manifest {
	t.Helper()
	var manifest ssync.Manifest
	if err := json.Unmarshal([]byte(sb.ReadFile(filepath.Join(targetPath, ssync.ManifestFile))), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return manifest
}

// assertEntries checks the target holds exactly the given skill folders, each
// with name: equal to its folder.
func assertEntries(t *testing.T, sb *testutil.Sandbox, targetPath string, want ...string) {
	t.Helper()
	got := filterVisible(sb.ListDir(targetPath))
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("target entries = %v, want %v", got, want)
	}
	for _, name := range want {
		if content := sb.ReadFile(filepath.Join(targetPath, name, "SKILL.md")); !strings.Contains(content, "\nname: "+name+"\n") {
			t.Errorf("%s/SKILL.md name: does not match its folder:\n%s", name, content)
		}
	}
}

func TestSync_TargetNamingPrefixed_KeepsSameNamedSkillsFromTwoRepos(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "prefixed", "copy")

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "collision")

	assertEntries(t, sb, targetPath, "emil-design-prototype", "mattpocock-skills-prototype", "my-skill")
	if src := sb.ReadFile(filepath.Join(sb.SourcePath, "_emil-design/skills/prototype/SKILL.md")); !strings.Contains(src, "\nname: prototype\n") {
		t.Fatalf("source SKILL.md was modified:\n%s", src)
	}
	if got := readSyncManifest(t, sb, targetPath).Naming["emil-design-prototype"]; got != "prefixed" {
		t.Fatalf("manifest naming = %q, want prefixed", got)
	}

	// A second sync finds every copy current.
	again := sb.RunCLI("sync", "--json")
	again.AssertSuccess(t)
	var summary struct{ Linked, Updated int }
	if err := json.Unmarshal([]byte(again.Stdout), &summary); err != nil {
		t.Fatalf("parse sync --json: %v\n%s", err, again.Stdout)
	}
	if summary.Linked != 0 || summary.Updated != 0 {
		t.Fatalf("second sync copied again: %+v", summary)
	}
}

func TestSync_TargetNamingPrefixed_RejectedOutsideCopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "prefixed", "merge")

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, `target naming "prefixed" requires copy mode`)
	result.AssertAnyOutputContains(t, "set mode: copy on the target")
	if entries := sb.ListDir(targetPath); len(entries) != 0 {
		t.Fatalf("merge target was written: %v", entries)
	}
}

func TestPrefixedNamingOutsideCopyMode_IsFlaggedBeforeSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "prefixed", "merge")

	for _, args := range [][]string{{"status"}, {"status", "--json"}, {"doctor"}, {"target", "list", "--no-tui"}, {"target", "list", "--json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			sb.RunCLI(args...).AssertAnyOutputContains(t, "set mode: copy on the target")
		})
	}
}

func TestSync_TargetNamingPrefixed_SwitchesRenameInPlace(t *testing.T) {
	steps := []struct {
		naming string
		want   []string
	}{
		{"flat", []string{"_emil-design__skills__prototype", "_mattpocock-skills__skills__engineering__prototype", "my-skill"}},
		{"prefixed", []string{"emil-design-prototype", "mattpocock-skills-prototype", "my-skill"}},
		{"standard", []string{"my-skill"}}, // both prototypes collide under standard
		{"prefixed", []string{"emil-design-prototype", "mattpocock-skills-prototype", "my-skill"}},
	}

	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	for _, step := range steps {
		writeNamingConfig(sb, targetPath, step.naming, "copy")
		sb.RunCLI("sync").AssertSuccess(t)
		// Flat copies keep the source name:, so only check the folder list there.
		if step.naming == "flat" {
			if got := strings.Join(filterVisible(sb.ListDir(targetPath)), ","); got != strings.Join(step.want, ",") {
				t.Fatalf("after %s: entries = %s, want %v", step.naming, got, step.want)
			}
			continue
		}
		assertEntries(t, sb, targetPath, step.want...)
	}
}

func TestSync_TargetNamingPrefixed_StandardToPrefixedRenamesAndRewritesName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{
		"SKILL.md": "---\nname: prototype\n---\n# Emil prototype",
	})
	targetPath := sb.CreateTarget("claude")

	writeNamingConfig(sb, targetPath, "standard", "copy")
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "prototype")

	// The source is unchanged, so only the recorded naming forces the re-copy
	// that rewrites name:.
	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "emil-design-prototype")

	writeNamingConfig(sb, targetPath, "standard", "copy")
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "prototype")
	manifest := readSyncManifest(t, sb, targetPath)
	if _, ok := manifest.Managed["emil-design-prototype"]; ok || manifest.Naming["prototype"] != "standard" {
		t.Fatalf("manifest = %+v, want only prototype recorded as standard", manifest)
	}
}

func TestSync_TargetNamingPrefixed_MigratesManifestWithoutNamingRecords(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{
		"SKILL.md": "---\nname: prototype\n---\n# Emil prototype",
	})
	targetPath := sb.CreateTarget("claude")

	writeNamingConfig(sb, targetPath, "standard", "copy")
	sb.RunCLI("sync").AssertSuccess(t)

	// Rewrite the manifest the way releases before naming records wrote it.
	manifest := readSyncManifest(t, sb, targetPath)
	manifest.Naming = nil
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"naming"`) {
		t.Fatalf("old-style manifest still has naming: %s", data)
	}
	sb.WriteFile(filepath.Join(targetPath, ssync.ManifestFile), string(data))

	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "emil-design-prototype")
	if got := readSyncManifest(t, sb, targetPath).Naming["emil-design-prototype"]; got != "prefixed" {
		t.Fatalf("manifest naming = %q, want prefixed", got)
	}
}

func filterVisible(names []string) []string {
	var out []string
	for _, n := range names {
		if !strings.HasPrefix(n, ".") {
			out = append(out, n)
		}
	}
	return out
}

func TestTarget_SetPrefixedNamingNeedsCopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "flat", "merge")

	rejected := sb.RunCLI("target", "claude", "--target-naming", "prefixed")
	rejected.AssertFailure(t)
	rejected.AssertAnyOutputContains(t, "set --mode copy first")

	sb.RunCLI("target", "claude", "--mode", "copy").AssertSuccess(t)
	sb.RunCLI("target", "claude", "--target-naming", "prefixed").AssertSuccess(t)
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "emil-design-prototype", "mattpocock-skills-prototype", "my-skill")
}

func TestSync_TargetNaming_OldManifestDoesNotTakeAnotherSkillsEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# top"})
	// "aaa/dev" syncs before "dev", so its lookup runs before dev records its naming.
	sb.CreateNestedSkill("aaa/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# nested"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "flat", "copy")
	sb.RunCLI("sync").AssertSuccess(t)

	// Under an old manifest, "dev" is also aaa/dev's standard name; it
	// still belongs to the top-level skill that syncs to it.
	manifest := readSyncManifest(t, sb, targetPath)
	manifest.Naming = nil
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(targetPath, ssync.ManifestFile), string(data))

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "kept legacy")
	if got := sb.ReadFile(filepath.Join(targetPath, "dev", "SKILL.md")); !strings.Contains(got, "# top") {
		t.Fatalf("dev was overwritten:\n%s", got)
	}
	if got := sb.ReadFile(filepath.Join(targetPath, "aaa__dev", "SKILL.md")); !strings.Contains(got, "# nested") {
		t.Fatalf("aaa__dev has the wrong content:\n%s", got)
	}
}

func TestSync_TargetNamingPrefixed_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "_emil-design/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# Emil"})
	sb.CreateProjectSkill(projectRoot, "_mattpocock-skills/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# Matt"})
	sb.WriteProjectConfig(projectRoot, `target_naming: prefixed
targets:
  - name: claude
    skills:
      mode: copy
`)

	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)
	assertEntries(t, sb, filepath.Join(projectRoot, ".claude", "skills"), "emil-design-prototype", "mattpocock-skills-prototype")
}

func TestDiff_TargetNamingChange_ReportsRename(t *testing.T) {
	for _, tc := range []struct{ mode, from, to, oldName, newName string }{
		{"copy", "standard", "prefixed", "prototype", "emil-design-prototype"},
		{"merge", "flat", "standard", "_emil-design__skills__prototype", "prototype"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{
				"SKILL.md": "---\nname: prototype\n---\n# Emil prototype",
			})
			targetPath := sb.CreateTarget("claude")
			writeNamingConfig(sb, targetPath, tc.from, tc.mode)
			sb.RunCLI("sync").AssertSuccess(t)

			writeNamingConfig(sb, targetPath, tc.to, tc.mode)
			result := sb.RunCLI("diff", "--no-tui")
			result.AssertSuccess(t)
			result.AssertRowContains(t, "Renamed", tc.newName)
			result.AssertOutputNotContains(t, "New")
		})
	}
}

func TestTargetAdd_GlobalPrefixedNamingUnderMergeUsesCopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	sb.WriteConfig(strings.Replace(sb.ReadFile(sb.ConfigPath), "targets:", "mode: merge\ntargets:", 1))

	added := sb.RunCLI("target", "add", "cursor", sb.CreateTarget("cursor"))
	added.AssertSuccess(t)
	added.AssertOutputContains(t, "mode copy")
	sb.RunCLI("sync").AssertOutputNotContains(t, "requires copy mode")
}

func TestDiff_StaleRecordedNamingIsModified(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	sb.RunCLI("sync").AssertSuccess(t)

	// As if a sync renamed the entry and saved the manifest, then stopped before re-copying it.
	manifest := readSyncManifest(t, sb, targetPath)
	manifest.Naming["emil-design-prototype"] = "standard"
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	sb.WriteFile(filepath.Join(targetPath, ssync.ManifestFile), string(data))

	result := sb.RunCLI("diff", "--no-tui")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "Modified", "emil-design-prototype")
}

func TestSync_TargetNamingPrefixed_ReportsCollisionsOnlyThePrefixCreates(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// Different names, but _a's b-c becomes a-b-c, the local skill's name.
	sb.CreateNestedSkill("_a/b-c", map[string]string{"SKILL.md": "---\nname: b-c\n---\n# A"})
	sb.CreateSkill("a-b-c", map[string]string{"SKILL.md": "---\nname: a-b-c\n---\n# Local"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "prefixed", "copy")

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "duplicate skill names")
	result.AssertOutputContains(t, "_a/ vs a-b-c/")
	// The tracked skill cannot be renamed in SKILL.md; say what prefixing did and what to change.
	result.AssertOutputContains(t, "A tracked skill cannot be renamed in SKILL.md")
	result.AssertOutputContains(t, "--name")
	result.AssertOutputNotContains(t, "Rename one in SKILL.md")
}

func TestSync_TargetNamingPrefixed_CollisionHintStaysWhenSourceAlreadyHasDuplicates(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// Two ordinary skills already share a name; the tracked skill joins them once prefixed.
	sb.CreateNestedSkill("_alpha/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# A"})
	sb.CreateSkill("alpha-prototype", map[string]string{"SKILL.md": "---\nname: alpha-prototype\n---\n# One"})
	sb.CreateNestedSkill("team/alpha-prototype", map[string]string{"SKILL.md": "---\nname: alpha-prototype\n---\n# Two"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "prefixed", "copy")

	result := sb.RunCLI("sync")
	result.AssertOutputContains(t, "A tracked skill cannot be renamed in SKILL.md")
	result.AssertOutputContains(t, "--name")
	// Re-tracking leaves the two ordinary skills clashing, so their remedy stays too.
	result.AssertOutputContains(t, "Rename one in SKILL.md")
}

func TestSync_TargetNamingPrefixed_CollisionPrintsBothHintsAcrossTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// On the prefixed target the tracked skill joins the clash; on the standard one only the ordinary pair clashes.
	sb.CreateNestedSkill("_alpha/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# A"})
	sb.CreateSkill("alpha-prototype", map[string]string{"SKILL.md": "---\nname: alpha-prototype\n---\n# One"})
	sb.CreateNestedSkill("team/alpha-prototype", map[string]string{"SKILL.md": "---\nname: alpha-prototype\n---\n# Two"})
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
      mode: copy
      target_naming: prefixed
  codex:
    skills:
      path: ` + sb.CreateTarget("codex") + `
      mode: copy
      target_naming: standard
`)

	result := sb.RunCLI("sync")
	result.AssertOutputContains(t, "A tracked skill cannot be renamed in SKILL.md")
	result.AssertOutputContains(t, "Rename one in SKILL.md")
}

func TestSync_TargetNamingPrefixed_SameRepoDuplicateStillOffersFilters(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// Re-tracking cannot separate two skills of one repo, so filters must be offered.
	sb.CreateNestedSkill("_repo/a/dup", map[string]string{"SKILL.md": "---\nname: dup\n---\n# A"})
	sb.CreateNestedSkill("_repo/b/dup", map[string]string{"SKILL.md": "---\nname: dup\n---\n# B"})
	writeNamingConfig(sb, sb.CreateTarget("claude"), "prefixed", "copy")

	sb.RunCLI("sync").AssertOutputContains(t, "adjust include/exclude filters")
}

func TestSync_TargetNamingPrefixed_UnderscoreFolderWithoutRepoIsNotTracked(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	// "_drafts" is an ordinary folder here (no .git), so nothing is tracked or re-trackable.
	sb.CreateNestedSkill("org/_drafts/dup", map[string]string{"SKILL.md": "---\nname: dup\n---\n# One"})
	sb.CreateSkill("dup", map[string]string{"SKILL.md": "---\nname: dup\n---\n# Two"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "prefixed", "copy")

	result := sb.RunCLI("sync")
	result.AssertOutputContains(t, "Rename one in SKILL.md")
	result.AssertOutputNotContains(t, "A tracked skill cannot be renamed")
}

func TestSync_TargetNamingPrefixed_FromMergeFlatPrunesLinksAndCopies(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	sb.CreateNestedSkill("_bmad/skills/bmad-ux", map[string]string{"SKILL.md": "---\nname: bmad-ux\n---\n# UX"})
	writeNamingConfig(sb, targetPath, "flat", "merge")
	sb.RunCLI("sync").AssertSuccess(t)

	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	// Prefixing keeps the two prototypes apart, so there is nothing to report.
	result.AssertOutputNotContains(t, "duplicate skill names")
	assertEntries(t, sb, targetPath, "bmad-ux", "emil-design-prototype", "mattpocock-skills-prototype", "my-skill")
}

func TestTarget_SetModeAndNamingInOneCommand(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "flat", "merge")

	// Each order of the two settings must pass through a valid pair.
	sb.RunCLI("target", "claude", "--mode", "copy", "--target-naming", "prefixed").AssertSuccess(t)
	sb.RunCLI("sync").AssertSuccess(t)
	assertEntries(t, sb, targetPath, "emil-design-prototype", "mattpocock-skills-prototype", "my-skill")

	sb.RunCLI("target", "claude", "--target-naming", "flat", "--mode", "merge").AssertSuccess(t)
	if cfg := sb.ReadFile(sb.ConfigPath); !strings.Contains(cfg, "mode: merge") || !strings.Contains(cfg, "target_naming: flat") {
		t.Fatalf("expected merge + flat in config:\n%s", cfg)
	}

	rejected := sb.RunCLI("target", "claude", "--mode", "merge", "--target-naming", "prefixed")
	rejected.AssertFailure(t)
	rejected.AssertAnyOutputContains(t, "requires copy mode")
	if cfg := sb.ReadFile(sb.ConfigPath); strings.Contains(cfg, "prefixed") {
		t.Fatalf("a rejected pair must not change the config:\n%s", cfg)
	}
}

func TestSync_TargetNamingPrefixed_FromMergeKeepsLinkWhenLocalSkillTakesNewName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_bmad/skills/ux", map[string]string{"SKILL.md": "---\nname: ux\n---\n# UX"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "flat", "merge")
	sb.RunCLI("sync").AssertSuccess(t)

	local := filepath.Join(targetPath, "bmad-ux", "SKILL.md")
	sb.WriteFile(local, "---\nname: bmad-ux\n---\n# Mine")
	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "kept legacy managed entry _bmad__skills__ux")

	legacy := filepath.Join(targetPath, "_bmad__skills__ux")
	if sb.IsSymlink(legacy) || !sb.FileExists(filepath.Join(legacy, "SKILL.md")) {
		t.Fatalf("expected %s to stay as a managed copy", legacy)
	}
	if got := sb.ReadFile(local); !strings.Contains(got, "# Mine") {
		t.Fatalf("local skill changed:\n%s", got)
	}
}

func TestDiff_LocalFolderOnNewName_ReportsKeptLegacyEntry(t *testing.T) {
	for _, tc := range []struct{ mode, from, to, newName, oldName string }{
		{"copy", "standard", "prefixed", "emil-design-prototype", "prototype"},
		{"merge", "flat", "standard", "prototype", "_emil-design__skills__prototype"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{
				"SKILL.md": "---\nname: prototype\n---\n# Emil prototype",
			})
			targetPath := sb.CreateTarget("claude")
			writeNamingConfig(sb, targetPath, tc.from, tc.mode)
			sb.RunCLI("sync").AssertSuccess(t)
			sb.WriteFile(filepath.Join(targetPath, tc.newName, "SKILL.md"), "---\nname: "+tc.newName+"\n---\n# Mine")

			// Sync keeps the old entry and leaves the folder alone, even with --force.
			writeNamingConfig(sb, targetPath, tc.to, tc.mode)
			result := sb.RunCLI("diff", "--no-tui")
			result.AssertSuccess(t)
			result.AssertRowContains(t, "Local only, skill kept under old name", tc.newName+" (stays at "+tc.oldName+")")
			result.AssertOutputNotContains(t, "sync --force")
			result.AssertOutputNotContains(t, "to sync")
			result.AssertOutputNotContains(t, "in sync")
			result.AssertOutputContains(t, "1 target: 1 kept under old name")
			result.AssertOutputContains(t, "after renaming or removing the local folders")
			if got := diffJSONActions(t, sb)[tc.newName]; got != "kept" {
				t.Errorf("diff --json action for %s = %q, want kept", tc.newName, got)
			}
			// Sync leaves the folder alone, so its files are not shown as deletions.
			sb.RunCLI("diff", "--no-tui", "--stat").AssertOutputNotContains(t, "SKILL.md")
		})
	}
}

func TestSync_TargetNamingPrefixed_LeavesUnmanagedLinkAlone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_bmad/skills/ux", map[string]string{"SKILL.md": "---\nname: ux\n---\n# UX"})
	targetPath := sb.CreateTarget("claude")
	// The user's own link, under the flat name, with no manifest record.
	userLink := filepath.Join(targetPath, "_bmad__skills__ux")
	sb.CreateSymlink(filepath.Join(sb.SourcePath, "_bmad", "skills", "ux"), userLink)
	writeNamingConfig(sb, targetPath, "prefixed", "copy")
	sb.RunCLI("sync").AssertSuccess(t)

	if !sb.IsSymlink(userLink) {
		t.Fatal("an unmanaged link must not be migrated")
	}
	if sb.IsSymlink(filepath.Join(targetPath, "bmad-ux")) || !sb.FileExists(filepath.Join(targetPath, "bmad-ux", "SKILL.md")) {
		t.Fatal("expected a managed copy at bmad-ux")
	}
}

func TestTarget_SetModeAndNamingReportsInheritedNaming(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := prefixedFixture(t, sb)
	writeNamingConfig(sb, targetPath, "standard", "copy") // claude inherits standard

	result := sb.RunCLI("target", "claude", "--mode", "copy", "--target-naming", "prefixed")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "target naming: standard -> prefixed")
}

// diffJSONActions maps each item name in diff --json to its action.
func diffJSONActions(t *testing.T, sb *testutil.Sandbox) map[string]string {
	t.Helper()
	result := sb.RunCLI("diff", "--json")
	result.AssertSuccess(t)
	var out struct {
		Targets []struct {
			Items []struct{ Name, Action string } `json:"items"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.Stdout)
	}
	actions := map[string]string{}
	for _, target := range out.Targets {
		for _, item := range target.Items {
			actions[item.Name] = item.Action
		}
	}
	return actions
}

func TestDiff_KeptOnlyTargetIsCountedInSummary(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# Emil"})
	claude := sb.CreateTarget("claude")
	cursor := sb.CreateTarget("cursor")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: copy
targets:
  claude:
    path: ` + claude + `
  cursor:
    path: ` + cursor + `
`)
	sb.RunCLI("sync").AssertSuccess(t)
	sb.WriteFile(filepath.Join(claude, "emil-design-prototype", "SKILL.md"), "---\nname: emil-design-prototype\n---\n# Mine")
	sb.RunCLI("target", "claude", "--target-naming", "prefixed").AssertSuccess(t)

	result := sb.RunCLI("diff", "--no-tui")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "2 targets: 1 kept under old name, 1 in sync")
}

// A merge-made link renamed by a copy-mode migration points at the source,
// so the rename has no file changes to show, and a dry run copies to the new name.
func TestDiff_RenamedMergeLinkShowsNoFileChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("_emil-design/skills/prototype", map[string]string{"SKILL.md": "---\nname: prototype\n---\n# Emil"})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "flat", "merge")
	sb.RunCLI("sync").AssertSuccess(t)
	sb.RunCLI("target", "claude", "--mode", "copy", "--target-naming", "prefixed").AssertSuccess(t)

	result := sb.RunCLI("diff", "--no-tui")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "Renamed", "emil-design-prototype")
	result.AssertOutputNotContains(t, "bytes")

	dryRun := sb.RunCLI("sync", "--dry-run")
	dryRun.AssertSuccess(t)
	dryRun.AssertAnyOutputContains(t, "-> "+filepath.Join(targetPath, "emil-design-prototype"))
	if out := dryRun.Stdout + dryRun.Stderr; strings.Contains(out, "-> "+filepath.Join(targetPath, "_emil-design__skills__prototype")+"\n") {
		t.Errorf("dry run copies to the old name:\n%s", out)
	}
}
