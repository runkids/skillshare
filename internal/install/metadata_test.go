package install

import (
	"os"
	"path/filepath"
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
	if got, want := info.Mode().Perm(), os.FileMode(0644); got != want {
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
