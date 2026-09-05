package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/utils"
)

// MetaFileName is the name of the skillshare metadata file stored in each skill directory.
const MetaFileName = ".skillshare-meta.json"

// SkillMeta contains metadata about an installed skill or agent
type SkillMeta struct {
	Source      string            `json:"source"`                // Original source input
	Kind        string            `json:"kind,omitempty"`        // "skill" (default/empty) or "agent"
	Type        string            `json:"type"`                  // Source type (github, local, etc.)
	InstalledAt time.Time         `json:"installed_at"`          // Installation timestamp
	RepoURL     string            `json:"repo_url,omitempty"`    // Git repo URL (for git sources)
	Subdir      string            `json:"subdir,omitempty"`      // Subdirectory path (for monorepo)
	Version     string            `json:"version,omitempty"`     // Git commit hash or version
	TreeHash    string            `json:"tree_hash,omitempty"`   // Git tree SHA of Subdir
	FileHashes  map[string]string `json:"file_hashes,omitempty"` // sha256:<hex> per file
	Branch      string            `json:"branch,omitempty"`      // Git branch (when non-default)
}

// EffectiveKind returns "skill" if Kind is empty, otherwise the Kind value.
func (m *SkillMeta) EffectiveKind() string {
	if m.Kind == "" {
		return "skill"
	}
	return m.Kind
}

// Deprecated: WriteMeta writes per-skill sidecar files.
// New code should use MetadataStore.Set() + MetadataStore.Save() instead.
func WriteMeta(skillPath string, meta *SkillMeta) error {
	metaPath := filepath.Join(skillPath, MetaFileName)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// Deprecated: ReadMeta reads per-skill sidecar files.
// New code should use LoadMetadata() + MetadataStore.Get() instead.
func ReadMeta(skillPath string) (*SkillMeta, error) {
	metaPath := filepath.Join(skillPath, MetaFileName)

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No metadata file, not an error
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// Deprecated: HasMeta checks for per-skill sidecar files.
// New code should use MetadataStore.Has() instead.
func HasMeta(skillPath string) bool {
	metaPath := filepath.Join(skillPath, MetaFileName)
	_, err := os.Stat(metaPath)
	return err == nil
}

// ComputeFileHashes walks skillPath and returns a map of relative file paths
// to their "sha256:<hex>" digests. It skips .skillshare-meta.json and .git/.
// Directory symlinks are skipped; file symlinks hash their target content.
func ComputeFileHashes(skillPath string) (map[string]string, error) {
	hashes := make(map[string]string)

	err := filepath.Walk(skillPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == MetaFileName {
			return nil
		}
		if info.Name() == MetadataFileName {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Walk does not follow symlinks, but FileHashFormatted does.
			// Leave stat failures to hashing so they retain the existing error path.
			if target, err := os.Stat(path); err == nil && target.IsDir() {
				return nil
			}
		}

		rel, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return fmt.Errorf("relative path for %s: %w", path, relErr)
		}

		formatted, hashErr := utils.FileHashFormatted(path)
		if hashErr != nil {
			return fmt.Errorf("hashing %s: %w", path, hashErr)
		}
		// Normalize path separators to /
		hashes[filepath.ToSlash(rel)] = formatted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

// Deprecated: NewMetaFromSource creates a SkillMeta from a Source.
// New code should use MetadataStore.SetFromSource() instead.
func NewMetaFromSource(source *Source) *SkillMeta {
	meta := &SkillMeta{
		Source:      source.Raw,
		Type:        source.MetaType(),
		InstalledAt: time.Now(),
		Branch:      source.Branch,
	}

	if source.IsGit() {
		meta.RepoURL = source.CloneURL
	}

	if source.HasSubdir() {
		meta.Subdir = strings.ReplaceAll(source.Subdir, "\\", "/")
	}

	return meta
}

// Deprecated: RefreshMetaHashes recomputes per-skill sidecar hashes.
// New code should use MetadataStore.RefreshHashes() instead.
func RefreshMetaHashes(skillPath string) {
	meta, err := ReadMeta(skillPath)
	if err != nil || meta == nil || meta.FileHashes == nil {
		return
	}
	hashes, err := ComputeFileHashes(skillPath)
	if err != nil {
		return
	}
	meta.FileHashes = hashes
	_ = WriteMeta(skillPath, meta)
}
