package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestMetadataStore_SetAndGet(t *testing.T) {
	s := NewMetadataStore()
	now := time.Now()
	entry := &MetadataEntry{
		Source:      "org/repo",
		Kind:        "skill",
		Type:        "github",
		Tracked:     true,
		Group:       "mygroup",
		Branch:      "main",
		Into:        "frontend",
		InstalledAt: now,
		RepoURL:     "https://github.com/org/repo.git",
		Subdir:      "skills/foo",
		Version:     "abc123",
		TreeHash:    "deadbeef",
		FileHashes:  map[string]string{"SKILL.md": "sha256:aabbcc"},
	}

	s.Set("foo", entry)
	got := s.Get("foo")

	if got == nil {
		t.Fatal("Get returned nil after Set")
	}
	if got.Source != entry.Source {
		t.Errorf("Source = %q, want %q", got.Source, entry.Source)
	}
	if got.Kind != entry.Kind {
		t.Errorf("Kind = %q, want %q", got.Kind, entry.Kind)
	}
	if got.Type != entry.Type {
		t.Errorf("Type = %q, want %q", got.Type, entry.Type)
	}
	if got.Tracked != entry.Tracked {
		t.Errorf("Tracked = %v, want %v", got.Tracked, entry.Tracked)
	}
	if got.Group != entry.Group {
		t.Errorf("Group = %q, want %q", got.Group, entry.Group)
	}
	if got.Branch != entry.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, entry.Branch)
	}
	if got.Into != entry.Into {
		t.Errorf("Into = %q, want %q", got.Into, entry.Into)
	}
	if !got.InstalledAt.Equal(entry.InstalledAt) {
		t.Errorf("InstalledAt = %v, want %v", got.InstalledAt, entry.InstalledAt)
	}
	if got.RepoURL != entry.RepoURL {
		t.Errorf("RepoURL = %q, want %q", got.RepoURL, entry.RepoURL)
	}
	if got.Subdir != entry.Subdir {
		t.Errorf("Subdir = %q, want %q", got.Subdir, entry.Subdir)
	}
	if got.Version != entry.Version {
		t.Errorf("Version = %q, want %q", got.Version, entry.Version)
	}
	if got.TreeHash != entry.TreeHash {
		t.Errorf("TreeHash = %q, want %q", got.TreeHash, entry.TreeHash)
	}
	if len(got.FileHashes) != 1 || got.FileHashes["SKILL.md"] != "sha256:aabbcc" {
		t.Errorf("FileHashes = %v, want map with one entry", got.FileHashes)
	}
}

func TestMetadataStore_GetMissing(t *testing.T) {
	s := NewMetadataStore()
	got := s.Get("nonexistent")
	if got != nil {
		t.Errorf("Get nonexistent = %v, want nil", got)
	}
}

func TestMetadataStore_GetByPath_NilStore(t *testing.T) {
	var s *MetadataStore // load failure can leave a nil store
	if got := s.GetByPath("any/skill"); got != nil {
		t.Errorf("GetByPath on nil store = %v, want nil (must not panic)", got)
	}
}

func TestMetadataStore_Has(t *testing.T) {
	s := NewMetadataStore()
	s.Set("present", &MetadataEntry{Source: "org/repo"})

	if !s.Has("present") {
		t.Error("Has(present) = false, want true")
	}
	if s.Has("absent") {
		t.Error("Has(absent) = true, want false")
	}
}

func TestMetadataStore_Remove(t *testing.T) {
	s := NewMetadataStore()
	s.Set("to-remove", &MetadataEntry{Source: "org/repo"})

	if !s.Has("to-remove") {
		t.Fatal("entry should exist before Remove")
	}

	s.Remove("to-remove")

	if s.Has("to-remove") {
		t.Error("entry still present after Remove")
	}
	if s.Get("to-remove") != nil {
		t.Error("Get after Remove should return nil")
	}
}

func TestMetadataStore_Remove_Nonexistent(t *testing.T) {
	s := NewMetadataStore()
	// Should not panic
	s.Remove("nonexistent")
}

func TestMetadataStore_List(t *testing.T) {
	s := NewMetadataStore()
	s.Set("zebra", &MetadataEntry{})
	s.Set("alpha", &MetadataEntry{})
	s.Set("mango", &MetadataEntry{})

	names := s.List()

	if len(names) != 3 {
		t.Fatalf("List() = %v, want 3 entries", names)
	}
	want := []string{"alpha", "mango", "zebra"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("List()[%d] = %q, want %q", i, names[i], w)
		}
	}
}

func TestMetadataStore_List_Empty(t *testing.T) {
	s := NewMetadataStore()
	names := s.List()
	if len(names) != 0 {
		t.Errorf("List() on empty store = %v, want []", names)
	}
}

func TestMetadataEntry_EffectiveKind(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want string
	}{
		{"empty kind defaults to skill", "", "skill"},
		{"explicit skill", "skill", "skill"},
		{"agent", "agent", "agent"},
		{"custom kind preserved", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &MetadataEntry{Kind: tt.kind}
			got := e.EffectiveKind()
			if got != tt.want {
				t.Errorf("EffectiveKind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewMetadataStore_InitialState(t *testing.T) {
	s := NewMetadataStore()
	if s == nil {
		t.Fatal("NewMetadataStore returned nil")
	}
	if s.Version != 1 {
		t.Errorf("Version = %d, want 1", s.Version)
	}
	if s.Entries == nil {
		t.Error("Entries map is nil")
	}
	if len(s.Entries) != 0 {
		t.Errorf("Entries not empty on new store: %v", s.Entries)
	}
}

func TestMetadataStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewMetadataStore()
	store.Set("my-skill", &MetadataEntry{
		Source:      "github.com/user/repo",
		Type:        "github",
		InstalledAt: time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
		FileHashes:  map[string]string{"SKILL.md": "sha256:abc123"},
	})

	if err := store.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists with repository-friendly permissions.
	metaPath := filepath.Join(dir, MetadataFileName)
	info, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("metadata file not created: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0644); runtime.GOOS != "windows" && got != want { // no permission bits on Windows
		t.Fatalf("metadata file mode = %v, want %v", got, want)
	}

	// Load and verify round-trip
	loaded, err := LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata failed: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("version = %d, want 1", loaded.Version)
	}
	entry := loaded.Get("my-skill")
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Source != "github.com/user/repo" {
		t.Errorf("source = %q, want %q", entry.Source, "github.com/user/repo")
	}
	if entry.FileHashes["SKILL.md"] != "sha256:abc123" {
		t.Errorf("file hash mismatch")
	}
}

func TestLoadMetadata_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store, err := LoadMetadata(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.Version != 1 {
		t.Errorf("version = %d, want 1", store.Version)
	}
	if len(store.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(store.Entries))
	}
}

func TestLoadMetadata_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, MetadataFileName), []byte("{invalid"), 0644)
	_, err := LoadMetadata(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMetadataStore_SaveAtomic_NoTempFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewMetadataStore()
	store.Set("a", &MetadataEntry{Source: "s1"})
	if err := store.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != MetadataFileName {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestMetadataStore_SaveCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	store := NewMetadataStore()
	store.Set("x", &MetadataEntry{Source: "s"})
	if err := store.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, MetadataFileName)); err != nil {
		t.Fatalf("file should exist in nested dir: %v", err)
	}
}

func TestMetadataPath(t *testing.T) {
	got := MetadataPath("/some/dir")
	want := filepath.Join("/some/dir", MetadataFileName)
	if got != want {
		t.Errorf("MetadataPath = %q, want %q", got, want)
	}
}

func TestMetadataStore_SetFromSource(t *testing.T) {
	store := NewMetadataStore()
	source := &Source{
		Raw:      "github.com/user/repo",
		CloneURL: "https://github.com/user/repo.git",
		Branch:   "dev",
		Subdir:   "skills\\review",
	}
	source.Type = SourceTypeGitHub

	entry := store.SetFromSource("review", source)
	if entry.Source != "github.com/user/repo" {
		t.Errorf("source = %q", entry.Source)
	}
	if entry.RepoURL != "https://github.com/user/repo.git" {
		t.Errorf("repo_url = %q", entry.RepoURL)
	}
	if entry.Branch != "dev" {
		t.Errorf("branch = %q", entry.Branch)
	}
	if entry.Subdir != "skills/review" {
		t.Errorf("subdir = %q, want forward slashes", entry.Subdir)
	}
	if entry.InstalledAt.IsZero() {
		t.Error("installed_at should be set")
	}
	if !store.Has("review") {
		t.Error("entry not stored")
	}
}

func TestMetadataEntry_ComputeEntryHashes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Test"), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	entry := &MetadataEntry{}
	if err := entry.ComputeEntryHashes(dir); err != nil {
		t.Fatalf("ComputeEntryHashes failed: %v", err)
	}
	if _, ok := entry.FileHashes["SKILL.md"]; !ok {
		t.Error("expected SKILL.md in file hashes")
	}
	if len(entry.FileHashes) != 1 {
		t.Errorf("expected 1 hash (SKILL.md only), got %d: %v", len(entry.FileHashes), entry.FileHashes)
	}
}

func TestMetadataStore_RefreshHashes(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# V1"), 0644)

	store := NewMetadataStore()
	entry := &MetadataEntry{
		Source:     "test",
		FileHashes: map[string]string{"SKILL.md": "sha256:old"},
	}
	store.Set("my-skill", entry)

	store.RefreshHashes("my-skill", skillDir)

	refreshed := store.Get("my-skill")
	if refreshed.FileHashes["SKILL.md"] == "sha256:old" {
		t.Error("hashes should have been refreshed")
	}
	if refreshed.FileHashes["SKILL.md"] == "" {
		t.Error("hash should not be empty after refresh")
	}
}

func TestMetadataStore_RefreshHashes_NoOp(t *testing.T) {
	store := NewMetadataStore()
	// No entry — should not panic
	store.RefreshHashes("nonexistent", "/tmp")

	// Entry without FileHashes — should not compute
	store.Set("x", &MetadataEntry{Source: "s"})
	store.RefreshHashes("x", "/tmp")
	if store.Get("x").FileHashes != nil {
		t.Error("should not compute hashes when FileHashes is nil")
	}
}

// The repair looks up the repo's own key only: the basename fallback of
// GetByPath would hand it a different top-level item's entry.
func TestRefreshTrackedRepoMetadata_LeavesBasenameSiblingAlone(t *testing.T) {
	src := t.TempDir()
	repo := filepath.Join(src, "org", "_team")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("_team", &MetadataEntry{Source: "github.com/example/team", Type: "github", Version: "v1"})
	if err := store.Save(src); err != nil {
		t.Fatal(err)
	}

	if _, err := RefreshTrackedRepoMetadata(src, "org/_team", repo); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMetadata(src)
	if err != nil {
		t.Fatal(err)
	}
	if e := got.Get("_team"); e == nil || e.Tracked || e.Version != "v1" {
		t.Errorf("sibling entry was rewritten: %+v", e)
	}
}

// An --into repo with no entry of its own must not refresh the root-skill
// hashes of a top-level repo that shares its basename.
func TestRefreshTrackedRepoMetadata_NestedRepoWithoutEntry(t *testing.T) {
	src := t.TempDir()
	repo := filepath.Join(src, "org", "_team")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: team\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("_team", &MetadataEntry{Source: "github.com/example/team", Tracked: true, FileHashes: map[string]string{"SKILL.md": "sha256:top"}})
	if err := store.Save(src); err != nil {
		t.Fatal(err)
	}

	if _, err := RefreshTrackedRepoMetadata(src, "org/_team", repo); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMetadata(src)
	if err != nil {
		t.Fatal(err)
	}
	if h := got.Get("_team").FileHashes["SKILL.md"]; h != "sha256:top" {
		t.Errorf("top-level _team hashes were overwritten: %q", h)
	}
}

// TestMovedEntryKey_IgnoresRecordUnderUnavailableLink verifies that a record
// below a source link that cannot be read is not treated as moved: the skill
// may still be there once the link returns.
func TestMovedEntryKey_IgnoresRecordUnderUnavailableLink(t *testing.T) {
	sourceDir := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(sourceDir, "linked")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	copyDir := filepath.Join(sourceDir, "other", "demo")
	if err := os.MkdirAll(copyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyDir, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hashes, err := ComputeFileHashes(copyDir)
	if err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("linked/demo", &MetadataEntry{Source: "github.com/user/repo/demo", Group: "linked", FileHashes: hashes})

	if key, _ := store.MovedEntryKey(sourceDir, "other/demo", copyDir, nil); key != "" {
		t.Errorf("MovedEntryKey() = %q, want no match below an unavailable link", key)
	}
}

// TestMoveEntry_DropsLegacyPathAuditAcceptance verifies that findings accepted
// for a legacy basename key's path do not stay behind for whatever later
// occupies that path.
func TestMoveEntry_DropsLegacyPathAuditAcceptance(t *testing.T) {
	store := NewMetadataStore()
	store.Set("demo", &MetadataEntry{Source: "github.com/user/repo/demo", Group: "old"})
	store.AuditAccepted = map[string][]string{"old/demo": {"accepted-key"}}

	store.MoveEntry("demo", "new/demo")

	if got, ok := store.AuditAccepted["old/demo"]; ok {
		t.Errorf("accepted findings left at the old path: %v", got)
	}
}

// TestMoveEntry_ClearsStaleAuditAcceptanceAtDestination verifies that a moved
// skill does not inherit findings accepted for whatever held its new path.
func TestMoveEntry_ClearsStaleAuditAcceptanceAtDestination(t *testing.T) {
	store := NewMetadataStore()
	store.Set("old/demo", &MetadataEntry{Source: "github.com/user/repo/demo", Group: "old"})
	store.AuditAccepted = map[string][]string{"new/demo": {"stale-key"}}

	store.MoveEntry("old/demo", "new/demo")

	if got, ok := store.AuditAccepted["new/demo"]; ok {
		t.Errorf("moved skill inherited accepted findings: %v", got)
	}
}

// TestMovedEntryKey_ReportsHashFailure verifies that a candidate that cannot
// be hashed is reported, so the search counts as incomplete: it may be a
// second copy that would make the move ambiguous.
func TestMovedEntryKey_ReportsHashFailure(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("root, and Windows, read files regardless of mode")
	}
	sourceDir := t.TempDir()
	copyDir := filepath.Join(sourceDir, "other", "demo")
	if err := os.MkdirAll(copyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyDir, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0); err != nil {
		t.Fatal(err)
	}
	store := NewMetadataStore()
	store.Set("demo", &MetadataEntry{Source: "github.com/user/repo/demo", FileHashes: map[string]string{"SKILL.md": "sha256:x"}})

	if _, err := store.MovedEntryKey(sourceDir, "other/demo", copyDir, nil); err == nil {
		t.Error("MovedEntryKey() hid a candidate it could not hash")
	}
}

// TestMetadataStore_MovePath re-keys a folder's records whether they are keyed
// by full path or by a legacy basename with a Group, and carries the findings
// accepted for them, with or without an entry of their own.
func TestMetadataStore_MovePath(t *testing.T) {
	store := NewMetadataStore()
	store.Set("frontend/full", &MetadataEntry{Source: "s/full", Group: "frontend"})
	store.Set("legacy", &MetadataEntry{Source: "s/legacy", Group: "frontend/deep"})
	store.Set("frontend", &MetadataEntry{Source: "s/folder-skill"})
	store.Set("frontend-other", &MetadataEntry{Source: "s/other"})
	store.AuditAccepted = map[string][]string{
		"frontend/full":        {"a"},
		"frontend/deep/legacy": {"b"},
		"frontend/local":       {"c"}, // accepted for a skill with no entry
	}

	if got := store.MovePath("frontend", "archive/frontend"); got != 3 {
		t.Fatalf("MovePath moved %d entries, want 3", got)
	}

	for _, key := range []string{"archive/frontend/full", "archive/frontend/deep/legacy", "archive/frontend"} {
		if !store.Has(key) {
			t.Errorf("missing entry %q; keys = %v", key, store.List())
		}
	}
	if got := store.Get("archive/frontend/deep/legacy"); got.Group != "archive/frontend/deep" {
		t.Errorf("legacy entry group = %q, want archive/frontend/deep", got.Group)
	}
	if got := store.Get("archive/frontend"); got.Group != "archive" {
		t.Errorf("folder skill group = %q, want archive", got.Group)
	}
	if !store.Has("frontend-other") {
		t.Error("a sibling that only shares the prefix moved too")
	}
	for _, key := range []string{"archive/frontend/full", "archive/frontend/deep/legacy", "archive/frontend/local"} {
		if len(store.AuditAccepted[key]) != 1 {
			t.Errorf("accepted findings for %q = %v, want them carried", key, store.AuditAccepted[key])
		}
	}
	if len(store.AuditAccepted) != 3 {
		t.Errorf("accepted findings left at old paths: %v", store.AuditAccepted)
	}
}
