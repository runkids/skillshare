package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"

	"gopkg.in/yaml.v3"
)

func TestReconcileGlobalSkills_NestedTrackedRepoWithoutEntry(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	remotes := map[string]string{}
	for _, name := range []string{"_team", "org/_team"} {
		repoRoot := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(repoRoot, 0755); err != nil {
			t.Fatal(err)
		}
		remote := testutil.SetupBareRemoteRepo(t, repoRoot)
		testutil.SeedRemoteBranch(t, repoRoot, remote, "main", map[string]string{"README.md": name})
		testutil.RunGit(t, "", "clone", remote, filepath.Join(sourceDir, filepath.FromSlash(name)))
		remotes[name] = remote
	}

	top := install.MetadataEntry{Source: remotes["_team"], Tracked: true, Branch: "main", FileHashes: map[string]string{"README.md": "unchanged"}}
	store := install.NewMetadataStore()
	store.Set("_team", &top)
	wantTop := top
	cfg := &Config{Source: sourceDir}
	for pass := 1; pass <= 2; pass++ {
		if err := ReconcileGlobalSkills(cfg, store); err != nil {
			t.Fatal(err)
		}
		store = install.LoadMetadataOrNew(sourceDir)
		if got := store.Get("_team"); !reflect.DeepEqual(got, &wantTop) {
			t.Errorf("pass %d: top-level entry = %+v, want %+v", pass, got, wantTop)
		}
		if got := store.Get("org/_team"); got == nil || got.Source != remotes["org/_team"] || !got.Tracked || got.Group != "org" || got.Branch != "main" {
			t.Errorf("pass %d: nested entry = %+v, want source %q, tracked, group org, branch main", pass, got, remotes["org/_team"])
		}
	}
}

func TestReconcileGlobalSkills_NestedTrackedRepoLegacyEntry(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	remote := testutil.SetupBareRemoteRepo(t, root)
	testutil.SeedRemoteBranch(t, root, remote, "main", map[string]string{"README.md": "# team"})
	testutil.RunGit(t, "", "clone", remote, filepath.Join(sourceDir, "org", "_team"))
	store := install.NewMetadataStore()
	want := install.MetadataEntry{Source: "github.com/example/team/custom", Tracked: true, Group: "org", Branch: "custom", FileHashes: map[string]string{"README.md": "unchanged"}}
	entry := want
	store.Set("_team", &entry)
	if err := ReconcileGlobalSkills(&Config{Source: sourceDir}, store); err != nil {
		t.Fatal(err)
	}
	store = install.LoadMetadataOrNew(sourceDir)
	if got := store.Get("org/_team"); !reflect.DeepEqual(got, &want) {
		t.Errorf("migrated entry = %+v, want %+v", got, want)
	}
	if store.Has("_team") {
		t.Error("legacy key was not migrated")
	}
}

func TestReconcileGlobalSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill directory on disk
	skillPath := filepath.Join(sourceDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	// Pre-populate store with the entry (simulating post-install state)
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if !store.Has("my-skill") {
		t.Fatal("expected store to have 'my-skill'")
	}
	entry := store.Get("my-skill")
	if entry.Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", entry.Source)
	}
}

func TestReconcileGlobalSkills_UpdatesExistingSource(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	skillPath := filepath.Join(sourceDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo-v1"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'my-skill'")
	}
	// Source should remain as-is since reconcile reads from the existing store entry
	if entry.Source != "github.com/user/repo-v1" {
		t.Errorf("expected source 'github.com/user/repo-v1', got %q", entry.Source)
	}
}

func TestReconcileGlobalSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill directory without metadata in the store
	skillPath := filepath.Join(sourceDir, "local-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Local skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries (no meta), got %d", len(store.List()))
	}
}

func TestReconcileGlobalSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(store.List()))
	}
}

func TestReconcileGlobalSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills") // does not exist

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills should not fail for missing dir: %v", err)
	}
}

func TestReconcileGlobalSkills_NestedSkillSetsGroup(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a nested skill: frontend/pdf
	skillPath := filepath.Join(sourceDir, "frontend", "pdf")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("pdf", &install.MetadataEntry{
		Source: "anthropics/skills/skills/pdf",
		Group:  "frontend",
	})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	// After reconcile, nested skills use full-path keys (e.g. "frontend/pdf").
	entry := store.Get("frontend/pdf")
	if entry == nil {
		t.Fatal("expected store to have 'frontend/pdf'")
	}
	if entry.Group != "frontend" {
		t.Errorf("expected group 'frontend', got %q", entry.Group)
	}
	// Legacy basename key should be removed after migration.
	if store.Has("pdf") {
		t.Error("expected legacy basename key 'pdf' to be removed")
	}
}

func TestReconcileGlobalSkills_PrunesStaleEntries(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create only one skill on disk
	skillPath := filepath.Join(sourceDir, "alive-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("alive-skill", &install.MetadataEntry{Source: "github.com/user/alive"})
	store.Set("deleted-skill", &install.MetadataEntry{Source: "github.com/user/deleted"})
	store.Set("frontend/gone-skill", &install.MetadataEntry{Source: "github.com/user/gone", Group: "frontend"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	names := store.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d: %v", len(names), names)
	}
	if !store.Has("alive-skill") {
		t.Errorf("expected surviving entry 'alive-skill'")
	}
}

// TestReconcileGlobalSkills_FollowsMovedSkill verifies that an installed skill
// moved to another folder keeps its install record (issue #510).
func TestReconcileGlobalSkills_FollowsMovedSkill(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	moved := filepath.Join(sourceDir, "new", "demo")
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
	store := install.NewMetadataStore()
	store.Set("old/demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Group: "old", FileHashes: hashes})
	store.AuditAccepted = map[string][]string{"old/demo": {"accepted-key"}}

	if err := ReconcileGlobalSkills(&Config{Source: sourceDir}, store); err != nil {
		t.Fatal(err)
	}

	if got := store.Get("new/demo"); got == nil || got.Source != "github.com/user/repo/demo" || got.Group != "new" {
		t.Errorf("moved skill entry = %+v, want the old record under new/demo", got)
	}
	if got := store.AuditAccepted["new/demo"]; !reflect.DeepEqual(got, []string{"accepted-key"}) {
		t.Errorf("accepted audit findings = %v, want them moved to new/demo", got)
	}
}

// TestReconcileGlobalSkills_IgnoresSameNameSkillWithOtherFiles verifies that a
// different skill sharing the name never takes a gone skill's record, so a
// later update cannot overwrite it from that source.
func TestReconcileGlobalSkills_IgnoresSameNameSkillWithOtherFiles(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
	own := filepath.Join(sourceDir, "new", "demo")
	if err := os.MkdirAll(own, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(own, "SKILL.md"), []byte("---\nname: demo\n---\nmine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("old/demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Group: "old", FileHashes: map[string]string{"SKILL.md": "sha256:other"}})

	if err := ReconcileGlobalSkills(&Config{Source: sourceDir}, store); err != nil {
		t.Fatal(err)
	}

	if got := store.Get("new/demo"); got != nil {
		t.Errorf("unrelated skill took the record: %+v", got)
	}
}

func TestReconcileGlobalSkills_KeepsMetadataOfUnavailableSourceLink(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "unmounted"), filepath.Join(sourceDir, "_dev-skills")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))

	for _, follow := range []bool{true, false} {
		store := install.NewMetadataStore()
		store.Set("_dev-skills", &install.MetadataEntry{Source: "github.com/user/dev-skills", Tracked: true})
		cfg := &Config{Source: sourceDir, FollowSourceLinks: follow}
		if err := ReconcileGlobalSkills(cfg, store); err != nil {
			t.Fatalf("ReconcileGlobalSkills: %v", err)
		}
		// Off, the link is invisible and its entry goes, as before; on, the
		// walk knows it missed the link and keeps the entry.
		if store.Has("_dev-skills") != follow {
			t.Errorf("follow=%v: entry kept = %v", follow, store.Has("_dev-skills"))
		}
	}
}

// Only a _-prefixed checkout is a tracked repo; an unprefixed one stays a
// regular entry, and a tracked flag an older reconcile wrote on it is cleared
// without dropping the entry (#476).
func TestReconcileGlobalSkills_TracksOnlyPrefixedCheckouts(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")
	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	remote := testutil.SetupBareRemoteRepo(t, root)
	testutil.SeedRemoteBranch(t, root, remote, "main", map[string]string{"SKILL.md": "# skill"})
	for _, name := range []string{"_team", "org/_infra", "solo", "org/side"} {
		testutil.RunGit(t, "", "clone", remote, filepath.Join(sourceDir, filepath.FromSlash(name)))
	}

	store := install.NewMetadataStore()
	store.Set("solo", &install.MetadataEntry{Source: remote, Tracked: true})
	if err := ReconcileGlobalSkills(&Config{Source: sourceDir}, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	for name, want := range map[string]bool{"_team": true, "org/_infra": true, "solo": false} {
		entry := store.GetByPath(name)
		if entry == nil {
			t.Errorf("%s: entry dropped", name)
		} else if entry.Tracked != want {
			t.Errorf("%s: Tracked = %v, want %v", name, entry.Tracked, want)
		}
	}
	if store.Has("org/side") {
		t.Error("org/side: an unprefixed checkout without an entry must not be registered")
	}
}
