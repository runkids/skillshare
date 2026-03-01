package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryCache_L1_SameProcess(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")

	dc := New(cacheDir)

	skills1, err := dc.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills1) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills1))
	}

	// Add skill — L1 should return cached
	writeSkill(t, filepath.Join(src, "skill-b"), "---\nname: b\n---\n# B")

	skills2, err := dc.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills2) != 1 {
		t.Errorf("expected L1 cache hit (1 skill), got %d", len(skills2))
	}
}

func TestDiscoveryCache_LiteFullSeparation(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	content := "---\nname: a\ntargets:\n  - claude\n---\n# A"
	writeSkill(t, filepath.Join(src, "skill-a"), content)

	dc := New(cacheDir)

	// Call Lite first
	liteSkills, _, err := dc.DiscoverLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if liteSkills[0].Targets != nil {
		t.Errorf("lite should have nil targets, got %v", liteSkills[0].Targets)
	}

	// Call Full — must NOT return Lite's nil-Targets
	fullSkills, err := dc.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if fullSkills[0].Targets == nil {
		t.Fatal("full discovery returned nil Targets — L1 lite/full pollution!")
	}
	if len(fullSkills[0].Targets) != 1 || fullSkills[0].Targets[0] != "claude" {
		t.Errorf("expected [claude], got %v", fullSkills[0].Targets)
	}
}

func TestDiscoveryCache_Invalidate(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")

	dc := New(cacheDir)
	if _, err := dc.Discover(src); err != nil {
		t.Fatal(err)
	}

	writeSkill(t, filepath.Join(src, "skill-b"), "---\nname: b\n---\n# B")
	dc.Invalidate(src)

	skills, err := dc.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills after invalidate, got %d", len(skills))
	}
}

func TestDiscoveryCache_DiscoverLite(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\ntargets:\n  - claude\n---\n# A")

	dc := New(cacheDir)
	skills, repos, err := dc.DiscoverLite(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1, got %d", len(skills))
	}
	if skills[0].Targets != nil {
		t.Errorf("expected nil targets in lite, got %v", skills[0].Targets)
	}
	_ = repos
}

func TestDiscoveryCache_L2_DiskPersistence(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")

	dc1 := New(cacheDir)
	skills1, err := dc1.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills1) != 1 {
		t.Fatalf("expected 1, got %d", len(skills1))
	}

	// Verify disk cache exists
	diskPath := diskCachePath(cacheDir, src)
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		t.Fatal("expected disk cache file to exist")
	}

	// New instance should hit L2
	dc2 := New(cacheDir)
	skills2, err := dc2.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills2) != 1 {
		t.Errorf("expected 1 from disk cache, got %d", len(skills2))
	}
}

func TestDiscoveryCache_L2_NewSkillDetectedByCountGuard(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")

	dc1 := New(cacheDir)
	if _, err := dc1.Discover(src); err != nil {
		t.Fatal(err)
	}

	// Add skill externally — count guard detects the mismatch
	writeSkill(t, filepath.Join(src, "skill-b"), "---\nname: b\n---\n# B")

	dc2 := New(cacheDir)
	skills, err := dc2.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 after new skill (count guard), got %d", len(skills))
	}
}

func TestDiscoveryCache_L2_NestedEditInvalidation(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "_team", "coding"), "---\nname: coding\n---\n# Coding")

	dc1 := New(cacheDir)
	skills1, err := dc1.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if skills1[0].Targets != nil {
		t.Fatal("expected nil targets initially")
	}

	// Edit nested SKILL.md — root dir mtime does NOT change
	writeSkill(t, filepath.Join(src, "_team", "coding"), "---\nname: coding\ntargets:\n  - claude\n---\n# Coding")

	dc2 := New(cacheDir)
	skills2, err := dc2.Discover(src)
	if err != nil {
		t.Fatal(err)
	}
	if skills2[0].Targets == nil || len(skills2[0].Targets) != 1 {
		t.Errorf("expected [claude] after nested edit, got %v", skills2[0].Targets)
	}
}

func TestDiscoveryCache_InvalidateDeletesDisk(t *testing.T) {
	src := t.TempDir()
	cacheDir := t.TempDir()
	writeSkill(t, filepath.Join(src, "skill-a"), "---\nname: a\n---\n# A")

	dc := New(cacheDir)
	if _, err := dc.Discover(src); err != nil {
		t.Fatal(err)
	}

	diskPath := diskCachePath(cacheDir, src)
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		t.Fatal("expected disk cache before invalidate")
	}

	dc.Invalidate(src)

	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Error("expected disk cache deleted after invalidate")
	}
}
