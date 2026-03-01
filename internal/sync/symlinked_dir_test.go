package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

// --- Discover tests: source dir is a symlink ---

func TestDiscoverSourceSkills_SourceIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "dotfiles", "skills")
	symlinkSource := filepath.Join(tmp, "symlinked-source")

	// Create real source with a skill
	writeSkillMD(t, filepath.Join(realSource, "my-skill"), "---\nname: my-skill\n---\n# My Skill")

	// Symlink source dir (simulates dotfiles manager)
	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	skills, err := DiscoverSourceSkills(symlinkSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].RelPath != "my-skill" {
		t.Errorf("RelPath = %q, want %q", skills[0].RelPath, "my-skill")
	}
	if skills[0].FlatName != "my-skill" {
		t.Errorf("FlatName = %q, want %q", skills[0].FlatName, "my-skill")
	}

	// SourcePath should be based on the symlink path (logical), not the resolved path
	wantPrefix := symlinkSource + string(os.PathSeparator)
	if got := skills[0].SourcePath; got != filepath.Join(symlinkSource, "my-skill") {
		t.Errorf("SourcePath = %q, want path under symlinked dir %q", got, wantPrefix)
	}
}

func TestDiscoverSourceSkillsLite_SourceIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "dotfiles", "skills")
	symlinkSource := filepath.Join(tmp, "symlinked-source")

	writeSkillMD(t, filepath.Join(realSource, "alpha"), "---\nname: alpha\n---\n# Alpha")
	writeSkillMD(t, filepath.Join(realSource, "beta"), "---\nname: beta\n---\n# Beta")

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	skills, _, err := DiscoverSourceSkillsLite(symlinkSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	for _, s := range skills {
		// Each SourcePath must be resolvable via os.Stat
		if _, err := os.Stat(s.SourcePath); err != nil {
			t.Errorf("SourcePath %q should be resolvable: %v", s.SourcePath, err)
		}
	}
}

func TestDiscoverSourceSkills_NestedSkillsUnderSymlinkedSource(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "real")
	symlinkSource := filepath.Join(tmp, "link")

	writeSkillMD(t, filepath.Join(realSource, "group", "sub-skill"), "---\nname: sub\n---\n# Sub")

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	skills, err := DiscoverSourceSkills(symlinkSource)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].FlatName != "group__sub-skill" {
		t.Errorf("FlatName = %q, want %q", skills[0].FlatName, "group__sub-skill")
	}
}

// --- SyncTargetMerge: source dir is a symlink ---

func TestSyncTargetMerge_SourceIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "dotfiles", "skills")
	symlinkSource := filepath.Join(tmp, "symlinked-source")
	tgt := filepath.Join(tmp, "target")

	writeSkillMD(t, filepath.Join(realSource, "alpha"), "---\nname: alpha\n---\n# Alpha")
	os.MkdirAll(tgt, 0755)

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: tgt, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, symlinkSource, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Fatalf("expected 1 linked, got %d", len(result.Linked))
	}

	// Verify the created symlink is resolvable
	linkPath := filepath.Join(tgt, "alpha")
	linkTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	// os.Stat follows symlinks — must resolve to a real directory
	info, err := os.Stat(linkPath)
	if err != nil {
		t.Fatalf("symlink %q -> %q does not resolve: %v", linkPath, linkTarget, err)
	}
	if !info.IsDir() {
		t.Errorf("expected resolved symlink to be a directory")
	}
}

// --- SyncTargetMerge: target dir is a symlink (the #456 scenario) ---

func TestSyncTargetMerge_TargetIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "dotfiles", "claude-skills")
	symlinkTarget := filepath.Join(tmp, "symlinked-target")

	writeSkillMD(t, filepath.Join(src, "my-skill"), "---\nname: my-skill\n---\n# My Skill")
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: symlinkTarget, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Fatalf("expected 1 linked, got %d", len(result.Linked))
	}

	// Verify the symlink was created inside the symlinked target
	linkPath := filepath.Join(symlinkTarget, "my-skill")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected symlink at %q: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected a symlink")
	}

	// Critical: the symlink must resolve correctly when accessed through the
	// symlinked target path (this is exactly what vercel/skills #456 fails at)
	resolved, err := os.Stat(linkPath)
	if err != nil {
		t.Fatalf("symlink does not resolve through symlinked target dir: %v", err)
	}
	if !resolved.IsDir() {
		t.Error("expected resolved path to be a directory")
	}

	// Also verify from the real target directory
	realLinkPath := filepath.Join(realTarget, "my-skill")
	if _, err := os.Stat(realLinkPath); err != nil {
		t.Fatalf("symlink should also be accessible from real target path: %v", err)
	}
}

// --- SyncTargetMerge: both source and target are symlinks ---

func TestSyncTargetMerge_BothSourceAndTargetAreSymlinks(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "dotfiles", "skills")
	realTarget := filepath.Join(tmp, "dotfiles", "claude-skills")
	symlinkSource := filepath.Join(tmp, "link-source")
	symlinkTarget := filepath.Join(tmp, "link-target")

	writeSkillMD(t, filepath.Join(realSource, "skill-a"), "---\nname: skill-a\n---\n# A")
	writeSkillMD(t, filepath.Join(realSource, "skill-b"), "---\nname: skill-b\n---\n# B")
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: symlinkTarget, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, symlinkSource, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 2 {
		t.Fatalf("expected 2 linked, got %d", len(result.Linked))
	}

	// Every created symlink must resolve correctly
	for _, name := range []string{"skill-a", "skill-b"} {
		linkPath := filepath.Join(symlinkTarget, name)
		if _, err := os.Stat(linkPath); err != nil {
			t.Errorf("symlink %q does not resolve: %v", linkPath, err)
		}
		// Also accessible from real target
		realPath := filepath.Join(realTarget, name)
		if _, err := os.Stat(realPath); err != nil {
			t.Errorf("symlink %q not accessible from real target: %v", realPath, err)
		}
	}
}

// --- Chained symlinks: symlink → symlink → real dir ---

func TestSyncTargetMerge_ChainedSourceSymlink(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "real-source")
	link1 := filepath.Join(tmp, "link1")
	link2 := filepath.Join(tmp, "link2")
	tgt := filepath.Join(tmp, "target")

	writeSkillMD(t, filepath.Join(realSource, "my-skill"), "---\nname: my-skill\n---\n# Skill")
	os.MkdirAll(tgt, 0755)

	// link1 → realSource, link2 → link1 (two-level chain)
	if err := os.Symlink(realSource, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: tgt, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, link2, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Fatalf("expected 1 linked through chained symlink, got %d", len(result.Linked))
	}

	linkPath := filepath.Join(tgt, "my-skill")
	if _, err := os.Stat(linkPath); err != nil {
		t.Fatalf("symlink created from chained source does not resolve: %v", err)
	}
}

func TestSyncTargetMerge_ChainedTargetSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "real-target")
	link1 := filepath.Join(tmp, "target-link1")
	link2 := filepath.Join(tmp, "target-link2")

	writeSkillMD(t, filepath.Join(src, "my-skill"), "---\nname: my-skill\n---\n# Skill")
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: link2, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Fatalf("expected 1 linked through chained target, got %d", len(result.Linked))
	}

	// Must resolve from all levels
	for _, p := range []string{link2, link1, realTarget} {
		path := filepath.Join(p, "my-skill")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("symlink not accessible from %q: %v", p, err)
		}
	}
}

// --- Idempotency: re-sync with symlinked dirs ---

func TestSyncTargetMerge_IdempotentWithSymlinkedSource(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "real")
	symlinkSource := filepath.Join(tmp, "link")
	tgt := filepath.Join(tmp, "target")

	writeSkillMD(t, filepath.Join(realSource, "alpha"), "---\nname: alpha\n---\n# Alpha")
	os.MkdirAll(tgt, 0755)

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// First sync
	r1, err := SyncTargetMerge("test", target, symlinkSource, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Linked) != 1 {
		t.Fatalf("first sync: expected 1 linked, got %d", len(r1.Linked))
	}

	// Second sync — should detect already-linked
	r2, err := SyncTargetMerge("test", target, symlinkSource, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Linked) != 1 {
		t.Errorf("second sync: expected 1 already-linked, got %d", len(r2.Linked))
	}
	if len(r2.Updated) != 0 {
		t.Errorf("second sync: expected 0 updated, got %d", len(r2.Updated))
	}
}

// --- Prune with symlinked target dir ---

func TestPruneOrphanLinks_TargetIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "real-target")
	symlinkTarget := filepath.Join(tmp, "link-target")

	writeSkillMD(t, filepath.Join(src, "keep"), "---\nname: keep\n---\n# Keep")
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: symlinkTarget, Mode: "merge"}

	// Sync two skills
	writeSkillMD(t, filepath.Join(src, "gone"), "---\nname: gone\n---\n# Gone")
	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Remove one skill from source
	os.RemoveAll(filepath.Join(src, "gone"))

	// Prune through symlinked target path
	result, err := PruneOrphanLinks(symlinkTarget, src, nil, nil, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed orphan, got %d: %v", len(result.Removed), result.Removed)
	}

	// Surviving skill should still be accessible
	if _, err := os.Stat(filepath.Join(symlinkTarget, "keep")); err != nil {
		t.Errorf("surviving skill should still be accessible through symlinked target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(realTarget, "keep")); err != nil {
		t.Errorf("surviving skill should be accessible through real target: %v", err)
	}
}

// --- CheckStatus with symlinked dirs ---

func TestCheckStatusMerge_TargetIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "real-target")
	symlinkTarget := filepath.Join(tmp, "link-target")
	skillSrc := filepath.Join(src, "my-skill")

	os.MkdirAll(skillSrc, 0755)
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	// Create a skill symlink inside the symlinked target
	if err := os.Symlink(skillSrc, filepath.Join(symlinkTarget, "my-skill")); err != nil {
		t.Fatal(err)
	}

	status, linked, local := CheckStatusMerge(symlinkTarget, src)
	if status != StatusMerged {
		t.Errorf("expected StatusMerged, got %s", status)
	}
	if linked != 1 {
		t.Errorf("expected 1 linked, got %d", linked)
	}
	if local != 0 {
		t.Errorf("expected 0 local, got %d", local)
	}
}

// --- FindLocalSkills with symlinked target ---

func TestFindLocalSkills_TargetIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "real-target")
	symlinkTarget := filepath.Join(tmp, "link-target")

	os.MkdirAll(src, 0755)
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	// Create a local skill inside the symlinked target
	localSkill := filepath.Join(symlinkTarget, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# Local"), 0644)

	skills, err := FindLocalSkills(symlinkTarget, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 local skill, got %d", len(skills))
	}
	if skills[0].Name != "local-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "local-skill")
	}
}

// --- Manifest correctness through symlinked target ---

func TestSyncTargetMerge_ManifestCorrectThroughSymlink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	realTarget := filepath.Join(tmp, "real-target")
	symlinkTarget := filepath.Join(tmp, "link-target")

	writeSkillMD(t, filepath.Join(src, "alpha"), "---\nname: alpha\n---\n# Alpha")
	os.MkdirAll(realTarget, 0755)

	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Fatal(err)
	}

	target := config.TargetConfig{Path: symlinkTarget, Mode: "merge"}
	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Manifest should be readable from both paths
	m1, err := ReadManifest(symlinkTarget)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := ReadManifest(realTarget)
	if err != nil {
		t.Fatal(err)
	}

	if len(m1.Managed) != 1 || len(m2.Managed) != 1 {
		t.Errorf("manifest mismatch: symlink=%d, real=%d", len(m1.Managed), len(m2.Managed))
	}
	if _, ok := m1.Managed["alpha"]; !ok {
		t.Error("alpha missing from manifest (via symlink)")
	}
	if _, ok := m2.Managed["alpha"]; !ok {
		t.Error("alpha missing from manifest (via real path)")
	}
}

// --- PullSkill (collect) with symlinked source ---

func TestPullSkill_SourceIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "real-source")
	symlinkSource := filepath.Join(tmp, "link-source")
	tgt := filepath.Join(tmp, "target")

	os.MkdirAll(realSource, 0755)

	if err := os.Symlink(realSource, symlinkSource); err != nil {
		t.Fatal(err)
	}

	// Create a local skill in target
	localSkill := filepath.Join(tgt, "new-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# New"), 0644)

	skill := LocalSkillInfo{Name: "new-skill", Path: localSkill}
	if err := PullSkill(skill, symlinkSource, false); err != nil {
		t.Fatal(err)
	}

	// Verify skill was pulled to real source via symlink
	pulledPath := filepath.Join(realSource, "new-skill", "SKILL.md")
	if _, err := os.Stat(pulledPath); err != nil {
		t.Fatalf("skill should exist at real source path after pull: %v", err)
	}

	// Also accessible via symlink
	if _, err := os.Stat(filepath.Join(symlinkSource, "new-skill", "SKILL.md")); err != nil {
		t.Fatalf("skill should be accessible via symlinked source: %v", err)
	}
}

// --- isSymlinkToSource: symlink alias equivalence ---

func TestIsSymlinkToSource_AliasEquivalence(t *testing.T) {
	tmp := t.TempDir()
	realSource := filepath.Join(tmp, "real-source")
	symlinkAlias := filepath.Join(tmp, "alias-source")

	os.MkdirAll(realSource, 0755)

	// symlinkAlias -> realSource
	if err := os.Symlink(realSource, symlinkAlias); err != nil {
		t.Fatal(err)
	}

	// Case 1: Target symlink points to the REAL path, source is the alias
	targetLink1 := filepath.Join(tmp, "target1")
	if err := os.Symlink(realSource, targetLink1); err != nil {
		t.Fatal(err)
	}

	if !isSymlinkToSource(targetLink1, symlinkAlias) {
		t.Error("isSymlinkToSource should return true when target points to the real path of a symlinked source alias")
	}

	// Case 2: Target points to alias, source is the real path
	targetLink2 := filepath.Join(tmp, "target2")
	if err := os.Symlink(symlinkAlias, targetLink2); err != nil {
		t.Fatal(err)
	}

	if !isSymlinkToSource(targetLink2, realSource) {
		t.Error("isSymlinkToSource should return true when target points to symlink alias of source")
	}
}
