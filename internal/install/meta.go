package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const metaFileName = ".skillshare-meta.json"

// SkillMeta contains metadata about an installed skill
type SkillMeta struct {
	Source      string    `json:"source"`             // Original source input
	Type        string    `json:"type"`               // Source type (github, local, etc.)
	InstalledAt time.Time `json:"installed_at"`       // Installation timestamp
	RepoURL     string    `json:"repo_url,omitempty"` // Git repo URL (for git sources)
	Subdir      string    `json:"subdir,omitempty"`   // Subdirectory path (for monorepo)
	Version     string    `json:"version,omitempty"`  // Git commit hash or version
}

// WriteMeta saves metadata to the skill directory
func WriteMeta(skillPath string, meta *SkillMeta) error {
	metaPath := filepath.Join(skillPath, metaFileName)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// ReadMeta loads metadata from the skill directory
func ReadMeta(skillPath string) (*SkillMeta, error) {
	metaPath := filepath.Join(skillPath, metaFileName)

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

// HasMeta checks if a skill directory has metadata
func HasMeta(skillPath string) bool {
	metaPath := filepath.Join(skillPath, metaFileName)
	_, err := os.Stat(metaPath)
	return err == nil
}

// NewMetaFromSource creates a SkillMeta from a Source
func NewMetaFromSource(source *Source) *SkillMeta {
	meta := &SkillMeta{
		Source:      source.Raw,
		Type:        source.MetaType(),
		InstalledAt: time.Now(),
	}

	if source.IsGit() {
		meta.RepoURL = source.CloneURL
	}

	if source.HasSubdir() {
		meta.Subdir = strings.ReplaceAll(source.Subdir, "\\", "/")
	}

	return meta
}
