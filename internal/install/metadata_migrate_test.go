package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMigrateMetadata_FromSidecars verifies that a skill dir with a .skillshare-meta.json
// sidecar is migrated: entry appears in store, old sidecar removed, .metadata.json created.
func TestMigrateMetadata_FromSidecars(t *testing.T) {
	dir := t.TempDir()

	// Create skill dir with SKILL.md and sidecar
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n# Content"), 0644)

	meta := &SkillMeta{
		Source:      "github.com/user/repo",
		Type:        "github",
		RepoURL:     "https://github.com/user/repo",
		InstalledAt: time.Now(),
		FileHashes:  map[string]string{"SKILL.md": "sha256:abc123"},
	}
	writeSkillMetaSidecar(t, skillDir, meta)

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Entry should be present
	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected entry 'my-skill' in store")
	}
	if entry.Source != "github.com/user/repo" {
		t.Errorf("Source = %q, want %q", entry.Source, "github.com/user/repo")
	}
	if entry.RepoURL != "https://github.com/user/repo" {
		t.Errorf("RepoURL = %q, want %q", entry.RepoURL, "https://github.com/user/repo")
	}
	if len(entry.FileHashes) == 0 {
		t.Error("expected FileHashes to be populated")
	}

	// Old sidecar should be removed
	sidecarPath := filepath.Join(skillDir, MetaFileName)
	if _, err := os.Stat(sidecarPath); err == nil {
		t.Error("expected old sidecar to be removed")
	}

	// .metadata.json should exist
	if _, err := os.Stat(filepath.Join(dir, MetadataFileName)); err != nil {
		t.Errorf(".metadata.json not created: %v", err)
	}
}

// TestMigrateMetadata_FromRegistry verifies that registry.yaml entries are migrated
// and the old registry.yaml is removed.
func TestMigrateMetadata_FromRegistry(t *testing.T) {
	dir := t.TempDir()

	// Create skill dir (no sidecar)
	skillDir := filepath.Join(dir, "team-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: team-skill\n---\n"), 0644)

	// Write registry.yaml
	registryYAML := `skills:
  - name: team-skill
    source: github.com/org/repo
    tracked: true
    branch: main
`
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0644)

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := store.Get("team-skill")
	if entry == nil {
		t.Fatal("expected entry 'team-skill' in store")
	}
	if entry.Source != "github.com/org/repo" {
		t.Errorf("Source = %q, want %q", entry.Source, "github.com/org/repo")
	}
	if !entry.Tracked {
		t.Error("expected Tracked = true")
	}
	if entry.Branch != "main" {
		t.Errorf("Branch = %q, want %q", entry.Branch, "main")
	}

	// Old registry.yaml should be removed
	if _, err := os.Stat(filepath.Join(dir, "registry.yaml")); err == nil {
		t.Error("expected old registry.yaml to be removed")
	}
}

// TestMigrateMetadata_MergesRegistryAndSidecar verifies that registry fields (group, branch)
// and sidecar fields (repo_url, file_hashes) are merged into a single entry.
func TestMigrateMetadata_DropsOSMetadataHashes(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeSkillMetaSidecar(t, skillDir, &SkillMeta{
		Source:     "github.com/user/repo",
		RepoURL:    "https://github.com/user/repo",
		FileHashes: map[string]string{"SKILL.md": "sha256:a", "assets/.DS_Store": "sha256:b"},
	})

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get("my-skill").FileHashes; len(got) != 1 {
		t.Errorf("FileHashes = %v, want only SKILL.md", got)
	}
}

func TestMigrateMetadata_MergesRegistryAndSidecar(t *testing.T) {
	dir := t.TempDir()

	// Registry has group + branch
	registryYAML := `skills:
  - name: review
    source: github.com/org/tools
    group: frontend
    branch: develop
`
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0644)

	// Sidecar has repo_url + file_hashes (inside group/name subdir)
	skillDir := filepath.Join(dir, "review")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := &SkillMeta{
		Source:     "github.com/org/tools",
		Type:       "github",
		RepoURL:    "https://github.com/org/tools",
		FileHashes: map[string]string{"SKILL.md": "sha256:def456"},
	}
	writeSkillMetaSidecar(t, skillDir, meta)

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := store.Get("review")
	if entry == nil {
		t.Fatal("expected entry 'review' in store")
	}
	// From registry
	if entry.Group != "frontend" {
		t.Errorf("Group = %q, want %q", entry.Group, "frontend")
	}
	if entry.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", entry.Branch, "develop")
	}
	// From sidecar
	if entry.RepoURL != "https://github.com/org/tools" {
		t.Errorf("RepoURL = %q, want %q", entry.RepoURL, "https://github.com/org/tools")
	}
	if len(entry.FileHashes) == 0 {
		t.Error("expected FileHashes to be populated from sidecar")
	}
}

// TestMigrateMetadata_Idempotent verifies that when .metadata.json already exists,
// it is loaded as-is without any migration being attempted.
func TestMigrateMetadata_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Pre-create .metadata.json with known content
	existing := NewMetadataStore()
	existing.Set("pre-existing", &MetadataEntry{
		Source: "github.com/user/existing",
		Kind:   "skill",
	})
	if err := existing.Save(dir); err != nil {
		t.Fatal(err)
	}

	// Also write a registry.yaml that should NOT be processed
	registryYAML := `skills:
  - name: new-skill
    source: github.com/user/new
`
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0644)

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have the pre-existing entry
	if !store.Has("pre-existing") {
		t.Error("expected 'pre-existing' entry from .metadata.json")
	}
	// Should NOT have new-skill (migration was skipped)
	if store.Has("new-skill") {
		t.Error("migration should have been skipped — new-skill should not appear")
	}
	// registry.yaml should still exist (was not cleaned up)
	if _, err := os.Stat(filepath.Join(dir, "registry.yaml")); err != nil {
		t.Error("expected registry.yaml to still exist when migration was skipped")
	}
}

// TestMigrateMetadata_AgentSidecars verifies that agent sidecar files
// (reviewer.skillshare-meta.json) are migrated with Kind="agent".
func TestMigrateMetadata_AgentSidecars(t *testing.T) {
	dir := t.TempDir()

	// Create reviewer.md (the agent file) and reviewer.skillshare-meta.json (sidecar)
	os.WriteFile(filepath.Join(dir, "reviewer.md"), []byte("# Reviewer Agent"), 0644)

	meta := &SkillMeta{
		Source:      "github.com/org/agents",
		Type:        "github",
		RepoURL:     "https://github.com/org/agents",
		InstalledAt: time.Now(),
		Version:     "abc123",
	}
	sidecarPath := filepath.Join(dir, "reviewer"+".skillshare-meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecarPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	store, err := LoadMetadataWithMigration(dir, "agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := store.Get("reviewer")
	if entry == nil {
		t.Fatal("expected entry 'reviewer' in store")
	}
	if entry.Kind != "agent" {
		t.Errorf("Kind = %q, want %q", entry.Kind, "agent")
	}
	if entry.RepoURL != "https://github.com/org/agents" {
		t.Errorf("RepoURL = %q, want %q", entry.RepoURL, "https://github.com/org/agents")
	}
	if entry.Version != "abc123" {
		t.Errorf("Version = %q, want %q", entry.Version, "abc123")
	}

	// Old sidecar should be removed
	if _, err := os.Stat(sidecarPath); err == nil {
		t.Error("expected agent sidecar to be removed after migration")
	}

	// .metadata.json should exist
	if _, err := os.Stat(filepath.Join(dir, MetadataFileName)); err != nil {
		t.Errorf(".metadata.json not created: %v", err)
	}
}

// TestMigrateMetadata_EmptyDir verifies that an empty dir returns an empty store without error.
func TestMigrateMetadata_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	store, err := LoadMetadataWithMigration(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Entries) != 0 {
		t.Errorf("expected empty store, got %d entries", len(store.Entries))
	}

	// .metadata.json should NOT be created when nothing was migrated
	if _, err := os.Stat(filepath.Join(dir, MetadataFileName)); err == nil {
		t.Error("expected .metadata.json to not be created for empty dir")
	}
}

// writeSkillMetaSidecar is a test helper that writes a .skillshare-meta.json sidecar
// inside the given skill directory.
func writeSkillMetaSidecar(t *testing.T, skillDir string, meta *SkillMeta) {
	t.Helper()
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal SkillMeta: %v", err)
	}
	path := filepath.Join(skillDir, MetaFileName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}
