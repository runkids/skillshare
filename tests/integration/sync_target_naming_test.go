//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func TestSync_TargetNamingStandard_MergeUsesBareName(t *testing.T) {
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
    mode: merge
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "dev")) {
		t.Fatal("expected standard target entry 'dev' to be a symlink")
	}
	if sb.FileExists(filepath.Join(targetPath, "frontend__dev")) {
		t.Fatal("did not expect flat entry frontend__dev in standard mode")
	}
}

func TestSync_TargetNamingStandard_CopyUsesBareName(t *testing.T) {
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
    mode: copy
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(targetPath, "dev", "SKILL.md")) {
		t.Fatal("expected standard target entry 'dev' to be copied")
	}
	if sb.FileExists(filepath.Join(targetPath, "frontend__dev")) {
		t.Fatal("did not expect flat entry frontend__dev in standard mode")
	}

	var manifest ssync.Manifest
	data := sb.ReadFile(filepath.Join(targetPath, ssync.ManifestFile))
	if err := json.Unmarshal([]byte(data), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if _, ok := manifest.Managed["dev"]; !ok {
		t.Fatal("expected manifest to track bare name 'dev'")
	}
	if _, ok := manifest.Managed["frontend__dev"]; ok {
		t.Fatal("did not expect manifest to retain flat key frontend__dev")
	}
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

func TestSync_TargetNamingStandard_CollisionWarnsAndSkipsBoth(t *testing.T) {
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

	sb.WriteConfig(`source: ` + sb.SourcePath + `
target_naming: standard
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "duplicate skill names")

	if sb.FileExists(filepath.Join(targetPath, "dev")) {
		t.Fatal("did not expect colliding bare name dev to be synced")
	}
	if !sb.IsSymlink(filepath.Join(targetPath, "alpha")) {
		t.Fatal("expected non-conflicting skill alpha to sync")
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
