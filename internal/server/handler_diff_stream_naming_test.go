package server

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

func TestComputeTargetDiff_NamingChangeReportsRename(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	for _, dir := range []string{filepath.Join(source, "_emil-design", "skills", "prototype"), filepath.Join(target, "prototype")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: prototype\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &ssync.Manifest{Managed: map[string]string{"prototype": "x"}, Naming: map[string]string{"prototype": "standard"}}
	if err := ssync.WriteManifest(target, manifest); err != nil {
		t.Fatal(err)
	}
	discovered, err := ssync.DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}

	dt := (&Server{}).computeTargetDiff("claude", config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: target, Mode: "copy", TargetNaming: "prefixed"}}, discovered, "merge", source, nil)
	if len(dt.Items) != 1 || dt.Items[0].Skill != "emil-design-prototype" || dt.Items[0].Action != "update" || dt.Items[0].Reason != "renamed from prototype (target naming changed)" {
		t.Fatalf("items = %+v, want one update renaming prototype", dt.Items)
	}
}

func TestComputeTargetDiff_StaleRecordedNamingIsAnUpdate(t *testing.T) {
	// A sync renamed the entry and saved the manifest, then stopped before re-copying it.
	source, target := t.TempDir(), t.TempDir()
	src := filepath.Join(source, "_emil-design", "skills", "prototype")
	for _, dir := range []string{src, filepath.Join(target, "emil-design-prototype")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: prototype\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	checksum, err := ssync.DirChecksumWithIgnore(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &ssync.Manifest{Managed: map[string]string{"emil-design-prototype": checksum}, Naming: map[string]string{"emil-design-prototype": "standard"}}
	if err := ssync.WriteManifest(target, manifest); err != nil {
		t.Fatal(err)
	}
	discovered, err := ssync.DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}

	dt := (&Server{}).computeTargetDiff("claude", config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: target, Mode: "copy", TargetNaming: "prefixed"}}, discovered, "merge", source, nil)
	if len(dt.Items) != 1 || dt.Items[0].Action != "update" || dt.Items[0].Reason != "target naming changed" {
		t.Fatalf("items = %+v, want one update for the stale naming", dt.Items)
	}
}

func TestComputeTargetDiff_LocalFolderOnNewNameKeepsLegacyEntry(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	for _, dir := range []string{filepath.Join(source, "_emil-design", "skills", "prototype"), filepath.Join(target, "prototype"), filepath.Join(target, "emil-design-prototype")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: prototype\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// prototype is the managed copy; emil-design-prototype is the user's folder.
	manifest := &ssync.Manifest{Managed: map[string]string{"prototype": "x"}, Naming: map[string]string{"prototype": "standard"}}
	if err := ssync.WriteManifest(target, manifest); err != nil {
		t.Fatal(err)
	}
	discovered, err := ssync.DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}

	dt := (&Server{}).computeTargetDiff("claude", config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: target, Mode: "copy", TargetNaming: "prefixed"}}, discovered, "merge", source, nil)
	if len(dt.Items) != 1 || dt.Items[0].Skill != "emil-design-prototype" || dt.Items[0].Action != "kept" || dt.Items[0].Reason != ssync.KeptLegacyReason("prototype") {
		t.Fatalf("items = %+v, want the local folder reported with the kept legacy entry", dt.Items)
	}
}
