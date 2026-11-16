package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestNamingDiscardGrantRechecksRenamedCopy(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	skillPath := filepath.Join(source, "group", "alpha")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: alpha\n---\n# Source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	skills := []DiscoveredSkill{{SourcePath: skillPath, RelPath: "group/alpha", FlatName: "group__alpha"}}
	tc := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: target, TargetNaming: "flat"}}
	if _, err := SyncTargetCopyWithSkills("custom", tc, skills, source, false, false, nil); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(target, "group__alpha")
	if err := os.WriteFile(filepath.Join(legacy, "SKILL.md"), []byte("# Snapshotted edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := CopyOutputFingerprint(legacy)
	if err != nil {
		t.Fatal(err)
	}
	grant := DiscardEntry{Target: "custom", Dir: cleanEntryDir(target), Entry: "group__alpha", Fingerprint: fingerprint}
	grants := []DiscardEntry{grant}
	tc.Skills.TargetNaming = "standard"
	changed := false
	result, err := syncTargetCopyWithSkillsOptions("custom", tc, skills, source, false, false, nil,
		CopyOptions{ProfileAware: true, Discard: grants}, func(step string) {
			if step == "copy_remove" {
				changed = true
				if err := os.WriteFile(filepath.Join(target, "alpha", "SKILL.md"), []byte("# Later edit\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	if err != nil || !changed || len(result.KeptModified) != 1 || result.KeptModified[0].Entry != "alpha" {
		t.Fatalf("renamed copy must be rechecked before replacement: %+v, %v", result, err)
	}
	data, err := os.ReadFile(filepath.Join(target, "alpha", "SKILL.md"))
	if err != nil || string(data) != "# Later edit\n" {
		t.Fatalf("migration discarded the later edit: %q, %v", data, err)
	}
	if grants[0] != grant {
		t.Fatal("migration changed the caller's discard grant")
	}
}
