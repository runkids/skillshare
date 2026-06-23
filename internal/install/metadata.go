package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MetadataFileName is the centralized metadata file stored in each directory.
const MetadataFileName = ".metadata.json"

const metadataFileMode os.FileMode = 0644

// Metadata kind constants for LoadMetadataWithMigration.
const (
	MetadataKindSkill = ""      // default kind for skills directories
	MetadataKindAgent = "agent" // kind for agents directories
)

// MetadataStore holds all entries for a single directory (skills/ or agents/).
type MetadataStore struct {
	Version int                       `json:"version"`
	Entries map[string]*MetadataEntry `json:"entries"`
}

// MetadataEntry merges the old SkillMeta + RegistryEntry fields.
type MetadataEntry struct {
	// Registry fields
	Source  string `json:"source"`
	Kind    string `json:"kind,omitempty"`
	Type    string `json:"type,omitempty"`
	Tracked bool   `json:"tracked,omitempty"`
	Group   string `json:"group,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Into    string `json:"into,omitempty"`

	// Meta fields
	InstalledAt time.Time         `json:"installed_at,omitzero"`
	RepoURL     string            `json:"repo_url,omitempty"`
	Subdir      string            `json:"subdir,omitempty"`
	Version     string            `json:"version,omitempty"`
	TreeHash    string            `json:"tree_hash,omitempty"`
	FileHashes  map[string]string `json:"file_hashes,omitempty"`
}

// NewMetadataStore returns an empty store with version 1.
func NewMetadataStore() *MetadataStore {
	return &MetadataStore{
		Version: 1,
		Entries: make(map[string]*MetadataEntry),
	}
}

// Get returns the entry for the given name, or nil if not found.
func (s *MetadataStore) Get(name string) *MetadataEntry {
	return s.Entries[name]
}

// Set adds or replaces an entry.
func (s *MetadataStore) Set(name string, entry *MetadataEntry) {
	s.Entries[name] = entry
}

// Remove deletes an entry by name.
func (s *MetadataStore) Remove(name string) {
	delete(s.Entries, name)
}

// Has returns true if an entry exists for the given name.
func (s *MetadataStore) Has(name string) bool {
	_, ok := s.Entries[name]
	return ok
}

// GetByPath looks up an entry by its full relative path (e.g. "mygroup/keep-nested").
// It first tries a direct key lookup, then falls back to matching group+basename.
// This handles the case where entries are stored with basename keys but have a Group field.
func (s *MetadataStore) GetByPath(relPath string) *MetadataEntry {
	if s == nil {
		return nil
	}
	// Direct lookup (works for top-level skills where key == relPath)
	if e := s.Entries[relPath]; e != nil {
		return e
	}
	// Basename + group lookup (for nested skills stored with basename key)
	base := filepath.Base(relPath)
	group := ""
	if dir := filepath.Dir(relPath); dir != "." {
		group = filepath.ToSlash(dir)
	}
	if e := s.Entries[base]; e != nil && e.Group == group {
		return e
	}
	// Basename-only fallback: tracked repos store metadata with short keys
	// (e.g. "agent-browser") but discovery produces full relPaths including
	// the repo prefix (e.g. "_repo/agent-browser/agent-browser").
	if e := s.Entries[base]; e != nil && e.Group == "" {
		return e
	}
	return nil
}

// KeyToRelPath returns the effective relative path for a store entry.
// For full-path keys it returns the key as-is; for legacy basename keys
// it prepends the entry's Group.
func KeyToRelPath(key string, entry *MetadataEntry) string {
	if entry != nil && entry.Group != "" && !strings.HasPrefix(key, entry.Group+"/") {
		return entry.Group + "/" + key
	}
	return key
}

// MigrateLegacyKey promotes a legacy basename key to a full-path key.
// Returns true if migration occurred. No-op if the key is already full-path.
func (s *MetadataStore) MigrateLegacyKey(fullPath string, existing *MetadataEntry) bool {
	if s.Has(fullPath) {
		return false
	}
	s.Remove(filepath.Base(fullPath))
	s.Set(fullPath, existing)
	return true
}

// List returns sorted entry names.
func (s *MetadataStore) List() []string {
	names := make([]string, 0, len(s.Entries))
	for name := range s.Entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EffectiveKind returns "skill" if Kind is empty.
func (e *MetadataEntry) EffectiveKind() string {
	if e.Kind == "" {
		return "skill"
	}
	return e.Kind
}

// RemoveByNames removes entries matching the given names, including group members.
// Handles direct key matches, full-path matches (group/name), and group membership.
// Works with both legacy basename keys and full-path keys.
func (s *MetadataStore) RemoveByNames(names map[string]bool) {
	for _, key := range s.List() {
		entry := s.Get(key)
		fullName := KeyToRelPath(key, entry)
		if names[key] || names[fullName] {
			s.Remove(key)
			continue
		}
		// Also match by basename for backward compat (e.g. uninstall "foo" should
		// remove full-path key "frontend/foo").
		if entry != nil && entry.Group != "" {
			basename := key
			if idx := strings.LastIndex(key, "/"); idx >= 0 {
				basename = key[idx+1:]
			}
			if names[basename] {
				s.Remove(key)
				continue
			}
		}
		// Group directory uninstall: remove member skills
		if entry != nil && entry.Group != "" {
			for rn := range names {
				if entry.Group == rn || strings.HasPrefix(entry.Group, rn+"/") {
					s.Remove(key)
					break
				}
			}
		}
	}
}

// WriteMetaToStore writes a SkillMeta to the centralized .metadata.json store.
// sourceDir is the skills root (if empty, defaults to parent of destPath).
// destPath is the installed skill path.
func WriteMetaToStore(sourceDir, destPath string, meta *SkillMeta) error {
	if sourceDir == "" {
		sourceDir = filepath.Dir(destPath)
	}
	rel, err := filepath.Rel(sourceDir, destPath)
	if err != nil {
		return fmt.Errorf("relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if meta.Kind == MetadataKindAgent {
		rel = strings.TrimSuffix(rel, ".md")
	}

	// Extract group from relative path (e.g. "frontend/foo" → group "frontend").
	group := ""
	if idx := strings.LastIndex(rel, "/"); idx >= 0 {
		group = rel[:idx]
	}

	store, loadErr := LoadMetadata(sourceDir)
	if loadErr != nil {
		store = NewMetadataStore()
	}

	// Use full relative path as key to avoid collisions between grouped
	// skills with the same basename (e.g. "frontend/foo" vs "backend/foo").
	// Remove any legacy basename-only key for this group+basename pair.
	if group != "" {
		basename := rel[strings.LastIndex(rel, "/")+1:]
		if old := store.Get(basename); old != nil && old.Group == group {
			store.Remove(basename)
		}
	}

	store.Set(rel, &MetadataEntry{
		Source:      meta.Source,
		Kind:        meta.Kind,
		Type:        meta.Type,
		Group:       group,
		InstalledAt: meta.InstalledAt,
		RepoURL:     meta.RepoURL,
		Subdir:      meta.Subdir,
		Version:     meta.Version,
		TreeHash:    meta.TreeHash,
		FileHashes:  meta.FileHashes,
		Branch:      meta.Branch,
	})
	return store.Save(sourceDir)
}

// loadMetadataFile reads .metadata.json from the given directory (pure read, no migration).
// Returns an empty store (version 1) if the file does not exist.
func loadMetadataFile(dir string) (*MetadataStore, error) {
	path := filepath.Join(dir, MetadataFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewMetadataStore(), nil
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var store MetadataStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}
	if store.Entries == nil {
		store.Entries = make(map[string]*MetadataEntry)
	}
	return &store, nil
}

// LoadMetadata reads .metadata.json and cleans up any lingering sidecar files.
// Sidecar migration is idempotent — if no sidecars exist, it's a fast no-op
// (one ReadDir per call). This ensures sidecars created after initial migration
// (e.g. by agent install) are always cleaned up regardless of which command runs.
func LoadMetadata(dir string) (*MetadataStore, error) {
	store, err := loadMetadataFile(dir)
	if err != nil {
		return nil, err
	}
	if cleanupSidecars(store, dir) {
		store.Save(dir) //nolint:errcheck
	}
	return store, nil
}

// LoadMetadataOrNew loads metadata from dir, returning an empty store on error.
func LoadMetadataOrNew(dir string) *MetadataStore {
	store, _ := LoadMetadata(dir)
	if store == nil {
		return NewMetadataStore()
	}
	return store
}

// Save writes .metadata.json atomically (temp file → rename).
func (s *MetadataStore) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(dir, MetadataFileName)
	tmp, err := os.CreateTemp(dir, ".metadata-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Chmod(metadataFileMode); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	return nil
}

// MetadataPath returns the .metadata.json path for the given directory.
func MetadataPath(dir string) string {
	return filepath.Join(dir, MetadataFileName)
}

// SetFromSource creates an entry from a Source and stores it. Returns the entry.
func (s *MetadataStore) SetFromSource(name string, src *Source) *MetadataEntry {
	entry := &MetadataEntry{
		Source:      src.Raw,
		Type:        src.MetaType(),
		InstalledAt: time.Now(),
		Branch:      src.Branch,
	}
	if src.IsGit() {
		entry.RepoURL = src.CloneURL
	}
	if src.HasSubdir() {
		entry.Subdir = strings.ReplaceAll(src.Subdir, "\\", "/")
	}
	s.Entries[name] = entry
	return entry
}

// ComputeEntryHashes walks skillPath and populates FileHashes with sha256 digests.
// Delegates to ComputeFileHashes in meta.go.
func (e *MetadataEntry) ComputeEntryHashes(skillPath string) error {
	hashes, err := ComputeFileHashes(skillPath)
	if err != nil {
		return err
	}
	e.FileHashes = hashes
	return nil
}

// RefreshHashes recomputes file hashes for an entry that already has them.
// No-op if entry doesn't exist or has no FileHashes.
func (s *MetadataStore) RefreshHashes(relPath, skillPath string) {
	entry := s.GetByPath(relPath)
	if entry == nil || entry.FileHashes == nil {
		return
	}
	hashes, err := ComputeFileHashes(skillPath)
	if err != nil {
		return
	}
	entry.FileHashes = hashes
}

// RefreshTrackedRootSkillHashes recomputes file hashes for tracked repositories
// that expose a SKILL.md at the repository root.
func (s *MetadataStore) RefreshTrackedRootSkillHashes(relPath, repoPath string) (bool, error) {
	if _, err := os.Stat(filepath.Join(repoPath, "SKILL.md")); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	entry := s.GetByPath(relPath)
	if entry == nil {
		return false, nil
	}

	hashes, err := ComputeFileHashes(repoPath)
	if err != nil {
		return false, err
	}
	if stringMapsEqual(entry.FileHashes, hashes) {
		return false, nil
	}
	entry.FileHashes = hashes
	return true, nil
}

// RefreshTrackedRootSkillMetadata loads the metadata store, refreshes hashes
// for a tracked root-skill repo, and saves only when metadata changed.
func RefreshTrackedRootSkillMetadata(sourceDir, relPath, repoPath string) error {
	store, err := LoadMetadataWithMigration(sourceDir, "")
	if err != nil {
		return err
	}
	changed, err := store.RefreshTrackedRootSkillHashes(filepath.ToSlash(relPath), repoPath)
	if err != nil || !changed {
		return err
	}
	return store.Save(sourceDir)
}

func stringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, aValue := range a {
		if b[key] != aValue {
			return false
		}
	}
	return true
}
