//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func TestSync_IncludePatternMatchingNothing_IsReported(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Dev",
	})
	targetPath := sb.CreateTarget("claude")

	writeInclude := func(naming, include string) {
		sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: ` + naming + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
    include: [` + include + `]
`)
	}

	// Filters match the source-path name, so this is the pattern that works.
	writeInclude("standard", "frontend__dev")
	first := sb.RunCLI("sync")
	first.AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(targetPath, "dev")) {
		t.Fatal("expected frontend__dev to link under the bare name dev")
	}
	first.AssertOutputNotContains(t, "matches no skill")

	// The same skill addressed by the name the target shows matches nothing, and
	// the entry the previous filter created is pruned. The mismatch has to be
	// visible instead of silent.
	writeInclude("standard", "dev")
	second := sb.RunCLI("sync")
	second.AssertSuccess(t)
	second.AssertOutputContains(t, `claude: include filter "dev" matches no skill`)
	second.AssertOutputContains(t, "frontend__dev")

	// The heuristic also runs in the default flat naming mode.
	writeInclude("flat", "dev")
	third := sb.RunCLI("sync")
	third.AssertSuccess(t)
	third.AssertOutputContains(t, `include filter "dev" matches no skill`)
}

func TestSync_TargetNamingStandard_InvalidSkillWarnsAndSkips(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: wrong-name\n---\n# Wrong",
	})
	sb.CreateSkill("alpha", map[string]string{
		"SKILL.md": "---\nname: alpha\n---\n# Alpha",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skill(s) skipped (selection or naming)")
	result.AssertAnyOutputContains(t, "skipped frontend/dev because")

	if !sb.IsSymlink(filepath.Join(targetPath, "alpha")) {
		t.Fatal("expected valid skill alpha to sync")
	}
	if sb.FileExists(filepath.Join(targetPath, "dev")) {
		t.Fatal("did not expect invalid skill to sync as dev")
	}
	if sb.FileExists(filepath.Join(targetPath, "frontend__dev")) {
		t.Fatal("did not expect invalid skill to sync as flat legacy name")
	}
}

func TestSync_TargetNamingStandard_CollisionSyncsBothUnderFlatNames(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Frontend Dev",
	})
	sb.CreateNestedSkill("backend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Backend Dev",
	})
	sb.CreateSkill("alpha", map[string]string{
		"SKILL.md": "---\nname: alpha\n---\n# Alpha",
	})
	targetPath := sb.CreateTarget("claude")
	writeNamingConfig(sb, targetPath, "standard", "merge")

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "duplicate skill names")

	if sb.FileExists(filepath.Join(targetPath, "dev")) {
		t.Fatal("no skill should take the shared name dev")
	}
	for _, name := range []string{"frontend__dev", "backend__dev", "alpha"} {
		if !sb.IsSymlink(filepath.Join(targetPath, name)) {
			t.Fatalf("expected %s to sync", name)
		}
	}
}

// A copy moves to its flat name when another skill takes its name, and back
// when that skill is gone, without being pruned on the way.
func TestSync_TargetNamingStandard_FollowsACollisionBothWays(t *testing.T) {
	for _, mode := range []string{"copy", "merge"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()

			sb.CreateNestedSkill("frontend/dev", map[string]string{
				"SKILL.md": "---\nname: dev\n---\n# Frontend Dev",
			})
			targetPath := sb.CreateTarget("claude")
			writeNamingConfig(sb, targetPath, "standard", mode)
			sb.RunCLI("sync").AssertSuccess(t)

			backend := sb.CreateNestedSkill("backend/dev", map[string]string{
				"SKILL.md": "---\nname: dev\n---\n# Backend Dev",
			})
			sb.RunCLI("sync").AssertSuccess(t)
			if sb.FileExists(filepath.Join(targetPath, "dev")) {
				t.Fatal("dev should have moved to its flat name")
			}
			if !strings.Contains(sb.ReadFile(filepath.Join(targetPath, "frontend__dev", "SKILL.md")), "Frontend Dev") {
				t.Fatal("frontend__dev should hold the frontend skill")
			}
			if !strings.Contains(sb.ReadFile(filepath.Join(targetPath, "backend__dev", "SKILL.md")), "Backend Dev") {
				t.Fatal("backend__dev should hold the backend skill")
			}
			if mode == "copy" {
				if got := readSyncManifest(t, sb, targetPath).Naming["frontend__dev"]; got != "flat" {
					t.Fatalf("manifest naming for frontend__dev = %q, want flat", got)
				}
			}

			if err := os.RemoveAll(backend); err != nil {
				t.Fatal(err)
			}
			sb.RunCLI("sync").AssertSuccess(t)
			if sb.FileExists(filepath.Join(targetPath, "frontend__dev")) || sb.FileExists(filepath.Join(targetPath, "backend__dev")) {
				t.Fatal("the flat names should be gone once the collision is")
			}
			if !strings.Contains(sb.ReadFile(filepath.Join(targetPath, "dev", "SKILL.md")), "Frontend Dev") {
				t.Fatal("dev should hold the frontend skill again")
			}
		})
	}
}

func TestSync_TargetNamingStandard_MergeMigratesManagedFlatEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Dev",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)
	sb.RunCLI("sync").AssertSuccess(t)

	legacyPath := filepath.Join(targetPath, "frontend__dev")
	if !sb.IsSymlink(legacyPath) {
		t.Fatal("expected initial flat entry frontend__dev")
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "dev")) {
		t.Fatal("expected managed entry to migrate to bare name dev")
	}
	if sb.FileExists(legacyPath) {
		t.Fatal("did not expect legacy flat entry to remain after migration")
	}
}

func TestSync_TargetNamingStandard_CopyMigratesManagedFlatEntry(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Dev",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)

	legacyPath := filepath.Join(targetPath, "frontend__dev")
	if !sb.FileExists(filepath.Join(legacyPath, "SKILL.md")) {
		t.Fatal("expected initial flat copy frontend__dev")
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(targetPath, "dev", "SKILL.md")) {
		t.Fatal("expected managed copy to migrate to bare name dev")
	}
	if sb.FileExists(legacyPath) {
		t.Fatal("did not expect legacy flat copy to remain after migration")
	}

	var manifest ssync.Manifest
	data := sb.ReadFile(filepath.Join(targetPath, ssync.ManifestFile))
	if err := json.Unmarshal([]byte(data), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if _, ok := manifest.Managed["dev"]; !ok {
		t.Fatal("expected manifest to track migrated bare name dev")
	}
	if _, ok := manifest.Managed["frontend__dev"]; ok {
		t.Fatal("did not expect manifest to retain legacy flat key")
	}
}

func TestSync_TargetNamingStandard_PreservesLegacyManagedEntryWhenDestinationExists(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Dev",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)
	sb.RunCLI("sync").AssertSuccess(t)

	legacyPath := filepath.Join(targetPath, "frontend__dev")
	localPath := filepath.Join(targetPath, "dev")
	if err := os.MkdirAll(localPath, 0755); err != nil {
		t.Fatalf("mkdir local path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localPath, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write local skill: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "kept legacy managed entry")

	if !sb.IsSymlink(legacyPath) {
		t.Fatal("expected legacy managed entry to be preserved when bare name already exists")
	}
	if !sb.FileExists(filepath.Join(localPath, "SKILL.md")) {
		t.Fatal("expected existing bare-name local skill to be preserved")
	}
}

func TestSync_TargetNaming_IgnoredInSymlinkMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/dev", map[string]string{
		"SKILL.md": "---\nname: dev\n---\n# Dev",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(targetPath) {
		t.Fatal("expected entire target directory to be symlinked in symlink mode")
	}
	if got := sb.SymlinkTarget(targetPath); got != sb.SourcePath {
		t.Fatalf("symlink target = %q, want %q", got, sb.SourcePath)
	}
	if strings.Contains(result.Output(), "kept legacy managed entry") {
		t.Fatal("did not expect target naming migration warnings in symlink mode")
	}
}

// A target that inherits symlink mode holds the source as it is, so sync has
// no shared name to warn about there.
func TestSync_InheritedSymlinkModeReportsNoSharedNames(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	for _, dir := range []string{"frontend/dev", "backend/dev"} {
		sb.CreateNestedSkill(dir, map[string]string{"SKILL.md": "---\nname: dev\n---\n# " + dir})
	}
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: symlink\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n")

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	if strings.Contains(result.Output(), "duplicate skill names") {
		t.Fatalf("sync warns about shared names on a symlink-mode target:\n%s", result.Output())
	}
}

// When a shared name stops being shared, the skill left with it takes the name
// back, even though the other skill's entry still holds it.
func TestSync_TargetNamingStandard_NameReturnsWhenTheOtherSkillLeaves(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			top := sb.CreateSkill("dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# top"})
			sb.CreateNestedSkill("a/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# nested"})
			sameMtimes(t, top, filepath.Join(sb.SourcePath, "a", "dev"))
			targetPath := sb.CreateTarget("claude")
			sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n    mode: " + mode + "\n")
			sb.RunCLI("sync").AssertSuccess(t)

			if err := os.RemoveAll(top); err != nil {
				t.Fatal(err)
			}
			sb.RunCLI("sync").AssertSuccess(t)

			if got := sb.ReadFile(filepath.Join(targetPath, "dev", "SKILL.md")); !strings.Contains(got, "# nested") {
				t.Fatalf("dev holds %q, want the nested skill", got)
			}
			if sb.FileExists(filepath.Join(targetPath, "a__dev")) {
				t.Fatal("a__dev should be gone once the name is free")
			}
		})
	}
}

// A skill that takes over a copy's name gets its own content, even when both
// sources carry the same modification times.
func TestSync_TargetNamingStandardCopy_NewOwnerOfANameIsCopied(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	nested := sb.CreateNestedSkill("a/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# nested"})
	sameMtimes(t, nested)
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n    mode: copy\n")
	sb.RunCLI("sync").AssertSuccess(t)

	top := sb.CreateSkill("dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# top"})
	sameMtimes(t, top)
	sb.RunCLI("sync").AssertSuccess(t)

	if got := sb.ReadFile(filepath.Join(targetPath, "dev", "SKILL.md")); !strings.Contains(got, "# top") {
		t.Fatalf("dev holds %q, want the top-level skill", got)
	}
	if got := sb.ReadFile(filepath.Join(targetPath, "a__dev", "SKILL.md")); !strings.Contains(got, "# nested") {
		t.Fatalf("a__dev holds %q, want the nested skill", got)
	}
}

// Diff must show the link sync repoints when a top-level skill takes a name
// another skill's link holds.
func TestDiff_TargetNamingStandard_ListsALinkToRepoint(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("a/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# nested"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n")
	sb.RunCLI("sync").AssertSuccess(t)
	sb.CreateSkill("dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# top"})

	result := sb.RunCLI("diff", "--json")
	result.AssertSuccess(t)
	var out struct {
		Targets []struct {
			Items []struct{ Name, Action string } `json:"items"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("diff --json: %v\n%s", err, result.Stdout)
	}
	for _, item := range out.Targets[0].Items {
		if item.Name == "dev" {
			return
		}
	}
	t.Fatalf("diff does not list dev, which sync repoints: %s", result.Stdout)
}

// When the other skill leaves, diff must list the old entry sync prunes, as
// well as the link it repoints.
func TestDiff_TargetNamingStandard_ListsTheEntryAHandoverPrunes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	top := sb.CreateSkill("dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# top"})
	sb.CreateNestedSkill("a/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# nested"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n")
	sb.RunCLI("sync").AssertSuccess(t)
	if err := os.RemoveAll(top); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("diff", "--json")
	result.AssertSuccess(t)
	if !strings.Contains(result.Stdout, `"name": "a__dev"`) || !strings.Contains(result.Stdout, `"name": "dev"`) {
		t.Fatalf("diff should list the repointed dev and the pruned a__dev: %s", result.Stdout)
	}
}

// sameMtimes gives every file and folder in the skill directories one time, as
// cp -a or an archive can.
func sameMtimes(t *testing.T, dirs ...string) {
	t.Helper()
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chtimes(path, when, when)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// A skill that keeps its old name because its new one is taken keeps that
// entry: another skill must not rename it away and copy over it.
func TestSync_TargetNamingStandardCopy_KeptOldNameStaysWithItsSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateNestedSkill("a/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# a"})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntarget_naming: standard\ntargets:\n  claude:\n    path: " + targetPath + "\n    mode: copy\n")
	sb.RunCLI("sync").AssertSuccess(t)
	sb.WriteFile(filepath.Join(targetPath, "a__dev", "notes.md"), "mine")
	sb.CreateNestedSkill("b/dev", map[string]string{"SKILL.md": "---\nname: dev\n---\n# b"})

	sb.RunCLI("sync").AssertSuccess(t)

	if got := sb.ReadFile(filepath.Join(targetPath, "dev", "SKILL.md")); !strings.Contains(got, "# a") {
		t.Fatalf("dev holds %q, want a/dev's kept copy", got)
	}
	if got := sb.ReadFile(filepath.Join(targetPath, "b__dev", "SKILL.md")); !strings.Contains(got, "# b") {
		t.Fatalf("b__dev holds %q, want b/dev", got)
	}
}
