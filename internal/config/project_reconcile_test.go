package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
)

func TestReconcileProjectSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill directory on disk
	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	// Pre-populate store with the entry (simulating post-install state)
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if !store.Has("my-skill") {
		t.Fatal("expected store to have 'my-skill'")
	}
	entry := store.Get("my-skill")
	if entry.Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", entry.Source)
	}
}

func TestReconcileProjectSkills_UpdatesExistingSource(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo-v1"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'my-skill'")
	}
	if entry.Source != "github.com/user/repo-v1" {
		t.Errorf("expected source 'github.com/user/repo-v1', got %q", entry.Source)
	}
}

func TestReconcileProjectSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill directory without metadata in the store
	skillPath := filepath.Join(skillsDir, "local-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Local skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries (no meta), got %d", len(store.List()))
	}
}

func TestReconcileProjectSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(store.List()))
	}
}

func TestReconcileProjectSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills") // does not exist

	cfg := &ProjectConfig{}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills should not fail for missing dir: %v", err)
	}
}

func TestReconcileProjectSkills_NestedSkillSetsGroup(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a nested skill: tools/my-skill
	skillPath := filepath.Join(skillsDir, "tools", "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{
		Source: "github.com/user/repo",
		Group:  "tools",
	})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	// After reconcile, nested skills use full-path keys (e.g. "tools/my-skill").
	entry := store.Get("tools/my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'tools/my-skill'")
	}
	if entry.Group != "tools" {
		t.Errorf("expected group 'tools', got %q", entry.Group)
	}
	// Legacy basename key should be removed after migration.
	if store.Has("my-skill") {
		t.Error("expected legacy basename key 'my-skill' to be removed")
	}
}

// Issue #157: reconcile should add newly installed skill to ProjectConfig.Skills
func TestReconcileProjectSkills_AddsToConfigSkills(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	skillPath := filepath.Join(skillsDir, "new-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	// Write initial config.yaml
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}

	store := install.NewMetadataStore()
	store.Set("new-skill", &install.MetadataEntry{Source: "github.com/user/repo"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 config skill, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Name != "new-skill" {
		t.Errorf("skill name = %q, want %q", cfg.Skills[0].Name, "new-skill")
	}
	if cfg.Skills[0].Source != "github.com/user/repo" {
		t.Errorf("skill source = %q, want %q", cfg.Skills[0].Source, "github.com/user/repo")
	}

	// Verify config.yaml on disk has the skill
	loaded, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if len(loaded.Skills) != 1 {
		t.Fatalf("expected 1 skill in loaded config, got %d", len(loaded.Skills))
	}
}

// Issue #157: reconcile should remove config skills for uninstalled skills
func TestReconcileProjectSkills_PrunesConfigSkills(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Only "alive" exists on disk
	if err := os.MkdirAll(filepath.Join(skillsDir, "alive"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills: []SkillEntry{
			{Name: "alive", Source: "github.com/user/alive"},
			{Name: "gone", Source: "github.com/user/gone"},
		},
	}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}

	store := install.NewMetadataStore()
	store.Set("alive", &install.MetadataEntry{Source: "github.com/user/alive"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 config skill after prune, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Name != "alive" {
		t.Errorf("remaining skill = %q, want %q", cfg.Skills[0].Name, "alive")
	}
}

func TestReconcileProjectSkills_PrunesStaleEntries(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create only one skill on disk
	skillPath := filepath.Join(skillsDir, "alive-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("alive-skill", &install.MetadataEntry{Source: "github.com/user/alive"})
	store.Set("deleted-skill", &install.MetadataEntry{Source: "github.com/user/deleted"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	names := store.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d: %v", len(names), names)
	}
	if !store.Has("alive-skill") {
		t.Error("expected alive-skill to survive prune")
	}
	if store.Has("deleted-skill") {
		t.Error("expected deleted-skill to be pruned")
	}
}

// TestReconcileProjectSkills_KeepsChangedDeclarationOnMove verifies that a
// gone record whose declaration now names another source is not moved to a
// copy of the old source, which would replace the user's declaration.
func TestReconcileProjectSkills_KeepsChangedDeclarationOnMove(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	copyDir := filepath.Join(skillsDir, "new", "demo")
	if err := os.MkdirAll(copyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyDir, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := install.ComputeFileHashes(copyDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "demo", Group: "old", Source: "github.com/user/b/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("old/demo", &install.MetadataEntry{Source: "github.com/user/a/demo", Group: "old", FileHashes: hashes})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatal(err)
	}

	if got := store.Get("new/demo"); got != nil {
		t.Errorf("old source's record moved to the copy: %+v", got)
	}
}

// TestReconcileProjectSkills_FollowsMovedSkill verifies that a declared skill
// moved to another folder keeps its record in project mode.
func TestReconcileProjectSkills_FollowsMovedSkill(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	moved := filepath.Join(skillsDir, "new", "demo")
	if err := os.MkdirAll(moved, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moved, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := install.ComputeFileHashes(moved)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "demo", Group: "old", Source: "github.com/user/repo/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("old/demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Group: "old", FileHashes: hashes})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatal(err)
	}

	if got := store.Get("new/demo"); got == nil || got.Source != "github.com/user/repo/demo" {
		t.Errorf("moved skill entry = %+v, want the record under new/demo", got)
	}
}

// TestReconcileProjectSkills_MovesGitignoreRule verifies that a moved skill's
// managed ignore rule moves with it, so the old path is not ignored forever.
func TestReconcileProjectSkills_MovesGitignoreRule(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	moved := filepath.Join(skillsDir, "new", "demo")
	if err := os.MkdirAll(moved, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moved, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitignore := filepath.Join(root, ".skillshare", ".gitignore")
	if err := os.WriteFile(gitignore, []byte("# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/old/demo/\n# END SKILLSHARE MANAGED\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := install.ComputeFileHashes(moved)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "demo", Group: "old", Source: "github.com/user/repo/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("old/demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Group: "old", FileHashes: hashes})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(gitignore)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "skills/old/demo") || !strings.Contains(string(data), "skills/new/demo/") {
		t.Errorf(".gitignore = %q, want the rule moved from skills/old/demo to skills/new/demo", data)
	}
}

// TestReconcileProjectSkills_MovedSkillKeepsGroupAndPin follows a move made by
// skillmove: the record and the lock pin are re-keyed, then reconcile runs. The
// config entry must take the new group and the pin must keep the older commit.
func TestReconcileProjectSkills_MovedSkillKeepsGroupAndPin(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	moved := filepath.Join(skillsDir, "grp", "demo")
	if err := os.MkdirAll(moved, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moved, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pinned := strings.Repeat("a", 40)
	installed := strings.Repeat("b", 40)
	dir := filepath.Join(root, ".skillshare")
	lock := &install.Lock{Skills: map[string]install.LockEntry{"demo": {Source: "github.com/user/repo/demo", Commit: pinned}}}
	if !lock.MovePin("demo", "grp/demo") {
		t.Fatal("no pin to move")
	}
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "demo", Source: "github.com/user/repo/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Commit: installed})
	store.MovePath("demo", "grp/demo")

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Skills) != 1 || cfg.Skills[0].Group != "grp" || cfg.Skills[0].Name != "demo" {
		t.Errorf("config skills = %+v, want one entry demo in group grp", cfg.Skills)
	}
	got, err := install.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pin := got.Skills["grp/demo"]; pin.Commit != pinned {
		t.Errorf("pin = %+v, want the deliberately older commit kept", pin)
	}
	if _, stale := got.Skills["demo"]; stale {
		t.Error("the pin of the old path was left behind")
	}
}

// TestReconcileProjectSkills_HandMovedSkillKeepsPin verifies that adopting a
// copy moved with mv re-keys the lock pin instead of pinning the local commit.
func TestReconcileProjectSkills_HandMovedSkillKeepsPin(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	moved := filepath.Join(skillsDir, "grp", "demo")
	if err := os.MkdirAll(moved, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moved, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := install.ComputeFileHashes(moved)
	if err != nil {
		t.Fatal(err)
	}
	pinned := strings.Repeat("a", 40)
	installed := strings.Repeat("b", 40)
	dir := filepath.Join(root, ".skillshare")
	lock := &install.Lock{Skills: map[string]install.LockEntry{"demo": {Source: "github.com/user/repo/demo", Commit: pinned}}}
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "demo", Source: "github.com/user/repo/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Commit: installed, FileHashes: hashes})

	if err := AdoptMovedProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatal(err)
	}

	got, err := install.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if pin := got.Skills["grp/demo"]; pin.Commit != pinned {
		t.Errorf("pin = %+v, want the shared commit kept under grp/demo", pin)
	}
	if _, stale := got.Skills["demo"]; stale {
		t.Error("the pin of the old path was left behind")
	}
}
