package install

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/sourcewalk"
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
	// AuditAccepted maps a resource relPath to audit.AcceptKey values the user
	// accepted via --force. Kept outside Entries so it survives entry rewrites
	// and works for tracked repos that have no entry of their own.
	AuditAccepted map[string][]string `json:"audit_accepted,omitempty"`
	// TargetOverrides maps the source-relative path of a skill inside a tracked
	// repo to the targets set from the dashboard. Stored here so the repo's
	// SKILL.md stays untouched and the clone stays clean for update. An empty
	// list means "all targets"; a missing key falls back to the frontmatter.
	TargetOverrides map[string][]string `json:"target_overrides,omitempty"`
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
	Commit      string            `json:"commit,omitempty"` // full SHA; the lockfile records it
	FileHashes  map[string]string `json:"file_hashes,omitempty"`
	Layout      string            `json:"layout,omitempty"` // local installs: LayoutSkillFile or LayoutDirectory
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
	delete(s.AuditAccepted, name)
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
	// Keys are slash-form; a path from the filesystem is OS-native.
	relPath = filepath.ToSlash(relPath)
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

// MovedEntryKey returns the key of the recorded skill that dir, found at
// relPath under sourcePath, is a moved copy of: a plain skill of the same name
// whose recorded directory is gone and whose recorded file hashes match dir.
// It returns "" unless exactly one entry matches, and an error when dir cannot
// be hashed: it might be a copy, so a search that hit one is incomplete.
func (s *MetadataStore) MovedEntryKey(sourcePath, relPath, dir string, follow *sourcewalk.Follow) (string, error) {
	relPath = filepath.ToSlash(relPath)
	// A directory with a record of its own is that skill, not a moved copy;
	// GetByPath may return the gone record itself through its basename lookup.
	owner := s.GetByPath(relPath)
	var hashes map[string]string
	found := ""
	for _, key := range s.List() {
		e := s.Entries[key]
		if owner != nil && owner != e {
			continue
		}
		old := filepath.ToSlash(KeyToRelPath(key, e))
		if e.Tracked || len(e.FileHashes) == 0 || old == relPath || path.Base(old) != path.Base(relPath) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(sourcePath, filepath.FromSlash(old))); !os.IsNotExist(err) {
			continue
		}
		// Below a source link that cannot be read, the skill may still exist.
		if first, _, nested := strings.Cut(old, "/"); nested {
			top := filepath.Join(sourcePath, first)
			if _, err := os.Lstat(top); err == nil {
				if _, err := os.Stat(top); err != nil {
					continue
				}
			}
		}
		if hashes == nil {
			var err error
			if hashes, err = ComputeFileHashes(dir, follow); err != nil {
				return "", err
			}
		}
		if !maps.Equal(hashes, e.FileHashes) {
			continue
		}
		if found != "" {
			return "", nil
		}
		found = key
	}
	return found, nil
}

// MatchesDeclaration reports whether this plain-skill record still installs
// what skill declares, so a moved copy of it can stand in for skill.
func (e *MetadataEntry) MatchesDeclaration(skill SkillEntryDTO) bool {
	return !e.Tracked && !skill.Tracked && e.Source == skill.Source && e.Branch == skill.Branch
}

// MoveEntry re-keys the entry at oldKey to newKey, with the audit findings
// accepted for it.
func (s *MetadataStore) MoveEntry(oldKey, newKey string) {
	entry := s.Entries[oldKey]
	oldPath := filepath.ToSlash(KeyToRelPath(oldKey, entry))
	accepted, ok := s.AuditAccepted[oldPath]
	s.Remove(oldKey)
	delete(s.AuditAccepted, oldPath)
	delete(s.AuditAccepted, newKey)
	s.Set(newKey, entry)
	if ok {
		s.AuditAccepted[newKey] = accepted
	}
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

// RemoveByNames removes what the store holds for uninstalled skills, groups
// and tracked repos. names are their resolved source-relative paths: an entry
// goes when it is one of them or lies below one, whether its key is a full
// path or a legacy basename with a Group. A basename alone never matches, so
// "foo" and "frontend/foo" stay independent.
func (s *MetadataStore) RemoveByNames(names map[string]bool) {
	for name := range names {
		s.RemoveTargetOverrides(name)
	}
	for _, key := range s.List() {
		entry := s.Get(key)
		full := KeyToRelPath(key, entry)
		for name := range names {
			if full == name ||
				strings.HasPrefix(key, name+"/") || strings.HasPrefix(full, name+"/") ||
				inLegacyRepoGroup(entry, name) {
				s.Remove(key)
				break
			}
		}
	}
}

// inLegacyRepoGroup matches entries written when a top-level tracked repo
// "_team" grouped its skills as "team". Untracked entries and nested repos are
// left out: a plain group or a sibling repo may share that name.
func inLegacyRepoGroup(entry *MetadataEntry, repo string) bool {
	return entry != nil && entry.Tracked && entry.Group != "" &&
		!strings.Contains(repo, "/") && repo == "_"+entry.Group
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
		Layout:      meta.Layout,
		Branch:      meta.Branch,
		Commit:      meta.Commit,
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
		Commit:      src.Commit,
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
func (e *MetadataEntry) ComputeEntryHashes(skillPath string, follow ...*sourcewalk.Follow) error {
	hashes, err := ComputeFileHashes(skillPath, follow...)
	if err != nil {
		return err
	}
	e.FileHashes = hashes
	return nil
}

// RefreshHashes recomputes file hashes for an entry that already has them.
// No-op if entry doesn't exist or has no FileHashes.
func (s *MetadataStore) RefreshHashes(relPath, skillPath string, follow ...*sourcewalk.Follow) {
	entry := s.GetByPath(relPath)
	if entry == nil || entry.FileHashes == nil {
		return
	}
	hashes, err := ComputeFileHashes(skillPath, follow...)
	if err != nil {
		return
	}
	entry.FileHashes = hashes
}

// HasFileHashes reports whether the entry at relPath records file hashes,
// i.e. whether RefreshHashes would recompute them.
func (s *MetadataStore) HasFileHashes(relPath string) bool {
	entry := s.GetByPath(relPath)
	return entry != nil && entry.FileHashes != nil
}

// SetFileHashes stores hashes computed by ComputeFileHashes for an entry that
// already has them. No-op if entry doesn't exist or has no FileHashes. It lets
// callers hash files without holding the lock that guards the store.
func (s *MetadataStore) SetFileHashes(relPath string, hashes map[string]string) {
	entry := s.GetByPath(relPath)
	if entry == nil || entry.FileHashes == nil {
		return
	}
	entry.FileHashes = hashes
}

// RefreshTrackedRootSkillHashes recomputes file hashes for tracked repositories
// that expose a SKILL.md at the repository root.
func (s *MetadataStore) RefreshTrackedRootSkillHashes(relPath, repoPath string, follow ...*sourcewalk.Follow) (bool, error) {
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

	hashes, err := ComputeFileHashes(repoPath, follow...)
	if err != nil {
		return false, err
	}
	if stringMapsEqual(entry.FileHashes, hashes) {
		return false, nil
	}
	entry.FileHashes = hashes
	return true, nil
}

// RefreshTrackedRepoMetadata refreshes a tracked repo's metadata entry and
// reports whether it saved a change. It restores an entry that the old
// dashboard update rewrote as a regular install (#473), but only for a
// tracked checkout, and refreshes root-skill hashes.
func RefreshTrackedRepoMetadata(sourceDir, relPath, repoPath string, follow ...*sourcewalk.Follow) (bool, error) {
	relPath = filepath.ToSlash(relPath)
	store, err := LoadMetadataWithMigration(sourceDir, "")
	if err != nil {
		return false, err
	}
	// Only the repo's own entry: GetByPath's basename fallback can return a
	// top-level item that shares the basename of an --into repo.
	entry := store.GetByPath(relPath)
	if entry == nil || (store.Get(relPath) == nil && entry.Group != path.Dir(relPath)) {
		return false, nil
	}
	// Reconcile may already have set tracked again, leaving the other fields.
	repaired := false
	if (!entry.Tracked || entry.Type != "" || entry.RepoURL != "") && IsTrackedCheckout(repoPath) {
		// Keep only what install --track and reconcile record.
		*entry = MetadataEntry{Source: entry.Source, Kind: entry.Kind, Tracked: true, Group: entry.Group, Branch: entry.Branch}
		repaired = true
	}
	changed, err := store.RefreshTrackedRootSkillHashes(relPath, repoPath, follow...)
	if err != nil || !(changed || repaired) {
		return false, err
	}
	return true, store.Save(sourceDir)
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
