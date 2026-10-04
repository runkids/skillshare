package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/sourcefs"
)

// cleanupSidecars runs both skill and agent sidecar migration on dir.
// Returns true if any sidecars were found and cleaned up.
func cleanupSidecars(store *MetadataStore, dir string) bool {
	before := storeFingerprint(store)
	migrateSkillSidecars(store, dir)
	migrateAgentSidecars(store, dir)
	return storeFingerprint(store) != before
}

// storeFingerprint returns a cheap fingerprint for change detection.
func storeFingerprint(s *MetadataStore) uint64 {
	var h uint64
	for k, e := range s.Entries {
		h += uint64(len(k))
		if e != nil && e.Source != "" {
			h += uint64(len(e.Source))
		}
	}
	return h
}

// LoadMetadataWithMigration loads .metadata.json, or migrates from old format if needed.
// kind is "" for skills directories, "agent" for agents directories.
// When .metadata.json already exists, LoadMetadata handles sidecar cleanup automatically.
func LoadMetadataWithMigration(dir, kind string) (*MetadataStore, error) {
	// Fast path: .metadata.json exists — LoadMetadata handles sidecar cleanup.
	metaPath := filepath.Join(dir, MetadataFileName)
	if _, err := os.Stat(metaPath); err == nil {
		return LoadMetadata(dir)
	}

	store := NewMetadataStore()

	// Phase 1: Migrate registry.yaml entries
	migrateRegistryEntries(store, dir, kind)
	if parent := filepath.Dir(dir); parent != dir {
		migrateRegistryEntries(store, parent, kind)
	}

	// Phase 2: Migrate sidecar .skillshare-meta.json files
	if kind == "agent" {
		migrateAgentSidecars(store, dir)
	} else {
		migrateSkillSidecars(store, dir)
	}

	// Phase 3: Save if we found anything to migrate
	if len(store.Entries) > 0 {
		if err := store.Save(dir); err != nil {
			return store, err
		}
	}

	// Phase 4: Clean up old registry.yaml (in dir and parent)
	cleanupOldRegistry(dir)

	return store, nil
}

// localRegistryEntry mirrors config.SkillEntry without importing internal/config.
type localRegistryEntry struct {
	Name    string `yaml:"name"`
	Kind    string `yaml:"kind,omitempty"`
	Source  string `yaml:"source"`
	Tracked bool   `yaml:"tracked,omitempty"`
	Group   string `yaml:"group,omitempty"`
	Branch  string `yaml:"branch,omitempty"`
}

// localRegistry mirrors config.Registry without importing internal/config.
type localRegistry struct {
	Skills []localRegistryEntry `yaml:"skills,omitempty"`
}

// migrateRegistryEntries reads registry.yaml in dir and merges matching entries into store.
// For skills dirs (kind=""), agent entries are skipped.
// For agents dirs (kind="agent"), skill entries are skipped.
func migrateRegistryEntries(store *MetadataStore, dir, kind string) {
	registryPath := filepath.Join(dir, "registry.yaml")
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return
	}

	var reg localRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return
	}

	for _, e := range reg.Skills {
		if e.Name == "" || e.Source == "" {
			continue
		}

		isAgent := e.Kind == "agent"

		// Filter: skills dir skips agent entries, agents dir skips skill entries
		if kind == "agent" && !isAgent {
			continue
		}
		if kind == "" && isAgent {
			continue
		}

		entry := store.Get(e.Name)
		if entry == nil {
			entry = &MetadataEntry{}
			store.Set(e.Name, entry)
		}

		entry.Source = e.Source
		entry.Kind = e.Kind
		entry.Tracked = e.Tracked
		entry.Group = e.Group
		entry.Branch = e.Branch
	}
}

// migrateSkillSidecars walks subdirectories of dir, looks for .skillshare-meta.json
// inside each, reads as SkillMeta, merges fields into store entry, and removes old sidecar.
func migrateSkillSidecars(store *MetadataStore, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	root, err := sourcefs.Open(dir)
	if err != nil {
		return
	}
	defer root.Close()

	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		skillName := de.Name()
		skillPath := filepath.Join(dir, skillName)
		walkSkillDir(store, root, skillPath, skillName, "")
	}
}

// walkSkillDir recursively walks a skill directory to find .skillshare-meta.json sidecars.
// group is the parent group prefix (empty for top-level skills). A sidecar is
// removed through root; one that is or sits below a link stays, and the next
// load skips it again.
func walkSkillDir(store *MetadataStore, root *sourcefs.Root, skillPath, name, group string) {
	sidecarPath := filepath.Join(skillPath, MetaFileName)
	if _, err := os.Stat(sidecarPath); err == nil {
		// This directory has a sidecar — it's a leaf skill
		mergeSkillSidecar(store, name, group, sidecarPath)
		if rel, err := root.Rel(sidecarPath); err == nil {
			root.Remove(rel) //nolint:errcheck
		}
		return
	}

	// Check if this has subdirectories (nested skills)
	subEntries, err := os.ReadDir(skillPath)
	if err != nil {
		return
	}

	for _, sub := range subEntries {
		if sub.IsDir() {
			subGroup := name
			if group != "" {
				subGroup = group + "/" + name
			}
			walkSkillDir(store, root, filepath.Join(skillPath, sub.Name()), sub.Name(), subGroup)
		}
	}
}

// mergeSkillSidecar reads a SkillMeta sidecar and merges its fields into the store.
func mergeSkillSidecar(store *MetadataStore, name, group, sidecarPath string) {
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return
	}

	var meta SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}

	// Use full-path key for grouped skills.
	key := name
	if group != "" {
		key = group + "/" + name
	}

	entry := store.Get(key)
	if entry == nil {
		entry = &MetadataEntry{}
		store.Set(key, entry)
	}

	// Merge sidecar fields — sidecar has richer data
	if meta.Source != "" && entry.Source == "" {
		entry.Source = meta.Source
	}
	if meta.Kind != "" {
		entry.Kind = meta.Kind
	}
	if meta.Type != "" {
		entry.Type = meta.Type
	}
	if !meta.InstalledAt.IsZero() {
		entry.InstalledAt = meta.InstalledAt
	}
	if meta.RepoURL != "" {
		entry.RepoURL = meta.RepoURL
	}
	if meta.Subdir != "" {
		entry.Subdir = meta.Subdir
	}
	if meta.Version != "" {
		entry.Version = meta.Version
	}
	if meta.TreeHash != "" {
		entry.TreeHash = meta.TreeHash
	}
	if meta.FileHashes != nil {
		entry.FileHashes = meta.FileHashes
	}
	if meta.Branch != "" && entry.Branch == "" {
		entry.Branch = meta.Branch
	}
	if group != "" && entry.Group == "" {
		entry.Group = group
	}
	// Detect tracked repos: top-level parent starts with "_"
	root := group
	if idx := strings.Index(root, "/"); idx >= 0 {
		root = root[:idx]
	}
	if len(root) > 0 && root[0] == '_' {
		entry.Tracked = true
	}
}

// migrateAgentSidecars recursively scans dir for *.skillshare-meta.json files,
// merges them into the centralized store with Kind="agent", and removes the sidecars.
func migrateAgentSidecars(store *MetadataStore, dir string) {
	walkAgentSidecars(store, dir, "")
}

// walkAgentSidecars recursively walks dir for agent sidecar files.
// group is the parent prefix (empty for top-level, e.g. "demo" for agents/demo/).
func walkAgentSidecars(store *MetadataStore, dir, group string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	const suffix = ".skillshare-meta.json"
	for _, de := range entries {
		if de.IsDir() {
			subGroup := de.Name()
			if group != "" {
				subGroup = group + "/" + de.Name()
			}
			walkAgentSidecars(store, filepath.Join(dir, de.Name()), subGroup)
			continue
		}
		if !strings.HasSuffix(de.Name(), suffix) {
			continue
		}

		agentName := strings.TrimSuffix(de.Name(), suffix)
		if agentName == "" {
			continue
		}
		// Use full-path key for grouped agents (e.g. "demo/reviewer")
		key := agentName
		if group != "" {
			key = group + "/" + agentName
		}

		sidecarPath := filepath.Join(dir, de.Name())
		mergeAgentSidecar(store, key, group, sidecarPath)
		os.Remove(sidecarPath)
	}
}

// mergeAgentSidecar reads a SkillMeta sidecar and merges its fields into the store.
func mergeAgentSidecar(store *MetadataStore, key, group, sidecarPath string) {
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return
	}

	var meta SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}

	entry := store.Get(key)
	if entry == nil {
		entry = &MetadataEntry{}
		store.Set(key, entry)
	}

	if meta.Source != "" && entry.Source == "" {
		entry.Source = meta.Source
	}
	entry.Kind = "agent"
	if meta.Type != "" {
		entry.Type = meta.Type
	}
	if !meta.InstalledAt.IsZero() {
		entry.InstalledAt = meta.InstalledAt
	}
	if meta.RepoURL != "" {
		entry.RepoURL = meta.RepoURL
	}
	if meta.Subdir != "" {
		entry.Subdir = meta.Subdir
	}
	if meta.Version != "" {
		entry.Version = meta.Version
	}
	if meta.TreeHash != "" {
		entry.TreeHash = meta.TreeHash
	}
	if meta.FileHashes != nil {
		entry.FileHashes = meta.FileHashes
	}
	if meta.Branch != "" && entry.Branch == "" {
		entry.Branch = meta.Branch
	}
	if group != "" && entry.Group == "" {
		entry.Group = group
	}
}

// cleanupOldRegistry removes the old registry.yaml from dir, through a
// handle so a registry.yaml that is a link stays, and from the config folder
// above dir.
func cleanupOldRegistry(dir string) {
	if root, err := sourcefs.Open(dir); err == nil {
		root.Remove("registry.yaml") //nolint:errcheck
		root.Close()
	}
	if parent := filepath.Dir(dir); parent != dir {
		os.Remove(filepath.Join(parent, "registry.yaml"))
	}
}
