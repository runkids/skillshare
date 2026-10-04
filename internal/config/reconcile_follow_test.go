package config

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
)

// newFollowReconcileSource declares `_f`, a followed group holding skill a and
// a git repo, and `_off`, an entry whose target is missing.
func newFollowReconcileSource(t *testing.T) (*Config, *sourcewalk.FollowSet) {
	t.Helper()
	root, ext := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "skills")
	for _, dir := range []string{filepath.Join(ext, "a"), filepath.Join(ext, "repo", ".git"), source} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(ext, filepath.Join(source, "_f")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "unmounted"), filepath.Join(source, "_off")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".skillfollow"), []byte("_f\n_off\n"), 0644); err != nil {
		t.Fatal(err)
	}
	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{})
	return &Config{Source: source}, &set
}

func TestReconcileFollow_MarksFollowedNamesLiveWithoutWriting(t *testing.T) {
	cfg, set := newFollowReconcileSource(t)
	store := install.NewMetadataStore()
	store.Set("_f/a", &install.MetadataEntry{Source: "github.com/user/a"})

	if err := ReconcileGlobalSkillsWithOptions(cfg, store, ReconcileOptions{Follow: set}); err != nil {
		t.Fatal(err)
	}
	entry := store.Get("_f/a")
	if entry == nil || entry.Group != "" || entry.Tracked {
		t.Fatalf("followed metadata was pruned or updated: %+v", entry)
	}
	if store.Has("_f/repo") {
		t.Fatalf("following created a tracked entry: %+v", store.Get("_f/repo"))
	}
}

func TestReconcileFollow_KeepsMetadataOnlyUnderUnavailableEntry(t *testing.T) {
	cfg, set := newFollowReconcileSource(t)
	store := install.NewMetadataStore()
	for _, name := range []string{"_off", "_off/a", "gone/b"} {
		store.Set(name, &install.MetadataEntry{Source: "github.com/user/" + filepath.Base(name)})
	}

	if err := ReconcileGlobalSkillsWithOptions(cfg, store, ReconcileOptions{Follow: set}); err != nil {
		t.Fatal(err)
	}
	if !store.Has("_off") || !store.Has("_off/a") {
		t.Fatalf("metadata under the unavailable entry was pruned: %v", store.List())
	}
	if store.Has("gone/b") {
		t.Fatal("stale metadata outside the unavailable entry was kept")
	}
}

// Legacy basename keys carry their group in the entry. Followed names are never
// migrated, so reconcile must read them as logical paths before pruning.
func TestReconcileFollow_KeepsLegacyGroupedKeysUnderFollowedEntries(t *testing.T) {
	cfg, set := newFollowReconcileSource(t)
	store := install.NewMetadataStore()
	store.Set("a", &install.MetadataEntry{Source: "github.com/user/a", Group: "_f"})
	store.Set("b", &install.MetadataEntry{Source: "github.com/user/b", Group: "_off"})
	store.Set("c", &install.MetadataEntry{Source: "github.com/user/c", Group: "gone"})

	if err := ReconcileGlobalSkillsWithOptions(cfg, store, ReconcileOptions{Follow: set}); err != nil {
		t.Fatal(err)
	}
	if !store.Has("a") {
		t.Fatal("legacy key under the followed entry was pruned")
	}
	if !store.Has("b") {
		t.Fatal("legacy key under the unavailable entry was pruned")
	}
	if store.Has("c") {
		t.Fatal("stale legacy key outside any declaration was kept")
	}
}

func TestReconcileFollow_WithoutPolicyIsUnchanged(t *testing.T) {
	cfg, _ := newFollowReconcileSource(t)
	store := install.NewMetadataStore()
	store.Set("_f/a", &install.MetadataEntry{Source: "github.com/user/a"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatal(err)
	}
	if store.Has("_f/a") {
		t.Fatal("legacy reconcile never walks links and prunes their metadata")
	}
}
